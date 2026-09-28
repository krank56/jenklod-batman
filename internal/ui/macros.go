package ui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
	"github.com/krank56/jenklod-batman/internal/macro"
)

// macrosState is the macro picker (m) and, while open, its editor.
type macrosState struct {
	cursor   int
	returnTo screen
	edit     *macroEdit
}

type editMode int

const (
	editSteps   editMode = iota
	editName             // typing the macro name
	editKind             // choosing the type of a new step
	editPick             // choosing its job
	editTimeout          // typing a wait/input timeout
)

type macroEdit struct {
	orig   string // name before editing; "" for a new macro
	name   textinput.Model
	steps  []config.Step
	cursor int
	mode   editMode
	dirty  bool

	kind       string // step kind being added
	pick       textinput.Model
	pickCursor int
	timeout    textinput.Model
}

// macroRun is the macro running in the background. Only Update touches
// it; the runner goroutine talks through events.
type macroRun struct {
	name        string
	step, total int
	lines       []string
	done        bool
	err         error
	cancel      context.CancelFunc
	events      chan tea.Msg
}

type macroEventMsg struct {
	run *macroRun
	ev  macro.Event
}

type macroDoneMsg struct {
	run *macroRun
	err error
}

// stepDetailMsg carries the parameter definitions of a build step's job.
type stepDetailMsg struct {
	job    jenkins.Job
	step   int
	detail *jenkins.Detail
	err    error
}

// macroActionMsg is what a y/n confirmation on the macro screens does.
type macroActionMsg struct {
	action string // "run", "delete", "discard"
	name   string
}

func waitMacro(r *macroRun) tea.Cmd { return func() tea.Msg { return <-r.events } }

func (m Model) openMacros() (tea.Model, tea.Cmd) {
	m.macros.returnTo = m.scr
	m.macros.cursor = clamp(m.macros.cursor, 0, max(0, len(m.cfg.Macros())-1))
	m.scr = scrMacros
	return m, nil
}

func (m Model) leaveMacros() (tea.Model, tea.Cmd) {
	m.scr = m.macros.returnTo
	switch m.scr {
	case scrLog:
		// Its polling stopped while hidden.
		if m.log.more {
			gen := m.log.gen
			return m, func() tea.Msg { return logPollMsg{gen: gen} }
		}
	case scrBuilds:
		m.builds.loading = true
		return m, m.loadDetail(m.builds.job)
	case scrParams, scrMacros, scrMacroEdit:
		m.scr = scrJobs
	}
	return m, nil
}

func (m Model) selectedMacro() (config.Macro, bool) {
	ms := m.cfg.Macros()
	if m.macros.cursor < 0 || m.macros.cursor >= len(ms) {
		return config.Macro{}, false
	}
	return ms[m.macros.cursor], true
}

func (m Model) macrosKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	switch key {
	case "j", "k", "down", "up", "g", "G", "home", "end", "ctrl+d", "ctrl+u", "pgdown", "pgup":
		m.macros.cursor = moveCursor(m.macros.cursor, len(m.cfg.Macros()), key, m.listHeight())
		return m, nil
	case "h", "left", "esc", "backspace":
		return m.leaveMacros()
	case "n":
		m.macros.edit = newMacroEdit(config.Macro{})
		m.scr = scrMacroEdit
		return m, textinput.Blink
	case "c":
		if m.run == nil || m.run.done {
			m.setStatus("No macro is running", true)
			return m, nil
		}
		m.run.cancel()
		m.setStatus("Cancelling "+m.run.name+"… builds already started keep running", false)
		return m, nil
	}
	mac, ok := m.selectedMacro()
	if !ok {
		return m, nil
	}
	switch key {
	case "enter", "l", "right":
		if m.run != nil && !m.run.done {
			m.setStatus(m.run.name+" is still running — press c to cancel it first", true)
			return m, nil
		}
		m.confirm = &confirmState{
			prompt: "Run macro " + mac.Name + "?\n\n" + stepList(mac.Steps),
			onYes:  func() tea.Msg { return macroActionMsg{action: "run", name: mac.Name} },
		}
	case "e":
		m.macros.edit = newMacroEdit(mac)
		m.scr = scrMacroEdit
	case "d":
		m.confirm = &confirmState{
			prompt: "Delete macro " + mac.Name + "?",
			onYes:  func() tea.Msg { return macroActionMsg{action: "delete", name: mac.Name} },
		}
	}
	return m, nil
}

