package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/krank56/jenklod-batman/internal/jenkins"
)

// stripMax is how many watched jobs the strip lists before "+N more".
const stripMax = 5

// showStrip reports whether the watched jobs strip sits under the header:
// on every browsing screen but the root listing, which has its own
// Watched section, and never under a dialog or a form.
func (m Model) showStrip() bool {
	if !m.cfg.StripOn() || len(m.cfg.Pins()) == 0 || m.confirm != nil || m.help || m.whatsNew.Version != "" || m.arm != nil {
		return false
	}
	switch m.scr {
	case scrJobs:
		return len(m.jobs.path) > 0 || m.jobs.filtering || m.jobs.filter.Value() != ""
	case scrBuilds, scrLog, scrMacros:
		return true
	}
	return false
}

// stripLines is the height of the strip, 0 when hidden.
func (m Model) stripLines() int {
	if !m.showStrip() {
		return 0
	}
	return min(len(m.cfg.Pins()), stripMax) + 1 // + the rule under it
}

// stripRank orders the strip: waiting for input, then broken or failed,
// then running, then the rest.
func (m Model) stripRank(name string) int {
	st := m.watch[name]
	switch {
	case st == nil || !st.known && st.err == nil:
		return 3
	case st.last != nil && len(st.last.Inputs) > 0:
		return 0
	case st.err != nil:
		return 1
	case st.last == nil:
		return 3
	case st.last.Building:
		return 2
	}
	switch st.last.Status() {
	case jenkins.StatusFailure, jenkins.StatusUnstable:
		return 1
	}
	return 3
}

// stripView lists the watched jobs that matter most, one line each, like
// the root's Watched section but without a cursor.
func (m Model) stripView() string {
	pins := slices.Clone(m.cfg.Pins())
	// Stable: equal ranks keep the order the user gave them.
	slices.SortStableFunc(pins, func(a, b string) int { return cmp.Compare(m.stripRank(a), m.stripRank(b)) })
	shown := pins[:min(len(pins), stripMax)]
	nameW := 0
	for _, p := range shown {
		nameW = max(nameW, lipgloss.Width(p))
	}
	nameW = min(nameW, 40)
	var b strings.Builder
	for _, p := range shown {
		b.WriteString("  " + truncate(m.pinRow(m.pinJob(p), false, nameW), m.w-2) + "\n")
	}
	rule := strings.Repeat("─", max(1, min(m.w-4, 60)))
	if more := len(pins) - len(shown); more > 0 {
		rule = strings.Repeat("─", max(1, min(m.w-4, 60)-12)) + fmt.Sprintf(" +%d more", more)
	}
	b.WriteString(sDim.Render("  " + rule))
	return b.String()
}

// toggleStrip is W: hide or show the strip, and save it.
func (m Model) toggleStrip() (tea.Model, tea.Cmd) {
	on := !m.cfg.StripOn()
	m.cfg.SetStrip(on)
	if err := m.cfg.Save(); err != nil {
		m.cfg.SetStrip(!on)
		m.setErr(fmt.Errorf("saving the strip setting: %w", err))
		return m, nil
	}
	if on {
		m.setStatus("◆ Watched jobs shown on every screen", false)
	} else {
		m.setStatus("Watched jobs only on the root screen — W shows them everywhere", false)
	}
	return m, nil
}

// autoWatch watches jobs built from here, so their builds show up in the
// strip and notify. It returns a poll when it added any.
func (m *Model) autoWatch(names ...string) tea.Cmd {
	var added []string
	for _, n := range names {
		if m.cfg.Watch(n) {
			added = append(added, n)
		}
	}
	if len(added) == 0 {
		return nil
	}
	if err := m.cfg.Save(); err != nil {
		for _, n := range added {
			m.cfg.TogglePin(n)
		}
		m.setErr(fmt.Errorf("watching %s: %w", strings.Join(added, ", "), err))
		return nil
	}
	return m.pollWatched()
}
