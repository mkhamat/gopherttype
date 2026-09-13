package mascot

import (
	"image/color"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Public renderer contract: a fixed 32x12 frame encoded from 128x96 samples.
// Render mutates cells and the cached ANSI string during Update/preview
// evaluation; View never works, only returns the immutable cached string.
const (
	Width        = 32
	Height       = 12
	SampleWidth  = 128
	SampleHeight = 96
)

// Cell sampling geometry. Each cell integrates cellRows x cellCols samples
// split into four quadrants of quadSize samples each.
const (
	cellCols = SampleWidth / Width   // 4
	cellRows = SampleHeight / Height // 8
	quadSize = (cellCols / 2) * (cellRows / 2)
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

// NeutralPose is the rig's resting pose: yaw 0 and pitch 0, facing straight
// ahead, matching art/mascot-rig.mjs `neutral`. It is also the per-field
// fallback for non-finite pose inputs, so it must equal the rig neutral.
func NeutralPose() Pose {
	return Pose{Yaw: 0, Pitch: 0, EyeOpen: 0.95, Lift: 0.06, Bob: 0}
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

// paletteRGB holds palette entries 1..8. Entry 0 is the matching background
// and is filled in per render, so it is left zero here.
var paletteRGB = [9]rgb{
	{},
	{0x34, 0x7b, 0x91}, // #347b91
	{0x52, 0x9a, 0xaf}, // #529aaf
	{0x70, 0xb7, 0xc8}, // #70b7c8
	{0x8d, 0xcd, 0xdb}, // #8dcddb
	{0xb2, 0xe1, 0xe8}, // #b2e1e8
	{0xff, 0xff, 0xff}, // #ffffff
	{0x10, 0x27, 0x2f}, // #10272f
	{0xef, 0xd0, 0xa0}, // #efd0a0
}

var quadrantRunes = [16]rune{' ', '▘', '▝', '▀', '▖', '▌', '▞', '▛', '▗', '▚', '▐', '▜', '▄', '▙', '▟', '█'}

// Renderer owns fixed sample/cell storage, the RGB distance table and the
// cached style-run ANSI string. It is not safe for concurrent use.
type Renderer struct {
	samples [SampleWidth * SampleHeight]sample
	cells   [Width * Height]Cell
	dist    [9][9]float64
	styles  [9][9]string

	scratch []byte
	view    string

	lastPose Pose
	lastBG   rgb
	hasFrame bool
}

// NewRenderer builds the renderer with its fixed style cache. It performs no
// rendering and allocates no per-frame scratch.
func NewRenderer() *Renderer {
	r := &Renderer{}
	for fg := 0; fg < 9; fg++ {
		for bg := 0; bg < 9; bg++ {
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

	if !r.hasFrame || bg != r.lastBG {
		r.buildDistances(bg)
	}

	sampleScene(p, &r.samples)
	cells := r.encode()
	changed := !r.hasFrame || cells != r.cells
	if changed {
		r.cells = cells
		r.view = r.serialize()
	}
	r.lastPose = p
	r.lastBG = bg
	r.hasFrame = true
	return changed
}

// buildDistances precomputes squared RGB distances. Entry 0 is the matching
// background, so a background change invalidates the whole table.
func (r *Renderer) buildDistances(bg rgb) {
	var colors [9]rgb
	colors[0] = bg
	copy(colors[1:], paletteRGB[1:])
	for a := 0; a < 9; a++ {
		for b := 0; b < 9; b++ {
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
			idx := int(c.FG)*9 + int(c.BG)
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

func (r *Renderer) encode() [Width * Height]Cell {
	var cells [Width * Height]Cell
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			var q [4][quadSize]sample
			var n [4]int
			for dy := 0; dy < cellRows; dy++ {
				for dx := 0; dx < cellCols; dx++ {
					qi := 0
					if dy >= cellRows/2 {
						qi += 2
					}
					if dx >= cellCols/2 {
						qi++
					}
					q[qi][n[qi]] = r.samples[(y*cellRows+dy)*SampleWidth+x*cellCols+dx]
					n[qi]++
				}
			}
			cells[y*Width+x] = r.encodeCell(&q)
		}
	}
	return cells
}

// encodeCell ports the JS encoder exactly. Colors are the distinct present
// palette indices ascending; the best two-color pair minimizes the sum of
// role-weighted squared RGB errors, with a strict `<` tie rule and quadrant
// bits set where the higher-index (foreground) color is closer.
func (r *Renderer) encodeCell(q *[4][quadSize]sample) Cell {
	var colors [9]uint8
	n := 0
	for qi := 0; qi < 4; qi++ {
		for _, s := range q[qi] {
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
	}
	slices.Sort(colors[:n])

	if n == 1 {
		return Cell{Glyph: ' ', FG: colors[0], BG: colors[0]}
	}

	var errors [4][9]float64
	for qi := 0; qi < 4; qi++ {
		for ci := 0; ci < n; ci++ {
			var sum float64
			for _, s := range q[qi] {
				sum += weightOf(s.role) * r.dist[s.color][colors[ci]]
			}
			errors[qi][ci] = sum
		}
	}

	// Enumerate ascending pairs lexicographically; strict `<` keeps the first
	// pair on a tie. If 0 occurs, only pairs containing 0 qualify.
	best := math.Inf(1)
	var fg, bg uint8
	var mask int
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if colors[0] == 0 && colors[i] != 0 {
				continue
			}
			var err float64
			var bits int
			for qi := 0; qi < 4; qi++ {
				err += math.Min(errors[qi][i], errors[qi][j])
				if errors[qi][j] < errors[qi][i] {
					bits |= 1 << qi
				}
			}
			if err < best {
				best = err
				bg = colors[i]
				fg = colors[j]
				mask = bits
			}
		}
	}
	return Cell{Glyph: quadrantRunes[mask], FG: fg, BG: bg}
}

// weightOf gives dark facial features a stronger say in pair selection.
func weightOf(rl role) float64 {
	switch rl {
	case rolePupil, roleNose, roleMouth, roleTooth, roleSeam, roleEar:
		return 2
	case roleMuzzle:
		return 1.5
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
