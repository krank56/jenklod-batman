// Package ui is the Bubble Tea front end of jenklod-batman.
package ui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jenklod-batman/internal/config"
	"jenklod-batman/internal/jenkins"
	"jenklod-batman/internal/notify"
)

type screen int

const (
	scrSplash screen = iota
	scrWizard
	scrJobs
	scrBuilds
	scrLog
	scrParams
)

const frameDelay = 100 * time.Millisecond

// Options configure a new Model.
type Options struct {
	Config   *config.Config
	Client   *jenkins.Client // nil when the setup wizard must run
	NoAnim   bool
	WizardFn WizardFunc
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg    *config.Config
	client *jenkins.Client
	anim   bool

	w, h  int
	scr   screen
	frame int

	splash splash
	cat    cat
	wiz    wizard

	jobs   jobsState
	builds buildsState
	log    logState
	params paramsState

	confirm *confirmState
	help    bool

	status    string
	statusErr bool
	statusAt  time.Time

	watch        map[string]*watchState
	notifyWarned bool
}

type jobsState struct {
	path      []string
	items     []jenkins.Job
	cursor    int
	cursors   []int // cursor per parent level, to restore on "back"
	filter    textinput.Model
	filtering bool
	loading   bool
	err       error
}

type buildsState struct {
	job     jenkins.Job
	detail  *jenkins.Detail
	cursor  int
	loading bool
	err     error
	// pendingBuild opens the build flow once the detail (and so the
	// parameter definitions) has loaded.
	pendingBuild bool
	// noInputAPI is set when the controller lacks the Stage View API, so
	// pending input steps cannot be listed.
	noInputAPI bool
}

type logState struct {
	job     jenkins.Job
	number  int
	gen     int
	text    *strings.Builder // pointer: the model is copied by value on every update
	offset  int64
	more    bool
	follow  bool
	vp      viewport.Model
	loading bool
	err     error
	inputs  []jenkins.InputRequest
	polls   int
}

// paramsState is the form for build parameters, and also for answering an
// input step when input is set.
type paramsState struct {
	job    jenkins.Job
	fields []paramField
	focus  int

	input    *jenkins.InputRequest
	number   int
	returnTo screen
}

type paramField struct {
	def    jenkins.Param
	input  textinput.Model
	on     bool
	choice int
}

type confirmState struct {
	prompt string
	onYes  tea.Cmd
}

type watchState struct {
	number   int
	building bool
	known    bool
	asked    map[string]bool // "<build>/<input id>" already notified
}

// New builds the root model.
func New(o Options) Model {
	fi := textinput.New()
	fi.Prompt = "/"
	fi.Placeholder = "filter"
	m := Model{
		cfg:    o.Config,
		client: o.Client,
		anim:   !o.NoAnim,
		cat:    newCat(time.Now().UnixNano()),
		wiz:    newWizard(o.Config, o.WizardFn),
		watch:  map[string]*watchState{},
		scr:    scrSplash,
	}
	m.jobs.filter = fi
	m.log.follow = true
	if !m.anim {
		m.scr = m.afterSplash()
	}
	return m
}

func (m Model) afterSplash() screen {
	if m.client == nil {
		return scrWizard
	}
	return scrJobs
}

// ---- messages ---------------------------------------------------------

type tickMsg struct{}
type watchTickMsg struct{}

type jobsMsg struct {
	path []string
	jobs []jenkins.Job
	err  error
}

type detailMsg struct {
	job        jenkins.Job
	detail     *jenkins.Detail
	noInputAPI bool
	err        error
}

type logInputsMsg struct {
	gen    int
	inputs []jenkins.InputRequest
}

type logMsg struct {
	gen   int
	chunk *jenkins.LogChunk
	err   error
}

type logPollMsg struct{ gen int }

type actionMsg struct {
	ok      string
	err     error
	refresh bool
}

type watchResultMsg struct {
	name  string
	build *jenkins.Build // Inputs filled in while it runs
	err   error
}

// ---- commands ---------------------------------------------------------

func tick() tea.Cmd { return tea.Tick(frameDelay, func(time.Time) tea.Msg { return tickMsg{} }) }

