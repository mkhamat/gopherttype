package mascot

import (
	"image/color"
	"math"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Public renderer contract: a fixed 32x12 frame encoded from 64x48 samples.
// Render mutates cells and the cached ANSI string during Update/preview
// evaluation; View never works, only returns the immutable cached string.
const (
	Width        = 32
	Height       = 12
	SampleWidth  = 64
	SampleHeight = 48
)

// Pose controls the whole head. Angles are degrees; eye openness is 0..1, and
// lift/bob are world units. Zero Pose is a valid forward, closed-eye pose, not
// an implicit neutral sentinel.
type Pose struct {
	Yaw     float64
	Pitch   float64
	EyeOpen float64
	Lift    float64
	Bob     float64
}

// NeutralPose is the accepted baseline camera: yaw 0, pitch 14 down.
func NeutralPose() Pose {
	return Pose{Yaw: 0, Pitch: 14, EyeOpen: 0.95, Lift: 0.06, Bob: 0}
}

// Cell is one terminal cell. Index 0 in either channel means the terminal
// default (ANSI 39/49), never black.
type Cell struct {
	Glyph rune
	FG    uint8
	BG    uint8
}

// Frame is 384 row-major cells: cell (x, y) is at y*Width+x.
type Frame [Width * Height]Cell

// rgb is an 8-bit matching color. Palette index 0 uses the actual background.
type rgb struct{ r, g, b uint8 }

// defaultBackgroundRGB mirrors styles.Background's dark approximation.
var defaultBackgroundRGB = rgb{0x16, 0x1b, 0x22}

// paletteRGB holds palette entries 1..7. Entry 0 is the matching background
// and is filled in per render, so it is left zero here.
var paletteRGB = [8]rgb{
	{},
	{0x35, 0x77, 0x89},
	{0x49, 0x93, 0xa5},
	{0x66, 0xaf, 0xbf},
	{0x85, 0xc7, 0xd2},
	{0xa3, 0xd7, 0xdf},
	{0xf3, 0xf4, 0xe9},
	{0x14, 0x2c, 0x38},
}

var (
	quadrantRunes = [16]rune{' ', '▘', '▝', '▀', '▖', '▌', '▞', '▛', '▗', '▚', '▐', '▜', '▄', '▙', '▟', '█'}
	brailleBits   = [8]int{0, 3, 1, 4, 2, 5, 6, 7}
)

// Renderer owns fixed sample/cell storage, the RGB distance table and the
// cached style-run ANSI string. It is not safe for concurrent use.
type Renderer struct {
	samples [SampleWidth * SampleHeight]sample
	cells   [Width * Height]Cell
	prev    [Width * Height]Cell
	dist    [8][8]float64
	styles  [8][8]string

	scratch []byte
	view    string

	lastPose Pose
	lastBG   rgb
	distBG   rgb
	hasFrame bool
	distSet  bool
}

// NewRenderer builds the renderer with its fixed style cache. It performs no
// rendering and allocates no per-frame scratch.
func NewRenderer() *Renderer {
	r := &Renderer{}
	for fg := 0; fg < 8; fg++ {
		for bg := 0; bg < 8; bg++ {
			r.styles[fg][bg] = ansi.NewStyle().
				ForegroundColor(styleColor(fg)).
				BackgroundColor(styleColor(bg)).
				String()
		}
	}
	return r
}

// styleColor maps a palette index to an ANSI color. Index 0 is the terminal
// default (39/49), not the matching background RGB.
func styleColor(i int) ansi.Color {
	if i == 0 {
		return nil
	}
	c := paletteRGB[i]
	return color.RGBA{R: c.r, G: c.g, B: c.b, A: 0xff}
}

// Frame returns a copy of the current cells for preview/tests.
func (r *Renderer) Frame() Frame {
	return r.cells
}

// View returns the cached ANSI string. It never renders or mutates.
func (r *Renderer) View() string {
	return r.view
}

// Render sanitizes the pose, traces and encodes the scene and refreshes the
// cached ANSI when the appearance changed. Identical sanitized pose and
// background skip raycasting; identical cells skip serialization. It reports
// whether the cached serialized appearance changed.
func (r *Renderer) Render(pose Pose, background color.Color) bool {
	p := sanitizePose(pose)
	bg := backgroundRGB(background)
	if r.hasFrame && p == r.lastPose && bg == r.lastBG {
		return false
	}

	if !r.distSet || bg != r.distBG {
		r.buildDistances(bg)
		r.distBG = bg
		r.distSet = true
	}

	sampleScene(p, &r.samples)
	cells := r.encode(p)
	changed := !r.hasFrame || cells != r.prev
	r.cells = cells
	if changed {
		r.view = r.serialize()
		r.prev = cells
	}
	r.lastPose = p
	r.lastBG = bg
	r.hasFrame = true
	return changed
}

// buildDistances precomputes squared RGB distances. Entry 0 is the matching
// background, so a background change invalidates the whole table.
func (r *Renderer) buildDistances(bg rgb) {
	var colors [8]rgb
	colors[0] = bg
	copy(colors[1:], paletteRGB[1:])
	for a := 0; a < 8; a++ {
		for b := 0; b < 8; b++ {
			dr := float64(colors[a].r) - float64(colors[b].r)
			dg := float64(colors[a].g) - float64(colors[b].g)
			db := float64(colors[a].b) - float64(colors[b].b)
			r.dist[a][b] = dr*dr + dg*dg + db*db
		}
	}
}

// serialize emits style runs: a new SGR sequence only when the FG/BG pair
// changes, then the glyphs, then a reset at each row end. Colored spaces and
// trailing cells are preserved verbatim.
func (r *Renderer) serialize() string {
	r.scratch = r.scratch[:0]
	for y := 0; y < Height; y++ {
		if y > 0 {
			r.scratch = append(r.scratch, '\n')
		}
		prev := -1
		for x := 0; x < Width; x++ {
			c := r.cells[y*Width+x]
			idx := int(c.FG)*8 + int(c.BG)
			if idx != prev {
				r.scratch = append(r.scratch, r.styles[c.FG][c.BG]...)
				prev = idx
			}
			r.scratch = utf8.AppendRune(r.scratch, c.Glyph)
		}
		r.scratch = append(r.scratch, ansi.ResetStyle...)
	}
	return string(r.scratch)
}

// encode gathers each 2x4 sample group in row-major order and encodes it.
func (r *Renderer) encode(pose Pose) [Width * Height]Cell {
	var cells [Width * Height]Cell
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			var group [8]sample
			n := 0
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					group[n] = r.samples[(y*4+dy)*SampleWidth+x*2+dx]
					n++
				}
			}
			cells[y*Width+x] = r.encodeGroup(group, pose)
		}
	}
	return cells
}

