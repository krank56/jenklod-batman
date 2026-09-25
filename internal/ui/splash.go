package ui

import (
	"math"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The splash is a small Gotham night: a searchlight sweeps the sky, the
// bat-signal lights up, and Batman drops onto a rooftop next to the cat.
// It runs on the shared animation tick (100ms per frame).

const (
	splashSweepEnd  = 18 // beam reaches the signal
	splashSignalEnd = 28 // signal fully lit
	splashDropEnd   = 40 // Batman landed
	splashAutoExit  = 65 // continue without a key press
)

var bigBat = []string{
	`       _==/          i     i          \==_       `,
	`     /XX/            |\___/|            \XX\     `,
	`   /XXXX\            |XXXXX|            /XXXX\   `,
	`  |XXXXXX\_         _XXXXXXX_         _/XXXXXX|  `,
	` XXXXXXXXXXXxxxxxxxXXXXXXXXXXXxxxxxxxXXXXXXXXXXX `,
	`|XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX|`,
	`XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`,
	`|XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX|`,
	` XXXXXX/^^^^"\XXXXXXXXXXXXXXXXXXXXX/^^^^^\XXXXXX `,
	`  |XXX|       \XXX/^^\XXXXX/^^\XXX/       |XXX|  `,
	`    \XX\       \X/    \XXX/    \X/       /XX/    `,
	`       "\       "      \X/      "      /"       `,
}

var smallBat = []string{
	`  /\     /\  `,
	` /XX\_^_/XX\ `,
	`|XXXXXXXXXXX|`,
	` \X/ \X/ \X/ `,
}

var batman = []string{
	`  /\   /\  `,
	`  | \_/ |  `,
	`  ( o o )  `,
	` _/\___/\_ `,
	`/   |||   \`,
	`\___|_|___/`,
}

var catRoof = []string{
	` /\_/\ `,
	`(=^.^=)`,
}

type cellStyle uint8

const (
	cSky cellStyle = iota
	cStar
	cStarDim
	cBeam
	cSignal
	cLogo
	cBuilding
	cWindow
	cBatman
	cCat
	cTitle
	cHint
)

var cellStyles = map[cellStyle]lipgloss.Style{
	cSky:      lipgloss.NewStyle().Background(batBlack),
	cStar:     lipgloss.NewStyle().Background(batBlack).Foreground(lipgloss.Color("#FFFFFF")),
	cStarDim:  lipgloss.NewStyle().Background(batBlack).Foreground(dim),
	cBeam:     lipgloss.NewStyle().Background(batBlack).Foreground(lipgloss.Color("#8A7A2E")),
	cSignal:   lipgloss.NewStyle().Background(batYellow).Foreground(batYellow),
	cLogo:     lipgloss.NewStyle().Background(batYellow).Foreground(batBlack).Bold(true),
	cBuilding: lipgloss.NewStyle().Background(batBlack).Foreground(gotham),
	cWindow:   lipgloss.NewStyle().Background(gotham).Foreground(batYellow),
	cBatman:   lipgloss.NewStyle().Background(batBlack).Foreground(lipgloss.Color("#9CA3AF")).Bold(true),
	cCat:      lipgloss.NewStyle().Background(batBlack).Foreground(lipgloss.Color("#C4B5FD")),
	cTitle:    lipgloss.NewStyle().Background(batBlack).Foreground(batYellow).Bold(true),
	cHint:     lipgloss.NewStyle().Background(batBlack).Foreground(dim),
}

type canvas struct {
	w, h  int
	runes [][]rune
	style [][]cellStyle
}

func newCanvas(w, h int) *canvas {
	c := &canvas{w: w, h: h, runes: make([][]rune, h), style: make([][]cellStyle, h)}
	for y := range c.runes {
		c.runes[y] = []rune(strings.Repeat(" ", w))
		c.style[y] = make([]cellStyle, w)
	}
	return c
}

func (c *canvas) set(x, y int, r rune, s cellStyle) {
	if x >= 0 && y >= 0 && x < c.w && y < c.h {
		c.runes[y][x] = r
		c.style[y][x] = s
	}
}

// text writes a string; spaces are transparent unless opaque is set.
func (c *canvas) text(x, y int, s string, st cellStyle, opaque bool) {
	for i, r := range []rune(s) {
		if r != ' ' || opaque {
			c.set(x+i, y, r, st)
		}
	}
}

func (c *canvas) render() string {
	var b strings.Builder
	for y := 0; y < c.h; y++ {
		start := 0
		for x := 1; x <= c.w; x++ {
			if x == c.w || c.style[y][x] != c.style[y][start] {
				b.WriteString(cellStyles[c.style[y][start]].Render(string(c.runes[y][start:x])))
				start = x
			}
		}
		if y < c.h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

type building struct{ x, w, h int }

type splash struct {
	w, h      int
	stars     [][3]int // x, y, phase
	buildings []building
}

func newSplash(w, h int) splash {
	s := splash{w: w, h: h}
	rng := rand.New(rand.NewSource(1939)) // Detective Comics #27
	for i := 0; i < w*h/40; i++ {
		s.stars = append(s.stars, [3]int{rng.Intn(w), rng.Intn(max(1, h*2/3)), rng.Intn(20)})
	}
	for x := 0; x < w; {
		bw := 5 + rng.Intn(9)
		bh := 3 + rng.Intn(max(1, h/4))
		s.buildings = append(s.buildings, building{x, bw, bh})
		x += bw + rng.Intn(2)
	}
	return s
}

func (s splash) view(frame int) string {
	w, h := s.w, s.h
	if w < 20 || h < 10 {
		return "JENKLOD-BATMAN"
	}
	c := newCanvas(w, h)

	for _, st := range s.stars {
		if (frame+st[2])%20 < 14 {
			c.set(st[0], st[1], '.', cStarDim)
		} else {
			c.set(st[0], st[1], '*', cStar)
		}
	}

	logo := bigBat
	if w < len(bigBat[0])+10 || h < len(bigBat)+16 {
		logo = smallBat
	}
	lw, lh := len([]rune(logo[0])), len(logo)
	cx, cy := w/2, 2+lh/2+1
	rx, ry := float64(lw)/2+4, float64(lh)/2+1.5

	// Searchlight beam from the bottom left towards the signal.
	ox, oy := w/6, h-2
	sweep := math.Min(1, float64(frame)/splashSweepEnd)
	tx := float64(ox) + (float64(cx)-float64(ox))*(0.25+0.75*easeOut(sweep))
	ty := float64(cy) + (1-easeOut(sweep))*float64(h/4)
	for y := oy; y >= int(ty); y-- {
		t := float64(oy-y) / math.Max(1, float64(oy)-ty)
		mid := float64(ox) + (tx-float64(ox))*t
		half := 0.5 + t*rx*0.6
		for x := int(mid - half); x <= int(mid+half); x++ {
			c.set(x, y, '░', cBeam)
		}
	}

	// The signal: an ellipse of light, then the bat inside it.
	if frame >= splashSweepEnd {
		lit := math.Min(1, float64(frame-splashSweepEnd+1)/float64(splashSignalEnd-splashSweepEnd))
		for y := range h {
			for x := range w {
				dx, dy := float64(x-cx)/rx, float64(y-cy)/ry
				if dx*dx+dy*dy <= lit {
					c.set(x, y, ' ', cSignal)
				}
			}
		}
		if lit >= 1 {
			for i, line := range logo {
				for j, r := range []rune(line) {
					x, y := cx-lw/2+j, cy-lh/2+i
					switch r {
					case ' ':
					case 'X', 'x':
						c.set(x, y, '█', cLogo)
					default:
						c.set(x, y, r, cLogo)
					}
				}
			}
		}
	}

	// Skyline with lit windows that flicker now and then.
	ground := h - 1
	for bi, b := range s.buildings {
		top := ground - b.h
		for y := top; y <= ground; y++ {
			for x := b.x; x < b.x+b.w; x++ {
				c.set(x, y, '█', cBuilding)
				if y > top && y < ground && (x-b.x)%2 == 1 && x < b.x+b.w-1 && (x*7+y*13+bi+frame/15)%5 == 0 {
					c.set(x, y, '▪', cWindow)
				}
			}
		}
	}

	// Batman drops onto the roof of a building right of centre; the cat
	// already waits on the next one.
	roofIdx := len(s.buildings) * 2 / 3
	if roofIdx >= len(s.buildings) {
		roofIdx = len(s.buildings) - 1
	}
	roof := s.buildings[roofIdx]
	roofTop := ground - roof.h
	bx := roof.x + roof.w/2 - len([]rune(batman[0]))/2
	if frame >= splashSignalEnd {
		fall := math.Min(1, float64(frame-splashSignalEnd)/float64(splashDropEnd-splashSignalEnd))
		landY := roofTop - len(batman)
		by := int(float64(-len(batman)) + (float64(landY+len(batman)))*easeIn(fall))
		for i, line := range batman {
			c.text(bx, by+i, line, cBatman, false)
		}
	}
	if roofIdx+1 < len(s.buildings) {
		cr := s.buildings[roofIdx+1]
		catLines := catRoof
		if frame%40 >= 38 {
			catLines = []string{catRoof[0], `(=-.-=)`}
		}
		for i, line := range catLines {
			c.text(cr.x+1, ground-cr.h-len(catLines)+i, line, cCat, false)
		}
	}

	if frame >= splashDropEnd {
		title := "J E N K L O D  ·  B A T M A N"
		ty := cy + lh/2 + 3
		c.text(cx-len([]rune(title))/2, ty, title, cTitle, true)
		if frame >= splashDropEnd+5 {
			quote := `"I'm Batman." — and I run your builds.`
			c.text(cx-len([]rune(quote))/2, ty+2, quote, cHint, true)
		}
		if (frame/5)%2 == 0 {
			hint := "press any key"
			c.text(cx-len(hint)/2, ty+4, hint, cHint, true)
		}
	}
	return c.render()
}

func easeOut(t float64) float64 { return 1 - (1-t)*(1-t) }
func easeIn(t float64) float64  { return t * t }