func (m Model) watchTick() tea.Cmd {
	return tea.Tick(time.Duration(m.cfg.PollSeconds)*time.Second, func(time.Time) tea.Msg { return watchTickMsg{} })
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func (m Model) loadJobs(path []string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		jobs, err := c.ListJobs(cx, path)
		return jobsMsg{path: path, jobs: jobs, err: err}
	}
}

func (m Model) loadDetail(job jenkins.Job) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		d, err := c.JobDetail(cx, job.Path)
		if err != nil || !job.IsPipeline() {
			return detailMsg{job: job, detail: d, err: err}
		}
		supported, err := c.AttachInputs(cx, job.Path, d.Builds)
		return detailMsg{job: job, detail: d, noInputAPI: !supported, err: err}
	}
}

func (m Model) loadLogInputs(gen int, job jenkins.Job, number int) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		ins, _ := c.PendingInputs(cx, job.Path, number) // best effort
		return logInputsMsg{gen: gen, inputs: ins}
	}
}

func (m Model) proceedInput(job jenkins.Job, number int, in jenkins.InputRequest, values map[string]string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		if err := c.ProceedInput(cx, job.Path, number, in, values); err != nil {
			return actionMsg{err: fmt.Errorf("proceed %s #%d: %w", job.FullName(), number, err)}
		}
		return actionMsg{ok: fmt.Sprintf("▶ %s #%d: %s", job.FullName(), number, in.ProceedText), refresh: true}
	}
}

func (m Model) abortInput(job jenkins.Job, number int, in jenkins.InputRequest) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		if err := c.AbortInput(cx, job.Path, number, in.ID); err != nil {
			return actionMsg{err: fmt.Errorf("abort %s #%d: %w", job.FullName(), number, err)}
		}
		return actionMsg{ok: fmt.Sprintf("✋ %s #%d aborted at %q", job.FullName(), number, in.Message), refresh: true}
	}
}

func (m Model) loadLog(gen int, job jenkins.Job, number int, offset int64) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		chunk, err := c.Log(cx, job.Path, number, offset)
		return logMsg{gen: gen, chunk: chunk, err: err}
	}
}

func (m Model) trigger(job jenkins.Job, params map[string]string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		if err := c.Trigger(cx, job.Path, params); err != nil {
			return actionMsg{err: fmt.Errorf("build %s: %w", job.FullName(), err)}
		}
		return actionMsg{ok: "🦇 Build queued: " + job.FullName(), refresh: true}
	}
}

func (m Model) abort(job jenkins.Job, number int) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		if err := c.Abort(cx, job.Path, number); err != nil {
			return actionMsg{err: fmt.Errorf("abort %s #%d: %w", job.FullName(), number, err)}
		}
		return actionMsg{ok: fmt.Sprintf("Abort requested: %s #%d", job.FullName(), number), refresh: true}
	}
}

func (m Model) pollWatched() tea.Cmd {
	var cmds []tea.Cmd
	for _, name := range m.cfg.Pinned {
		name, c := name, m.client
		cmds = append(cmds, func() tea.Msg {
			cx, cancel := ctx()
			defer cancel()
			path := jenkins.SplitFullName(name)
			b, err := c.LastBuild(cx, path)
			if err == nil && b != nil && b.Building {
				b.Inputs, _ = c.PendingInputs(cx, path, b.Number) // best effort
			}
			return watchResultMsg{name: name, build: b, err: err}
		})
	}
	return tea.Batch(cmds...)
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		if err := notify.Open(url); err != nil {
			return actionMsg{err: fmt.Errorf("open browser: %w", err)}
		}
		return actionMsg{ok: "Opened in browser"}
	}
}

// ---- lifecycle --------------------------------------------------------

// Init starts the animation clock and, when connected, the first load.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), textinput.Blink}
	if m.client != nil {
		cmds = append(cmds, m.connected()...)
	}
	return tea.Batch(cmds...)
}

func (m *Model) connected() []tea.Cmd {
	m.jobs.loading = true
	return []tea.Cmd{m.loadJobs(nil), m.pollWatched(), m.watchTick()}
}

