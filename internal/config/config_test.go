package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	want := []Macro{{Name: "ship", Arg: "ENV", Steps: []Step{
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

func TestReposRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	c, _ := Load(path)
	c.URL = "https://ci.gotham/"
	if got := c.Repo("gotham/deploy"); got != "" {
		t.Errorf("no repo yet: %q", got)
	}
	c.SetRepo("gotham/deploy", "wayne/gotham")
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `"gotham/deploy" = "wayne/gotham"`) {
		t.Errorf("file:\n%s", raw)
	}
	again, _ := Load(path)
	if got := again.Repo("gotham/deploy"); got != "wayne/gotham" {
		t.Errorf("after load: %q", got)
	}
	again.URL = "https://ci.metropolis"
	if got := again.Repo("gotham/deploy"); got != "" {
		t.Errorf("repo leaked to another controller: %q", got)
	}
}

func TestCheckUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	c, _ := Load(path)
	if !c.UpdatesOn() {
		t.Error("checking for updates is on by default")
	}
	c.Save()
	if raw, _ := os.ReadFile(path); strings.Contains(string(raw), "check_updates") {
		t.Errorf("the default is not written out:\n%s", raw)
	}
	os.WriteFile(path, []byte("check_updates = false\n"), 0o600)
	if c, _ := Load(path); c.UpdatesOn() {
		t.Error("check_updates = false turns it off")
	}
}

func TestMusicRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	c, _ := Load(path)
	if !c.MusicOn() || c.Volume() != DefaultVolume {
		t.Fatalf("fresh config: music %v, volume %d; want on at %d", c.MusicOn(), c.Volume(), DefaultVolume)
	}
	v := 70
	c.SetMusic(false)
	c.MusicVolume = &v
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "music = false") {
		t.Errorf("off not written:\n%s", b)
	}
	again, _ := Load(path)
	if again.MusicOn() || again.Volume() != 70 {
		t.Errorf("after save: music %v, volume %d", again.MusicOn(), again.Volume())
	}

	// Hand-edited out of range: clamped.
	os.WriteFile(path, []byte("music_volume = 250\n"), 0o600)
	again, _ = Load(path)
	if again.Volume() != 100 {
		t.Errorf("volume 250 read as %d, want 100", again.Volume())
	}
}
