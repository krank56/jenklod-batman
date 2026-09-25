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
