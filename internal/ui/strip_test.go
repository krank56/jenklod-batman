package ui

import (
	"strings"
	"testing"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
)

func TestStripOnEveryScreenButRoot(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	m = drive(t, m, key("enter")) // gotham/
	m = drive(t, m, key("w"))     // watch deploy
	v := snapshot(t, "40-strip-folder", m)
	if !strings.Contains(v, "gotham/deploy") || !strings.Contains(v, "#42") {
		t.Fatalf("the folder lacks the strip:\n%s", v)
	}
	m = drive(t, m, key("h"))
	if m.showStrip() {
		t.Error("the root has its own Watched section: no strip")
	}
	m = drive(t, m, key("enter")) // the pin: its builds
	m = drive(t, m, key("enter")) // a log
	if v := snapshot(t, "41-strip-log", m); !strings.Contains(v, "gotham/deploy") {
		t.Fatalf("the log lacks the strip:\n%s", v)
	}
	if m.log.vp.Height != m.h-5-m.stripLines() {
		t.Errorf("log height %d, strip %d", m.log.vp.Height, m.stripLines())
	}
	m = drive(t, m, key("W"))
	if m.showStrip() || cfg.StripOn() {
		t.Error("W hides the strip")
	}
	again, _ := config.Load(cfg.Path())
	if again.StripOn() {
		t.Error("W is saved")
	}
	if m.log.vp.Height != m.h-5 {
		t.Errorf("the log takes the room back: %d", m.log.vp.Height)
	}
}

func TestStripOrderAndOverflow(t *testing.T) {
	cfg := newCfg(t)
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		cfg.TogglePin(p)
	}
	m := New(Options{Config: cfg, NoAnim: true})
	m.w, m.h = 120, 30
	m.watch["c"] = &watchState{known: true, last: &jenkins.Build{Number: 1, Building: true}}
	m.watch["e"] = &watchState{known: true, last: &jenkins.Build{Number: 2, Result: "FAILURE"}}
	m.watch["g"] = &watchState{known: true, last: &jenkins.Build{Number: 3, Building: true,
		Inputs: []jenkins.InputRequest{{ID: "x", Message: "Ship?"}}}}
	var names []string
	for _, l := range strings.Split(m.stripView(), "\n") {
		if f := strings.Fields(l); len(f) > 1 && len(f[1]) == 1 {
			names = append(names, f[1])
		}
	}
	if got := strings.Join(names, ""); got != "gecab" {
		t.Errorf("order = %q, want gecab", got)
	}
	if !strings.Contains(m.stripView(), "+2 more") {
		t.Errorf("no overflow note:\n%s", m.stripView())
	}
}

func TestBuildingWatchesTheJob(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("b"))
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("y"))
	if !cfg.IsPinned("gotham/deploy") {
		t.Error("a build started from here watches its job")
	}
}

func TestMacroWatchesItsJobs(t *testing.T) {
	mac := config.Macro{Steps: []config.Step{
		{Kind: config.StepBuild, Job: "a"}, {Kind: config.StepWait, Job: "a"},
		{Kind: config.StepBuild, Job: "b"}, {Kind: config.StepBuild, Job: "a"}}}
	if got := strings.Join(mac.BuiltJobs(), ","); got != "a,b" {
		t.Errorf("BuiltJobs = %q", got)
	}
}
