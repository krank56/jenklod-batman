// Package notify shows macOS notifications and opens URLs.
package notify

import (
	"os/exec"
	"runtime"
)

// Send posts a desktop notification. Text is passed as osascript arguments,
// never interpolated into the script, so job names cannot inject AppleScript.
func Send(title, body string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return exec.Command("osascript",
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		title, body,
	).Run()
}

// Open opens a URL in the default browser.
func Open(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, url).Start()
}
