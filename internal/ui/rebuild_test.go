package ui

import (
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
)

func rebuildModel(t *testing.T) (Model, *fakeJenkins) {
	t.Helper()
	fj := &fakeJenkins{}
	srv := httptest.NewServer(fj)
	t.Cleanup(srv.Close)
	client, _ := jenkins.New(srv.URL, "bruce", "fake-test-value")
	cfg, _ := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	m := New(Options{Config: cfg, Client: client, NoAnim: true})
	m = drive(t, m, cmdMsg{m.Init()})
	m = drive(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(t, m, key("enter")) // gotham/
	return m, fj
}

func TestRebuildSelectedBuild(t *testing.T) {
	m, fj := rebuildModel(t)
	m = drive(t, m, key("enter")) // deploy builds
	m = drive(t, m, key("j"))     // #41
	m = drive(t, m, key("R"))
	if m.scr != scrParams || m.params.rebuildOf != 41 {
		t.Fatalf("R should open the form from #41, screen=%d rebuildOf=%d", m.scr, m.params.rebuildOf)
	}
	v := snapshot(t, "20-rebuild-form", m)
	if !strings.Contains(v, "Rebuild gotham/deploy") || !strings.Contains(v, "staging") || !strings.Contains(v, "[ ] false") {
		t.Fatalf("form should hold #41's values:\n%s", v)
	}
	if !strings.Contains(m.status, "dropped OLD") {
		t.Errorf("status = %q", m.status)
	}
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("y"))
	fj.mu.Lock()
	got := fj.triggered
	fj.mu.Unlock()
	if want := map[string]string{"ENV": "staging", "DRY_RUN": "false"}; !reflect.DeepEqual(got, want) {
		t.Errorf("triggered with %v, want %v", got, want)
	}
	if m.scr != scrBuilds {
		t.Errorf("after rebuild screen=%d", m.scr)
	}
}

func TestRebuildFromJobsList(t *testing.T) {
	m, _ := rebuildModel(t)
	m = drive(t, m, key("R")) // deploy: its newest build, #42
	if m.scr != scrParams || m.params.rebuildOf != 42 {
		t.Fatalf("R should open the form from #42, screen=%d rebuildOf=%d", m.scr, m.params.rebuildOf)
	}
	if got := m.params.values(); got["ENV"] != "dev" || got["DRY_RUN"] != "true" {
		t.Errorf("invalid and new values should fall back to defaults: %v", got)
	}
	if !strings.Contains(m.status, "reset ENV") || !strings.Contains(m.status, "new DRY_RUN") {
		t.Errorf("status = %q", m.status)
	}
	m = drive(t, m, key("esc"))
	if m.scr != scrBuilds {
		t.Errorf("esc should return to the builds, screen=%d", m.scr)
	}
}

func TestRebuildFromLog(t *testing.T) {
	m, _ := rebuildModel(t)
	m = drive(t, m, key("L")) // deploy #42's log, without the job detail
	if m.scr != scrLog {
		t.Fatalf("L should open the log, screen=%d", m.scr)
	}
	m = drive(t, m, key("R"))
	if m.scr != scrParams || m.params.rebuildOf != 42 || m.params.returnTo != scrLog {
		t.Fatalf("screen=%d rebuildOf=%d returnTo=%d", m.scr, m.params.rebuildOf, m.params.returnTo)
	}
}

func TestRebuildRefusesPasswordAndFile(t *testing.T) {
	for _, typ := range []jenkins.ParamType{jenkins.ParamPassword, jenkins.ParamFile} {
		m := New(Options{Config: &config.Config{}, NoAnim: true})
		m.scr = scrBuilds
		m.builds.job = jenkins.Job{Name: "deploy", Path: []string{"deploy"}}
		m.builds.detail = &jenkins.Detail{Buildable: true, Params: []jenkins.Param{{Name: "ENV"}, {Name: "SECRET", Type: typ}},
			Builds: []jenkins.Build{{Number: 3}}}
		next, cmd := m.startRebuild(3, scrBuilds)
		mm := next.(Model)
		if cmd != nil || mm.scr != scrBuilds || !strings.Contains(mm.status, "SECRET") {
			t.Errorf("type %d: screen=%d status=%q cmd=%v", typ, mm.scr, mm.status, cmd != nil)
		}
	}
}

func TestRebuildWithoutParamsOrBuilds(t *testing.T) {
	m := New(Options{Config: &config.Config{}, NoAnim: true})
	m.scr = scrBuilds
	m.builds.job = jenkins.Job{Name: "lint", Path: []string{"lint"}}
	m.builds.detail = &jenkins.Detail{Buildable: true, Builds: []jenkins.Build{{Number: 3}}}
	next, _ := m.startRebuild(3, scrBuilds)
	if mm := next.(Model); mm.confirm == nil || !strings.Contains(mm.confirm.prompt, "Start a build of lint") {
		t.Errorf("no parameters should ask like b: %+v", mm.confirm)
	}

	m.builds.detail = &jenkins.Detail{Buildable: true, Params: []jenkins.Param{{Name: "ENV", Default: "dev"}}}
	next, _ = m.startRebuild(0, scrBuilds)
	mm := next.(Model)
	if mm.scr != scrParams || mm.params.values()["ENV"] != "dev" || !strings.Contains(mm.status, "no previous build") {
		t.Errorf("no build should open the defaults: screen=%d status=%q", mm.scr, mm.status)
	}
}

func TestMergeParams(t *testing.T) {
	defs := []jenkins.Param{
		{Name: "ENV", Type: jenkins.ParamChoice, Choices: []string{"dev", "prod"}, Default: "dev"},
		{Name: "DRY", Type: jenkins.ParamBool, Default: "true"},
		{Name: "TAG", Default: "latest"},
	}
	out, notes := mergeParams(defs, map[string]string{"ENV": "prod", "DRY": "maybe", "TAG": "", "GONE": "1"})
	if out[0].Default != "prod" || out[1].Default != "true" || out[2].Default != "" {
		t.Errorf("merged = %+v", out)
	}
	if want := []string{"dropped GONE", "reset DRY"}; !reflect.DeepEqual(notes, want) {
		t.Errorf("notes = %q, want %q", notes, want)
	}
	if defs[0].Default != "dev" {
		t.Error("mergeParams must not change the job's definitions")
	}
}
