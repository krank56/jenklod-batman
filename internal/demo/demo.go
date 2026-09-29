// Package demo is a simulated Jenkins controller for screenshots, recordings
// and trying the TUI without a real server. Everything in it is fictional.
//
// It keeps state in memory: triggered builds wait briefly in the queue, then
// run with a console log that grows over time, may pause on an input step,
// and finish with a result.
package demo

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	classFolder    = "com.cloudbees.hudson.plugins.folder.Folder"
	classPipeline  = "org.jenkinsci.plugins.workflow.job.WorkflowJob"
	classFreestyle = "hudson.model.FreeStyleProject"
)

type param struct {
	Name, Type, Description, Default string
	Choices                          []string
}

type job struct {
	name     string
	class    string
	children []*job // folders only
	params   []param
	// duration and inputAt (0: never) shape each new build, in seconds.
	duration, inputAt float64
	stages            []string
	builds            []*build // newest first
	next              int
}

type build struct {
	number   int
	params   map[string]string
	cause    string
	started  time.Time
	duration float64 // seconds of work, not counting a pause on input
	inputAt  float64
	// progress is seconds of work done, advanced lazily on each request.
	progress float64
	lastTick time.Time
	answered bool
	result   string // "" while running
	log      []logLine
	stages   []string
}

type logLine struct {
	at   float64
	text string
}

type queued struct {
	job     *job
	params  map[string]string
	readyAt time.Time
	number  int
}

// Server is an http.Handler speaking enough of the Jenkins API for the TUI.
type Server struct {
	mu    sync.Mutex
	root  *job
	queue map[int]*queued
	qnext int
	now   func() time.Time
}

// New builds Gotham: a few folders, pipelines with parameters and an input
// step, and some history. One deploy is already running.
func New() *Server {
	s := &Server{queue: map[int]*queued{}, qnext: 100, now: time.Now}
	deployStages := []string{"Checkout", "Build", "Test", "Deploy to staging", "Deploy to prod"}
	envParams := []param{
		{Name: "ENV", Type: "ChoiceParameterDefinition", Choices: []string{"staging", "prod"}, Default: "staging", Description: "Where to deploy"},
		{Name: "DRY_RUN", Type: "BooleanParameterDefinition", Default: "false", Description: "Plan only, change nothing"},
		{Name: "VERSION", Type: "StringParameterDefinition", Default: "v2.7.1", Description: "Release to ship"},
	}
	pipe := func(name string, dur, inputAt float64, stages []string, params ...param) *job {
		return &job{name: name, class: classPipeline, duration: dur, inputAt: inputAt, stages: stages, params: params}
	}
	free := func(name string, dur float64) *job {
		return &job{name: name, class: classFreestyle, duration: dur, stages: []string{"Compile", "Lint", "Archive"}}
	}
	folder := func(name string, kids ...*job) *job { return &job{name: name, class: classFolder, children: kids} }

	s.root = folder("",
		folder("gotham",
			pipe("deploy", 16, 12, deployStages, envParams...),
			pipe("batcave-backup", 18, 0, []string{"Snapshot", "Encrypt", "Upload"}),
			pipe("signal-tower", 12, 0, []string{"Build", "Calibrate", "Test"}),
		),
		folder("arkham",
			pipe("cell-audit", 16, 0, []string{"Inventory", "Verify locks", "Report"}),
			free("inmate-tracker", 10),
		),
		folder("wayne-enterprises",
			pipe("payroll", 20, 0, []string{"Collect", "Compute", "Pay"}),
			folder("applied-sciences",
				pipe("grappling-hook-ci", 14, 0, []string{"Build", "Stress test", "Package"}),
				free("cowl-firmware", 12),
			),
		),
		free("batmobile-lint", 8),
		free("utility-belt", 9),
	)

	// History, oldest first, so numbers climb.
	past := func(j *job, results ...string) {
		for i, r := range results {
			b := s.newBuild(j, nil, "Started by timer")
			age := time.Duration(len(results)-i) * 47 * time.Minute
			b.started = b.started.Add(-age)
			b.progress, b.result, b.answered = b.duration, r, true
			b.lastTick = b.started
		}
	}
	find := func(p string) *job { return s.find(strings.Split(p, "/")) }
	past(find("gotham/deploy"), "SUCCESS", "SUCCESS", "FAILURE", "SUCCESS", "SUCCESS")
	past(find("gotham/batcave-backup"), "SUCCESS", "SUCCESS", "SUCCESS")
	past(find("gotham/signal-tower"), "SUCCESS", "UNSTABLE")
	past(find("arkham/cell-audit"), "SUCCESS", "FAILURE", "FAILURE")
	past(find("arkham/inmate-tracker"), "SUCCESS")
	past(find("wayne-enterprises/payroll"), "SUCCESS", "SUCCESS")
	past(find("wayne-enterprises/applied-sciences/grappling-hook-ci"), "SUCCESS", "ABORTED", "SUCCESS")
	past(find("wayne-enterprises/applied-sciences/cowl-firmware"), "SUCCESS")
	past(find("batmobile-lint"), "SUCCESS", "SUCCESS")
	past(find("utility-belt"), "FAILURE")

	// A deploy already on its way, and a long backup running.
	d := s.newBuild(find("gotham/deploy"), map[string]string{"ENV": "prod", "DRY_RUN": "false", "VERSION": "v2.7.0"}, "Started by user Bruce Wayne")
	d.progress = 8 // reaches its input a few seconds in
	b := s.newBuild(find("gotham/batcave-backup"), nil, "Started by timer")
	b.duration, b.progress = 90, 20
	return s
}

