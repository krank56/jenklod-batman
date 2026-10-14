package music

import (
	"bytes"
	"math"
	"testing"
	"time"
)

func TestSongParses(t *testing.T) {
	if _, err := parse(song); err != nil {
		t.Fatal(err)
	}
	// About a minute and a half, as promised in the docs.
	d := time.Duration(Length()) * time.Second / SampleRate
	if d < 80*time.Second || d > 100*time.Second {
		t.Errorf("song lasts %v, want about 1m30s", d)
	}
}

func TestParseRejectsBadBars(t *testing.T) {
	for _, b := range []bar{
		{chord: "Hm"},
		{chord: "Dm", lead: "D4 --"},
		{chord: "Dm", lead: "D4 -- -- -- -- -- -- -- -- -- -- -- -- -- -- X9"},
	} {
		if _, err := parse([]bar{b}); err == nil {
			t.Errorf("parse(%+v) = nil error", b)
		}
	}
}

func TestMidi(t *testing.T) {
	for note, want := range map[string]int{"A4": 69, "D4": 62, "Bb4": 70, "C#5": 73, "C0": 12} {
		if got, err := midi(note); err != nil || got != want {
			t.Errorf("midi(%q) = %d, %v; want %d", note, got, err, want)
		}
	}
}

func render(n int) [][2]float64 {
	buf := make([][2]float64, n)
	NewTune().Stream(buf)
	return buf
}

func rms(s [][2]float64) float64 {
	var sum float64
	for _, x := range s {
		sum += x[0]*x[0] + x[1]*x[1]
	}
	return math.Sqrt(sum / float64(2*len(s)))
}

func TestTuneIsAudibleAndBounded(t *testing.T) {
	buf := render(10 * SampleRate)
	for i, s := range buf {
		if math.IsNaN(s[0]) || math.Abs(s[0]) > 1 || math.Abs(s[1]) > 1 {
			t.Fatalf("sample %d = %v, outside [-1, 1]", i, s)
		}
	}
	// The first two seconds are the quiet intro; the A section is louder.
	intro, a := rms(buf[:2*SampleRate]), rms(buf[8*SampleRate:])
	if intro < 0.01 {
		t.Errorf("intro rms %.4f: silent", intro)
	}
	if a <= intro {
		t.Errorf("A section rms %.4f not louder than intro %.4f", a, intro)
	}
}

func TestTuneIsDeterministic(t *testing.T) {
	a, b := render(SampleRate), render(SampleRate)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("sample %d differs: %v vs %v", i, a[i], b[i])
		}
	}
}

func TestTuneLoops(t *testing.T) {
	tune := NewTune()
	buf := make([][2]float64, rowSamples)
	for range Length() / rowSamples {
		tune.Stream(buf)
	}
	if tune.pos != 0 || tune.Row() != 0 {
		t.Errorf("after one pass at sample %d, row %d; want back at the top", tune.pos, tune.Row())
	}
	tune.Stream(buf)
	if tune.Row() != 1 {
		t.Errorf("row %d after one more sixteenth, want 1", tune.Row())
	}
}

func TestGain(t *testing.T) {
	if gain(0) != 0 || gain(100) != 1 || gain(50) != 0.25 {
		t.Errorf("gain(0, 50, 100) = %v, %v, %v", gain(0), gain(50), gain(100))
	}
}

func TestWriteWAV(t *testing.T) {
	if testing.Short() {
		t.Skip("renders the whole song")
	}
	var b bytes.Buffer
	if err := WriteWAV(&b); err != nil {
		t.Fatal(err)
	}
	if got, want := b.Len(), 44+4*Length(); got != want {
		t.Errorf("WAV is %d bytes, want %d", got, want)
	}
	if !bytes.HasPrefix(b.Bytes(), []byte("RIFF")) {
		t.Error("no RIFF header")
	}
}
