package ui

import (
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
	}
	crumb := sCrumb.Render(" " + strings.Join(append([]string{"/"}, crumbs...), " › "))
	right := ""
	if n := len(m.cfg.Pinned); n > 0 {
		right = sPin.Render(fmt.Sprintf("👁 %d watched ", n))
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
		add("enter", "keep filter")
		add("esc", "clear")
	case m.scr == scrJobs:
		add("↵/l", "open")
		add("h", "up")
		add("/", "filter")
		add("b", "build")
		add("w", "watch")
		add("o", "browser")
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
		add("enter", "build")
		add("esc", "cancel")
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
	if m.jobs.filtering || m.jobs.filter.Value() != "" {
		b.WriteString(m.jobs.filter.View() + "\n")
		h--
	}
	jobs := m.visibleJobs()
	switch {
	case m.jobs.loading && len(jobs) == 0:
		b.WriteString(sKey.Render(spinner[m.frame%len(spinner)]) + sDim.Render(" Scanning Gotham for jobs…"))
		return b.String()
	case m.jobs.err != nil && len(jobs) == 0:
		b.WriteString(sErr.Render("✘ "+m.jobs.err.Error()) + "\n" + sDim.Render("press r to retry"))
		return b.String()
	case len(jobs) == 0:
		b.WriteString(sDim.Render("  Nothing here. Even the Batcave has empty corners."))
		return b.String()
	}
	start, end := window(m.jobs.cursor, len(jobs), h)
	for i := start; i < end; i++ {
		j := jobs[i]
		sel := i == m.jobs.cursor
		cur := "  "
		if sel {
			cur = sCursor.Render("▌ ")
		}
		var icon, name string
		if j.IsFolder() {
			icon = sKey.Render("▸")
			name = j.Name + "/"
		} else {
			s, building := jenkins.ColorStatus(j.Color)
			icon = statusIcon(s, building, m.frame)
			name = j.Name
		}
		if sel {
			name = sSelected.Render(name)
		}
		pin := ""
		if m.cfg.IsPinned(j.FullName()) {
			pin = " " + sPin.Render("◆")
		}
		b.WriteString(cur + icon + " " + name + pin + "\n")
	}
	if m.jobs.loading {
		b.WriteString(sDim.Render("  " + spinner[m.frame%len(spinner)] + " refreshing"))
	}
	return b.String()
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
		{"/", "filter jobs"},
		{"b", "build (asks first; form for parameters)"},
		{"x", "abort a running build (asks first)"},
		{"i", "answer a paused pipeline input (proceed / abort)"},
		{"w", "watch / unwatch job — notifies when builds finish"},
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
