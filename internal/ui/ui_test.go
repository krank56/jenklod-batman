package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
)

// fakeJenkins serves a tiny Gotham: a folder with a parameterised job.
type fakeJenkins struct {
	mu        sync.Mutex
	triggered map[string]string
	withInput bool
	answered  []string
}

func (f *fakeJenkins) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/json":
		w.Write([]byte(`{"jobs":[
			{"_class":"com.cloudbees.hudson.plugins.folder.Folder","name":"gotham"},
			{"_class":"hudson.model.FreeStyleProject","name":"batmobile-lint","color":"blue"}]}`))
	case "/job/gotham/api/json":
		w.Write([]byte(`{"jobs":[
			{"_class":"org.jenkinsci.plugins.workflow.job.WorkflowJob","name":"deploy","color":"red_anime"},
			{"_class":"org.jenkinsci.plugins.workflow.job.WorkflowJob","name":"batcave-backup","color":"yellow"}]}`))
	case "/job/gotham/job/deploy/api/json":
		now := time.Now().UnixMilli()
		w.Write([]byte(`{"name":"deploy","buildable":true,"builds":[
			{"number":42,"building":true,"timestamp":` + itoa(now-60000) + `,"estimatedDuration":120000,
			 "actions":[{"causes":[{"shortDescription":"Started by user Bruce Wayne"}]}]},
			{"number":41,"result":"FAILURE","duration":93000,"timestamp":` + itoa(now-3600000) + `,
			 "actions":[{"causes":[{"shortDescription":"Started by the Joker"}]}]}],
			"property":[{"parameterDefinitions":[
			 {"name":"ENV","type":"ChoiceParameterDefinition","choices":["dev","staging","prod"],"defaultParameterValue":{"value":"dev"}},
			 {"name":"DRY_RUN","type":"BooleanParameterDefinition","defaultParameterValue":{"value":true}}]}]}`))
	case "/job/gotham/job/deploy/42/logText/progressiveText":
		// Two chunks, like a real running build.
		if r.URL.Query().Get("start") == "0" {
			w.Header().Set("X-Text-Size", "100")
			w.Header().Set("X-More-Data", "true")
			w.Write([]byte("Started by user Bruce Wayne\n\x1b[8mha:AAAA\x1b[0m[Pipeline] stage\nDeploying to Gotham…\r\n"))
			return
		}
		w.Header().Set("X-Text-Size", "200")
		w.Write([]byte("Finished: SUCCESS\n"))
	case "/job/gotham/job/deploy/42/wfapi/pendingInputActions":
		if !f.withInput {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`[{"id":"Prod","proceedText":"Deploy","message":"Deploy to prod?",
			"inputs":[{"type":"ChoiceParameterDefinition","name":"REGION","definition":{"defaultVal":"eu","choices":["eu","us"]}}]}]`))
	case "/job/gotham/job/deploy/42/input/Prod/submit", "/job/gotham/job/deploy/42/input/Prod/abort":
		r.ParseForm()
		f.mu.Lock()
		f.answered = append(f.answered, r.URL.Path+" "+r.PostForm.Get("json"))
		f.mu.Unlock()
	case "/crumbIssuer/api/json":
		w.WriteHeader(http.StatusNotFound)
	case "/job/gotham/job/deploy/buildWithParameters":
		r.ParseForm()
		f.mu.Lock()
		f.triggered = map[string]string{"ENV": r.PostForm.Get("ENV"), "DRY_RUN": r.PostForm.Get("DRY_RUN")}
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	default:
		http.NotFound(w, r)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// drive feeds a message and synchronously runs the resulting commands,
// dropping timers (tea.Tick) so the test never waits on animation.
func drive(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	var cmd tea.Cmd
	if c, ok := msg.(cmdMsg); ok {
		cmd = c.c
	} else {
		next, c := m.Update(msg)
		m, cmd = next.(Model), c
	}
	for _, c := range flatten(cmd) {
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		select {
		case out := <-done:
			if out == nil {
				continue
			}
			if _, isTick := out.(tickMsg); isTick {
				continue
			}
			if inner := asCmds(out); inner != nil {
				for _, ic := range inner {
					if ic != nil {
						m = drive(t, m, cmdMsg{ic})
					}
				}
				continue
			}
			m = drive(t, m, out)
		case <-time.After(300 * time.Millisecond):
			// A timer; ignore.
		}
	}
	return m
}

