// Package notify shows desktop notifications (macOS and Linux) and opens
// URLs.
package notify

import (
	"errors"
	"os/exec"
	"runtime"
)

// ErrUnavailable means this system has no way to show notifications.
var ErrUnavailable = errors.New("desktop notifications unavailable")

// Send posts a desktop notification.
func Send(title, body string) error {
	cmd, err := command(runtime.GOOS, exec.LookPath, title, body)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// command picks the notifier for goos. Title and body are always passed as
// separate arguments, never interpolated into a script, so job names and
// input messages cannot inject anything.
func command(goos string, lookPath func(string) (string, error), title, body string) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		return exec.Command("osascript",
			"-e", "on run argv",
			"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run",
			title, body,
		), nil
	case "linux", "freebsd", "openbsd", "netbsd":
		// libnotify's notify-send talks to whatever notification daemon
		// the desktop runs (GNOME, KDE, dunst, mako...).
		p, err := lookPath("notify-send")
		if err != nil {
			return nil, errors.Join(ErrUnavailable, errors.New("install libnotify (notify-send)"))
		}
		// "--" stops a title starting with "-" being read as an option.
		return exec.Command(p, "--app-name=jenklod-batman", "--icon=dialog-information", "--", title, body), nil
	}
	return nil, ErrUnavailable
}

// Open opens a URL in the default browser.
func Open(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, url).Start()
}