// encodeGroup ports the JS encoder exactly.
func (r *Renderer) encodeGroup(group [8]sample, pose Pose) Cell {
	// Distinct present colors, ascending.
	var colors [8]uint8
	n := 0
	for _, s := range group {
		seen := false
		for i := 0; i < n; i++ {
			if colors[i] == s.color {
				seen = true
				break
			}
		}
		if !seen {
			colors[n] = s.color
			n++
		}
	}
	for i := 1; i < n; i++ {
		for j := i; j > 0 && colors[j-1] > colors[j]; j-- {
			colors[j-1], colors[j] = colors[j], colors[j-1]
		}
	}

	if n == 1 {
		return Cell{Glyph: ' ', FG: colors[0], BG: colors[0]}
	}

	// Enumerate ascending pairs lexicographically; strict `<` keeps the first
	// pair on a tie. If 0 occurs, only pairs containing 0 qualify.
	best := math.Inf(1)
	var fg, bg uint8
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			a, b := colors[i], colors[j]
			if colors[0] == 0 && a != 0 {
				continue
			}
			var err float64
			for _, s := range group {
				d := r.dist[s.color][a]
				if r.dist[s.color][b] < d {
					d = r.dist[s.color][b]
				}
				err += float64(weightOf(s.role)) * d
			}
			if err < best {
				best = err
				bg = a
				fg = b
			}
		}
	}
	if bg != 0 {
		fgCount, bgCount := 0, 0
		for _, s := range group {
			switch s.color {
			case fg:
				fgCount++
			case bg:
				bgCount++
			}
		}
		if fgCount > bgCount {
			fg, bg = bg, fg
		}
	}

	// Face roles take precedence over mouth; solid quadrant coverage.
	face := false
	var mouthCount int
	var mouthSumX float64
	var mouthUnder uint8
	for _, s := range group {
		switch s.role {
		case roleEye, rolePupil, roleNose, roleTooth:
			face = true
		case roleMouth:
			if mouthCount == 0 {
				mouthUnder = s.under
			}
			mouthSumX += s.x
			mouthCount++
		}
	}

	if !face && mouthCount > 0 {
		x := mouthSumX / float64(mouthCount)
		glyph := '─'
		if math.Abs(x) > 0.15 {
			if pose.Lift < 0 {
				if x < 0 {
					glyph = '╭'
				} else {
					glyph = '╮'
				}
			} else {
				if x < 0 {
					glyph = '╰'
				} else {
					glyph = '╯'
				}
			}
		}
		return Cell{Glyph: glyph, FG: 7, BG: mouthUnder}
	}

	if face {
		// TL/TR/BL/BR group two vertically adjacent samples each.
		quadrants := [4][2]int{{0, 2}, {1, 3}, {4, 6}, {5, 7}}
		var mask int
		for q := 0; q < 4; q++ {
			var errFG, errBG float64
			for _, i := range quadrants[q] {
				s := group[i]
				w := float64(weightOf(s.role))
				errFG += w * r.dist[s.color][fg]
				errBG += w * r.dist[s.color][bg]
			}
			if errFG < errBG {
				mask |= 1 << q
			}
		}
		return Cell{Glyph: quadrantRunes[mask], FG: fg, BG: bg}
	}

	// Otherwise Braille, using the ported bit order. Ties choose BG.
	var mask int
	for i, s := range group {
		if r.dist[s.color][fg] < r.dist[s.color][bg] {
			mask |= 1 << brailleBits[i]
		}
	}
	glyph := ' '
	if mask != 0 {
		glyph = rune(0x2800 + mask)
	}
	return Cell{Glyph: glyph, FG: fg, BG: bg}
}

