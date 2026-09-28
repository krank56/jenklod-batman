package ui

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
)

// start connects a model to a fake Gotham, past the splash.
func start(t *testing.T, cfg *config.Config) (Model, *fakeJenkins) {
	t.Helper()
	fj := &fakeJenkins{}
	srv := httptest.NewServer(fj)
	t.Cleanup(srv.Close)
	client, _ := jenkins.New(srv.URL, "bruce", "fake-test-value")
	cfg.URL = srv.URL
	m := New(Options{Config: cfg, Client: client, NoAnim: true})
	m = drive(t, m, cmdMsg{m.Init()})
	m = drive(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m, fj
}

func newCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, _ := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	return cfg
}

func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = drive(t, m, key(string(r)))
	}
	return m
}

func TestWatchedSectionOnRoot(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	m = drive(t, m, key("enter")) // gotham/
	m = drive(t, m, key("w"))     // pin deploy (runs a poll)
	m = drive(t, m, key("h"))     // back to the root
	v := snapshot(t, "20-watched", m)
	for _, want := range []string{"◆ WATCHED", "gotham/deploy", "#42", "RUNNING", "Bruce Wayne", "gotham/", "batmobile-lint", "⟳10s"} {
		if !strings.Contains(v, want) {
			t.Errorf("root lacks %q:\n%s", want, v)
		}
	}
	// The pin comes first; enter jumps into its folder, so back lands in gotham/.
	if r, _ := m.selectedRow(); !r.pin {
		t.Fatalf("cursor row = %+v", r)
	}
	m = drive(t, m, key("enter"))
	if m.scr != scrBuilds || m.builds.job.FullName() != "gotham/deploy" {
		t.Fatalf("screen=%d job=%s", m.scr, m.builds.job.FullName())
	}
	m = drive(t, m, key("h"))
	if m.scr != scrJobs || jenkins.JoinPath(m.jobs.path) != "gotham" {
		t.Fatalf("back went to %q", m.jobs.path)
	}
	if r, _ := m.selectedRow(); r.job.Name != "deploy" {
		t.Errorf("cursor on %q, want deploy", r.job.Name)
	}
}

func TestSearchAcrossFolders(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	m = drive(t, m, key("/"))
	m = typeText(t, m, "bkp")
	v := snapshot(t, "21-search", m)
	if !strings.Contains(v, "gotham/batcave-backup") {
		t.Fatalf("search lacks the nested job:\n%s", v)
	}
	if strings.Contains(v, "batmobile-lint") {
		t.Errorf("non-match listed:\n%s", v)
	}
	m = drive(t, m, key("enter")) // keep results
	m = drive(t, m, key("enter")) // open
	if m.scr != scrBuilds || m.builds.job.FullName() != "gotham/batcave-backup" {
		t.Fatalf("screen=%d job=%s", m.scr, m.builds.job.FullName())
	}
	if jenkins.JoinPath(m.jobs.path) != "gotham" || m.jobs.filter.Value() != "" {
		t.Errorf("jobs path=%v filter=%q", m.jobs.path, m.jobs.filter.Value())
	}
}

func TestSearchRanksCurrentFolderFirst(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	m = drive(t, m, key("enter")) // gotham/
	m = drive(t, m, key("/"))
	m = typeText(t, m, "b")
	rows := m.rows()
	if len(rows) < 2 || rows[0].other || rows[0].job.Name != "batcave-backup" {
		t.Fatalf("rows = %+v", rows)
	}
	var others []string
	for _, r := range rows {
		if r.other {
			others = append(others, r.job.FullName())
		}
	}
	if strings.Join(others, ",") != "batmobile-lint" {
		t.Errorf("elsewhere = %v", others)
	}
}

func TestReorderPinsAndCyclePoll(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	cfg.TogglePin("gotham/deploy")
	cfg.TogglePin("batmobile-lint")
	m = drive(t, m, key("J")) // move gotham/deploy down
	if got := strings.Join(cfg.Pins(), ","); got != "batmobile-lint,gotham/deploy" {
		t.Errorf("pins = %s", got)
	}
	if m.jobs.cursor != 1 {
		t.Errorf("cursor = %d, want it to follow the pin", m.jobs.cursor)
	}
	gen := m.watchGen
	m = drive(t, m, key("p"))
	if cfg.PollSeconds != 20 || m.watchGen == gen {
		t.Errorf("poll = %d gen = %d", cfg.PollSeconds, m.watchGen)
	}
	saved, _ := config.Load(cfg.Path())
	if saved.PollSeconds != 20 || strings.Join(saved.Pins(), ",") != "batmobile-lint,gotham/deploy" {
		t.Errorf("saved poll=%d pins=%v", saved.PollSeconds, saved.Pins())
	}
	// A tick scheduled with the old interval is ignored.
	if _, cmd := m.Update(watchTickMsg{gen: gen}); cmd != nil {
		t.Error("stale tick still polled")
	}
}

func TestPinRollsBackWhenSaveFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ro")
	os.Mkdir(dir, 0o500) // saving can't create the file
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	cfg, _ := config.Load(filepath.Join(dir, "config.toml"))
	m, _ := start(t, cfg)
	m = drive(t, m, key("enter")) // gotham/
	m = drive(t, m, key("w"))
	if cfg.IsPinned("gotham/deploy") {
		t.Error("pin kept although it was not saved")
	}
	if !m.statusErr || !strings.Contains(m.status, "saving pins") {
		t.Errorf("status = %q", m.status)
	}
}

