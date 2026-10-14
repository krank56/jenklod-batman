package music

import (
	"sync"
	"sync/atomic"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

// Player plays the tune on the default audio device. The device is opened
// on the first Play, so a Player that never plays touches no audio.
type Player struct {
	mu      sync.Mutex
	opened  bool
	playing bool

	tune   *Tune // owned by the speaker's goroutine while playing
	row    atomic.Int64
	volume atomic.Int64
}

// NewPlayer returns a stopped player at full volume.
func NewPlayer() *Player {
	p := &Player{}
	p.volume.Store(100)
	return p
}

// Play starts the song from the top. Playing while already playing does
// nothing.
func (p *Player) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.playing {
		return nil
	}
	if !p.opened {
		// A tenth of a second of buffer: toggles and volume feel immediate.
		if err := speaker.Init(SampleRate, SampleRate/10); err != nil {
			return err
		}
		p.opened = true
	} else if err := speaker.Resume(); err != nil {
		return err
	}
	p.tune = NewTune()
	p.row.Store(0)
	speaker.Play(beep.StreamerFunc(p.stream))
	p.playing = true
	return nil
}

// Stop silences the song and releases the device until the next Play.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing {
		return
	}
	speaker.Clear()
	_ = speaker.Suspend()
	p.playing = false
}

// SetVolume sets the volume in percent, 0 to 100.
func (p *Player) SetVolume(pct int) { p.volume.Store(int64(min(max(pct, 0), 100))) }

// Beat is the quarter note being played, counted from the top of the song.
func (p *Player) Beat() int { return int(p.row.Load()) / 4 }

func (p *Player) stream(samples [][2]float64) (int, bool) {
	n, ok := p.tune.Stream(samples)
	g := gain(int(p.volume.Load()))
	for i := range samples[:n] {
		samples[i][0] *= g
		samples[i][1] *= g
	}
	p.row.Store(int64(p.tune.Row()))
	return n, ok
}

// gain maps a volume percentage to an amplitude. Loudness is roughly
// logarithmic, so the curve is squared: 50% is a quarter of full scale.
func gain(pct int) float64 {
	v := float64(pct) / 100
	return v * v
}
