package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/krank56/jenklod-batman/internal/jenkins"
)

const footerLines = 3 // cat lane (2) + status line

// View renders the whole screen.
func (m Model) View() string {
	if m.w == 0 {
		return ""
	}
	switch m.scr {
	case scrSplash:
		return m.splash.view(m.frame)
	case scrWizard:
		return m.wiz.view(m.w, m.h, m.frame)
	}

	bodyH := max(1, m.h-1-footerLines)
	var body string
	switch {
	case m.confirm != nil:
		body = m.confirmView(bodyH)
	case m.help:
		body = m.helpView(bodyH)
	default:
		switch m.scr {
		case scrJobs:
			body = m.jobsView(bodyH)
		case scrBuilds:
			body = m.buildsView(bodyH)
		case scrLog:
			body = m.logView()
		case scrParams:
			body = m.paramsView()
		case scrMacros:
			body = m.macrosView(bodyH)
		case scrMacroEdit:
			body = m.macroEditView(bodyH)
		}
	}
	body = lipgloss.NewStyle().Width(m.w).Height(bodyH).MaxHeight(bodyH).Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), body, m.footerView())
}

func (m Model) headerView() string {
	title := sTitle.Render("🦇 JENKLOD-BATMAN")
	host := ""
	if m.client != nil {
		host = sDim.Render(" " + m.client.Host() + " ")
	}
	var crumbs []string
	switch m.scr {
	case scrJobs:
		crumbs = m.jobs.path
	case scrBuilds, scrParams:
		crumbs = m.builds.job.Path
	case scrLog:
		crumbs = append(append([]string{}, m.log.job.Path...), fmt.Sprintf("#%d", m.log.number))
	case scrMacros:
		crumbs = []string{"macros"}
	case scrMacroEdit:
		crumbs = []string{"macros", m.macroEditTitle()}
	}
	if m.scr == scrParams && m.params.forMacro {
		crumbs = []string{"macros", m.macroEditTitle(), m.params.job.FullName()}
	}
	crumb := sCrumb.Render(" " + strings.Join(append([]string{"/"}, crumbs...), " › "))
	right := ""
	if r := m.run; r != nil && !r.done {
		right = sWait.Render(fmt.Sprintf("▶ %s %d/%d ", r.name, r.step, r.total))
	}
	if n := len(m.cfg.Pins()); n > 0 {
		right += sPin.Render(fmt.Sprintf("👁 %d watched ", n)) + sDim.Render(fmt.Sprintf("⟳%ds ", m.cfg.PollSeconds))
	}
	left := title + host + crumb
	gap := m.w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return truncate(left, m.w)
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) footerView() string {
	lane := m.cat.view(m.w)
	var hints []string
	add := func(k, d string) { hints = append(hints, sKey.Render(k)+sDim.Render(" "+d)) }
	switch {
	case m.confirm != nil:
		add("y", "yes")
		add("n", "no")
	case m.scr == scrJobs && m.jobs.filtering:
		add("↑/↓", "move")
		add("enter", "keep results")
		add("esc", "clear")
	case m.scr == scrJobs:
		r, _ := m.selectedRow()
		add("↵/l", "open")
		add("h", "up")
		add("/", "search")
		add("b", "build")
		add("L", "last log")
		add("x", "abort")
		add("w", "watch")
		if r.pin {
			add("J/K", "move")
		}
		add("m", "macros")
		add("p", "poll")
		add("?", "help")
		add("q", "quit")
	case m.scr == scrBuilds:
		add("↵", "log")
		if _, ok := m.waitingBuild(); ok {
			add("i", "answer input")
		}
		add("b", "build")
		add("x", "abort")
		add("w", "watch")
		add("o", "browser")
		add("h", "back")
	case m.scr == scrLog:
		if len(m.log.inputs) > 0 {
			add("i", "answer input")
		}
		add("f", "follow")
		add("g/G", "top/end")
		add("x", "abort")
		add("o", "browser")
		add("h", "back")
	case m.scr == scrParams && m.params.input != nil:
		if len(m.params.fields) > 0 {
			add("tab", "next")
			add("←/→", "change")
		}
		add("enter", m.params.input.ProceedText)
		add("ctrl+x", "abort")
		add("esc", "later")
	case m.scr == scrParams:
		add("tab", "next")
		add("←/→", "change")
		if m.params.forMacro {
			add("enter", "save step")
		} else {
			add("enter", "build")
		}
		add("esc", "cancel")
	case m.scr == scrMacros:
		add("↵", "run")
		add("n", "new")
		add("e", "edit")
		add("d", "delete")
		if m.run != nil && !m.run.done {
			add("c", "cancel run")
		}
		add("h", "back")
	case m.scr == scrMacroEdit:
		for _, h := range m.macroEditHints() {
			add(h[0], h[1])
		}
	}
	line := strings.Join(hints, "  ")
	if m.status != "" {
		st := sOK
		if m.statusErr {
			st = sErr
		}
		msg := st.Render(truncate(m.status, max(10, m.w-lipgloss.Width(line)-3)))
		gap := m.w - lipgloss.Width(line) - lipgloss.Width(msg)
		if gap >= 2 {
			line += strings.Repeat(" ", gap) + msg
		} else {
			line = msg
		}
	}
	return lane + "\n" + truncate(line, m.w)
}