func (m *Model) setStatus(s string, isErr bool) {
	m.status, m.statusErr, m.statusAt = s, isErr, time.Now()
}

func (m *Model) setErr(err error) {
	if errors.Is(err, jenkins.ErrAuth) {
		m.setStatus(err.Error()+" — run with --setup to re-enter it", true)
		return
	}
	m.setStatus(err.Error(), true)
}

// Update routes messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.splash = newSplash(m.w, m.h)
		m.resizeLog()
		return m, nil

	case tickMsg:
		m.frame++
		if m.anim {
			m.cat.step(m.w)
		}
		if m.scr == scrSplash && m.frame >= splashAutoExit {
			m.scr = m.afterSplash()
		}
		if !m.statusAt.IsZero() && !m.statusErr && time.Since(m.statusAt) > 6*time.Second {
			m.status = ""
		}
		var cmd tea.Cmd
		// Keep a running job's history fresh while looking at it.
		if m.scr == scrBuilds && !m.builds.loading && m.frame%50 == 0 && m.anyBuilding() {
			m.builds.loading = true
			cmd = m.loadDetail(m.builds.job)
		}
		return m, tea.Batch(tick(), cmd)

	case wizardDoneMsg:
		if msg.err != nil {
			m.wiz.err = msg.err
			m.wiz.busy = false
			return m, nil
		}
		m.client = msg.client
		m.setStatus("Connected as "+msg.who+" — welcome to the cave.", false)
		m.scr = scrJobs
		return m, tea.Batch(m.connected()...)

	case jobsMsg:
		if !pathEqual(msg.path, m.jobs.path) {
			return m, nil
		}
		m.jobs.loading = false
		m.jobs.err = msg.err
		if msg.err != nil {
			m.setErr(msg.err)
			return m, nil
		}
		m.jobs.items = msg.jobs
		m.jobs.cursor = clamp(m.jobs.cursor, 0, max(0, len(m.visibleJobs())-1))
		return m, nil

	case detailMsg:
		if msg.job.FullName() != m.builds.job.FullName() {
			return m, nil
		}
		m.builds.loading = false
		m.builds.err = msg.err
		if msg.err != nil {
			m.setErr(msg.err)
			return m, nil
		}
		m.builds.detail = msg.detail
		m.builds.noInputAPI = msg.noInputAPI
		m.builds.cursor = clamp(m.builds.cursor, 0, max(0, len(msg.detail.Builds)-1))
		if m.builds.pendingBuild {
			m.builds.pendingBuild = false
			return m.startBuild()
		}
		return m, nil

	case backToBuildsMsg:
		if m.scr == scrParams {
			m.scr = m.params.returnTo
		}
		if m.scr == scrLog {
			m.log.inputs = nil
			return m, m.loadLogInputs(m.log.gen, m.log.job, m.log.number)
		}
		m.builds.loading = true
		return m, m.loadDetail(m.builds.job)

	case logInputsMsg:
		if msg.gen == m.log.gen {
			m.log.inputs = msg.inputs
		}
		return m, nil

	case logMsg:
		if msg.gen != m.log.gen {
			return m, nil
		}
		m.log.loading = false
		if msg.err != nil {
			m.log.err = msg.err
			m.setErr(msg.err)
			return m, nil
		}
		m.log.text.WriteString(cleanLog(msg.chunk.Text))
		m.log.offset = msg.chunk.Next
		m.log.more = msg.chunk.More
		m.log.vp.SetContent(m.log.text.String())
		if m.log.follow {
			m.log.vp.GotoBottom()
		}
		if m.log.more {
			gen := m.log.gen
			return m, tea.Tick(time.Second, func(time.Time) tea.Msg { return logPollMsg{gen: gen} })
		}
		m.log.inputs = nil
		return m, nil

	case logPollMsg:
		if msg.gen != m.log.gen || m.scr != scrLog {
			return m, nil
		}
		m.log.polls++
		cmd := m.loadLog(m.log.gen, m.log.job, m.log.number, m.log.offset)
		if m.log.polls%3 == 0 && m.log.job.IsPipeline() {
			cmd = tea.Batch(cmd, m.loadLogInputs(m.log.gen, m.log.job, m.log.number))
		}
		return m, cmd

	case actionMsg:
		if msg.err != nil {
			m.setErr(msg.err)
		} else if msg.ok != "" {
			m.setStatus(msg.ok, false)
		}
		if msg.refresh && m.scr == scrBuilds {
			m.builds.loading = true
			return m, m.loadDetail(m.builds.job)
		}
		if msg.refresh && m.scr == scrLog {
			m.log.inputs = nil
			return m, m.loadLogInputs(m.log.gen, m.log.job, m.log.number)
		}
		return m, nil

	case watchTickMsg:
		return m, tea.Batch(m.pollWatched(), m.watchTick())

	case watchResultMsg:
		return m, m.handleWatch(msg)

	case notifyFailedMsg:
		// Say it once; the in-app status line still shows every event.
		if !m.notifyWarned {
			m.notifyWarned = true
			m.setStatus("Desktop notification failed: "+strings.ReplaceAll(msg.err.Error(), "\n", ": "), true)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.scr == scrWizard {
		var cmd tea.Cmd
		m.wiz, cmd = m.wiz.update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) handleWatch(msg watchResultMsg) tea.Cmd {
	if msg.err != nil || msg.build == nil || !m.cfg.IsPinned(msg.name) {
		return nil
	}
	b := msg.build
	st := m.watch[msg.name]
	if st == nil {
		st = &watchState{}
		m.watch[msg.name] = st
	}
	finished := !b.Building && st.known &&
		(b.Number > st.number || (b.Number == st.number && st.building))
	st.number, st.building, st.known = b.Number, b.Building, true
	if b.Building {
		return m.notifyInputs(msg.name, st, b)
	}
	if !finished {
		return nil
	}
	title := fmt.Sprintf("%s #%d: %s", msg.name, b.Number, b.Result)
	body := "Finished in " + fmtDur(b.Duration)
	switch b.Status() {
	case jenkins.StatusSuccess:
		body = "Gotham is safe. " + body
		m.setStatus("✔ "+title, false)
	default:
		body = "The Joker struck. " + body
		m.setStatus("✘ "+title, true)
	}
	return sendNotification("🦇 "+title, body)
}

// notifyInputs tells about each input step once, even on the first poll:
// a paused deploy is waiting on a human, however long it has been.
func (m *Model) notifyInputs(name string, st *watchState, b *jenkins.Build) tea.Cmd {
	if st.asked == nil {
		st.asked = map[string]bool{}
	}
	var cmds []tea.Cmd
	for _, in := range b.Inputs {
		k := fmt.Sprintf("%d/%s", b.Number, in.ID)
		if st.asked[k] {
			continue
		}
		st.asked[k] = true
		title := fmt.Sprintf("%s #%d needs you", name, b.Number)
		m.setStatus("⏸ "+title+": "+in.Message, false)
		msg := in.Message
		cmds = append(cmds, sendNotification("🦇 "+title, msg+" — open jenklod-batman and press i"))
	}
	return tea.Batch(cmds...)
}

type notifyFailedMsg struct{ err error }

func sendNotification(title, body string) tea.Cmd {
	return func() tea.Msg {
		if err := notify.Send(title, body); err != nil {
			return notifyFailedMsg{err}
		}
		return nil
	}
}

func (m Model) anyBuilding() bool {
	if m.builds.detail == nil {
		return false
	}
	for _, b := range m.builds.detail.Builds {
		if b.Building {
			return true
		}
	}
	return false
}

// ---- keys -------------------------------------------------------------

func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.scr == scrSplash {
		m.scr = m.afterSplash()
		return m, nil
	}
	if m.scr == scrWizard {
		var cmd tea.Cmd
		m.wiz, cmd = m.wiz.update(k)
		return m, cmd
	}
	if m.confirm != nil {
		switch k.String() {
		case "y", "Y":
			cmd := m.confirm.onYes
			m.confirm = nil
			return m, cmd
		case "n", "N", "esc", "q":
			m.confirm = nil
			m.setStatus("Cancelled", false)
		}
		return m, nil
	}
	if m.help {
		m.help = false
		return m, nil
	}
	if m.scr == scrJobs && m.jobs.filtering {
		return m.filterKey(k)
	}
	if m.scr == scrParams {
		return m.paramsKey(k)
	}

	switch k.String() {
	case "?":
		m.help = true
		return m, nil
	case "a":
		m.anim = !m.anim
		return m, nil
	case "q":
		if m.scr == scrJobs {
			return m, tea.Quit
		}
		return m.back()
	}

	switch m.scr {
	case scrJobs:
		return m.jobsKey(k)
	case scrBuilds:
		return m.buildsKey(k)
	case scrLog:
		return m.logKey(k)
	}
	return m, nil
}

