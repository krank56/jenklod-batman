package ui

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/krank56/jenklod-batman/internal/jenkins"
)

// pastBuilds is how many finished builds are read for the last answer.
const pastBuilds = 3

// armed is an answer given ahead of time for a running build's next input
// step. It lives as long as the TUI does.
type armed struct {
	job     jenkins.Job
	number  int
	proceed bool
	expect  string // the message the step must ask, "" for any
	label   string // the proceed text last seen, for display
}

func armKey(name string, number int) string { return fmt.Sprintf("%s#%d", name, number) }

func (a *armed) answer() string {
	if !a.proceed {
		return "Abort"
	}
	return cmp.Or(a.label, "Proceed")
}

// armForm is the dialog i opens on a build that has not paused yet.
type armForm struct {
	job      jenkins.Job
	number   int
	loading  bool
	past     *jenkins.PastInput // nil: none found in the last builds
	err      error
	proceed  bool
	existing bool // already armed: d disarms
}

type pastInputMsg struct {
	key   string
	past  jenkins.PastInput
	found bool
	err   error
}

type armedResultMsg struct {
	key   string
	build *jenkins.Build
	err   error
}

type autoAnsweredMsg struct {
	a   armed
	in  jenkins.InputRequest
	err error
}

func (m Model) loadPastInput(job jenkins.Job, number int) tea.Cmd {
	c, key := m.client, armKey(job.FullName(), number)
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		d, err := c.JobDetail(cx, job.Path)
		if err != nil {
			return pastInputMsg{key: key, err: err}
		}
		var nums []int
		for _, b := range d.Builds {
			if !b.Building && b.Number != number && len(nums) < pastBuilds {
				nums = append(nums, b.Number)
			}
		}
		p, ok, err := c.LastInput(cx, job.Path, nums)
		return pastInputMsg{key: key, past: p, found: ok, err: err}
	}
}

// openArm is i on a running build that is not waiting yet.
func (m Model) openArm(job jenkins.Job, number int) (tea.Model, tea.Cmd) {
	if !job.IsPipeline() {
		m.setStatus(job.FullName()+" is not a pipeline: it never pauses for input", true)
		return m, nil
	}
	f := &armForm{job: job, number: number, proceed: true}
	if a := m.armed[armKey(job.FullName(), number)]; a != nil {
		f.existing, f.proceed = true, a.proceed
		if a.expect != "" {
			f.past = &jenkins.PastInput{Message: a.expect, ProceedText: a.label}
		}
		m.arm = f
		return m, nil
	}
	f.loading = true
	m.arm = f
	return m, m.loadPastInput(job, number)
}

func (m Model) handlePastInput(msg pastInputMsg) (tea.Model, tea.Cmd) {
	f := m.arm
	if f == nil || f.existing || armKey(f.job.FullName(), f.number) != msg.key {
		return m, nil
	}
	f.loading, f.err = false, msg.err
	if msg.found {
		p := msg.past
		f.past, f.proceed = &p, p.Approved
	}
	return m, nil
}

func (m Model) armKeyPress(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.arm
	switch k.String() {
	case "esc", "q":
		m.arm = nil
		m.setStatus("Cancelled", false)
	case "left", "right", "h", "l", "tab", "shift+tab", " ":
		f.proceed = !f.proceed
	case "d":
		if f.existing {
			delete(m.armed, armKey(f.job.FullName(), f.number))
			m.arm = nil
			m.setStatus(fmt.Sprintf("Disarmed %s #%d: its input will wait for you", f.job.FullName(), f.number), false)
		}
	case "enter":
		if f.loading {
			m.setStatus("Still reading the last builds…", false)
			return m, nil
		}
		a := &armed{job: f.job, number: f.number, proceed: f.proceed}
		if f.past != nil {
			a.expect, a.label = f.past.Message, f.past.ProceedText
		}
		m.armed[armKey(f.job.FullName(), f.number)] = a
		m.arm = nil
		at := "at its first input"
		if a.expect != "" {
			at = fmt.Sprintf("if it asks %q", a.expect)
		}
		m.setStatus(fmt.Sprintf("⏵ %s #%d armed: %s %s", f.job.FullName(), f.number, a.answer(), at), false)
		return m, m.pollArmed()
	}
	return m, nil
}

