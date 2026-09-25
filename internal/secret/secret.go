// Package secret resolves the Jenkins API token: $JENKINS_TOKEN when set,
// otherwise the system keyring (macOS Keychain, or the Secret Service on
// Linux: gnome-keyring, KWallet...). The token is never written to disk by us.
package secret

import (
	"errors"
	"os"
	"runtime"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	service = "jenklod-batman"
	// EnvVar overrides the keyring for one-off runs.
	EnvVar = "JENKINS_TOKEN"
)

// Source says where a token came from.
type Source string

const (
	FromEnv      Source = "$" + EnvVar
	FromKeychain Source = "keyring"
)

// StoreName is how the keyring is called on this system, for messages.
func StoreName() string {
	if runtime.GOOS == "darwin" {
		return "macOS Keychain"
	}
	return "system keyring"
}

// ErrMissing means no token is available.
var ErrMissing = errors.New("no API token found")

func account(url, user string) string {
	return user + "@" + strings.TrimRight(url, "/")
}

// Get returns the token for user on the controller at url.
func Get(url, user string) (string, Source, error) {
	if t := os.Getenv(EnvVar); t != "" {
		return t, FromEnv, nil
	}
	t, err := keyring.Get(service, account(url, user))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", "", ErrMissing
	}
	if err != nil {
		return "", "", err
	}
	return t, FromKeychain, nil
}

// Set stores the token in the keyring.
func Set(url, user, token string) error {
	return keyring.Set(service, account(url, user), token)
}

// Delete removes the stored token, if any.
func Delete(url, user string) error {
	err := keyring.Delete(service, account(url, user))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