func window(cursor, n, height int) (int, int) {
	start := 0
	if cursor >= height {
		start = cursor - height + 1
	}
	return start, min(n, start+height)
}

func (m Model) jobsView(h int) string {
	var b strings.Builder
	searching := m.jobs.filtering || m.jobs.filter.Value() != ""
	if searching {
		line := m.jobs.filter.View()
		switch {
		case m.indexLoading:
			line += sDim.Render("  " + spinner[m.frame%len(spinner)] + " indexing all folders…")
		case m.indexErr != nil:
			line += sErr.Render("  other folders unavailable — press r to retry")
		}
		b.WriteString(line + "\n")
		h--
	}
	rows := m.rows()
	pins := 0
	for _, r := range rows {
		if r.pin {
			pins++
		}
	}
	if pins > 0 {
		b.WriteString(sPin.Render("◆ WATCHED") + sDim.Render(fmt.Sprintf("  polled every %ds", m.cfg.PollSeconds)) + "\n")
		h -= 2 // heading + the rule under the last pin
	}
	switch {
	case m.jobs.loading && len(rows) == 0:
		b.WriteString(sKey.Render(spinner[m.frame%len(spinner)]) + sDim.Render(" Scanning Gotham for jobs…"))
		return b.String()
	case m.jobs.err != nil && len(rows) == 0:
		b.WriteString(sErr.Render("✘ "+m.jobs.err.Error()) + "\n" + sDim.Render("press r to retry"))
		return b.String()
	case len(rows) == 0 && searching:
		b.WriteString(sDim.Render("  No job matches, in this folder or any other."))
		return b.String()
	case len(rows) == 0:
		b.WriteString(sDim.Render("  Nothing here. Even the Batcave has empty corners."))
		return b.String()
	}
	pinW := 0
	for _, r := range rows[:pins] {
		pinW = max(pinW, lipgloss.Width(r.job.FullName()))
	}
	pinW = min(pinW, 40)
	start, end := window(m.jobs.cursor, len(rows), max(1, h))
	for i := start; i < end; i++ {
		r := rows[i]
		sel := i == m.jobs.cursor
		cur := "  "
		if sel {
			cur = sCursor.Render("▌ ")
		}
		if r.pin {
			b.WriteString(cur + m.pinRow(r.job, sel, pinW) + "\n")
			if i == pins-1 && i+1 < end {
				b.WriteString(sDim.Render("  "+strings.Repeat("─", max(1, min(m.w-4, 60)))) + "\n")
			}
			continue
		}
		var icon, name string
		if r.job.IsFolder() {
			icon = sKey.Render("▸")
			name = r.job.Name + "/"
		} else {
			s, building := jenkins.ColorStatus(r.job.Color)
			icon = statusIcon(s, building, m.frame)
			name = r.job.Name
		}
		if sel {
			name = sSelected.Render(name)
		}
		if r.other {
			if parent := r.job.Path[:len(r.job.Path)-1]; len(parent) > 0 {
				name = sDim.Render(jenkins.JoinPath(parent)+"/") + name
			}
		}
		pin := ""
		if m.cfg.IsPinned(r.job.FullName()) {
			pin = " " + sPin.Render("◆")
		}
		b.WriteString(cur + icon + " " + name + pin + "\n")
	}
	if m.jobs.loading {
		b.WriteString(sDim.Render("  " + spinner[m.frame%len(spinner)] + " refreshing"))
	}
	return b.String()
}