func (m Model) back() (tea.Model, tea.Cmd) {
	switch m.scr {
	case scrLog:
		m.log.gen++ // stop polling
		m.scr = scrBuilds
		m.builds.loading = true
		return m, m.loadDetail(m.builds.job)
	case scrParams:
		m.scr = m.params.returnTo
	case scrBuilds:
		m.scr = scrJobs
	case scrJobs:
		if len(m.jobs.path) == 0 {
			return m, nil
		}
		m.jobs.path = m.jobs.path[:len(m.jobs.path)-1]
		m.jobs.cursor = 0
		if n := len(m.jobs.cursors); n > 0 {
			m.jobs.cursor = m.jobs.cursors[n-1]
			m.jobs.cursors = m.jobs.cursors[:n-1]
		}
		m.jobs.items = nil
		m.jobs.loading = true
		m.clearFilter()
		return m, m.loadJobs(m.jobs.path)
	}
	return m, nil
}

func (m *Model) clearFilter() {
	m.jobs.filter.SetValue("")
	m.jobs.filter.Blur()
	m.jobs.filtering = false
}

func (m Model) visibleJobs() []jenkins.Job {
	q := strings.ToLower(m.jobs.filter.Value())
	if q == "" {
		return m.jobs.items
	}
	var out []jenkins.Job
	for _, j := range m.jobs.items {
		if strings.Contains(strings.ToLower(j.Name), q) {
			out = append(out, j)
		}
	}
	return out
}

