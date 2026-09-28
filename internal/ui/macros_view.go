package ui

import (
	"fmt"
	"strings"

	"github.com/krank56/jenklod-batman/internal/config"
)

func (m Model) macrosView(h int) string {
	var b strings.Builder
	b.WriteString(sLabel.Render("Macros") + sDim.Render("  on "+m.client.Host()+" · also: jenklod-batman run <name>") + "\n\n")
	h -= 2
	ms := m.cfg.Macros()
	if len(ms) == 0 {
		b.WriteString(sDim.Render("  No macros yet. Press n to create one."))
	}
	runH := 0
	if m.run != nil {
		runH = min(len(m.run.lines), 8) + 3
	}
	start, end := window(m.macros.cursor, len(ms), max(1, h-runH))
	for i := start; i < end; i++ {
		mac := ms[i]
		sel := i == m.macros.cursor
		cur, name := "  ", pad(mac.Name, 20)
		if sel {
			cur, name = sCursor.Render("▌ "), sSelected.Render(name)
		}
		summary := fmt.Sprintf("%d step(s)", len(mac.Steps))
		if len(mac.Steps) > 0 {
			summary += " · " + mac.Steps[0].String()
			if len(mac.Steps) > 1 {
				summary += " → …"
			}
		}
		b.WriteString(cur + name + " " + sDim.Render(truncate(summary, max(0, m.w-26))) + "\n")
	}
	if r := m.run; r != nil {
		var head string
		switch {
		case !r.done:
			head = sWait.Render(fmt.Sprintf("%s ▶ %s  step %d/%d", spinner[m.frame%len(spinner)], r.name, r.step, r.total)) +
				sDim.Render("  — c cancels")
		case r.err != nil:
			head = sErr.Render("✘ " + r.name + " failed: " + r.err.Error())
		default:
			head = sOK.Render("✔ " + r.name + " done")
		}
		b.WriteString("\n" + truncate(head, m.w) + "\n")
		lines := r.lines[max(0, len(r.lines)-8):]
		for _, l := range lines {
			b.WriteString(sDim.Render("  "+truncate(l, m.w-4)) + "\n")
		}
	}
	return b.String()
}

func (m Model) macroEditTitle() string {
	e := m.macros.edit
	if e == nil {
		return ""
	}
	if n := strings.TrimSpace(e.name.Value()); n != "" {
		return n
	}
	return "new"
}

func (m Model) macroEditHints() [][2]string {
	e := m.macros.edit
	switch e.mode {
	case editName:
		return [][2]string{{"enter", "done"}}
	case editKind:
		return [][2]string{{"b", "build"}, {"x", "abort"}, {"w", "wait"}, {"i", "input"}, {"esc", "cancel"}}
	case editPick:
		return [][2]string{{"↑/↓", "move"}, {"enter", "pick"}, {"esc", "cancel"}}
	case editTimeout:
		return [][2]string{{"enter", "set"}, {"esc", "cancel"}}
	}
	hints := [][2]string{{"a", "add step"}}
	if e.cursor < len(e.steps) {
		switch e.steps[e.cursor].Kind {
		case config.StepBuild:
			hints = append(hints, [2]string{"↵", "params"})
		case config.StepInput:
			hints = append(hints, [2]string{"t", "timeout"}, [2]string{"space", "proceed/abort"})
		case config.StepWait:
			hints = append(hints, [2]string{"t", "timeout"})
		}
		hints = append(hints, [2]string{"J/K", "move"}, [2]string{"d", "delete"})
	}
	return append(hints, [2]string{"r", "rename"}, [2]string{"ctrl+s", "save"}, [2]string{"esc", "close"})
}

func (m Model) macroEditView(h int) string {
	e := m.macros.edit
	var b strings.Builder
	name := e.name.View()
	if e.mode != editName {
		name = sLabel.Render(m.macroEditTitle())
	}
	dirty := ""
	if e.dirty {
		dirty = sWait.Render("  ● unsaved")
	}
	b.WriteString(sDim.Render("Macro ") + name + dirty + "\n\n")
	h -= 2

	switch e.mode {
	case editKind:
		b.WriteString(sLabel.Render("Add a step:") + "\n\n")
		for _, k := range [][2]string{
			{"b", "build a job (with parameters)"},
			{"x", "abort a job's running build"},
			{"w", "wait for a build to finish successfully"},
			{"i", "answer a build's pending input (proceed or abort)"},
		} {
			b.WriteString("  " + sKey.Render(k[0]) + "  " + k[1] + "\n")
		}
		b.WriteString("\n" + sDim.Render("Abort, wait and input act on the build an earlier step of this macro started,\nelse on the job's newest running build."))
		return b.String()
	case editPick:
		b.WriteString(sLabel.Render("Step: "+e.kind) + "\n" + e.pick.View() + "\n")
		h -= 2
		cands := m.pickCandidates()
		if len(cands) == 0 {
			msg := "  No job matches."
			if m.indexLoading {
				msg = "  " + spinner[m.frame%len(spinner)] + " indexing all folders…"
			} else if m.indexErr != nil {
				msg = "  Couldn't load the job list: " + m.indexErr.Error()
			}
			b.WriteString(sDim.Render(msg))
			return b.String()
		}
		start, end := window(e.pickCursor, len(cands), max(1, h))
		for i := start; i < end; i++ {
			cur, n := "  ", cands[i].FullName()
			if i == e.pickCursor {
				cur, n = sCursor.Render("▌ "), sSelected.Render(n)
			}
			b.WriteString(cur + n + "\n")
		}
		return b.String()
	}

	if len(e.steps) == 0 {
		b.WriteString(sDim.Render("  No steps yet. Press a to add one."))
	}
	start, end := window(e.cursor, len(e.steps), max(1, h-2))
	for i := start; i < end; i++ {
		cur, desc := "  ", fmt.Sprintf("%d. %s", i+1, e.steps[i])
		if i == e.cursor {
			cur, desc = sCursor.Render("▌ "), sSelected.Render(desc)
		}
		b.WriteString(cur + truncate(desc, m.w-4) + "\n")
	}
	if e.mode == editTimeout {
		b.WriteString("\n" + e.timeout.View())
	}
	return b.String()
}
