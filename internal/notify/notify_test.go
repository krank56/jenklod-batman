package notify

import (
	"errors"
	"os/exec"
	"slices"
	"testing"
)

func found(name string) (string, error) { return "/usr/bin/" + name, nil }
func missing(string) (string, error)    { return "", exec.ErrNotFound }

func TestLinuxUsesNotifySend(t *testing.T) {
	cmd, err := command("linux", found, "-rf deploy #3", "Deploy to prod?")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/usr/bin/notify-send", "--app-name=jenklod-batman", "--icon=dialog-information", "--", "-rf deploy #3", "Deploy to prod?"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q\nwant  %q", cmd.Args, want)
	}
}

func TestLinuxWithoutNotifySend(t *testing.T) {
	if _, err := command("linux", missing, "t", "b"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestDarwinPassesTextAsArguments(t *testing.T) {
	cmd, err := command("darwin", missing, `a" & do shell script "x`, "b")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(cmd.Args); cmd.Args[n-2] != `a" & do shell script "x` || cmd.Args[n-1] != "b" {
		t.Errorf("args = %q", cmd.Args)
	}
}

func TestOtherSystems(t *testing.T) {
	if _, err := command("windows", found, "t", "b"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestPlayer(t *testing.T) {
	if cmd, _ := player("darwin", missing, "a.mp3"); cmd.Args[0] != "afplay" {
		t.Errorf("darwin: %v", cmd.Args)
	}
	if cmd, _ := player("linux", found, "a.mp3"); !slices.Equal(cmd.Args, []string{"/usr/bin/mpg123", "-q", "--", "a.mp3"}) {
		t.Errorf("linux: %v", cmd.Args)
	}
	if _, err := player("linux", missing, "a.mp3"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no player: %v", err)
	}
}
