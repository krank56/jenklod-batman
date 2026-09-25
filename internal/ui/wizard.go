package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
	"github.com/krank56/jenklod-batman/internal/secret"
)

// WizardFunc validates the entered settings, persists them (config file +
// keyring) and returns a ready client. An empty token means "use the one
// already available" ($JENKINS_TOKEN or keyring).
type WizardFunc func(url, user, token string) (client *jenkins.Client, who string, err error)

type wizardDoneMsg struct {
	client *jenkins.Client
	who    string
	err    error
}

type wizard struct {
	inputs []textinput.Model
	focus  int
	busy   bool
	err    error
	submit WizardFunc
}

func newWizard(cfg *config.Config, fn WizardFunc) wizard {
	mk := func(label, placeholder, value string) textinput.Model {
		ti := textinput.New()
		ti.Prompt = label
		ti.Placeholder = placeholder
		ti.SetValue(value)
		ti.Width = 48
		return ti
	}
	url := mk("Jenkins URL  ", "https://jenkins.example.com", cfg.URL)
	user := mk("User         ", "bruce.wayne", cfg.User)
	token := mk("API token    ", "paste it — stored in the "+secret.StoreName(), "")
	token.EchoMode = textinput.EchoPassword
	token.EchoCharacter = '•'
	w := wizard{inputs: []textinput.Model{url, user, token}, submit: fn}
	w.setFocus(0)
	return w
}

func (w *wizard) setFocus(i int) {
	w.focus = (i + len(w.inputs)) % len(w.inputs)
	for j := range w.inputs {
		if j == w.focus {
			w.inputs[j].Focus()
		} else {
			w.inputs[j].Blur()
		}
	}
}

func (w wizard) update(msg tea.Msg) (wizard, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if w.busy {
			return w, nil
		}
		switch k.String() {
		case "esc":
			return w, tea.Quit
		case "tab", "down":
			w.setFocus(w.focus + 1)
			return w, nil
		case "shift+tab", "up":
			w.setFocus(w.focus - 1)
			return w, nil
		case "enter":
			if w.focus < len(w.inputs)-1 {
				w.setFocus(w.focus + 1)
				return w, nil
			}
			url := strings.TrimSpace(w.inputs[0].Value())
			user := strings.TrimSpace(w.inputs[1].Value())
			token := strings.TrimSpace(w.inputs[2].Value())
			w.busy, w.err = true, nil
			fn := w.submit
			return w, func() tea.Msg {
				c, who, err := fn(url, user, token)
				return wizardDoneMsg{client: c, who: who, err: err}
			}
		}
	}
	var cmd tea.Cmd
	w.inputs[w.focus], cmd = w.inputs[w.focus].Update(msg)
	return w, cmd
}

func (w wizard) view(width, height, frame int) string {
	var b strings.Builder
	b.WriteString(sTitle.Render("🦇 JENKLOD-BATMAN · SETUP") + "\n\n")
	b.WriteString(sDim.Render("Connect to your Jenkins. The URL and user go to the config file;") + "\n")
	b.WriteString(sDim.Render("the token goes to the "+secret.StoreName()+", never to disk.") + "\n\n")
	for _, in := range w.inputs {
		b.WriteString(in.View() + "\n")
	}
	b.WriteString("\n")
	b.WriteString(sDim.Render("Create a token in Jenkins: your name → Security → API Token → Add new token.") + "\n")
	b.WriteString(sDim.Render("Leave the token empty to use $"+secret.EnvVar+" or the one already in the "+secret.StoreName()+".") + "\n\n")
	switch {
	case w.busy:
		b.WriteString(sKey.Render(spinner[frame%len(spinner)]) + " Lighting the bat-signal…")
	case w.err != nil:
		b.WriteString(sErr.Render("✘ " + w.err.Error()))
	default:
		b.WriteString(sKey.Render("tab") + sDim.Render(" next  ") + sKey.Render("enter") + sDim.Render(" connect  ") + sKey.Render("esc") + sDim.Render(" quit"))
	}
	box := sBox.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
