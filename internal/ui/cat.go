package ui

import (
	"math/rand"
	"strings"
)

// cat is the little companion wandering along the status bar. It is purely
// decorative: its mood is random, not tied to build results.
type cat struct {
	x     int
	dir   int // +1 right, -1 left
	state catState
	ticks int // ticks left in the current state
	age   int
	rng   *rand.Rand
}

type catState int

const (
	catWalk catState = iota
	catSit
	catGroom
	catSleep
)

const catWidth = 10

// Two-line sprites, each line catWidth wide.
var (
	catWalkRight = [][2]string{
		{"   /\\_/\\  ", " ~(=o.o=) "},
		{"   /\\_/\\  ", " _(=o.o=) "},
	}
	catWalkLeft = [][2]string{
		{"  /\\_/\\   ", " (=o.o=)~ "},
		{"  /\\_/\\   ", " (=o.o=)_ "},
	}
	catSitFrames = [][2]string{
		{"  /\\_/\\   ", " (=^.^=)~ "},
		{"  /\\_/\\   ", " (=-.-=)~ "}, // blink
	}
	catGroomFrames = [][2]string{
		{"  /\\_/\\   ", " (=^ω^=)/ "},
		{"  /\\_/\\   ", " (=^ω^=)\\ "},
	}
	catSleepFrames = [][2]string{
		{"  /\\_/\\ z ", " (=-.-=)  "},
		{"  /\\_/\\ zZ", " (=-.-=)  "},
		{"  /\\_/\\ Zz", " (=-ᴗ-=)  "},
	}
)

func newCat(seed int64) cat {
	return cat{dir: 1, state: catWalk, ticks: 60, rng: rand.New(rand.NewSource(seed))}
}

// step advances one animation tick within a lane of the given width.
func (c *cat) step(width int) {
	c.age++
	maxX := max(width-catWidth, 0)
	if c.x > maxX {
		c.x = maxX
	}
	if c.state == catWalk && c.age%2 == 0 {
		c.x += c.dir
		if c.x <= 0 || c.x >= maxX {
			c.dir = -c.dir
			c.x = clamp(c.x, 0, maxX)
		}
	}
	c.ticks--
	if c.ticks > 0 {
		return
	}
	// Pick the next mood. Sleeping cats mostly keep sleeping.
	r := c.rng.Intn(100)
	switch {
	case c.state == catSleep && r < 50:
		c.ticks = 80
	case r < 45:
		c.state, c.ticks = catWalk, 40+c.rng.Intn(120)
		if c.rng.Intn(3) == 0 {
			c.dir = -c.dir
		}
	case r < 70:
		c.state, c.ticks = catSit, 30+c.rng.Intn(50)
	case r < 85:
		c.state, c.ticks = catGroom, 20+c.rng.Intn(30)
	default:
		c.state, c.ticks = catSleep, 100+c.rng.Intn(150)
	}
}

func (c *cat) sprite() [2]string {
	switch c.state {
	case catWalk:
		frames := catWalkRight
		if c.dir < 0 {
			frames = catWalkLeft
		}
		return frames[(c.age/3)%len(frames)]
	case catSit:
		if c.age%30 < 2 {
			return catSitFrames[1]
		}
		return catSitFrames[0]
	case catGroom:
		return catGroomFrames[(c.age/4)%len(catGroomFrames)]
	default:
		return catSleepFrames[(c.age/8)%len(catSleepFrames)]
	}
}

// view renders the two-line lane.
func (c *cat) view(width int) string {
	sp := c.sprite()
	pad := strings.Repeat(" ", clamp(c.x, 0, max(0, width-catWidth)))
	return sCat.Render(pad+sp[0]) + "\n" + sCat.Render(pad+sp[1])
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
