package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Music plays the Gotham theme. *music.Player is the real one.
type Music interface {
	Play() error
	Stop()
	SetVolume(pct int)
	Beat() int // the quarter note being played
}

const volumeStep = 10

type musicMsg struct{ err error }

// startMusic opens the audio device off the UI goroutine: it can take a
// moment, and it can fail.
func (m Model) startMusic() tea.Cmd {
	mu, vol := m.music, m.cfg.Volume()
	return func() tea.Msg {
		mu.SetVolume(vol)
		return musicMsg{err: mu.Play()}
	}
}

func (m Model) musicStarted(msg musicMsg) (tea.Model, tea.Cmd) {
	switch {
	case !m.musicOn:
		// Turned off while the device was opening.
		m.music.Stop()
	case msg.err != nil:
		// The config keeps music on: next time, on a machine with sound,
		// it plays. Only music asked for says why it's silent; the
		// default stays quiet on headless machines.
		m.musicOn = false
		if m.cfg.Music != nil {
			m.setStatus("♫ Music unavailable: "+msg.err.Error(), true)
		}
	default:
		m.playing = true
	}
	return m, nil
}

// toggleMusic is M: it starts or stops the theme and saves the choice.
func (m Model) toggleMusic() (tea.Model, tea.Cmd) {
	if m.music == nil {
		m.setStatus("♫ Music unavailable in this build", true)
		return m, nil
	}
	on := !m.musicOn
	prev := m.cfg.Music
	m.cfg.SetMusic(on)
	if err := m.cfg.Save(); err != nil {
		m.cfg.Music = prev
		m.setErr(fmt.Errorf("saving music setting: %w", err))
		return m, nil
	}
	m.musicOn = on
	if !on {
		m.music.Stop()
		m.playing = false
		m.setStatus("♫ Music off", false)
		return m, nil
	}
	m.setStatus(fmt.Sprintf("♫ Gotham by night — volume %d%%, +/- to change", m.cfg.Volume()), false)
	return m, m.startMusic()
}

// changeVolume is + and -, while the music plays.
func (m Model) changeVolume(delta int) (tea.Model, tea.Cmd) {
	if !m.playing {
		return m, nil
	}
	prev := m.cfg.MusicVolume
	v := clamp(m.cfg.Volume()+delta, 0, 100)
	m.cfg.MusicVolume = &v
	if err := m.cfg.Save(); err != nil {
		m.cfg.MusicVolume = prev
		m.setErr(fmt.Errorf("saving music volume: %w", err))
		return m, nil
	}
	m.music.SetVolume(v)
	m.setStatus(fmt.Sprintf("♫ Volume %d%%", v), false)
	return m, nil
}
