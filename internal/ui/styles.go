package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/krank56/jenklod-batman/internal/jenkins"
)

var (
	batYellow = lipgloss.Color("#FFD500")
	batBlack  = lipgloss.Color("#0B0B0F")
	gotham    = lipgloss.Color("#1C1F2B")
	dim       = lipgloss.Color("#6B7084")
	fg        = lipgloss.Color("#D8DAE5")
	green     = lipgloss.Color("#4ADE80")
	red       = lipgloss.Color("#F87171")
	blue      = lipgloss.Color("#60A5FA")
	orange    = lipgloss.Color("#FBBF24")

	sTitle    = lipgloss.NewStyle().Foreground(batBlack).Background(batYellow).Bold(true).Padding(0, 1)
	sCrumb    = lipgloss.NewStyle().Foreground(fg)
	sDim      = lipgloss.NewStyle().Foreground(dim)
	sSelected = lipgloss.NewStyle().Foreground(batYellow).Bold(true)
	sCursor   = lipgloss.NewStyle().Foreground(batYellow)
	sKey      = lipgloss.NewStyle().Foreground(batYellow).Bold(true)
	sErr      = lipgloss.NewStyle().Foreground(red)
	sOK       = lipgloss.NewStyle().Foreground(green)
	sPin      = lipgloss.NewStyle().Foreground(batYellow)
	sCat      = lipgloss.NewStyle().Foreground(lipgloss.Color("#C4B5FD"))
	sBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(batYellow).Padding(1, 2)
	sLabel    = lipgloss.NewStyle().Foreground(fg).Bold(true)
	sWait     = lipgloss.NewStyle().Foreground(orange).Bold(true)
)

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// statusIcon renders the ball for a job or build.
func statusIcon(s jenkins.Status, building bool, frame int) string {
	if building {
		return lipgloss.NewStyle().Foreground(blue).Render(spinner[frame%len(spinner)])
	}
	switch s {
	case jenkins.StatusSuccess:
		return sOK.Render("●")
	case jenkins.StatusFailure:
		return sErr.Render("●")
	case jenkins.StatusUnstable:
		return lipgloss.NewStyle().Foreground(orange).Render("●")
	case jenkins.StatusAborted:
		return sDim.Render("◌")
	case jenkins.StatusDisabled:
		return sDim.Render("⊘")
	case jenkins.StatusNotBuilt:
		return sDim.Render("○")
	}
	return sDim.Render("·")
}

func resultText(b jenkins.Build) string {
	if b.Building {
		return lipgloss.NewStyle().Foreground(blue).Render("RUNNING")
	}
	switch b.Status() {
	case jenkins.StatusSuccess:
		return sOK.Render(b.Result)
	case jenkins.StatusFailure:
		return sErr.Render(b.Result)
	case jenkins.StatusUnstable:
		return lipgloss.NewStyle().Foreground(orange).Render(b.Result)
	}
	return sDim.Render(b.Result)
}