// pinRow is a watched job with its last build, as the last poll saw it.
func (m Model) pinRow(job jenkins.Job, sel bool, nameW int) string {
	full := job.FullName()
	name := pad(truncate(full, nameW), nameW)
	if sel {
		name = sSelected.Render(name)
	}
	st := m.watch[full]
	switch {
	case st != nil && st.err != nil:
		return sErr.Render("✗") + " " + name + "  " + sErr.Render(truncate(pollError(st.err), max(10, m.w-nameW-8)))
	case st == nil || !st.known:
		return sDim.Render("·") + " " + name + "  " + sDim.Render("…")
	case st.last == nil:
		return sDim.Render("○") + " " + name + "  " + sDim.Render("never built")
	}
	bd := *st.last
	dur := fmtDur(bd.Duration)
	if bd.Building {
		dur = progress(time.Since(bd.Started), bd.Estimated)
	}
	icon, result := statusIcon(bd.Status(), bd.Building, m.frame), resultText(bd)
	cause := bd.Cause
	if len(bd.Inputs) > 0 {
		icon, result = sWait.Render("⏸"), sWait.Render("INPUT")
		cause = bd.Inputs[0].Message
	}
	used := 2 + 2 + nameW + 2 + 7 + 1 + 10 + 1 + 10 + 1 + 18 + 1
	return fmt.Sprintf("%s %s  %s %s %s %s %s", icon, name, pad(fmt.Sprintf("#%d", bd.Number), 7),
		pad(result, 10), pad(fmtAgo(bd.Started), 10), pad(dur, 18), sDim.Render(truncate(cause, max(0, m.w-used))))
}

// pollError shortens the usual reasons a pin cannot be polled.
func pollError(err error) string {
	if errors.Is(err, jenkins.ErrNotFound) {
		return "not found — deleted or renamed? press w to unwatch"
	}
	return strings.ReplaceAll(err.Error(), "\n", " ")
}

