package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyPinsMoveToTheirController(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path, []byte("url = \"https://ci.gotham/\"\nuser = \"bruce\"\npinned = [\"gotham/deploy\", \"lint\"]\n"), 0o600)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Pins(); !reflect.DeepEqual(got, []string{"gotham/deploy", "lint"}) {
		t.Fatalf("pins = %v", got)
	}
	if c.PollSeconds != DefaultPollSeconds || DefaultPollSeconds != 10 {
		t.Errorf("poll = %d", c.PollSeconds)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	again, _ := Load(path)
	if again.Pinned != nil || !again.IsPinned("gotham/deploy") {
		t.Errorf("after save: legacy %v, pins %v", again.Pinned, again.Pins())
	}

	// Another controller starts with nothing.
	again.URL = "https://ci.metropolis"
	if again.IsPinned("gotham/deploy") || len(again.Pins()) != 0 {
		t.Errorf("pins leaked to another controller: %v", again.Pins())
	}
}

func TestMovePinAndPollPresets(t *testing.T) {
	c := &Config{URL: "https://ci", PollSeconds: 10}
	c.TogglePin("a")
	c.TogglePin("b")
	c.TogglePin("c")
	if !c.MovePin("c", -1) || c.MovePin("a", -1) || c.MovePin("x", 1) {
		t.Fatal("MovePin moved the wrong things")
	}
	if got := c.Pins(); !reflect.DeepEqual(got, []string{"a", "c", "b"}) {
		t.Errorf("order = %v", got)
	}
	var seen []int
	for range PollPresets {
		c.PollSeconds = c.NextPoll()
		seen = append(seen, c.PollSeconds)
	}
	if !reflect.DeepEqual(seen, []int{20, 30, 60, 5, 10}) {
		t.Errorf("cycle = %v", seen)
	}
}

func TestMacrosRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	c, _ := Load(path)
	c.URL = "https://ci"
	want := []Macro{{Name: "ship", Steps: []Step{
		{Kind: StepAbort, Job: "gotham/deploy"},
		{Kind: StepBuild, Job: "gotham/deploy", WithParams: true, Params: map[string]string{"ENV": "prod"}},
		{Kind: StepBuild, Job: "only-passwords", WithParams: true},
		{Kind: StepWait, Job: "gotham/deploy", TimeoutMinutes: 45},
		{Kind: StepInput, Job: "gotham/deploy", Abort: true},
	}}}
	c.SetMacros(want)
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Macros(), want) {
		t.Errorf("macros:\n got %+v\nwant %+v", got.Macros(), want)
	}
	if s := want[0].Steps[1].String(); s != "build gotham/deploy (ENV=prod)" {
		t.Errorf("describe = %q", s)
	}
	if s := want[0].Steps[4].String(); s != "abort input on gotham/deploy (30m)" {
		t.Errorf("describe = %q", s)
	}
}