// weightOf gives dark facial features a stronger say in pair selection.
func weightOf(rl role) int {
	switch rl {
	case rolePupil, roleNose, roleMouth, roleTooth:
		return 3
	default:
		return 1
	}
}

// sanitizePose clamps finite fields and falls back to neutral for NaN/Inf.
func sanitizePose(p Pose) Pose {
	n := NeutralPose()
	return Pose{
		Yaw:     finiteClamp(p.Yaw, -35, 35, n.Yaw),
		Pitch:   finiteClamp(p.Pitch, 0, 20, n.Pitch),
		EyeOpen: finiteClamp(p.EyeOpen, 0, 1, n.EyeOpen),
		Lift:    finiteClamp(p.Lift, -0.035, 0.10, n.Lift),
		Bob:     finiteClamp(p.Bob, -0.06, 0.06, n.Bob),
	}
}

func finiteClamp(v, lo, hi, fallback float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fallback
	}
	return clamp(v, lo, hi)
}

// backgroundRGB converts the background color to an 8-bit matching RGB. Nil
// uses the documented dark approximation. The high byte of each channel is
// used for RGBA colors, preserving exact fixture hex values.
func backgroundRGB(c color.Color) rgb {
	if c == nil {
		return defaultBackgroundRGB
	}
	r, g, b, _ := c.RGBA()
	return rgb{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}