func stepList(steps []config.Step) string {
	var b strings.Builder
	for i, s := range steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) handleMacroMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case macroEventMsg:
		r := msg.run
		r.step = msg.ev.Step
		r.lines = append(r.lines, fmt.Sprintf("%d/%d %s", msg.ev.Step, msg.ev.Total, msg.ev.Text))
		if n := len(r.lines); n > 50 {
			r.lines = r.lines[n-50:]
		}
		if r == m.run {
			m.setStatus(fmt.Sprintf("▶ %s %d/%d: %s", r.name, r.step, r.total, msg.ev.Text), false)
		}
		return m, waitMacro(r)

	case macroDoneMsg:
		r := msg.run
		r.done, r.err = true, msg.err
		if r != m.run {
			return m, nil
		}
		if msg.err != nil {
			m.setErr(fmt.Errorf("macro %s failed at %w", r.name, msg.err))
		} else {
			m.setStatus("✔ Macro "+r.name+" done", false)
		}
		if m.scr == scrBuilds {
			m.builds.loading = true
			return m, tea.Batch(m.loadDetail(m.builds.job), m.pollWatched())
		}
		return m, m.pollWatched()

	case macroActionMsg:
		switch msg.action {
		case "run":
			mac, ok := m.cfg.Macro(msg.name)
			if !ok {
				return m, nil
			}
			return m.startMacro(mac)
		case "delete":
			prev := m.cfg.Macros()
			var next []config.Macro
			for _, mac := range prev {
				if mac.Name != msg.name {
					next = append(next, mac)
				}
			}
			m.cfg.SetMacros(next)
			if err := m.cfg.Save(); err != nil {
				m.cfg.SetMacros(prev)
				m.setErr(fmt.Errorf("saving macros: %w", err))
				return m, nil
			}
			m.macros.cursor = clamp(m.macros.cursor, 0, max(0, len(next)-1))
			m.setStatus("Deleted macro "+msg.name, false)
		case "discard":
			m.macros.edit = nil
			m.scr = scrMacros
		}
		return m, nil

	case stepDetailMsg:
		e := m.macros.edit
		if e == nil || m.scr != scrMacroEdit {
			return m, nil
		}
		if msg.err != nil {
			m.setErr(msg.err)
			return m, nil
		}
		return m.editBuildParams(msg)
	}
	return m, nil
}

func (m Model) startMacro(mac config.Macro) (tea.Model, tea.Cmd) {
	cx, cancel := context.WithCancel(context.Background())
	r := &macroRun{name: mac.Name, total: len(mac.Steps), cancel: cancel, events: make(chan tea.Msg, 64)}
	m.run = r
	runner := &macro.Runner{
		Client: m.client,
		Poll:   m.macroPoll,
		Report: func(e macro.Event) { r.events <- macroEventMsg{run: r, ev: e} },
	}
	go func() {
		err := runner.Run(cx, mac, nil)
		cancel()
		r.events <- macroDoneMsg{run: r, err: err}
	}()
	m.setStatus("▶ Running macro "+mac.Name, false)
	return m, waitMacro(r)
}

// ---- editor -----------------------------------------------------------

func newMacroEdit(mac config.Macro) *macroEdit {
	name := textinput.New()
	name.Prompt = ""
	name.Placeholder = "deploy-prod"
	name.CharLimit = 60
	name.SetValue(mac.Name)
	pick := textinput.New()
	pick.Prompt = "job: "
	pick.Placeholder = "fuzzy search"
	to := textinput.New()
	to.Prompt = "timeout (minutes): "
	to.CharLimit = 5
	e := &macroEdit{orig: mac.Name, name: name, steps: slices.Clone(mac.Steps), pick: pick, timeout: to}
	if mac.Name == "" {
		e.mode = editName
		e.name.Focus()
	}
	return e
}