func (m Model) armView(h int) string {
	f := m.arm
	var b strings.Builder
	b.WriteString(sLabel.Render(fmt.Sprintf("Pre-answer %s #%d", f.job.FullName(), f.number)) + "\n")
	b.WriteString(sDim.Render("When it pauses on its first input, jk answers for you.") + "\n\n")
	proceed := "Proceed"
	switch {
	case f.loading:
		b.WriteString(sKey.Render(spinner[m.frame%len(spinner)]) + sDim.Render(fmt.Sprintf(" Reading the last %d builds…", pastBuilds)))
	case f.existing && f.past != nil:
		b.WriteString("Expects: " + sWait.Render(f.past.Message))
	case f.existing:
		b.WriteString(sDim.Render("Answers whatever it asks first."))
	case f.past != nil:
		p := f.past
		how := "aborted"
		if p.Approved {
			how = p.ProceedText
		}
		b.WriteString(sDim.Render(fmt.Sprintf("Last time (#%d): ", p.Number)) + sWait.Render(p.Message) +
			sDim.Render(" → "+how+", by "+p.By) + "\n" + sDim.Render("Only answers if it asks the same."))
	case f.err != nil:
		b.WriteString(sErr.Render("Couldn't read the last builds: "+truncate(f.err.Error(), 60)) + "\n" +
			sDim.Render("Answers whatever it asks first."))
	default:
		b.WriteString(sDim.Render(fmt.Sprintf("No answered input in the last %d builds:", pastBuilds)) + "\n" +
			sDim.Render("answers whatever it asks first."))
	}
	if f.past != nil && f.past.ProceedText != "" {
		proceed = f.past.ProceedText
	}
	opt := func(label string, on bool) string {
		if on {
			return sSelected.Render("‹ " + label + " ›")
		}
		return sDim.Render("  " + label + "  ")
	}
	b.WriteString("\n\n" + opt(proceed, f.proceed) + "   " + opt("Abort", !f.proceed) + "\n\n")
	b.WriteString(sKey.Render("enter") + sDim.Render(" arm   ") + sKey.Render("←/→") + sDim.Render(" change   "))
	if f.existing {
		b.WriteString(sKey.Render("d") + sDim.Render(" disarm   "))
	}
	b.WriteString(sKey.Render("esc") + sDim.Render(" cancel"))
	return lipgloss.Place(m.w, h, lipgloss.Center, lipgloss.Center, sBox.Render(b.String()))
}

// pollArmed checks each armed build, watched or not.
func (m Model) pollArmed() tea.Cmd {
	var cmds []tea.Cmd
	for key, a := range m.armed {
		key, job, n, c := key, a.job, a.number, m.client
		cmds = append(cmds, func() tea.Msg {
			cx, cancel := ctx()
			defer cancel()
			b, err := c.Build(cx, job.Path, n)
			if err == nil && b.Building {
				b.Inputs, err = c.PendingInputs(cx, job.Path, n)
			}
			return armedResultMsg{key: key, build: b, err: err}
		})
	}
	return tea.Batch(cmds...)
}

func (m Model) handleArmedResult(msg armedResultMsg) (tea.Model, tea.Cmd) {
	a := m.armed[msg.key]
	if a == nil {
		return m, nil
	}
	switch {
	case errors.Is(msg.err, jenkins.ErrNotFound):
		delete(m.armed, msg.key)
		m.setErr(fmt.Errorf("disarmed %s #%d: %w", a.job.FullName(), a.number, msg.err))
		return m, nil
	case msg.err != nil:
		return m, nil // try again on the next poll
	}
	return m, m.armedSaw(a.job.FullName(), msg.build)
}

