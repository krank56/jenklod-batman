// Package config loads and saves ~/.config/jenklod-batman/config.toml.
// The API token never lives here: see package secret.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// DefaultPollSeconds is how often pinned jobs are checked.
const DefaultPollSeconds = 10

// PollPresets are the intervals the poll key cycles through.
var PollPresets = []int{5, 10, 20, 30, 60}

// Config is the persisted configuration.
type Config struct {
	URL         string `toml:"url"`
	User        string `toml:"user"`
	PollSeconds int    `toml:"poll_seconds"`
	// Pinned is the pre-0.2 list of pins, not tied to a controller. Load
	// moves it into Servers; it is kept only to read old files.
	Pinned []string `toml:"pinned,omitempty"`
	// Servers holds what belongs to one controller, keyed by its URL.
	Servers map[string]*Server `toml:"servers,omitempty"`

	path string
}

// Server is the per-controller part of the config.
type Server struct {
	Pinned []string `toml:"pinned,omitempty"`
	Macros []Macro  `toml:"macros,omitempty"`
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
	// Old files pinned jobs globally; they belong to the configured URL.
	if len(c.Pinned) > 0 && c.URL != "" {
		s := c.server()
		for _, p := range c.Pinned {
			if !slices.Contains(s.Pinned, p) {
				s.Pinned = append(s.Pinned, p)
			}
		}
		c.Pinned = nil
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

func serverKey(url string) string { return strings.TrimRight(strings.TrimSpace(url), "/") }

// server is the entry for the current URL, created on demand.
func (c *Config) server() *Server {
	if c.Servers == nil {
		c.Servers = map[string]*Server{}
	}
	k := serverKey(c.URL)
	s := c.Servers[k]
	if s == nil {
		s = &Server{}
		c.Servers[k] = s
	}
	return s
}

// Pins lists the watched jobs of the current controller, in display order.
func (c *Config) Pins() []string {
	if s := c.Servers[serverKey(c.URL)]; s != nil {
		return s.Pinned
	}
	return nil
}

// IsPinned reports whether a job (by full name) is watched.
func (c *Config) IsPinned(fullName string) bool { return slices.Contains(c.Pins(), fullName) }

// TogglePin pins or unpins a job and returns the new state.
func (c *Config) TogglePin(fullName string) bool {
	s := c.server()
	if i := slices.Index(s.Pinned, fullName); i >= 0 {
		s.Pinned = slices.Delete(s.Pinned, i, i+1)
		return false
	}
	s.Pinned = append(s.Pinned, fullName)
	return true
}

// MovePin shifts a pin by delta places and reports whether it moved.
func (c *Config) MovePin(fullName string, delta int) bool {
	s := c.server()
	i := slices.Index(s.Pinned, fullName)
	j := i + delta
	if i < 0 || j < 0 || j >= len(s.Pinned) {
		return false
	}
	s.Pinned[i], s.Pinned[j] = s.Pinned[j], s.Pinned[i]
	return true
}

// NextPoll is the preset interval after the current one, wrapping around.
func (c *Config) NextPoll() int {
	for _, p := range PollPresets {
		if p > c.PollSeconds {
			return p
		}
	}
	return PollPresets[0]
}

// Macros lists the macros of the current controller.
func (c *Config) Macros() []Macro {
	if s := c.Servers[serverKey(c.URL)]; s != nil {
		return s.Macros
	}
	return nil
}

// Macro returns the macro called name.
func (c *Config) Macro(name string) (Macro, bool) {
	for _, m := range c.Macros() {
		if m.Name == name {
			return m, true
		}
	}
	return Macro{}, false
}

// SetMacros replaces the macros of the current controller.
func (c *Config) SetMacros(ms []Macro) { c.server().Macros = ms }