func (s *Server) find(path []string) *job {
	j := s.root
	for _, name := range path {
		var next *job
		for _, c := range j.children {
			if c.name == name {
				next = c
			}
		}
		if next == nil {
			return nil
		}
		j = next
	}
	return j
}

func (s *Server) newBuild(j *job, params map[string]string, cause string) *build {
	j.next++
	now := s.now()
	b := &build{number: j.next, params: params, cause: cause, started: now, lastTick: now,
		duration: j.duration, inputAt: j.inputAt, stages: j.stages}
	if b.inputAt > 0 && params["DRY_RUN"] == "true" {
		b.inputAt = 0 // nothing to approve on a dry run
	}
	b.log = scriptLog(j, b)
	j.builds = append([]*build{b}, j.builds...)
	return b
}

// scriptLog is what the console will show, line by line, as work progresses.
func scriptLog(j *job, b *build) []logLine {
	lines := []logLine{{0, b.cause}, {0, "Running on batcomputer-01 in /var/jenkins/workspace/" + j.name}}
	for _, k := range slices.Sorted(maps.Keys(b.params)) {
		lines = append(lines, logLine{0, fmt.Sprintf("  %s = %s", k, b.params[k])})
	}
	n := float64(len(b.stages))
	for i, st := range b.stages {
		at := b.duration * float64(i) / n
		if b.inputAt > 0 && strings.Contains(st, "prod") {
			at = b.inputAt
		}
		lines = append(lines, logLine{at, "[Pipeline] { (" + st + ")"})
		if b.inputAt > 0 && at == b.inputAt {
			lines = append(lines, logLine{at, "[Pipeline] input"}, logLine{at, "Deploy to prod?"}, logLine{at, "Proceed or Abort"})
		}
		lines = append(lines,
			logLine{at + 0.5, "  " + stageDetail(st)},
			logLine{at + 1.5, "  ✓ " + st + " complete"},
			logLine{at + 1.6, "[Pipeline] }"},
		)
	}
	sortLog(lines)
	return lines
}

func stageDetail(st string) string {
	switch st {
	case "Checkout":
		return "Cloning git@github.com:wayne-enterprises/gotham.git (main)"
	case "Build":
		return "go build ./...  (142 packages)"
	case "Test":
		return "ok  gotham/...  1,204 tests passed"
	case "Deploy to staging":
		return "Rolling out to staging-gotham: 3/3 pods ready"
	case "Deploy to prod":
		return "Rolling out to prod-gotham: 12/12 pods ready"
	}
	return "Working on " + strings.ToLower(st) + "…"
}

// advance moves a running build forward to now.
func (s *Server) advance(b *build) {
	now := s.now()
	if b.result != "" {
		return
	}
	b.progress += now.Sub(b.lastTick).Seconds()
	b.lastTick = now
	if b.pending() {
		b.progress = b.inputAt
		return
	}
	if b.progress >= b.duration {
		b.progress = b.duration
		b.result = "SUCCESS"
	}
}

