package music

import (
	"fmt"
	"math"
	"strings"
)

// The tune: "Gotham by Night", an original keygen-style loop in D minor.
// Its hook is a rising minor sixth, D to B♭, over an octave-jumping bass.
//
// Each bar is 16 rows (sixteenth notes). A lead row is a note such as
// "Bb4" or "C#5", "--" to hold the previous one, or ".." to release it.

const bpm = 136

// rowSamples is the length of one sixteenth note.
const rowSamples = SampleRate * 60 / bpm / 4

// Drum styles of a bar.
const (
	drumsNone   = iota
	drumsHats   // hi-hats only
	drumsBuild  // kick and hats, a snare roll on the last beat
	drumsDrive  // four-on-the-floor kick, backbeat snare, sixteenth hats
	drumsHalf   // half time: kick on 1, snare on 3, eighth hats
	drumsFillUp // drive with a snare roll on the last beat
)

type bar struct {
	chord string // root and quality, e.g. "Dm", "Bb", "A"
	lead  string // 16 rows, or "" for no lead
	drums int
	bass  bool
	arp   bool
	// filter opens the bass filter over the bar, 0 (closed) to 1 (open).
	filterFrom, filterTo float64
}

// Lead phrases, one bar each.
var (
	a1 = []string{
		"D4 -- -- D4 Bb4 -- -- -- -- -- A4 -- G4 -- A4 --",
		"F4 -- -- -- D4 -- -- -- F4 -- G4 -- A4 -- Bb4 --",
		"C5 -- -- -- -- -- Bb4 -- A4 -- -- -- G4 -- E4 --",
		"A4 -- -- -- -- -- -- -- C#5 -- -- -- E5 -- -- --",
	}
	a2 = []string{
		"D5 -- -- D5 Bb5 -- -- -- A5 -- -- -- F5 -- E5 --",
		"D5 -- -- -- -- -- F5 -- E5 -- D5 -- C5 -- Bb4 --",
		"C5 -- E5 -- G5 -- -- -- F5 -- E5 -- C5 -- -- --",
		"C#5 -- -- -- E5 -- -- -- A5 -- -- -- -- -- .. ..",
	}
	// The rooftop: long notes over a half-time beat.
	b1 = []string{
		"G4 -- -- -- -- -- -- -- Bb4 -- -- -- D5 -- -- --",
		"F5 -- -- -- -- -- -- -- -- -- -- -- E5 -- D5 --",
		"D5 -- -- -- -- -- -- -- F5 -- -- -- -- -- -- --",
		"E5 -- -- -- -- -- -- -- C#5 -- -- -- -- -- -- --",
		"Bb4 -- -- -- D5 -- -- -- G5 -- -- -- -- -- -- --",
		"A5 -- -- -- -- -- -- -- F5 -- -- -- D5 -- -- --",
		"Eb5 -- -- -- -- -- -- -- G5 -- -- -- Bb5 -- -- --",
		"A5 -- -- -- -- -- -- -- -- -- -- -- .. .. .. ..",
	}
)

var (
	chordsA = []string{"Dm", "Bb", "C", "A"}
	chordsB = []string{"Gm", "Dm", "Bb", "A", "Gm", "Dm", "Eb", "A"}
)

// song is the whole loop: intro, A, rooftop, A, rooftop. 52 bars, about
// a minute and a half.
var song = buildSong()

func buildSong() []bar {
	var s []bar
	// Intro: the bass filter opens, the arpeggios and drums come in.
	for i, c := range chordsA {
		b := bar{chord: c, bass: true, arp: i >= 2, filterFrom: float64(i) / 4, filterTo: float64(i+1) / 4}
		switch i {
		case 0, 1:
			b.drums = drumsHats
		case 2:
			b.drums = drumsBuild
		case 3:
			b.drums = drumsFillUp
		}
		s = append(s, b)
	}
	section := func(phrases [][]string, chords []string, drums int) {
		for _, p := range phrases {
			for i, l := range p {
				s = append(s, bar{chord: chords[i%len(chords)], lead: l, drums: drums, bass: true, arp: true, filterFrom: 1, filterTo: 1})
			}
		}
		s[len(s)-1].drums = drumsFillUp
	}
	section([][]string{a1, a2, a1, a2}, chordsA, drumsDrive)
	section([][]string{b1}, chordsB, drumsHalf)
	section([][]string{a1, a2, a1, a2}, chordsA, drumsDrive)
	section([][]string{b1}, chordsB, drumsHalf)
	return s
}