func TestBrokenPinIsShownNotNotified(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	cfg.TogglePin("gone")
	if cmd := m.handleWatch(watchResultMsg{name: "gone", err: fmt.Errorf("/job/gone: %w", jenkins.ErrNotFound)}); cmd != nil {
		t.Error("a broken pin must not notify")
	}
	v := snapshot(t, "22-broken-pin", m)
	if !strings.Contains(v, "✗") || !strings.Contains(v, "deleted or renamed?") {
		t.Errorf("broken pin not flagged:\n%s", v)
	}
	// It recovers on its own once polling works again.
	m.handleWatch(watchResultMsg{name: "gone", build: &jenkins.Build{Number: 3, Result: "SUCCESS"}})
	if m.watch["gone"].err != nil {
		t.Error("error stuck after a good poll")
	}
}

func TestFirstBuildBetweenPollsNotifies(t *testing.T) {
	cfg := newCfg(t)
	cfg.TogglePin("new-job")
	m := New(Options{Config: cfg, NoAnim: true})
	if cmd := m.handleWatch(watchResultMsg{name: "new-job"}); cmd != nil {
		t.Error("never built must not notify")
	}
	// Build #1 started and finished before the next poll.
	if cmd := m.handleWatch(watchResultMsg{name: "new-job", build: &jenkins.Build{Number: 1, Result: "SUCCESS"}}); cmd == nil {
		t.Error("the first build must notify")
	}
}

func TestMacroEditorSavesSteps(t *testing.T) {
	cfg := newCfg(t)
	m, _ := start(t, cfg)
	m = drive(t, m, key("m"))
	if m.scr != scrMacros {
		t.Fatalf("screen = %d", m.scr)
	}
	m = drive(t, m, key("n"))
	m = typeText(t, m, "ship")
	m = drive(t, m, key("enter"))

	// Step 1: build gotham/deploy with ENV=staging.
	m = drive(t, m, key("a"))
	m = drive(t, m, key("b"))
	m = typeText(t, m, "deploy")
	m = drive(t, m, key("enter"))
	if m.scr != scrParams || !m.params.forMacro {
		t.Fatalf("expected the params form, screen=%d", m.scr)
	}
	m = drive(t, m, key("right")) // ENV dev -> staging
	snapshot(t, "23-macro-params", m)
	m = drive(t, m, key("enter"))

	// Step 2: wait for it, 45 minutes.
	m = drive(t, m, key("a"))
	m = drive(t, m, key("w"))
	m = typeText(t, m, "gdep")
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("t"))
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	m = typeText(t, m, "45")
	m = drive(t, m, key("enter"))
	v := snapshot(t, "24-macro-editor", m)
	if !strings.Contains(v, "1. build gotham/deploy (DRY_RUN=true, ENV=staging)") || !strings.Contains(v, "2. wait for gotham/deploy to succeed (45m)") {
		t.Fatalf("editor:\n%s", v)
	}

	m = drive(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.scr != scrMacros {
		t.Fatalf("after save screen=%d status=%q", m.scr, m.status)
	}
	saved, _ := config.Load(cfg.Path())
	mac, ok := saved.Macro("ship")
	if !ok || len(mac.Steps) != 2 || mac.Steps[0].Params["ENV"] != "staging" || mac.Steps[1].TimeoutMinutes != 45 {
		t.Fatalf("saved = %+v", saved.Macros())
	}
	v = snapshot(t, "25-macros", m)
	if !strings.Contains(v, "ship") || !strings.Contains(v, "2 step(s)") {
		t.Errorf("picker:\n%s", v)
	}

	// Running asks first.
	m = drive(t, m, key("enter"))
	if m.confirm == nil || !strings.Contains(m.confirm.prompt, "Run macro ship?") {
		t.Fatalf("confirm = %+v", m.confirm)
	}
}

func TestMacroRunsInTheBackground(t *testing.T) {
	cfg := newCfg(t)
	m, fj := start(t, cfg)
	cfg.SetMacros([]config.Macro{{Name: "ship", Steps: []config.Step{
		{Kind: config.StepBuild, Job: "gotham/deploy", WithParams: true, Params: map[string]string{"ENV": "prod", "DRY_RUN": "false"}},
	}}})
	m = drive(t, m, key("m"))
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("y"))
	if m.run == nil || !m.run.done || m.run.err != nil {
		t.Fatalf("run = %+v", m.run)
	}
	if !strings.Contains(strings.Join(m.run.lines, "\n"), "started gotham/deploy #43") {
		t.Errorf("lines = %q", m.run.lines)
	}
	fj.mu.Lock()
	got := fj.triggered
	fj.mu.Unlock()
	if got["ENV"] != "prod" || got["DRY_RUN"] != "false" {
		t.Errorf("triggered with %v", got)
	}
	if v := snapshot(t, "26-macro-done", m); !strings.Contains(v, "✔ ship done") {
		t.Errorf("picker:\n%s", v)
	}
}