func (m Model) macroEditKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.macros.edit
	key := k.String()
	switch e.mode {
	case editName:
		switch key {
		case "enter", "tab", "esc":
			e.mode = editSteps
			e.name.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		e.name, cmd = e.name.Update(k)
		e.dirty = true
		return m, cmd

	case editKind:
		kinds := map[string]string{"b": config.StepBuild, "x": config.StepAbort, "w": config.StepWait, "i": config.StepInput}
		if kind, ok := kinds[key]; ok {
			e.kind = kind
			e.mode = editPick
			e.pick.SetValue("")
			e.pickCursor = 0
			e.pick.Focus()
			if m.index == nil && !m.indexLoading {
				m.indexLoading = true
				return m, tea.Batch(textinput.Blink, m.loadIndex())
			}
			return m, textinput.Blink
		}
		if key == "esc" {
			e.mode = editSteps
		}
		return m, nil

	case editPick:
		cands := m.pickCandidates()
		switch key {
		case "esc":
			e.mode = editSteps
			e.pick.Blur()
			return m, nil
		case "up", "ctrl+p":
			e.pickCursor = clamp(e.pickCursor-1, 0, max(0, len(cands)-1))
			return m, nil
		case "down", "ctrl+n", "tab":
			e.pickCursor = clamp(e.pickCursor+1, 0, max(0, len(cands)-1))
			return m, nil
		case "enter":
			if e.pickCursor >= len(cands) {
				return m, nil
			}
			job := cands[e.pickCursor]
			e.mode = editSteps
			e.pick.Blur()
			if e.kind == config.StepBuild {
				return m, m.loadStepDetail(job, -1)
			}
			e.steps = append(e.steps, config.Step{Kind: e.kind, Job: job.FullName()})
			e.cursor = len(e.steps) - 1
			e.dirty = true
			return m, nil
		}
		var cmd tea.Cmd
		e.pick, cmd = e.pick.Update(k)
		e.pickCursor = 0
		return m, cmd

	case editTimeout:
		switch key {
		case "esc":
			e.mode = editSteps
			e.timeout.Blur()
			return m, nil
		case "enter":
			n, err := strconv.Atoi(strings.TrimSpace(e.timeout.Value()))
			if err != nil || n <= 0 {
				m.setStatus("The timeout is a whole number of minutes", true)
				return m, nil
			}
			e.steps[e.cursor].TimeoutMinutes = n
			e.mode = editSteps
			e.timeout.Blur()
			e.dirty = true
			return m, nil
		}
		var cmd tea.Cmd
		e.timeout, cmd = e.timeout.Update(k)
		return m, cmd
	}

	// editSteps
	n := len(e.steps)
	switch key {
	case "j", "k", "down", "up", "g", "G", "home", "end":
		e.cursor = moveCursor(e.cursor, n, key, m.listHeight())
		return m, nil
	case "a":
		e.mode = editKind
		return m, nil
	case "r":
		e.mode = editName
		e.name.Focus()
		return m, textinput.Blink
	case "ctrl+s":
		return m.saveMacro()
	case "esc", "q":
		if !e.dirty {
			m.macros.edit = nil
			m.scr = scrMacros
			return m, nil
		}
		m.confirm = &confirmState{
			prompt: "Discard your changes to this macro?",
			onYes:  func() tea.Msg { return macroActionMsg{action: "discard"} },
		}
		return m, nil
	case "?":
		m.help = true
		return m, nil
	}
	if e.cursor >= n {
		return m, nil
	}
	s := &e.steps[e.cursor]
	switch key {
	case "J", "K":
		j := e.cursor + 1
		if key == "K" {
			j = e.cursor - 1
		}
		if j >= 0 && j < n {
			e.steps[e.cursor], e.steps[j] = e.steps[j], e.steps[e.cursor]
			e.cursor = j
			e.dirty = true
		}
	case "d", "x":
		e.steps = slices.Delete(e.steps, e.cursor, e.cursor+1)
		e.cursor = clamp(e.cursor, 0, max(0, len(e.steps)-1))
		e.dirty = true
	case " ":
		if s.Kind == config.StepInput {
			s.Abort = !s.Abort
			e.dirty = true
		}
	case "enter", "t":
		switch s.Kind {
		case config.StepBuild:
			if key == "enter" {
				return m, m.loadStepDetail(m.pinJob(s.Job), e.cursor)
			}
		case config.StepWait, config.StepInput:
			e.mode = editTimeout
			e.timeout.SetValue(strconv.Itoa(s.Timeout()))
			e.timeout.CursorEnd()
			e.timeout.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

// pickCandidates are the jobs (not folders) matching the picker query.
func (m Model) pickCandidates() []jenkins.Job {
	var jobs []jenkins.Job
	for _, j := range m.index {
		if !j.IsFolder() {
			jobs = append(jobs, j)
		}
	}
	q := m.macros.edit.pick.Value()
	if q == "" {
		return jobs
	}
	return rankJobs(q, jobs, jenkins.Job.FullName)
}

func (m Model) loadStepDetail(job jenkins.Job, step int) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		d, err := c.JobDetail(cx, job.Path)
		return stepDetailMsg{job: job, step: step, detail: d, err: err}
	}
}

// editBuildParams opens the params form for a build step, pre-filled with
// the step's saved values. Passwords are left out: they would be saved in
// clear text, so Jenkins uses their default instead.
func (m Model) editBuildParams(msg stepDetailMsg) (tea.Model, tea.Cmd) {
	e := m.macros.edit
	var saved map[string]string
	if msg.step >= 0 && msg.step < len(e.steps) {
		saved = e.steps[msg.step].Params
	}
	var defs []jenkins.Param
	skipped := 0
	for _, d := range msg.detail.Params {
		if d.Type == jenkins.ParamPassword {
			skipped++
			continue
		}
		if v, ok := saved[d.Name]; ok {
			d.Default = v
		}
		defs = append(defs, d)
	}
	switch {
	case len(msg.detail.Params) == 0:
		return m.saveBuildStep(msg.job, msg.step, nil)
	case len(defs) == 0: // only passwords: still buildWithParameters
		return m.saveBuildStep(msg.job, msg.step, map[string]string{})
	}
	m.params = newParams(msg.job, defs)
	m.params.forMacro = true
	m.params.macroStep = msg.step
	m.params.skippedPass = skipped
	m.params.returnTo = scrMacroEdit
	m.scr = scrParams
	return m, textinput.Blink
}

// saveBuildStep stores a build step at index step, or appends it when step
// is -1. nil vals means the job takes no parameters.
func (m Model) saveBuildStep(job jenkins.Job, step int, vals map[string]string) (tea.Model, tea.Cmd) {
	e := m.macros.edit
	s := config.Step{Kind: config.StepBuild, Job: job.FullName(), WithParams: vals != nil, Params: vals}
	if step >= 0 && step < len(e.steps) {
		e.steps[step] = s
		e.cursor = step
	} else {
		e.steps = append(e.steps, s)
		e.cursor = len(e.steps) - 1
	}
	e.dirty = true
	m.params = paramsState{}
	m.scr = scrMacroEdit
	return m, nil
}

func (m Model) saveMacro() (tea.Model, tea.Cmd) {
	e := m.macros.edit
	name := strings.TrimSpace(e.name.Value())
	switch {
	case name == "":
		m.setStatus("Give the macro a name first (r)", true)
		return m, nil
	case strings.ContainsAny(name, " \t="):
		m.setStatus("Macro names can't contain spaces or '=' (they're typed on the command line)", true)
		return m, nil
	case len(e.steps) == 0:
		m.setStatus("Add at least one step (a)", true)
		return m, nil
	}
	prev := m.cfg.Macros()
	var next []config.Macro
	replaced := false
	for _, mac := range prev {
		switch mac.Name {
		case e.orig:
			if e.orig != "" {
				next = append(next, config.Macro{Name: name, Steps: e.steps})
				replaced = true
				continue
			}
		case name:
			m.setStatus("A macro called "+name+" already exists", true)
			return m, nil
		}
		next = append(next, mac)
	}
	if !replaced {
		next = append(next, config.Macro{Name: name, Steps: e.steps})
	}
	m.cfg.SetMacros(next)
	if err := m.cfg.Save(); err != nil {
		m.cfg.SetMacros(prev)
		m.setErr(fmt.Errorf("saving macros: %w", err))
		return m, nil
	}
	m.macros.edit = nil
	m.scr = scrMacros
	for i, mac := range next {
		if mac.Name == name {
			m.macros.cursor = i
		}
	}
	m.setStatus("Saved macro "+name+" — run it here, or: jenklod-batman run "+name, false)
	return m, nil
}
