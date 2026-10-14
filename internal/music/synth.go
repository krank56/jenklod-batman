// Package music is the Gotham theme: a small SID-style synthesizer that
// renders an original chiptune, and a player for it.
package music

import "math"

// SampleRate is what the synthesizer renders at, in Hz.
const SampleRate = 44100

// frameSamples is one 50 Hz frame, the rate a C64 player steps its
// arpeggios at.
const frameSamples = SampleRate / 50

// rows is the parsed song. The song is a constant, so a parse error is a
// bug caught by the tests.
var rows = func() []row {
	r, err := parse(song)
	if err != nil {
		panic("music: " + err.Error())
	}
	return r
}()

// Length is the number of samples in one pass of the song.
func Length() int { return len(rows) * rowSamples }

// Tune renders the song, looping forever. It is not safe for concurrent
// use.
type Tune struct {
	pos   int // sample within the song
	noise uint32

	lead  voice
	arp   voice
	bass  voice
	kick  drum
	snare drum
	hat   drum

	arpTones []int
	cutoff   float64
	low, bp  float64 // bass filter state

	echo    []float64
	echoPos int
}

// NewTune starts the song from the top.
func NewTune() *Tune {
	return &Tune{
		noise: 0x1234567,
		lead:  voice{wave: wavePulse, a: 0.005, d: 0.25, s: 0.55, r: 0.12, gain: 0.20, vibrato: true, pwm: true},
		arp:   voice{wave: wavePulse, width: 0.25, a: 0.001, d: 0.09, s: 0.0, r: 0.02, gain: 0.09},
		bass:  voice{wave: waveSaw, a: 0.002, d: 0.12, s: 0.35, r: 0.03, gain: 0.30},
		echo:  make([]float64, 3*rowSamples),
	}
}

// Row is the sixteenth note being played, counted from the top of the song.
func (t *Tune) Row() int { return t.pos / rowSamples }

// Stream fills samples with stereo audio in [-1, 1]. It always fills all
// of them, so it matches beep.Streamer.
func (t *Tune) Stream(samples [][2]float64) (int, bool) {
	for i := range samples {
		if t.pos%rowSamples == 0 {
			t.trigger(rows[t.pos/rowSamples])
		}
		if t.pos%frameSamples == 0 && len(t.arpTones) > 0 {
			step := (t.pos / frameSamples) % len(t.arpTones)
			t.arp.freq = freq(t.arpTones[step])
		}
		samples[i] = t.sample()
		t.pos++
		if t.pos == Length() {
			t.pos = 0
		}
	}
	return len(samples), true
}

// Err is for beep.Streamer; rendering cannot fail.
func (t *Tune) Err() error { return nil }

func (t *Tune) trigger(r row) {
	switch {
	case r.lead != 0:
		t.lead.note(freq(r.lead))
	case r.release:
		t.lead.gate = false
	}
	if r.bassNote != 0 {
		t.bass.note(freq(r.bassNote))
	} else {
		t.bass.gate = false
	}
	t.arpTones = r.arp
	if len(r.arp) > 0 {
		t.arp.note(freq(r.arp[0]))
	}
	t.cutoff = r.cutoff
	if r.kick {
		t.kick = drum{len: 0.16, amp: 1}
	}
	if r.snare {
		t.snare = drum{len: 0.14, amp: 1}
	}
	if r.hat {
		t.hat = drum{len: 0.03, amp: 1}
		if r.open {
			t.hat.len = 0.09
		}
	}
}