// Bass rows: semitones above the chord root, an octave-jumping figure.
var bassFigure = [16]int{0, 0, 12, 0, 0, 12, 0, 12, 0, 0, 12, 0, 0, 12, 7, 12}

var pitchClass = map[string]int{
	"C": 0, "C#": 1, "Db": 1, "D": 2, "D#": 3, "Eb": 3, "E": 4, "F": 5,
	"F#": 6, "Gb": 6, "G": 7, "G#": 8, "Ab": 8, "A": 9, "A#": 10, "Bb": 10, "B": 11,
}

// midi parses a note like "Bb4" into its MIDI number.
func midi(note string) (int, error) {
	if len(note) < 2 {
		return 0, fmt.Errorf("bad note %q", note)
	}
	name, oct := note[:len(note)-1], note[len(note)-1]
	pc, ok := pitchClass[name]
	if !ok || oct < '0' || oct > '9' {
		return 0, fmt.Errorf("bad note %q", note)
	}
	return 12*(int(oct-'0')+1) + pc, nil
}

// chordNotes are the root (in octave 2, for the bass) and the arpeggio's
// tones above it, as MIDI numbers.
func chordNotes(chord string) (root int, tones []int, err error) {
	name, third := chord, 4
	if n, minor := strings.CutSuffix(chord, "m"); minor {
		name, third = n, 3
	}
	root, err = midi(name + "2")
	if err != nil {
		return 0, nil, fmt.Errorf("bad chord %q", chord)
	}
	r := root + 24 // the arpeggio sits two octaves up
	return root, []int{r, r + third, r + 7, r + 12}, nil
}

func freq(m int) float64 { return 440 * math.Pow(2, float64(m-69)/12) }

// rows is the song as triggers, one per row, parsed once.
type row struct {
	lead     int // MIDI note to start, 0 for none
	release  bool
	bassNote int
	arp      []int
	kick     bool
	snare    bool
	hat      bool
	open     bool    // an accented, longer hat
	cutoff   float64 // bass filter, 0 to 1
	barStart bool
}

func parse(bars []bar) ([]row, error) {
	rows := make([]row, 0, len(bars)*16)
	for bi, b := range bars {
		root, tones, err := chordNotes(b.chord)
		if err != nil {
			return nil, err
		}
		var lead []string
		if b.lead != "" {
			lead = strings.Fields(b.lead)
			if len(lead) != 16 {
				return nil, fmt.Errorf("bar %d: lead has %d rows, want 16", bi, len(lead))
			}
		}
		for i := range 16 {
			r := row{barStart: i == 0}
			if lead != nil {
				switch t := lead[i]; t {
				case "--":
				case "..":
					r.release = true
				default:
					if r.lead, err = midi(t); err != nil {
						return nil, fmt.Errorf("bar %d: %w", bi, err)
					}
				}
			}
			if b.bass {
				r.bassNote = root + bassFigure[i]
			}
			if b.arp {
				r.arp = tones
			}
			r.cutoff = b.filterFrom + (b.filterTo-b.filterFrom)*float64(i)/16
			drums(&r, b.drums, i)
			rows = append(rows, r)
		}
	}
	return rows, nil
}

func drums(r *row, style, i int) {
	switch style {
	case drumsHats:
		r.hat = i%2 == 0
	case drumsBuild:
		r.kick = i%4 == 0
		r.hat = true
		r.snare = i >= 12
	case drumsDrive, drumsFillUp:
		r.kick = i%4 == 0 || i == 10
		r.snare = i == 4 || i == 12
		r.hat = true
		r.open = i%4 == 2
		if style == drumsFillUp && i >= 12 {
			r.snare, r.hat = true, false
		}
	case drumsHalf:
		r.kick = i == 0 || i == 6
		r.snare = i == 8
		r.hat = i%2 == 0
		r.open = i == 14
	}
}