// armedSaw acts on a fresh look at a build, from any poll: answers its
// input if it is armed and paused, or drops it once it is over.
func (m *Model) armedSaw(name string, b *jenkins.Build) tea.Cmd {
	if b == nil {
		return nil
	}
	key := armKey(name, b.Number)
	a := m.armed[key]
	if a == nil {
		return nil
	}
	title := fmt.Sprintf("%s #%d", name, b.Number)
	if !b.Building {
		delete(m.armed, key)
		m.setStatus(title+" finished without asking: disarmed", false)
		return sendNotification("🦇 "+title+" finished without asking", "The pre-answer was not used.")
	}
	if len(b.Inputs) == 0 {
		return nil
	}
	delete(m.armed, key)
	in := b.Inputs[0]
	m.markAsked(name, b.Number, in.ID)
	leave := func(why string) tea.Cmd {
		m.setStatus("⏸ "+title+" needs you: "+why, true)
		return sendNotification("🦇 "+title+" needs you", why+" — open jenklod-batman and press i")
	}
	if a.expect != "" && in.Message != a.expect {
		return leave(fmt.Sprintf("it asks %q, not %q", in.Message, a.expect))
	}
	if !a.proceed {
		return m.autoAnswer(*a, in, nil)
	}
	vals, missing := inputDefaults(in.Params)
	if missing != "" {
		return leave(fmt.Sprintf("%q needs %s, which has no default", in.Message, missing))
	}
	return m.autoAnswer(*a, in, vals)
}

// markAsked keeps the watch poll from also notifying an input handled here.
func (m *Model) markAsked(name string, number int, id string) {
	st := m.watch[name]
	if st == nil {
		st = &watchState{}
		m.watch[name] = st
	}
	if st.asked == nil {
		st.asked = map[string]bool{}
	}
	st.asked[fmt.Sprintf("%d/%s", number, id)] = true
}

// inputDefaults is the step's own default for each field, or the name of
// the first field that has none (or is a password).
func inputDefaults(params []jenkins.Param) (map[string]string, string) {
	vals := map[string]string{}
	for _, p := range params {
		switch {
		case p.Type == jenkins.ParamPassword:
			return nil, p.Name
		case p.Type == jenkins.ParamBool:
			vals[p.Name] = fmt.Sprint(p.Default == "true")
		case p.Type == jenkins.ParamChoice && p.Default == "" && len(p.Choices) > 0:
			vals[p.Name] = p.Choices[0] // Jenkins' own default
		case p.Default == "":
			return nil, p.Name
		default:
			vals[p.Name] = p.Default
		}
	}
	return vals, ""
}

// autoAnswer sends the armed answer; vals is nil to abort.
func (m Model) autoAnswer(a armed, in jenkins.InputRequest, vals map[string]string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		cx, cancel := ctx()
		defer cancel()
		var err error
		if a.proceed {
			err = c.ProceedInput(cx, a.job.Path, a.number, in, vals)
		} else {
			err = c.AbortInput(cx, a.job.Path, a.number, in.ID)
		}
		return autoAnsweredMsg{a: a, in: in, err: err}
	}
}

func (m Model) handleAutoAnswered(msg autoAnsweredMsg) (tea.Model, tea.Cmd) {
	title := fmt.Sprintf("%s #%d", msg.a.job.FullName(), msg.a.number)
	if msg.err != nil {
		m.setErr(fmt.Errorf("pre-answer %s: %w", title, msg.err))
		return m, sendNotification("🦇 "+title+" needs you", "The pre-answer failed: "+msg.err.Error())
	}
	verb := "Auto-approved"
	if !msg.a.proceed {
		verb = "Auto-aborted"
	}
	m.setStatus(fmt.Sprintf("⏵ %s %s: %s", verb, title, msg.in.Message), false)
	return m, tea.Batch(m.refresh(), sendNotification("🦇 "+verb+" "+title, msg.in.Message))
}

// armedMark is the note on a running build's row that is armed, or "".
func (m Model) armedMark(name string, b jenkins.Build) string {
	if !b.Building || len(b.Inputs) > 0 {
		return ""
	}
	a := m.armed[armKey(name, b.Number)]
	if a == nil {
		return ""
	}
	s := "⏵ armed: " + a.answer()
	if a.expect != "" {
		s += fmt.Sprintf(" if asked %q", a.expect)
	}
	return s
}