// cmdMsg lets drive run a command produced by a batch/sequence.
type cmdMsg struct{ c tea.Cmd }

func flatten(cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return nil
	}
	return []tea.Cmd{cmd}
}

// asCmds unpacks tea.BatchMsg and the unexported sequence message.
func asCmds(msg tea.Msg) []tea.Cmd {
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Type().Elem() != reflect.TypeFor[tea.Cmd]() {
		return nil
	}
	out := make([]tea.Cmd, v.Len())
	for i := range out {
		out[i] = v.Index(i).Interface().(tea.Cmd)
	}
	return out
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func snapshot(t *testing.T, name string, m Model) string {
	t.Helper()
	v := m.View()
	if dir := os.Getenv("JB_SNAPSHOTS"); dir != "" {
		os.WriteFile(filepath.Join(dir, name+".txt"), []byte(v), 0o644)
	}
	return v
}

func TestFlow(t *testing.T) {
	fj := &fakeJenkins{}
	srv := httptest.NewServer(fj)
	defer srv.Close()
	client, _ := jenkins.New(srv.URL, "bruce", "fake-test-value")
	cfg, _ := config.Load(filepath.Join(t.TempDir(), "config.toml"))

	m := New(Options{Config: cfg, Client: client})
	// Run the Init commands but not the timers.
	m = drive(t, m, cmdMsg{m.Init()})
	m = drive(t, m, tea.WindowSizeMsg{Width: 100, Height: 36})
	for range splashDropEnd + 6 {
		m = drive(t, m, tickMsg{})
	}
	if v := snapshot(t, "01-splash", m); !strings.Contains(v, "B A T M A N") {
		t.Errorf("splash lacks title:\n%s", v)
	}

	m = drive(t, m, key("x")) // any key skips
	if v := snapshot(t, "02-root", m); !strings.Contains(v, "gotham/") || !strings.Contains(v, "batmobile-lint") {
		t.Fatalf("root listing:\n%s", v)
	}

	m = drive(t, m, key("enter"))
	if v := snapshot(t, "03-folder", m); !strings.Contains(v, "deploy") {
		t.Fatalf("folder listing:\n%s", v)
	}

	m = drive(t, m, key("w"))
	if !cfg.IsPinned("gotham/deploy") {
		t.Error("w did not pin the job")
	}

	m = drive(t, m, key("enter"))
	if v := snapshot(t, "04-builds", m); !strings.Contains(v, "#42") || !strings.Contains(v, "Joker") {
		t.Fatalf("builds:\n%s", v)
	}

	m = drive(t, m, key("enter"))
	// The second chunk arrives after the model has been copied around,
	// which used to panic on a by-value strings.Builder.
	m = drive(t, m, logPollMsg{gen: m.log.gen})
	v := snapshot(t, "05-log", m)
	if !strings.Contains(v, "Deploying to Gotham") || !strings.Contains(v, "Finished: SUCCESS") || strings.Contains(v, "ha:AAAA") {
		t.Fatalf("log:\n%s", v)
	}

	m = drive(t, m, key("h"))
	m = drive(t, m, key("b"))
	if m.scr != scrParams {
		t.Fatalf("b should open the params form, screen=%d", m.scr)
	}
	m = drive(t, m, key("right")) // ENV dev -> staging
	m = drive(t, m, key("tab"))
	m = drive(t, m, key(" ")) // DRY_RUN true -> false
	snapshot(t, "06-params", m)
	m = drive(t, m, key("enter"))
	if v := snapshot(t, "07-confirm", m); !strings.Contains(v, "Start gotham/deploy") {
		t.Fatalf("confirm:\n%s", v)
	}
	m = drive(t, m, key("y"))
	fj.mu.Lock()
	got := fj.triggered
	fj.mu.Unlock()
	if got["ENV"] != "staging" || got["DRY_RUN"] != "false" {
		t.Errorf("triggered with %v", got)
	}
	if m.scr != scrBuilds {
		t.Errorf("after build screen=%d", m.scr)
	}
	snapshot(t, "08-after-build", m)

	m = drive(t, m, key("?"))
	snapshot(t, "09-help", m)
}

func TestWatchNotifiesOnFinish(t *testing.T) {
	cfg, _ := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	cfg.Pinned = []string{"gotham/deploy"}
	m := New(Options{Config: cfg, NoAnim: true})
	if cmd := m.handleWatch(watchResultMsg{name: "gotham/deploy", build: &jenkins.Build{Number: 5, Building: true}}); cmd != nil {
		t.Error("first sighting must not notify")
	}
	if cmd := m.handleWatch(watchResultMsg{name: "gotham/deploy", build: &jenkins.Build{Number: 5, Building: true}}); cmd != nil {
		t.Error("still running must not notify")
	}
	if cmd := m.handleWatch(watchResultMsg{name: "gotham/deploy", build: &jenkins.Build{Number: 5, Result: "SUCCESS"}}); cmd == nil {
		t.Error("finish must notify")
	}
	if cmd := m.handleWatch(watchResultMsg{name: "gotham/deploy", build: &jenkins.Build{Number: 5, Result: "SUCCESS"}}); cmd != nil {
		t.Error("same finished build must not notify twice")
	}
	if cmd := m.handleWatch(watchResultMsg{name: "gotham/deploy", build: &jenkins.Build{Number: 7, Result: "FAILURE"}}); cmd == nil {
		t.Error("a new finished build must notify")
	}
}

func TestInputFlow(t *testing.T) {
	fj := &fakeJenkins{withInput: true}
	srv := httptest.NewServer(fj)
	defer srv.Close()
	client, _ := jenkins.New(srv.URL, "bruce", "fake-test-value")
	cfg, _ := config.Load(filepath.Join(t.TempDir(), "config.toml"))

	m := New(Options{Config: cfg, Client: client, NoAnim: true})
	m = drive(t, m, cmdMsg{m.Init()})
	m = drive(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drive(t, m, key("enter")) // gotham/
	m = drive(t, m, key("enter")) // deploy
	v := snapshot(t, "10-builds-waiting", m)
	if !strings.Contains(v, "#42 is waiting: Deploy to prod?") || !strings.Contains(v, "INPUT") {
		t.Fatalf("builds should flag the waiting input:\n%s", v)
	}

	// Answer from the log view: the banner shows there too.
	m = drive(t, m, key("enter"))
	if v := snapshot(t, "11-log-waiting", m); !strings.Contains(v, "Waiting for input: Deploy to prod?") {
		t.Fatalf("log should show the input banner:\n%s", v)
	}
	m = drive(t, m, key("i"))
	m = drive(t, m, key("right")) // REGION eu -> us
	if v := snapshot(t, "12-input-form", m); !strings.Contains(v, "Deploy to prod?") || !strings.Contains(v, "us") {
		t.Fatalf("input form:\n%s", v)
	}
	m = drive(t, m, key("enter"))
	if v := snapshot(t, "13-input-confirm", m); !strings.Contains(v, "→ Deploy?") {
		t.Fatalf("confirm:\n%s", v)
	}
	m = drive(t, m, key("n")) // chickened out: nothing sent
	m = drive(t, m, key("enter"))
	m = drive(t, m, key("y"))
	if m.scr != scrLog {
		t.Errorf("should return to the log, screen=%d", m.scr)
	}

	// Abort from the builds view.
	m = drive(t, m, key("h"))
	m = drive(t, m, key("i"))
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})
	if v := snapshot(t, "14-abort-confirm", m); !strings.Contains(v, "Abort gotham/deploy #42 at: Deploy to prod?") {
		t.Fatalf("abort confirm:\n%s", v)
	}
	m = drive(t, m, key("y"))

	fj.mu.Lock()
	defer fj.mu.Unlock()
	want := []string{
		`/job/gotham/job/deploy/42/input/Prod/submit {"parameter":[{"name":"REGION","value":"us"}]}`,
		`/job/gotham/job/deploy/42/input/Prod/abort `,
	}
	if strings.Join(fj.answered, "\n") != strings.Join(want, "\n") {
		t.Errorf("answered:\n%s\nwant:\n%s", strings.Join(fj.answered, "\n"), strings.Join(want, "\n"))
	}
}

func TestWatchNotifiesOnInputOnce(t *testing.T) {
	cfg, _ := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	cfg.Pinned = []string{"gotham/deploy"}
	m := New(Options{Config: cfg, NoAnim: true})
	b := &jenkins.Build{Number: 9, Building: true, Inputs: []jenkins.InputRequest{{ID: "Prod", Message: "Deploy to prod?"}}}
	if m.handleWatch(watchResultMsg{name: "gotham/deploy", build: b}) == nil {
		t.Error("a waiting input must notify, even on the first poll")
	}
	if m.handleWatch(watchResultMsg{name: "gotham/deploy", build: b}) != nil {
		t.Error("the same input must notify only once")
	}
}