func (m Model) buildsView(h int) string {
	var b strings.Builder
	job := m.builds.job
	d := m.builds.detail
	head := sLabel.Render(job.Name)
	if m.cfg.IsPinned(job.FullName()) {
		head += " " + sPin.Render("◆ watched")
	}
	if d != nil && len(d.Params) > 0 {
		head += sDim.Render(fmt.Sprintf("  · %d parameters", len(d.Params)))
	}
	if d != nil && !d.Buildable {
		head += sErr.Render("  · disabled")
	}
	if m.builds.loading {
		head += "  " + sKey.Render(spinner[m.frame%len(spinner)])
	}
	b.WriteString(head + "\n")
	h--
	if w, ok := m.waitingBuild(); ok {
		in := w.Inputs[0]
		b.WriteString(sWait.Render(fmt.Sprintf("⏸ #%d is waiting: %s", w.Number, in.Message)) +
			sDim.Render("  — press ") + sKey.Render("i") + sDim.Render(" to answer") + "\n")
		h--
	}
	b.WriteString("\n")
	h--
	if d == nil {
		if m.builds.err != nil {
			b.WriteString(sErr.Render("✘ " + m.builds.err.Error()))
		}
		return b.String()
	}
	if len(d.Builds) == 0 {
		b.WriteString(sDim.Render("  No builds yet. Press b to summon the first one."))
		return b.String()
	}
	b.WriteString(sDim.Render(fmt.Sprintf("    %-7s %-10s %-10s %-18s %s", "BUILD", "RESULT", "STARTED", "DURATION", "CAUSE")) + "\n")
	h--
	start, end := window(m.builds.cursor, len(d.Builds), h)
	for i := start; i < end; i++ {
		bd := d.Builds[i]
		sel := i == m.builds.cursor
		cur := "  "
		if sel {
			cur = sCursor.Render("▌ ")
		}
		num := fmt.Sprintf("#%-6d", bd.Number)
		if sel {
			num = sSelected.Render(num)
		}
		dur := fmtDur(bd.Duration)
		if bd.Building {
			dur = progress(time.Since(bd.Started), bd.Estimated)
		}
		icon, result := statusIcon(bd.Status(), bd.Building, m.frame), resultText(bd)
		cause := sDim.Render(truncate(bd.Cause, max(0, m.w-60)))
		if len(bd.Inputs) > 0 {
			icon, result = sWait.Render("⏸"), sWait.Render("INPUT")
			cause = sWait.Render(truncate(bd.Inputs[0].Message, max(0, m.w-60)))
		}
		row := fmt.Sprintf("%s%s %s %s %s %s %s",
			cur, icon, num, pad(result, 10), pad(fmtAgo(bd.Started), 10), pad(dur, 18), cause)
		b.WriteString(row + "\n")
	}
	return b.String()
}

// progress renders a small bar of elapsed vs. estimated time.
func progress(elapsed, estimated time.Duration) string {
	const width = 10
	if estimated <= 0 {
		return fmtDur(elapsed)
	}
	f := float64(elapsed) / float64(estimated)
	n := clamp(int(f*width), 0, width)
	return lipgloss.NewStyle().Foreground(blue).Render(strings.Repeat("█", n)+strings.Repeat("░", width-n)) +
		" " + fmtDur(elapsed)
}