func (m Model) selectedJob() (jenkins.Job, bool) {
	v := m.visibleJobs()
	if m.jobs.cursor < 0 || m.jobs.cursor >= len(v) {
		return jenkins.Job{}, false
	}
	return v[m.jobs.cursor], true
}

func (m Model) filterKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.clearFilter()
		m.jobs.cursor = 0
		return m, nil
	case "enter":
		m.jobs.filtering = false
		m.jobs.filter.Blur()
		return m, nil
	case "up", "down":
		return m.jobsKey(k)
	}
	var cmd tea.Cmd
	m.jobs.filter, cmd = m.jobs.filter.Update(k)
	m.jobs.cursor = 0
	return m, cmd
}

func (m Model) listHeight() int { return max(1, m.h-6) }

func moveCursor(cur, n int, key string, page int) int {
	switch key {
	case "j", "down":
		cur++
	case "k", "up":
		cur--
	case "g", "home":
		cur = 0
	case "G", "end":
		cur = n - 1
	case "ctrl+d", "pgdown":
		cur += page / 2
	case "ctrl+u", "pgup":
		cur -= page / 2
	}
	return clamp(cur, 0, max(0, n-1))
}

func (m Model) jobsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	switch key {
	case "j", "k", "down", "up", "g", "G", "home", "end", "ctrl+d", "ctrl+u", "pgdown", "pgup":
		m.jobs.cursor = moveCursor(m.jobs.cursor, len(m.visibleJobs()), key, m.listHeight())
		return m, nil
	case "/":
		m.jobs.filtering = true
		m.jobs.filter.Focus()
		return m, textinput.Blink
	case "esc":
		if m.jobs.filter.Value() != "" {
			m.clearFilter()
			return m, nil
		}
		return m.back()
	case "h", "left", "backspace":
		return m.back()
	case "r":
		m.jobs.loading = true
		return m, m.loadJobs(m.jobs.path)
	}

	job, ok := m.selectedJob()
	if !ok {
		return m, nil
	}
	switch key {
	case "l", "right", "enter":
		if job.IsFolder() {
			m.jobs.cursors = append(m.jobs.cursors, m.jobs.cursor)
			m.jobs.path = job.Path
			m.jobs.cursor = 0
			m.jobs.items = nil
			m.jobs.loading = true
			m.clearFilter()
			return m, m.loadJobs(job.Path)
		}
		return m.openBuilds(job)
	case "o":
		return m, openURL(m.client.BrowserURL(job.Path, 0))
	case "w":
		if job.IsFolder() {
			m.setStatus("Folders cannot be watched — pin a job inside it", true)
			return m, nil
		}
		return m.togglePin(job)
	case "b":
		if job.IsFolder() {
			return m, nil
		}
		// Parameters are only known from the job detail; fetch then build.
		m2, cmd := m.openBuilds(job)
		mm := m2.(Model)
		mm.builds.pendingBuild = true
		return mm, cmd
	}
	return m, nil
}

