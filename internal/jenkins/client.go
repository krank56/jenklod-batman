// Package jenkins is a small client for the parts of the Jenkins JSON API
// that jenklod-batman needs: browsing folders, build history, console logs,
// triggering and aborting builds.
package jenkins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client talks to a single Jenkins controller using a user + API token.
type Client struct {
	base  *url.URL
	user  string
	token string
	http  *http.Client

	crumbOnce sync.Once
	crumbHdr  string
	crumbVal  string
}

// New returns a client for the controller at baseURL.
func New(baseURL, user, token string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid Jenkins URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid Jenkins URL %q: scheme must be http or https", baseURL)
	}
	return &Client{
		base:  u,
		user:  user,
		token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Host is the controller's host name, for display.
func (c *Client) Host() string { return c.base.Host }

// ErrAuth is returned when Jenkins rejects the credentials.
var ErrAuth = errors.New("authentication failed: check the user name and API token")

// ErrNotFound is returned for 404 responses.
var ErrNotFound = errors.New("not found")

// SplitFullName turns "folder/sub/job" into its path segments.
func SplitFullName(fullName string) []string {
	var out []string
	for p := range strings.SplitSeq(fullName, "/") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// JoinPath is the inverse of SplitFullName.
func JoinPath(path []string) string { return strings.Join(path, "/") }

// jobPath builds "/job/a/job/b" for a folder path.
func jobPath(path []string) string {
	var b strings.Builder
	for _, p := range path {
		b.WriteString("/job/")
		b.WriteString(url.PathEscape(p))
	}
	return b.String()
}

// BrowserURL is the web UI address of a job, or of one of its builds when
// number > 0.
func (c *Client) BrowserURL(path []string, number int) string {
	s := c.base.String() + jobPath(path) + "/"
	if number > 0 {
		s += strconv.Itoa(number) + "/"
	}
	return s
}

func (c *Client) newRequest(ctx context.Context, method, p string, query url.Values, body io.Reader) (*http.Request, error) {
	// p is already escaped; keep RawPath so job names containing "/" or
	// spaces survive as a single segment.
	unescaped, err := url.PathUnescape(p)
	if err != nil {
		return nil, err
	}
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + unescaped
	u.RawPath = strings.TrimRight(c.base.EscapedPath(), "/") + p
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.token)
	req.Header.Set("User-Agent", "jenklod-batman")
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return resp, nil
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, ErrAuth
	case http.StatusForbidden:
		return nil, fmt.Errorf("forbidden: your Jenkins user lacks permission for %s", req.URL.Path)
	case http.StatusNotFound:
		return nil, fmt.Errorf("%s: %w", req.URL.Path, ErrNotFound)
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	return nil, fmt.Errorf("%s %s: %s %s", req.Method, req.URL.Path, resp.Status, strings.TrimSpace(string(snippet)))
}

func (c *Client) getJSON(ctx context.Context, p string, tree string, v any) error {
	q := url.Values{}
	if tree != "" {
		q.Set("tree", tree)
	}
	return c.getRaw(ctx, p+"/api/json", q, v)
}

// getRaw decodes a JSON endpoint that is not under /api/json.
func (c *Client) getRaw(ctx context.Context, p string, q url.Values, v any) error {
	req, err := c.newRequest(ctx, http.MethodGet, p, q, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

// Ping checks connectivity and credentials, returning the authenticated
// user's display name.
func (c *Client) Ping(ctx context.Context) (string, error) {
	var me struct {
		FullName string `json:"fullName"`
		ID       string `json:"id"`
	}
	if err := c.getJSON(ctx, "/me", "fullName,id", &me); err != nil {
		return "", err
	}
	if me.ID == "anonymous" || me.ID == "" {
		return "", ErrAuth
	}
	if me.FullName != "" {
		return me.FullName, nil
	}
	return me.ID, nil
}

// Job is an entry of a folder listing.
type Job struct {
	Name  string
	Class string
	Color string
	Path  []string
}

// FullName is the slash-separated path of the job.
func (j Job) FullName() string { return JoinPath(j.Path) }

// IsFolder reports whether the item contains other jobs.
func (j Job) IsFolder() bool {
	switch {
	case strings.HasSuffix(j.Class, ".Folder"),
		strings.HasSuffix(j.Class, ".OrganizationFolder"),
		strings.HasSuffix(j.Class, ".WorkflowMultiBranchProject"):
		return true
	}
	return false
}

// Status is a normalised job or build state.
type Status int

const (
	StatusUnknown Status = iota
	StatusSuccess
	StatusFailure
	StatusUnstable
	StatusAborted
	StatusNotBuilt
	StatusDisabled
)

// ColorStatus decodes the Jenkins ball "color" field.
func ColorStatus(color string) (Status, bool) {
	building := strings.HasSuffix(color, "_anime")
	switch strings.TrimSuffix(color, "_anime") {
	case "blue", "green":
		return StatusSuccess, building
	case "red":
		return StatusFailure, building
	case "yellow":
		return StatusUnstable, building
	case "aborted":
		return StatusAborted, building
	case "notbuilt", "nobuilt":
		return StatusNotBuilt, building
	case "disabled", "grey":
		return StatusDisabled, building
	}
	return StatusUnknown, building
}

// ResultStatus decodes a build "result" field.
func ResultStatus(result string) Status {
	switch result {
	case "SUCCESS":
		return StatusSuccess
	case "FAILURE":
		return StatusFailure
	case "UNSTABLE":
		return StatusUnstable
	case "ABORTED":
		return StatusAborted
	case "NOT_BUILT":
		return StatusNotBuilt
	}
	return StatusUnknown
}

// ListJobs lists the items of the folder at path (nil for the root).
func (c *Client) ListJobs(ctx context.Context, path []string) ([]Job, error) {
	var resp struct {
		Jobs []struct {
			Class string `json:"_class"`
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"jobs"`
	}
	if err := c.getJSON(ctx, jobPath(path), "jobs[name,color]", &resp); err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		p := append(append([]string{}, path...), j.Name)
		jobs = append(jobs, Job{Name: j.Name, Class: j.Class, Color: j.Color, Path: p})
	}
	return jobs, nil
}

// treeDepth is how many folder levels AllJobs descends.
const treeDepth = 8

// AllJobs lists every job and folder on the controller in one request,
// depth first, for searching across folders.
func (c *Client) AllJobs(ctx context.Context) ([]Job, error) {
	type item struct {
		Class string `json:"_class"`
		Name  string `json:"name"`
		Color string `json:"color"`
		Jobs  []item `json:"jobs"`
	}
	tree := "jobs[name,color]"
	for range treeDepth - 1 {
		tree = "jobs[name,color," + tree + "]"
	}
	var resp struct {
		Jobs []item `json:"jobs"`
	}
	if err := c.getJSON(ctx, "", tree, &resp); err != nil {
		return nil, err
	}
	var out []Job
	var walk func(parent []string, items []item)
	walk = func(parent []string, items []item) {
		for _, it := range items {
			p := append(append([]string{}, parent...), it.Name)
			out = append(out, Job{Name: it.Name, Class: it.Class, Color: it.Color, Path: p})
			walk(p, it.Jobs)
		}
	}
	walk(nil, resp.Jobs)
	return out, nil
}

// Build is one run of a job.
type Build struct {
	Number    int
	Result    string
	Building  bool
	Duration  time.Duration
	Estimated time.Duration
	Started   time.Time
	Cause     string
	// Inputs are pending input steps; only filled in for running pipeline
	// builds (see PendingInputs).
	Inputs []InputRequest
}

// Status is the normalised result of the build.
func (b Build) Status() Status { return ResultStatus(b.Result) }

// ParamType is the kind of a build parameter.
type ParamType int

const (
	ParamString ParamType = iota
	ParamText
	ParamBool
	ParamChoice
	ParamPassword
)

// Param is a build parameter definition.
type Param struct {
	Name        string
	Type        ParamType
	Description string
	Default     string
	Choices     []string
}

func paramType(t string) ParamType {
	switch t {
	case "BooleanParameterDefinition":
		return ParamBool
	case "ChoiceParameterDefinition":
		return ParamChoice
	case "TextParameterDefinition":
		return ParamText
	case "PasswordParameterDefinition":
		return ParamPassword
	}
	return ParamString
}

// Detail is what the builds screen shows for a job.
type Detail struct {
	Name      string
	Buildable bool
	Params    []Param
	Builds    []Build
}

type apiBuild struct {
	Number            int    `json:"number"`
	Result            string `json:"result"`
	Building          bool   `json:"building"`
	Duration          int64  `json:"duration"`
	EstimatedDuration int64  `json:"estimatedDuration"`
	Timestamp         int64  `json:"timestamp"`
	Actions           []struct {
		Causes []struct {
			ShortDescription string `json:"shortDescription"`
		} `json:"causes"`
	} `json:"actions"`
}

func (a apiBuild) toBuild() Build {
	b := Build{
		Number:    a.Number,
		Result:    a.Result,
		Building:  a.Building,
		Duration:  time.Duration(a.Duration) * time.Millisecond,
		Estimated: time.Duration(a.EstimatedDuration) * time.Millisecond,
		Started:   time.UnixMilli(a.Timestamp),
	}
	for _, act := range a.Actions {
		if len(act.Causes) > 0 {
			b.Cause = act.Causes[0].ShortDescription
			break
		}
	}
	return b
}

const buildTree = "number,result,building,duration,estimatedDuration,timestamp,actions[causes[shortDescription]]"

// JobDetail fetches the recent builds and parameter definitions of a job.
func (c *Client) JobDetail(ctx context.Context, path []string) (*Detail, error) {
	var resp struct {
		Name      string     `json:"name"`
		Buildable bool       `json:"buildable"`
		Builds    []apiBuild `json:"builds"`
		Property  []struct {
			ParameterDefinitions []struct {
				Class       string   `json:"_class"`
				Name        string   `json:"name"`
				Type        string   `json:"type"`
				Description string   `json:"description"`
				Choices     []string `json:"choices"`
				Default     *struct {
					Value any `json:"value"`
				} `json:"defaultParameterValue"`
			} `json:"parameterDefinitions"`
		} `json:"property"`
	}
	tree := "name,buildable,builds[" + buildTree + "]{0,30}," +
		"property[parameterDefinitions[name,type,description,choices,defaultParameterValue[value]]]"
	if err := c.getJSON(ctx, jobPath(path), tree, &resp); err != nil {
		return nil, err
	}
	d := &Detail{Name: resp.Name, Buildable: resp.Buildable}
	for _, b := range resp.Builds {
		d.Builds = append(d.Builds, b.toBuild())
	}
	for _, prop := range resp.Property {
		for _, pd := range prop.ParameterDefinitions {
			p := Param{Name: pd.Name, Description: pd.Description, Choices: pd.Choices, Type: paramType(pd.Type)}
			if pd.Default != nil && pd.Default.Value != nil && p.Type != ParamPassword {
				p.Default = fmt.Sprint(pd.Default.Value)
			}
			d.Params = append(d.Params, p)
		}
	}
	return d, nil
}

// LastBuild returns the most recent build of a job, or nil if it never ran.
func (c *Client) LastBuild(ctx context.Context, path []string) (*Build, error) {
	var resp struct {
		LastBuild *apiBuild `json:"lastBuild"`
	}
	if err := c.getJSON(ctx, jobPath(path), "lastBuild["+buildTree+"]", &resp); err != nil {
		return nil, err
	}
	if resp.LastBuild == nil {
		return nil, nil
	}
	b := resp.LastBuild.toBuild()
	return &b, nil
}

// LogChunk is a slice of console output.
type LogChunk struct {
	Text string
	Next int64
	More bool
}

// Log fetches console output starting at byte offset start.
func (c *Client) Log(ctx context.Context, path []string, number int, start int64) (*LogChunk, error) {
	q := url.Values{"start": {strconv.FormatInt(start, 10)}}
	p := jobPath(path) + "/" + strconv.Itoa(number) + "/logText/progressiveText"
	req, err := c.newRequest(ctx, http.MethodGet, p, q, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	next := start + int64(len(body))
	if s := resp.Header.Get("X-Text-Size"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			next = n
		}
	}
	return &LogChunk{
		Text: string(body),
		Next: next,
		More: resp.Header.Get("X-More-Data") == "true",
	}, nil
}

// crumb fetches the CSRF crumb once. Controllers that do not issue crumbs
// (or accept API tokens without one) simply get no header.
func (c *Client) crumb(ctx context.Context) {
	c.crumbOnce.Do(func() {
		var resp struct {
			Crumb             string `json:"crumb"`
			CrumbRequestField string `json:"crumbRequestField"`
		}
		if err := c.getJSON(ctx, "/crumbIssuer", "", &resp); err == nil {
			c.crumbHdr, c.crumbVal = resp.CrumbRequestField, resp.Crumb
		}
	})
}

func (c *Client) post(ctx context.Context, p string, form url.Values) error {
	_, err := c.postH(ctx, p, form)
	return err
}

// postH is post that also returns the response headers.
func (c *Client) postH(ctx context.Context, p string, form url.Values) (http.Header, error) {
	c.crumb(ctx)
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := c.newRequest(ctx, http.MethodPost, p, nil, body)
	if err != nil {
		return nil, err
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.crumbHdr != "" {
		req.Header.Set(c.crumbHdr, c.crumbVal)
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return resp.Header, nil
}

// Trigger queues a build. Parameterised jobs must pass their values in
// params (possibly empty to use every default); others pass nil.
func (c *Client) Trigger(ctx context.Context, path []string, params map[string]string) error {
	_, err := c.TriggerQueued(ctx, path, params)
	return err
}

var queueItem = regexp.MustCompile(`/queue/item/(\d+)/?$`)

// TriggerQueued is Trigger that also returns the queue item id Jenkins
// assigned (0 if it did not say), for QueuedBuild.
func (c *Client) TriggerQueued(ctx context.Context, path []string, params map[string]string) (int, error) {
	var (
		h   http.Header
		err error
	)
	if params == nil {
		h, err = c.postH(ctx, jobPath(path)+"/build", nil)
	} else {
		form := url.Values{}
		for k, v := range params {
			form.Set(k, v)
		}
		h, err = c.postH(ctx, jobPath(path)+"/buildWithParameters", form)
	}
	if err != nil {
		return 0, err
	}
	if m := queueItem.FindStringSubmatch(h.Get("Location")); m != nil {
		id, _ := strconv.Atoi(m[1])
		return id, nil
	}
	return 0, nil
}

// ErrQueueCancelled means a queued build was cancelled before it started.
var ErrQueueCancelled = errors.New("the queued build was cancelled")

// QueuedBuild returns the build number a queue item became, or 0 while it
// is still waiting in the queue.
func (c *Client) QueuedBuild(ctx context.Context, id int) (int, error) {
	var resp struct {
		Cancelled  bool `json:"cancelled"`
		Executable *struct {
			Number int `json:"number"`
		} `json:"executable"`
	}
	if err := c.getJSON(ctx, "/queue/item/"+strconv.Itoa(id), "cancelled,executable[number]", &resp); err != nil {
		return 0, err
	}
	if resp.Cancelled {
		return 0, ErrQueueCancelled
	}
	if resp.Executable == nil {
		return 0, nil
	}
	return resp.Executable.Number, nil
}

// Build fetches one build of a job.
func (c *Client) Build(ctx context.Context, path []string, number int) (*Build, error) {
	var resp apiBuild
	if err := c.getJSON(ctx, jobPath(path)+"/"+strconv.Itoa(number), buildTree, &resp); err != nil {
		return nil, err
	}
	b := resp.toBuild()
	return &b, nil
}

// Abort stops a running build.
func (c *Client) Abort(ctx context.Context, path []string, number int) error {
	return c.post(ctx, jobPath(path)+"/"+strconv.Itoa(number)+"/stop", nil)
}