func (b *build) pending() bool {
	return b.result == "" && b.inputAt > 0 && !b.answered && b.progress >= b.inputAt
}

// text is the console so far. It only ever grows, since the TUI reads it
// from byte offsets.
func (b *build) text() string {
	var sb strings.Builder
	for _, l := range b.log {
		if l.at > b.progress {
			break
		}
		sb.WriteString(l.text + "\n")
	}
	if b.result != "" {
		sb.WriteString("Finished: " + b.result + "\n")
	}
	return sb.String()
}

func (s *Server) color(j *job) string {
	if len(j.builds) == 0 {
		return "notbuilt"
	}
	last := j.builds[0]
	s.advance(last)
	res := last.result
	if res == "" && len(j.builds) > 1 {
		res = j.builds[1].result
	}
	c := map[string]string{"SUCCESS": "blue", "FAILURE": "red", "UNSTABLE": "yellow", "ABORTED": "aborted", "": "notbuilt"}[res]
	if last.result == "" {
		c += "_anime"
	}
	return c
}

// ---- HTTP -------------------------------------------------------------

var (
	reJob   = regexp.MustCompile(`^((?:/job/[^/]+)*)(?:/(\d+))?(/.*)?$`)
	reQueue = regexp.MustCompile(`^/queue/item/(\d+)/api/json$`)
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startQueued()
	r.ParseForm()
	p := r.URL.EscapedPath()
	switch {
	case p == "/me/api/json":
		writeJSON(w, map[string]string{"id": "bruce", "fullName": "Bruce Wayne"})
		return
	case p == "/crumbIssuer/api/json":
		http.NotFound(w, r)
		return
	}
	if m := reQueue.FindStringSubmatch(p); m != nil {
		s.serveQueue(w, r, m[1])
		return
	}
	m := reJob.FindStringSubmatch(p)
	if m == nil {
		http.NotFound(w, r)
		return
	}
	var path []string
	for _, seg := range strings.Split(strings.TrimPrefix(m[1], "/job/"), "/job/") {
		if seg != "" {
			name, _ := url.PathUnescape(seg)
			path = append(path, name)
		}
	}
	j := s.find(path)
	if j == nil {
		http.NotFound(w, r)
		return
	}
	rest := m[3]
	if m[2] != "" {
		n, _ := strconv.Atoi(m[2])
		s.serveBuild(w, r, j, n, rest)
		return
	}
	switch rest {
	case "/api/json":
		if j.class == classFolder {
			writeJSON(w, map[string]any{"jobs": s.listing(j)})
			return
		}
		writeJSON(w, s.detail(j))
	case "/build", "/buildWithParameters":
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		params := map[string]string{}
		for _, pd := range j.params {
			params[pd.Name] = pd.Default
			if v := r.PostForm.Get(pd.Name); v != "" || r.PostForm.Has(pd.Name) {
				params[pd.Name] = v
			}
		}
		if len(j.params) == 0 {
			params = nil
		}
		s.qnext++
		s.queue[s.qnext] = &queued{job: j, params: params, readyAt: s.now().Add(1500 * time.Millisecond)}
		w.Header().Set("Location", fmt.Sprintf("http://%s/queue/item/%d/", r.Host, s.qnext))
		w.WriteHeader(http.StatusCreated)
	default:
		http.NotFound(w, r)
	}
}

// startQueued turns queue items whose wait is over into builds.
func (s *Server) startQueued() {
	for _, q := range s.queue {
		if q.number == 0 && !s.now().Before(q.readyAt) {
			q.number = s.newBuild(q.job, q.params, "Started by user Bruce Wayne").number
		}
	}
}

func (s *Server) serveQueue(w http.ResponseWriter, r *http.Request, id string) {
	n, _ := strconv.Atoi(id)
	q := s.queue[n]
	if q == nil {
		http.NotFound(w, r)
		return
	}
	if q.number == 0 {
		writeJSON(w, map[string]any{"cancelled": false, "why": "Waiting for next available executor"})
		return
	}
	writeJSON(w, map[string]any{"cancelled": false, "executable": map[string]int{"number": q.number}})
}