func (m Model) togglePin(job jenkins.Job) (tea.Model, tea.Cmd) {
	on := m.cfg.TogglePin(job.FullName())
	if err := m.cfg.Save(); err != nil {
		m.setErr(fmt.Errorf("saving pins: %w", err))
		return m, nil
	}
	if on {
		m.setStatus("👁  Watching "+job.FullName(), false)
		return m, m.pollWatched()
	}
	delete(m.watch, job.FullName())
	m.setStatus("Stopped watching "+job.FullName(), false)
	return m, nil
}

func (m Model) openBuilds(job jenkins.Job) (tea.Model, tea.Cmd) {
	if m.builds.job.FullName() != job.FullName() {
		m.builds = buildsState{job: job}
	}
	m.builds.job = job
	m.builds.loading = true
	m.scr = scrBuilds
	return m, m.loadDetail(job)
}

func (m Model) buildsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	n := 0
	if m.builds.detail != nil {
		n = len(m.builds.detail.Builds)
	}
	switch key {
	case "j", "k", "down", "up", "g", "G", "home", "end", "ctrl+d", "ctrl+u", "pgdown", "pgup":
		m.builds.cursor = moveCursor(m.builds.cursor, n, key, m.listHeight())
		return m, nil
	case "h", "left", "esc", "backspace":
		return m.back()
	case "r":
		m.builds.loading = true
		return m, m.loadDetail(m.builds.job)
	case "w":
		return m.togglePin(m.builds.job)
	case "b":
		return m.startBuild()
	}
	b, ok := m.selectedBuild()
	switch key {
	case "o":
		num := 0
		if ok {
			num = b.Number
		}
		return m, openURL(m.client.BrowserURL(m.builds.job.Path, num))
	case "l", "right", "enter":
		if ok {
			return m.openLog(b.Number)
		}
	case "x":
		if ok && b.Building {
			return m.askAbort(b.Number)
		}
		m.setStatus("That build is not running", true)
	case "i":
		if m.builds.noInputAPI {
			m.setStatus("Can't list input steps: Jenkins lacks the Pipeline Stage View plugin — press o to answer in the browser", true)
			return m, nil
		}
		// Prefer the selected build, else the newest one that is waiting.
		if !ok || len(b.Inputs) == 0 {
			b, ok = m.waitingBuild()
		}
		if !ok {
			m.setStatus("No build is waiting for input", true)
			return m, nil
		}
		return m.openInput(m.builds.job, b.Number, b.Inputs[0], scrBuilds)
	}
	return m, nil
}

func (m Model) waitingBuild() (jenkins.Build, bool) {
	if m.builds.detail != nil {
		for _, b := range m.builds.detail.Builds {
			if len(b.Inputs) > 0 {
				return b, true
			}
		}
	}
	return jenkins.Build{}, false
}

func (m Model) openInput(job jenkins.Job, number int, in jenkins.InputRequest, from screen) (tea.Model, tea.Cmd) {
	m.params = newParams(job, in.Params)
	m.params.input = &in
	m.params.number = number
	m.params.returnTo = from
	m.scr = scrParams
	return m, textinput.Blink
}

func (m Model) selectedBuild() (jenkins.Build, bool) {
	d := m.builds.detail
	if d == nil || m.builds.cursor >= len(d.Builds) {
		return jenkins.Build{}, false
	}
	return d.Builds[m.builds.cursor], true
}