func pad(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

func (m Model) logView() string {
	state := sDim.Render("finished")
	if m.log.more {
		state = lipgloss.NewStyle().Foreground(blue).Render(spinner[m.frame%len(spinner)] + " running")
	}
	follow := sDim.Render("follow off")
	if m.log.follow {
		follow = sKey.Render("following")
	}
	head := sLabel.Render(fmt.Sprintf("%s #%d", m.log.job.Name, m.log.number)) + "  " + state + "  " + follow +
		sDim.Render(fmt.Sprintf("  %3.f%%", m.log.vp.ScrollPercent()*100))
	if m.log.loading && (m.log.text == nil || m.log.text.Len() == 0) {
		return head + "\n" + sDim.Render("Fetching the console from the Batcomputer…")
	}
	if m.log.err != nil && (m.log.text == nil || m.log.text.Len() == 0) {
		return head + "\n" + sErr.Render("✘ "+m.log.err.Error())
	}
	vp := m.log.vp.View()
	if len(m.log.inputs) > 0 {
		banner := sWait.Render("⏸ Waiting for input: "+m.log.inputs[0].Message) +
			sDim.Render("  — press ") + sKey.Render("i") + sDim.Render(" to answer")
		// Keep the total height: drop the viewport's first line.
		if _, rest, ok := strings.Cut(vp, "\n"); ok {
			vp = rest
		}
		return head + "\n" + banner + "\n" + vp
	}
	return head + "\n" + vp
}

func (m Model) paramsView() string {
	p := m.params
	var b strings.Builder
	if p.input != nil {
		b.WriteString(sWait.Render(fmt.Sprintf("⏸ %s #%d is waiting for you", p.job.FullName(), p.number)) + "\n\n")
		b.WriteString("  " + sLabel.Render(p.input.Message) + "\n\n")
		if len(p.fields) == 0 {
			b.WriteString("  " + sKey.Render("enter") + " " + p.input.ProceedText + "    " + sKey.Render("x") + " Abort\n")
		}
	} else if p.forMacro {
		b.WriteString(sLabel.Render("Macro step: build "+p.job.FullName()) + sDim.Render(" with these parameters") + "\n")
		if p.skippedPass > 0 {
			b.WriteString(sDim.Render(fmt.Sprintf("%d password parameter(s) are not saved in macros; Jenkins uses their default.", p.skippedPass)) + "\n")
		}
		b.WriteString("\n")
	} else {
		b.WriteString(sLabel.Render("Build "+p.job.FullName()) + sDim.Render(" with parameters") + "\n\n")
	}
	for i, f := range p.fields {
		focused := i == p.focus
		label := pad(f.def.Name, 24)
		if focused {
			label = sSelected.Render(label)
		} else {
			label = sLabel.Render(label)
		}
		var val string
		switch f.def.Type {
		case jenkins.ParamBool:
			if f.on {
				val = sOK.Render("[x] true")
			} else {
				val = sDim.Render("[ ] false")
			}
		case jenkins.ParamChoice:
			if len(f.def.Choices) == 0 {
				val = sDim.Render("(no choices)")
			} else {
				val = sKey.Render("‹ ") + f.def.Choices[f.choice] + sKey.Render(" ›") +
					sDim.Render(fmt.Sprintf("  %d/%d", f.choice+1, len(f.def.Choices)))
			}
		default:
			val = f.input.View()
		}
		cur := "  "
		if focused {
			cur = sCursor.Render("▌ ")
		}
		b.WriteString(cur + label + " " + val + "\n")
		if f.def.Description != "" && focused {
			b.WriteString("    " + sDim.Render(truncate(f.def.Description, m.w-6)) + "\n")
		}
	}
	return b.String()
}

func (m Model) confirmView(h int) string {
	box := sBox.Render(sLabel.Render(m.confirm.prompt) + "\n\n" +
		sKey.Render("y") + sDim.Render(" yes   ") + sKey.Render("n") + sDim.Render(" no"))
	return lipgloss.Place(m.w, h, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) helpView(h int) string {
	rows := [][2]string{
		{"j/k ↑/↓", "move"},
		{"g/G", "top / bottom"},
		{"ctrl+d/u", "half page down / up"},
		{"enter l →", "open folder, job or build log"},
		{"h ← esc", "back"},
		{"/", "search this folder, then every other (fuzzy)"},
		{"b", "build (asks first; form for parameters)"},
		{"x", "abort a running build (asks first)"},
		{"L", "open the last build's log (jobs list)"},
		{"i", "answer a paused pipeline input (proceed / abort)"},
		{"w", "watch / unwatch job — notifies when builds finish"},
		{"J/K", "move a watched job down / up"},
		{"p", "cycle the watch poll interval (saved)"},
		{"m", "macros: run, create, edit"},
		{"o", "open in browser"},
		{"f", "follow log output"},
		{"r", "refresh"},
		{"a", "toggle the cat"},
		{"q", "back / quit"},
	}
	var b strings.Builder
	b.WriteString(sTitle.Render("🦇 KEYS") + "\n\n")
	for _, r := range rows {
		b.WriteString(sKey.Render(pad(r[0], 12)) + " " + r[1] + "\n")
	}
	b.WriteString("\n" + sDim.Render(fmt.Sprintf("Watched jobs are polled every %ds.  Any key closes this.", m.cfg.PollSeconds)))
	return lipgloss.Place(m.w, h, lipgloss.Center, lipgloss.Center, sBox.Render(b.String()))
}
