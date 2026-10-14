package ui

import (
	"errors"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/krank56/jenklod-batman/internal/config"
)

// fakeMusic records what the UI asks of the player, without sound.
type fakeMusic struct {
	mu      sync.Mutex
	playing bool
	plays   int
	volume  int
	beat    int
	err     error
}

func (f *fakeMusic) Play() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.playing = true
	f.plays++
	return nil
}

func (f *fakeMusic) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.playing = false
}

func (f *fakeMusic) SetVolume(pct int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.volume = pct
}

func (f *fakeMusic) Beat() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.beat
}

func startWithMusic(t *testing.T, cfg *config.Config, fm *fakeMusic) Model {
	t.Helper()
	m := New(Options{Config: cfg, NoAnim: true, Music: fm})
	m.scr = scrJobs
	m = drive(t, m, cmdMsg{m.Init()})
	return drive(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
}

func TestMusicToggle(t *testing.T) {
	cfg := newCfg(t)
	fm := &fakeMusic{}
	m := startWithMusic(t, cfg, fm)
	if fm.plays != 0 || strings.Contains(m.View(), "♫") {
		t.Fatal("music plays before M")
	}

	m = drive(t, m, key("M"))
	if !fm.playing || !m.playing || !cfg.Music {
		t.Fatalf("after M: player %v, model %v, config %v", fm.playing, m.playing, cfg.Music)
	}
	if fm.volume != config.DefaultVolume {
		t.Errorf("played at volume %d, want %d", fm.volume, config.DefaultVolume)
	}
	if v := snapshot(t, "30-music", m); !strings.Contains(v, "♫") || !strings.Contains(v, "Gotham by night") {
		t.Errorf("no music marker or status:\n%s", v)
	}
	if saved, _ := config.Load(cfg.Path()); !saved.Music {
		t.Error("music=true not saved")
	}

	m = drive(t, m, key("M"))
	if fm.playing || m.playing || cfg.Music {
		t.Fatalf("after second M: player %v, model %v, config %v", fm.playing, m.playing, cfg.Music)
	}
	if saved, _ := config.Load(cfg.Path()); saved.Music {
		t.Error("music=false not saved")
	}
}

func TestMusicResumesFromConfig(t *testing.T) {
	cfg := newCfg(t)
	cfg.Music = true
	v := 30
	cfg.MusicVolume = &v
	fm := &fakeMusic{}
	m := startWithMusic(t, cfg, fm)
	if !fm.playing || !m.playing || fm.volume != 30 {
		t.Errorf("music=true in config: player %v, model %v, volume %d", fm.playing, m.playing, fm.volume)
	}
}

func TestMusicVolume(t *testing.T) {
	cfg := newCfg(t)
	fm := &fakeMusic{}
	m := startWithMusic(t, cfg, fm)

	m = drive(t, m, key("+"))
	if cfg.Volume() != config.DefaultVolume {
		t.Errorf("+ changed the volume to %d while silent", cfg.Volume())
	}

	m = drive(t, m, key("M"))
	m = drive(t, m, key("+"))
	m = drive(t, m, key("="))
	if fm.volume != 70 || cfg.Volume() != 70 {
		t.Errorf("after +, =: player %d, config %d; want 70", fm.volume, cfg.Volume())
	}
	if !strings.Contains(m.View(), "Volume 70%") {
		t.Errorf("no volume status:\n%s", m.View())
	}
	for range 10 {
		m = drive(t, m, key("-"))
	}
	if fm.volume != 0 {
		t.Errorf("volume %d after ten -, want 0", fm.volume)
	}
	if saved, _ := config.Load(cfg.Path()); saved.Volume() != 0 {
		t.Errorf("saved volume %d, want 0", saved.Volume())
	}
}

func TestMusicUnavailableKeepsConfig(t *testing.T) {
	cfg := newCfg(t)
	cfg.Music = true
	fm := &fakeMusic{err: errors.New("no audio device")}
	m := startWithMusic(t, cfg, fm)
	if m.playing || strings.Contains(m.headerView(), "♫") {
		t.Error("shows music playing although the device failed")
	}
	if m.status != "♫ Music unavailable: no audio device" || !m.statusErr {
		t.Errorf("status %q, error %v", m.status, m.statusErr)
	}
	if !cfg.Music {
		t.Error("a failed start turned music off in the config")
	}

	// Once the device works, M tries again rather than switching off.
	fm.err = nil
	m = drive(t, m, key("M"))
	if !fm.playing || !m.playing || !cfg.Music {
		t.Errorf("M after a failure: player %v, model %v, config %v", fm.playing, m.playing, cfg.Music)
	}
}

func TestMusicOffWhileStarting(t *testing.T) {
	cfg := newCfg(t)
	fm := &fakeMusic{}
	m := startWithMusic(t, cfg, fm)
	on, cmd := m.toggleMusic() // M, its command not run yet
	off, _ := on.(Model).toggleMusic()
	m = drive(t, off.(Model), cmd())
	if fm.playing || m.playing {
		t.Error("music plays although turned off before the device opened")
	}
}

func TestNoMusicInThisBuild(t *testing.T) {
	cfg := newCfg(t)
	m := New(Options{Config: cfg, NoAnim: true})
	m.scr = scrJobs
	m = drive(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(t, m, key("M"))
	if cfg.Music || !strings.Contains(m.status, "unavailable in this build") {
		t.Errorf("without a player: config %v, status %q", cfg.Music, m.status)
	}
}

func TestCatDancesToTheBeat(t *testing.T) {
	cfg := newCfg(t)
	fm := &fakeMusic{}
	m := startWithMusic(t, cfg, fm)
	m.anim = true
	m = drive(t, m, key("M"))
	m = drive(t, m, tickMsg{})
	snapshot(t, "31-cat-dancing", m)
	x := m.cat.x
	first := m.cat.view(m.w)
	fm.beat = 1
	m = drive(t, m, tickMsg{})
	if m.cat.view(m.w) == first || !strings.ContainsAny(first, "♪♫") {
		t.Errorf("cat does not dance:\n%s\n%s", first, m.cat.view(m.w))
	}
	if m.cat.x != x {
		t.Error("cat walked while dancing")
	}

	m = drive(t, m, key("M"))
	m = drive(t, m, tickMsg{})
	if strings.ContainsAny(m.cat.view(m.w), "♪♫") {
		t.Error("cat still dances after the music stopped")
	}
}