func (m Model) askAbort(number int) (tea.Model, tea.Cmd) {
	job := m.builds.job
	m.confirm = &confirmState{
		prompt: fmt.Sprintf("Abort %s #%d?", job.FullName(), number),
		onYes:  m.abort(job, number),
	}
	return m, nil
}

func (m Model) startBuild() (tea.Model, tea.Cmd) {
	d, job := m.builds.detail, m.builds.job
	if d == nil {
		return m, nil
	}
	if !d.Buildable {
		m.setStatus(job.FullName()+" is not buildable (disabled?)", true)
		return m, nil
	}
	if len(d.Params) == 0 {
		m.confirm = &confirmState{
			prompt: "Start a build of " + job.FullName() + "?",
			onYes:  m.trigger(job, nil),
		}
		return m, nil
	}
	m.params = newParams(job, d.Params)
	m.params.returnTo = scrBuilds
	m.scr = scrParams
	return m, textinput.Blink
}

func (m Model) openLog(number int) (tea.Model, tea.Cmd) {
	m.log.gen++
	m.log.job = m.builds.job
	m.log.number = number
	m.log.text = &strings.Builder{}
	m.log.offset = 0
	m.log.more = false
	m.log.err = nil
	m.log.follow = true
	m.log.loading = true
	m.resizeLog()
	m.log.vp.SetContent("")
	m.log.inputs = nil
	m.log.polls = 0
	m.scr = scrLog
	cmd := m.loadLog(m.log.gen, m.log.job, number, 0)
	if m.log.job.IsPipeline() {
		cmd = tea.Batch(cmd, m.loadLogInputs(m.log.gen, m.log.job, number))
	}
	return m, cmd
}

func (m *Model) resizeLog() {
	w, h := max(1, m.w), max(1, m.h-5)
	if m.log.vp.Width == 0 {
		m.log.vp = viewport.New(w, h)
		return
	}
	m.log.vp.Width, m.log.vp.Height = w, h
}