func (s *Server) serveBuild(w http.ResponseWriter, r *http.Request, j *job, n int, rest string) {
	var b *build
	for _, x := range j.builds {
		if x.number == n {
			b = x
		}
	}
	if b == nil {
		http.NotFound(w, r)
		return
	}
	s.advance(b)
	switch {
	case rest == "/api/json":
		writeJSON(w, s.buildJSON(b))
	case rest == "/logText/progressiveText":
		text := b.text()
		start, _ := strconv.Atoi(r.URL.Query().Get("start"))
		start = min(max(start, 0), len(text))
		w.Header().Set("X-Text-Size", strconv.Itoa(len(text)))
		if b.result == "" {
			w.Header().Set("X-More-Data", "true")
		}
		w.Write([]byte(text[start:]))
	case rest == "/wfapi/pendingInputActions":
		if j.class != classPipeline {
			http.NotFound(w, r)
			return
		}
		ins := []any{}
		if b.pending() {
			ins = append(ins, map[string]any{"id": "Prod", "message": "Deploy to prod?", "proceedText": "Deploy", "inputs": []any{}})
		}
		writeJSON(w, ins)
	case strings.HasPrefix(rest, "/input/Prod/"):
		if r.Method != http.MethodPost || !b.pending() {
			http.Error(w, "no such input", http.StatusBadRequest)
			return
		}
		if strings.HasSuffix(rest, "/abort") {
			b.log = append(b.log, logLine{b.progress, "Rejected by Bruce Wayne"})
			sortLog(b.log)
			b.result = "ABORTED"
			return
		}
		b.answered = true
		b.lastTick = s.now()
		b.log = append(b.log, logLine{b.inputAt, "Approved by Bruce Wayne"})
		sortLog(b.log)
	case rest == "/stop":
		if r.Method == http.MethodPost && b.result == "" {
			b.log = append(b.log, logLine{b.progress, "Aborted by Bruce Wayne"})
			sortLog(b.log)
			b.result = "ABORTED"
		}
	default:
		http.NotFound(w, r)
	}
}

func sortLog(ls []logLine) {
	for i := 1; i < len(ls); i++ {
		for k := i; k > 0 && ls[k].at < ls[k-1].at; k-- {
			ls[k], ls[k-1] = ls[k-1], ls[k]
		}
	}
}

func (s *Server) listing(folder *job) []map[string]any {
	out := []map[string]any{}
	for _, c := range folder.children {
		e := map[string]any{"_class": c.class, "name": c.name}
		if c.class == classFolder {
			e["jobs"] = s.listing(c)
		} else {
			e["color"] = s.color(c)
		}
		out = append(out, e)
	}
	return out
}

func (s *Server) buildJSON(b *build) map[string]any {
	s.advance(b)
	var result any
	if b.result != "" {
		result = b.result
	}
	dur := 0.0
	if b.result != "" {
		dur = b.duration
	}
	actions := []any{map[string]any{"causes": []any{map[string]string{"shortDescription": b.cause}}}}
	if len(b.params) > 0 {
		params := []any{}
		for _, k := range slices.Sorted(maps.Keys(b.params)) {
			params = append(params, map[string]string{"name": k, "value": b.params[k]})
		}
		actions = append(actions, map[string]any{"_class": "hudson.model.ParametersAction", "parameters": params})
	}
	return map[string]any{
		"number": b.number, "result": result, "building": b.result == "",
		"duration": int64(dur * 1000), "estimatedDuration": int64(b.duration * 1000),
		"timestamp": b.started.UnixMilli(),
		"actions":   actions,
	}
}

func (s *Server) detail(j *job) map[string]any {
	builds := []any{}
	for i, b := range j.builds {
		if i == 30 {
			break
		}
		builds = append(builds, s.buildJSON(b))
	}
	var defs []any
	for _, p := range j.params {
		d := map[string]any{"name": p.Name, "type": p.Type, "description": p.Description, "choices": p.Choices}
		var v any = p.Default
		if p.Type == "BooleanParameterDefinition" {
			v = p.Default == "true"
		}
		d["defaultParameterValue"] = map[string]any{"value": v}
		defs = append(defs, d)
	}
	out := map[string]any{"name": j.name, "buildable": true, "builds": builds,
		"property": []any{map[string]any{"parameterDefinitions": defs}}}
	if len(builds) > 0 {
		out["lastBuild"] = builds[0]
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
