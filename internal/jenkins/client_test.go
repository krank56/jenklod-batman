package jenkins

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, "bruce", "fake-test-value-7f3a")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestListJobsEscapesNestedPath(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if u, p, _ := r.BasicAuth(); u != "bruce" || p != "fake-test-value-7f3a" {
			t.Errorf("basic auth = %q/%q", u, p)
		}
		if got, want := r.URL.EscapedPath(), "/job/gotham%20city/job/cave/api/json"; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		w.Write([]byte(`{"jobs":[
			{"_class":"com.cloudbees.hudson.plugins.folder.Folder","name":"sub"},
			{"_class":"hudson.model.FreeStyleProject","name":"deploy","color":"red_anime"}]}`))
	})
	jobs, err := c.ListJobs(context.Background(), []string{"gotham city", "cave"})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || !jobs[0].IsFolder() || jobs[1].IsFolder() {
		t.Fatalf("jobs = %+v", jobs)
	}
	if got := jobs[1].FullName(); got != "gotham city/cave/deploy" {
		t.Errorf("full name = %q", got)
	}
	if s, building := ColorStatus(jobs[1].Color); s != StatusFailure || !building {
		t.Errorf("status = %v building = %v", s, building)
	}
}

func TestJobDetailParams(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"deploy","buildable":true,
			"builds":[{"number":7,"result":null,"building":true,"timestamp":1700000000000,
				"actions":[{},{"causes":[{"shortDescription":"Started by user Bruce"}]}]}],
			"property":[{},{"parameterDefinitions":[
				{"name":"ENV","type":"ChoiceParameterDefinition","choices":["dev","prod"],"defaultParameterValue":{"value":"dev"}},
				{"name":"DRY","type":"BooleanParameterDefinition","defaultParameterValue":{"value":true}},
				{"name":"PW","type":"PasswordParameterDefinition","defaultParameterValue":{"value":"hidden"}}]}]}`))
	})
	d, err := c.JobDetail(context.Background(), []string{"deploy"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Builds) != 1 || !d.Builds[0].Building || d.Builds[0].Cause != "Started by user Bruce" {
		t.Errorf("builds = %+v", d.Builds)
	}
	if len(d.Params) != 3 {
		t.Fatalf("params = %+v", d.Params)
	}
	if p := d.Params[0]; p.Type != ParamChoice || p.Default != "dev" || len(p.Choices) != 2 {
		t.Errorf("choice param = %+v", p)
	}
	if p := d.Params[1]; p.Type != ParamBool || p.Default != "true" {
		t.Errorf("bool param = %+v", p)
	}
	if p := d.Params[2]; p.Type != ParamPassword || p.Default != "" {
		t.Errorf("password default must not be exposed: %+v", p)
	}
}

func TestTriggerSendsCrumbAndParams(t *testing.T) {
	var posted bool
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.Write([]byte(`{"crumb":"abc","crumbRequestField":"Jenkins-Crumb"}`))
		case "/job/deploy/buildWithParameters":
			posted = true
			if r.Method != http.MethodPost || r.Header.Get("Jenkins-Crumb") != "abc" {
				t.Errorf("method %s crumb %q", r.Method, r.Header.Get("Jenkins-Crumb"))
			}
			r.ParseForm()
			if r.PostForm.Get("ENV") != "prod" {
				t.Errorf("form = %v", r.PostForm)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	})
	if err := c.Trigger(context.Background(), []string{"deploy"}, map[string]string{"ENV": "prod"}); err != nil {
		t.Fatal(err)
	}
	if !posted {
		t.Error("build was not posted")
	}
}

func TestLogProgressive(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/deploy/7/logText/progressiveText" || r.URL.Query().Get("start") != "10" {
			t.Errorf("unexpected %s", r.URL)
		}
		w.Header().Set("X-Text-Size", "42")
		w.Header().Set("X-More-Data", "true")
		w.Write([]byte("hello\n"))
	})
	chunk, err := c.Log(context.Background(), []string{"deploy"}, 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if chunk.Text != "hello\n" || chunk.Next != 42 || !chunk.More {
		t.Errorf("chunk = %+v", chunk)
	}
}

func TestAuthError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := c.ListJobs(context.Background(), nil)
	if err != ErrAuth {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "fake-test-value-7f3a") {
		t.Error("error leaks the token")
	}
}

func TestPendingInputsProceedAndAbort(t *testing.T) {
	var got []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/job/deploy/9/wfapi/pendingInputActions":
			w.Write([]byte(`[{"id":"Prod","proceedText":"Ship it","message":"Deploy to prod?",
				"inputs":[{"type":"BooleanParameterDefinition","name":"MIGRATE","definition":{"defaultVal":false}},
				          {"type":"ChoiceParameterDefinition","name":"REGION","definition":{"defaultVal":"eu","choices":["eu","us"]}}]},
				{"id":"Plain","message":"Go?","inputs":[]}]`))
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/deploy/9/input/Prod/submit":
			r.ParseForm()
			got = append(got, "submit "+r.PostForm.Get("proceed")+" "+r.PostForm.Get("json"))
		case "/job/deploy/9/input/Plain/proceedEmpty", "/job/deploy/9/input/Prod/abort":
			got = append(got, r.URL.Path)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
		}
	})
	ctx := context.Background()
	ins, err := c.PendingInputs(ctx, []string{"deploy"}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(ins) != 2 || ins[0].Message != "Deploy to prod?" || ins[1].ProceedText != "Proceed" {
		t.Fatalf("inputs = %+v", ins)
	}
	if p := ins[0].Params; len(p) != 2 || p[0].Type != ParamBool || p[0].Default != "false" || p[1].Choices[1] != "us" {
		t.Fatalf("params = %+v", p)
	}
	if err := c.ProceedInput(ctx, []string{"deploy"}, 9, ins[0], map[string]string{"MIGRATE": "true", "REGION": "us"}); err != nil {
		t.Fatal(err)
	}
	if err := c.ProceedInput(ctx, []string{"deploy"}, 9, ins[1], nil); err != nil {
		t.Fatal(err)
	}
	if err := c.AbortInput(ctx, []string{"deploy"}, 9, "Prod"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		`submit Ship it {"parameter":[{"name":"MIGRATE","value":true},{"name":"REGION","value":"us"}]}`,
		"/job/deploy/9/input/Plain/proceedEmpty",
		"/job/deploy/9/input/Prod/abort",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestAttachInputsWithoutStageView(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	builds := []Build{{Number: 3, Building: true}}
	ok, err := c.AttachInputs(context.Background(), []string{"deploy"}, builds)
	if ok || err != nil {
		t.Errorf("ok=%v err=%v", ok, err)
	}
}
