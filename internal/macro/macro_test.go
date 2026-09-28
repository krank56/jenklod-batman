package macro

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
)

// fake is a controller with one pipeline, gotham/deploy. Build #41 is
// running; a triggered build becomes #42, pauses on input "Prod" after one
// poll, and succeeds once it is answered.
type fake struct {
	mu        sync.Mutex
	calls     []string
	triggered map[string]string
	running41 bool
	waiting41 bool
	started   bool
	polls42   int
	answered  bool
	result42  string
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r.ParseForm()
	if r.Method == http.MethodPost {
		f.calls = append(f.calls, r.URL.Path)
	}
	switch r.URL.Path {
	case "/crumbIssuer/api/json":
		http.NotFound(w, r)
	case "/job/gotham/job/deploy/api/json":
		w.Write([]byte(`{"name":"deploy","buildable":true,"builds":[{"number":41,"building":` + boolStr(f.running41) + `}]}`))
	case "/job/gotham/job/deploy/41/api/json":
		w.Write([]byte(`{"number":41,"building":` + boolStr(f.running41) + `}`))
	case "/job/gotham/job/deploy/41/wfapi/pendingInputActions":
		if f.waiting41 {
			w.Write([]byte(`[{"id":"Ok","message":"Continue?","proceedText":"Go","inputs":[]}]`))
			return
		}
		w.Write([]byte(`[]`))
	case "/job/gotham/job/deploy/41/input/Ok/proceedEmpty":
		f.waiting41 = false
	case "/job/gotham/job/deploy/41/stop":
		f.running41 = false
	case "/job/gotham/job/deploy/buildWithParameters":
		f.triggered = map[string]string{}
		for k := range r.PostForm {
			f.triggered[k] = r.PostForm.Get(k)
		}
		f.started = true
		w.Header().Set("Location", "http://ci/queue/item/5/")
		w.WriteHeader(http.StatusCreated)
	case "/queue/item/5/api/json":
		w.Write([]byte(`{"executable":{"number":42}}`))
	case "/job/gotham/job/deploy/42/api/json":
		f.polls42++
		building := !f.answered
		res := `null`
		if !building {
			res = `"` + f.result42 + `"`
		}
		w.Write([]byte(`{"number":42,"building":` + boolStr(building) + `,"result":` + res + `}`))
	case "/job/gotham/job/deploy/42/wfapi/pendingInputActions":
		if f.answered || f.polls42 == 0 {
			w.Write([]byte(`[]`))
			return
		}
		w.Write([]byte(`[{"id":"Prod","message":"Deploy to prod?","proceedText":"Deploy","inputs":[]}]`))
	case "/job/gotham/job/deploy/42/input/Prod/proceedEmpty":
		f.answered = true
	default:
		http.NotFound(w, r)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func newRunner(t *testing.T, f *fake) (*Runner, *[]Event) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	c, err := jenkins.New(srv.URL, "bruce", "fake-test-value")
	if err != nil {
		t.Fatal(err)
	}
	var events []Event
	return &Runner{Client: c, Poll: time.Millisecond, Report: func(e Event) { events = append(events, e) }}, &events
}

func TestRunChainsStepsOnTheBuildItStarted(t *testing.T) {
	f := &fake{running41: true, result42: "SUCCESS"}
	r, events := newRunner(t, f)
	m := config.Macro{Name: "ship", Steps: []config.Step{
		{Kind: config.StepAbort, Job: "gotham/deploy"}, // the newest running build: #41
		{Kind: config.StepBuild, Job: "gotham/deploy", WithParams: true, Params: map[string]string{"ENV": "staging", "DRY": "true"}},
		{Kind: config.StepInput, Job: "gotham/deploy"}, // #42, the one step 2 started
		{Kind: config.StepWait, Job: "gotham/deploy"},
	}}
	if err := r.Run(context.Background(), m, map[string]string{"ENV": "prod"}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/job/gotham/job/deploy/41/stop",
		"/job/gotham/job/deploy/buildWithParameters",
		"/job/gotham/job/deploy/42/input/Prod/proceedEmpty",
	}
	if got := strings.Join(f.calls, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("calls:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
	if f.triggered["ENV"] != "prod" || f.triggered["DRY"] != "true" {
		t.Errorf("triggered with %v", f.triggered)
	}
	var texts []string
	for _, e := range *events {
		texts = append(texts, e.Text)
	}
	if all := strings.Join(texts, "\n"); !strings.Contains(all, "started gotham/deploy #42") || !strings.Contains(all, "#42 succeeded") {
		t.Errorf("events:\n%s", all)
	}
}

func TestRunStopsAtTheFirstFailure(t *testing.T) {
	f := &fake{result42: "FAILURE"}
	r, _ := newRunner(t, f)
	m := config.Macro{Name: "ship", Steps: []config.Step{
		{Kind: config.StepAbort, Job: "gotham/deploy"}, // nothing running
		{Kind: config.StepBuild, Job: "gotham/deploy", WithParams: true},
	}}
	err := r.Run(context.Background(), m, nil)
	var se *StepError
	if !errors.As(err, &se) || se.Step != 1 || !strings.Contains(err.Error(), "no running build") {
		t.Fatalf("err = %v", err)
	}
	if f.started {
		t.Error("step 2 ran after step 1 failed")
	}

	// A wait on a build that fails fails the macro.
	m.Steps = []config.Step{
		{Kind: config.StepBuild, Job: "gotham/deploy", WithParams: true},
		{Kind: config.StepInput, Job: "gotham/deploy"},
		{Kind: config.StepWait, Job: "gotham/deploy"},
	}
	if err := r.Run(context.Background(), m, nil); err == nil || !strings.Contains(err.Error(), "#42 finished: FAILURE") {
		t.Errorf("err = %v", err)
	}
}

func TestCheckOverrides(t *testing.T) {
	m := config.Macro{Name: "ship", Steps: []config.Step{
		{Kind: config.StepBuild, Job: "a", WithParams: true, Params: map[string]string{"ENV": "dev"}},
	}}
	if err := CheckOverrides(m, map[string]string{"ENV": "prod"}); err != nil {
		t.Error(err)
	}
	if err := CheckOverrides(m, map[string]string{"EVN": "prod"}); err == nil {
		t.Error("a typo must be rejected")
	}
}

func TestUnchainedInputLooksOnceAndSkips(t *testing.T) {
	steps := []config.Step{
		{Kind: config.StepInput, Job: "gotham/deploy"},
		{Kind: config.StepBuild, Job: "gotham/deploy", WithParams: true},
	}
	cases := []struct {
		name               string
		running, waiting   bool
		wantCall, wantText string
	}{
		{"nothing running", false, false, "", "has no running build — skipped"},
		{"running, no input", true, false, "", "#41 is not waiting for input — skipped"},
		{"running, waiting", true, true, "/job/gotham/job/deploy/41/input/Ok/proceedEmpty", "Go at \"Continue?\""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fake{running41: c.running, waiting41: c.waiting}
			r, events := newRunner(t, f)
			if err := r.Run(context.Background(), config.Macro{Name: "m", Steps: steps}, nil); err != nil {
				t.Fatal(err)
			}
			if !f.started {
				t.Error("the macro did not go on to the build step")
			}
			if c.wantCall != "" && (len(f.calls) == 0 || f.calls[0] != c.wantCall) {
				t.Errorf("calls = %v", f.calls)
			}
			if c.wantCall == "" && len(f.calls) != 1 { // just the build
				t.Errorf("calls = %v", f.calls)
			}
			if got := (*events)[1].Text; !strings.Contains(got, c.wantText) {
				t.Errorf("event = %q, want %q", got, c.wantText)
			}
		})
	}
}