func (m Model) logKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "h", "left", "esc", "backspace":
		return m.back()
	case "f", "F":
		m.log.follow = !m.log.follow
		if m.log.follow {
			m.log.vp.GotoBottom()
		}
		return m, nil
	case "G", "end":
		m.log.follow = true
		m.log.vp.GotoBottom()
		return m, nil
	case "g", "home":
		m.log.follow = false
		m.log.vp.GotoTop()
		return m, nil
	case "o":
		return m, openURL(m.client.BrowserURL(m.log.job.Path, m.log.number) + "console")
	case "i":
		if len(m.log.inputs) == 0 {
			m.setStatus("This build is not waiting for input", true)
			return m, nil
		}
		return m.openInput(m.log.job, m.log.number, m.log.inputs[0], scrLog)
	case "x":
		if m.log.more {
			job, num := m.log.job, m.log.number
			m.confirm = &confirmState{
				prompt: fmt.Sprintf("Abort %s #%d?", job.FullName(), num),
				onYes:  m.abort(job, num),
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.log.vp, cmd = m.log.vp.Update(k)
	if !m.log.vp.AtBottom() {
		m.log.follow = false
	}
	return m, cmd
}

// ---- params form ------------------------------------------------------

func newParams(job jenkins.Job, defs []jenkins.Param) paramsState {
	p := paramsState{job: job}
	for _, d := range defs {
		f := paramField{def: d}
		switch d.Type {
		case jenkins.ParamBool:
			f.on = d.Default == "true"
		case jenkins.ParamChoice:
			for i, c := range d.Choices {
				if c == d.Default {
					f.choice = i
				}
			}
		default:
			ti := textinput.New()
			ti.Prompt = ""
			ti.SetValue(d.Default)
			ti.CharLimit = 0
			ti.Width = 50
			if d.Type == jenkins.ParamPassword {
				ti.EchoMode = textinput.EchoPassword
				ti.EchoCharacter = '•'
			}
			f.input = ti
		}
		p.fields = append(p.fields, f)
	}
	p.focusField(0)
	return p
}

func (p *paramsState) focusField(i int) {
	p.focus = clamp(i, 0, len(p.fields)-1)
	for j := range p.fields {
		switch p.fields[j].def.Type {
		case jenkins.ParamBool, jenkins.ParamChoice:
			continue // no text input behind these
		}
		if j == p.focus {
			p.fields[j].input.Focus()
		} else {
			p.fields[j].input.Blur()
		}
	}
}

func (p paramsState) values() map[string]string {
	out := map[string]string{}
	for _, f := range p.fields {
		switch f.def.Type {
		case jenkins.ParamBool:
			out[f.def.Name] = fmt.Sprint(f.on)
		case jenkins.ParamChoice:
			if len(f.def.Choices) > 0 {
				out[f.def.Name] = f.def.Choices[f.choice]
			}
		default:
			out[f.def.Name] = f.input.Value()
		}
	}
	return out
}

func (m Model) paramsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m.params
	n := len(p.fields)
	switch k.String() {
	case "esc":
		m.scr = p.returnTo
		m.setStatus("Cancelled", false)
		return m, nil
	case "tab", "down":
		if n > 0 {
			p.focusField((p.focus + 1) % n)
		}
		return m, nil
	case "shift+tab", "up":
		if n > 0 {
			p.focusField((p.focus - 1 + n) % n)
		}
		return m, nil
	case "enter", "ctrl+s":
		return m.submitParams()
	case "ctrl+x":
		return m.rejectInput()
	}
	if n == 0 {
		if k.String() == "x" {
			return m.rejectInput()
		}
		return m, nil
	}
	f := &p.fields[p.focus]
	switch f.def.Type {
	case jenkins.ParamBool:
		switch k.String() {
		case " ", "left", "right":
			f.on = !f.on
		case "x":
			return m.rejectInput()
		}
		return m, nil
	case jenkins.ParamChoice:
		c := len(f.def.Choices)
		switch k.String() {
		case "x":
			return m.rejectInput()
		case "left", "h":
			if c > 0 {
				f.choice = (f.choice - 1 + c) % c
			}
		case "right", "l", " ":
			if c > 0 {
				f.choice = (f.choice + 1) % c
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(k)
	return m, cmd
}

func (m Model) submitParams() (tea.Model, tea.Cmd) {
	p := m.params
	job, vals := p.job, p.values()
	back := func() tea.Msg { return backToBuildsMsg{} }
	if in := p.input; in != nil {
		m.confirm = &confirmState{
			prompt: fmt.Sprintf("%s #%d asks: %s\n\n→ %s?", job.FullName(), p.number, in.Message, in.ProceedText),
			onYes:  tea.Sequence(m.proceedInput(job, p.number, *in, vals), back),
		}
		return m, nil
	}
	m.confirm = &confirmState{
		prompt: fmt.Sprintf("Start %s with these %d parameters?", job.FullName(), len(vals)),
		onYes:  tea.Sequence(m.trigger(job, vals), back),
	}
	return m, nil
}

// rejectInput answers "Abort" to an input step, which aborts the build.
func (m Model) rejectInput() (tea.Model, tea.Cmd) {
	p := m.params
	if p.input == nil {
		return m, nil
	}
	m.confirm = &confirmState{
		prompt: fmt.Sprintf("Abort %s #%d at: %s?", p.job.FullName(), p.number, p.input.Message),
		onYes: tea.Sequence(m.abortInput(p.job, p.number, *p.input), func() tea.Msg {
			return backToBuildsMsg{}
		}),
	}
	return m, nil
}

// backToBuildsMsg closes the form and refreshes the screen it came from.
type backToBuildsMsg struct{}

// ---- helpers ----------------------------------------------------------

func pathEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Jenkins console notes (hidden "ha:..." annotations) and carriage returns
// make a mess of a terminal viewport.
var consoleNote = regexp.MustCompile("\x1b\\[8mha:[^\x1b]*\x1b\\[0m")

func cleanLog(s string) string {
	s = consoleNote.ReplaceAllString(s, "")
	return strings.ReplaceAll(s, "\r", "")
}

func fmtDur(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	d = d.Round(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%02dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func fmtAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
