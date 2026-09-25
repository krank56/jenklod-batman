// Package config loads and saves ~/.config/jenklod-batman/config.toml.
// The API token never lives here: see package secret.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
)

// DefaultPollSeconds is how often pinned jobs are checked.
const DefaultPollSeconds = 20

// Config is the persisted configuration.
type Config struct {
	URL         string   `toml:"url"`
	User        string   `toml:"user"`
	PollSeconds int      `toml:"poll_seconds"`
	Pinned      []string `toml:"pinned"`

	path string
}

// DefaultPath honours $XDG_CONFIG_HOME, falling back to ~/.config.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "jenklod-batman", "config.toml"), nil
}

// Load reads the config at path. A missing file yields an empty config and
// no error; Complete reports whether it is usable.
func Load(path string) (*Config, error) {
	c := &Config{path: path}
	if _, err := toml.DecodeFile(path, c); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if c.PollSeconds <= 0 {
		c.PollSeconds = DefaultPollSeconds
	}
	return c, nil
}

// Complete reports whether URL and user are set.
func (c *Config) Complete() bool { return c.URL != "" && c.User != "" }

// Path is where the config is saved.
func (c *Config) Path() string { return c.path }

// Save writes the config with owner-only permissions.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

// IsPinned reports whether a job (by full name) is watched.
func (c *Config) IsPinned(fullName string) bool { return slices.Contains(c.Pinned, fullName) }

// TogglePin pins or unpins a job and returns the new state.
func (c *Config) TogglePin(fullName string) bool {
	if i := slices.Index(c.Pinned, fullName); i >= 0 {
		c.Pinned = slices.Delete(c.Pinned, i, i+1)
		return false
	}
	c.Pinned = append(c.Pinned, fullName)
	return true
}