func (t *Tune) sample() [2]float64 {
	sec := float64(t.pos) / SampleRate

	lead := t.lead.next(sec)

	// A dub echo on the lead: three sixteenths, fed back.
	wet := t.echo[t.echoPos]
	t.echo[t.echoPos] = lead + wet*0.38
	t.echoPos = (t.echoPos + 1) % len(t.echo)

	arp := t.arp.next(sec)

	// The bass goes through a resonant low-pass whose cutoff follows the
	// song and wobbles slowly, like a SID filter sweep.
	fc := 120 + t.cutoff*(700+500*math.Sin(2*math.Pi*sec/7.0))
	f := 2 * math.Sin(math.Pi*fc/SampleRate)
	hp := t.bass.next(sec) - t.low - 0.35*t.bp
	t.bp += f * hp
	t.low += f * t.bp
	bass := t.low

	dr := t.kick.kick() + 0.5*t.snare.snare(t.rand()) + 0.12*t.hat.hat(t.rand())

	mono := lead + bass + dr
	l := mono + 0.8*arp + 0.25*wet
	r := mono + 0.5*arp + 0.4*wet
	return [2]float64{math.Tanh(l * 1.2), math.Tanh(r * 1.2)}
}

// rand is the noise channel, a xorshift register in [-1, 1].
func (t *Tune) rand() float64 {
	x := t.noise
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	t.noise = x
	return float64(x)/float64(math.MaxUint32)*2 - 1
}

type wave int

const (
	wavePulse wave = iota
	waveSaw
)

// voice is one oscillator with an ADSR envelope, like a SID voice.
type voice struct {
	wave       wave
	width      float64 // pulse width, 0 to 0.5
	a, d, s, r float64 // seconds, seconds, level, seconds
	gain       float64
	vibrato    bool
	pwm        bool

	freq  float64
	phase float64
	gate  bool
	env   float64
	stage int // 0 attack, 1 decay/sustain, 2 release
	start float64
}

func (v *voice) note(f float64) {
	v.freq, v.gate, v.stage, v.start = f, true, 0, -1
}

func (v *voice) next(sec float64) float64 {
	if v.freq == 0 {
		return 0
	}
	if v.start < 0 {
		v.start = sec
	}
	age := sec - v.start
	dt := 1.0 / SampleRate
	switch {
	case !v.gate:
		v.env -= dt / v.r
	case v.stage == 0:
		v.env += dt / v.a
		if v.env >= 1 {
			v.env, v.stage = 1, 1
		}
	default:
		v.env -= (v.env - v.s) * dt / v.d * 4
	}
	if v.env <= 0 {
		v.env = 0
		return 0
	}
	f := v.freq
	if v.vibrato && age > 0.18 {
		f *= math.Pow(2, 0.25/12*math.Sin(2*math.Pi*5.5*age))
	}
	v.phase += f / SampleRate
	v.phase -= math.Floor(v.phase)
	var out float64
	switch v.wave {
	case waveSaw:
		out = 2*v.phase - 1
	default:
		w := v.width
		if v.pwm {
			w = 0.3 + 0.18*math.Sin(2*math.Pi*0.7*sec)
		}
		if v.phase < w {
			out = 1
		} else {
			out = -1
		}
	}
	return out * v.env * v.gain
}

// drum is a one-shot decaying hit.
type drum struct {
	len   float64 // seconds
	amp   float64
	age   float64
	phase float64
	prev  float64
}

func (d *drum) decay() float64 {
	if d.amp == 0 {
		return 0
	}
	d.age += 1.0 / SampleRate
	e := math.Exp(-5 * d.age / d.len)
	if d.age > 2*d.len {
		d.amp = 0
	}
	return e * d.amp
}

// kick is a sine falling from 150 to 45 Hz.
func (d *drum) kick() float64 {
	e := d.decay()
	if e == 0 {
		return 0
	}
	f := 45 + 105*math.Exp(-d.age*30)
	d.phase += f / SampleRate
	return 0.55 * e * math.Sin(2*math.Pi*d.phase)
}

// snare is noise over a short 190 Hz body.
func (d *drum) snare(n float64) float64 {
	e := d.decay()
	if e == 0 {
		return 0
	}
	d.phase += 190.0 / SampleRate
	return e * (0.5*n + 0.3*math.Sin(2*math.Pi*d.phase))
}

// hat is high-passed noise.
func (d *drum) hat(n float64) float64 {
	e := d.decay()
	if e == 0 {
		return 0
	}
	out := n - d.prev
	d.prev = n
	return e * out
}
