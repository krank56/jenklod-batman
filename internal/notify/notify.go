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

// Play starts an audio file in the background with the system player and
// returns a func that stops it. A missing file or player plays nothing.
func Play(path string) (stop func()) {
	cmd, err := player(runtime.GOOS, exec.LookPath, path)
	if path == "" || err != nil || cmd.Start() != nil {
		return func() {}
	}
	go cmd.Wait()
	return func() { cmd.Process.Kill() }
}

func player(goos string, lookPath func(string) (string, error), path string) (*exec.Cmd, error) {
	if goos == "darwin" {
		return exec.Command("afplay", path), nil
	}
	if p, err := lookPath("mpg123"); err == nil {
		return exec.Command(p, "-q", "--", path), nil
	}
	if p, err := lookPath("ffplay"); err == nil {
		return exec.Command(p, "-nodisp", "-autoexit", "-loglevel", "quiet", path), nil
	}
	return nil, ErrUnavailable
}
