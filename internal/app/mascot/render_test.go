package mascot

import (
	"image/color"
	"math"
	"strings"
	"testing"
	"unsafe"

	"github.com/charmbracelet/x/ansi"
)

// TestOutputShape guards the fixed 32x12 frame and row-end resets.
func TestOutputShape(t *testing.T) {
	r := NewRenderer()
	if got := r.View(); got != "" {
		t.Errorf("View before Render should be empty, got %q", got)
	}
	r.Render(NeutralPose(), nil)
	view := r.View()

	if n := strings.Count(view, "\n"); n != Height-1 {
		t.Errorf("want %d internal newlines, got %d", Height-1, n)
	}
	lines := strings.Split(view, "\n")
	if len(lines) != Height {
		t.Fatalf("want %d rows, got %d", Height, len(lines))
	}
	filled := 0
	for y, line := range lines {
		if w := ansi.StringWidth(ansi.Strip(line)); w != Width {
			t.Errorf("row %d width %d, want %d", y, w, Width)
		}
		if !strings.HasSuffix(line, ansi.ResetStyle) {
			t.Errorf("row %d missing reset suffix", y)
		}
	}
	for _, c := range r.Frame() {
		if c.Glyph == ' ' && c.BG != 0 {
			filled++
		}
	}
	if filled == 0 {
		t.Error("expected filled background spaces to be preserved")
	}
	if c := r.Frame()[0]; c.FG != 0 || c.BG != 0 {
		t.Errorf("outside-the-silhouette corner should be terminal default, got %+v", c)
	}
}

// TestDefaultChannelResets checks index 0 emits terminal-default FG/BG and that
// filled spaces (body/eye/tooth) survive serialization.
func TestDefaultChannelResets(t *testing.T) {
	r := NewRenderer()
	r.Render(NeutralPose(), nil)
	if view := r.View(); !strings.Contains(view, "\x1b[39;49m") {
		t.Error("default channel must emit 39;49 so old art is cleared")
	}
	filled, solids := 0, 0
	for _, c := range r.Frame() {
		if c.Glyph != ' ' {
			continue
		}
		if c.BG != 0 {
			filled++
		}
		if c.FG == c.BG && c.BG != 0 {
			solids++
		}
	}
	if filled == 0 {
		t.Error("expected filled colored spaces")
	}
	if solids == 0 {
		t.Error("expected solid one-color spaces (eye/tooth/body)")
	}
}

// TestPoseSanitizationAndCache guards clamping, non-finite fallback and the
// pose/background/cell caches.
func TestPoseSanitizationAndCache(t *testing.T) {
	clamped := NewRenderer()
	clamped.Render(Pose{Yaw: 1e3, Pitch: 1e3, EyeOpen: 1e3, Lift: 1e3, Bob: 1e3}, nil)
	atLimits := NewRenderer()
	atLimits.Render(Pose{Yaw: 35, Pitch: 20, EyeOpen: 1, Lift: 0.10, Bob: 0.06}, nil)
	if clamped.Frame() != atLimits.Frame() {
		t.Error("finite out-of-range pose should clamp to the limits")
	}

	n := NeutralPose()
	nonFinite := NewRenderer()
	nonFinite.Render(Pose{Yaw: math.NaN(), Pitch: math.Inf(1), EyeOpen: math.Inf(-1), Lift: math.NaN(), Bob: math.Inf(1)}, nil)
	neutral := NewRenderer()
	neutral.Render(n, nil)
	if nonFinite.Frame() != neutral.Frame() {
		t.Error("non-finite fields should fall back to neutral")
	}

	if neutral.Frame() == clamped.Frame() {
		t.Error("NeutralPose must not equal the clamped-limit pose")
	}

	r := NewRenderer()
	if !r.Render(n, nil) {
		t.Error("first Render should report a change")
	}
	if r.Render(n, nil) {
		t.Error("identical sanitized pose/background should be cached")
	}
	first := r.View()
	if !r.Render(Pose{Yaw: 35, Pitch: 14, EyeOpen: 0.95, Lift: 0.06, Bob: 0}, nil) {
		t.Error("changed pose should report a change")
	}
	if r.View() == first {
		t.Error("view should differ after a changed pose")
	}
	if !r.Render(n, nil) || r.View() != first {
		t.Error("returning to the previous pose must reproduce the earlier immutable view")
	}

	// A background change must recompute. The neutral pose happens to quantize
	// identically on dark and light, so use a pose whose cells differ.
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	changing := Pose{Yaw: 35, Pitch: 14, EyeOpen: 0.95, Lift: 0.06, Bob: 0}
	dark := NewRenderer()
	dark.Render(changing, nil)
	if !dark.Render(changing, white) {
		t.Error("background change must recompute when it changes cells")
	}
	if dark.Render(changing, white) {
		t.Error("same background and pose should be cached")
	}
}

// TestSanitizePoseFields pins the per-field fallback, the literal zero Pose and
// the omitted-field mapping (a JS omission is neutral, a Go zero value is not).
func TestSanitizePoseFields(t *testing.T) {
	n := NeutralPose()

	if got := sanitizePose(Pose{Yaw: math.NaN(), Pitch: math.Inf(1), EyeOpen: math.Inf(-1), Lift: math.NaN(), Bob: math.Inf(1)}); got != n {
		t.Errorf("non-finite fields should fall back to neutral, got %+v", got)
	}
	if got := sanitizePose(Pose{Yaw: 10, Pitch: math.NaN()}); got.Yaw != 10 || got.Pitch != n.Pitch {
		t.Errorf("only the non-finite field should fall back, got %+v", got)
	}

	// Zero Pose is a valid literal zero, not an implicit neutral sentinel.
	if got := sanitizePose(Pose{}); got != (Pose{}) {
		t.Errorf("zero Pose should stay literal zero, got %+v", got)
	}
	if (Pose{}) == n {
		t.Fatal("zero Pose must not equal NeutralPose")
	}

	if got := sanitizePose(Pose{Yaw: 100, Pitch: 50, EyeOpen: 2, Lift: 5, Bob: 1}); got != (Pose{Yaw: 35, Pitch: 20, EyeOpen: 1, Lift: 0.10, Bob: 0.06}) {
		t.Errorf("high clamps wrong: %+v", got)
	}
	if got := sanitizePose(Pose{Yaw: -100, Pitch: -50, EyeOpen: -1, Lift: -5, Bob: -1}); got != (Pose{Yaw: -35, Pitch: 0, EyeOpen: 0, Lift: -0.035, Bob: -0.06}) {
		t.Errorf("low clamps wrong: %+v", got)
	}
}

// TestBackgroundChangeRebuildsDistances proves a different actual RGB under the
// same theme rebuilds the matching distances and re-encodes, while a repeated
// pose/RGB is a no-op.
func TestBackgroundChangeRebuildsDistances(t *testing.T) {
	r := NewRenderer()
	pose := NeutralPose()
	dark1 := color.RGBA{R: 0x16, G: 0x1b, B: 0x22, A: 0xff}
	dark2 := color.RGBA{R: 0x16, G: 0x1b, B: 0x23, A: 0xff}

	r.Render(pose, dark1)
	before := r.dist[1][0]
	r.Render(pose, dark2)
	after := r.dist[1][0]
	if r.distBG != (rgb{0x16, 0x1b, 0x23}) {
		t.Errorf("distance background key = %+v, want #161b23", r.distBG)
	}
	if before == after {
		t.Error("distance table must change with the background RGB")
	}
	if r.Render(pose, dark2) {
		t.Error("identical pose/background should reuse the cached result")
	}
}

// TestIdenticalCellsReuseCachedString checks that a changed pose which still
// quantizes to the same cells keeps the exact previously returned string.
func TestIdenticalCellsReuseCachedString(t *testing.T) {
	r := NewRenderer()
	base := NeutralPose()
	r.Render(base, nil)
	first := r.View()
	firstPtr := unsafe.StringData(first)

	// Pitch boundaries near 14 are ~0.006 away, so +0.005 is a stable
	// different sanitized pose with byte-identical cells.
	alt := base
	alt.Pitch = 14.005
	if r.Render(alt, nil) {
		t.Fatal("pose with identical cells must not report a change")
	}
	if got := r.View(); got != first {
		t.Error("cached view changed for identical cells")
	}
	if unsafe.StringData(r.View()) != firstPtr {
		t.Error("identical cells must not allocate a new ANSI string")
	}
	if first != r.View() {
		t.Error("previously returned string must stay unchanged")
	}
}

// --- encoder-level cases ---------------------------------------------------
//
// These drive the private encoder with explicit sample groups and distance
// tables, so every mask, bit and tie rule is exercised without geometry. The
// end-to-end parity fixtures cover the real combinations.

func encRenderer(dist [8][8]float64) *Renderer {
	r := NewRenderer()
	r.dist = dist
	r.distSet = true
	return r
}

// pairDist builds the minimal table for two present colors: zero self distance
// and equal cross distance.
func pairDist(a, b uint8) [8][8]float64 {
	var d [8][8]float64
	d[a][a], d[b][b] = 0, 0
	d[a][b], d[b][a] = 1, 1
	return d
}

func encSample(c uint8, rl role) sample { return sample{color: c, role: rl} }

var quadrantIndex = [4][2]int{{0, 2}, {1, 3}, {4, 6}, {5, 7}}

// quadrantGroup builds a two-color group (fg 6, bg 1) whose face mask is
// exactly `mask`. Mixed quadrants keep the foreground count at or below the
// background count so the FG-count rule never swaps the two.
func quadrantGroup(mask int) [8]sample {
	setFull := [2]sample{encSample(6, roleTooth), encSample(6, roleFur)}
	setMixed := [2]sample{encSample(6, roleTooth), encSample(1, roleFur)}
	clearFull := [2]sample{encSample(1, roleEye), encSample(1, roleFur)}
	clearMixed := [2]sample{encSample(1, roleEye), encSample(6, roleFur)}

	var setIdx []int
	for i := 0; i < 4; i++ {
		if mask&(1<<i) != 0 {
			setIdx = append(setIdx, i)
		}
	}

	var q [4][2]sample
	for i := 0; i < 4; i++ {
		q[i] = clearFull
	}
	switch len(setIdx) {
	case 0:
		q[0] = clearMixed // keep a second color present
	case 1:
		q[setIdx[0]] = setFull
	case 2:
		q[setIdx[0]] = setFull
		q[setIdx[1]] = setFull
	case 3:
		q[setIdx[0]] = setFull
		q[setIdx[1]] = setMixed
		q[setIdx[2]] = setMixed
	case 4:
		q[0], q[1], q[2], q[3] = setMixed, setMixed, setMixed, setMixed
	}

	var g [8]sample
	for i := 0; i < 4; i++ {
		g[quadrantIndex[i][0]] = q[i][0]
		g[quadrantIndex[i][1]] = q[i][1]
	}
	return g
}

func TestEncoderQuadrantMasks(t *testing.T) {
	r := encRenderer(pairDist(1, 6))
	pose := NeutralPose()
	for mask := 0; mask < 16; mask++ {
		got := r.encodeGroup(quadrantGroup(mask), pose)
		if got.Glyph != quadrantRunes[mask] || got.FG != 6 || got.BG != 1 {
			t.Errorf("mask %d: got %q fg=%d bg=%d, want %q fg=6 bg=1", mask, got.Glyph, got.FG, got.BG, quadrantRunes[mask])
		}
	}
}

func TestEncoderBrailleBits(t *testing.T) {
	r := encRenderer(pairDist(1, 6))
	pose := NeutralPose()
	for dot := 0; dot < 8; dot++ {
		var g [8]sample
		for i := range g {
			g[i] = encSample(1, roleFur)
		}
		for i := range g {
			if brailleBits[i] == dot {
				g[i] = encSample(6, roleFur)
			}
		}
		want := rune(0x2800 + 1<<dot)
		got := r.encodeGroup(g, pose)
		if got.Glyph != want || got.FG != 6 || got.BG != 1 {
			t.Errorf("dot %d: got %q U+%04X fg=%d bg=%d, want %q U+%04X", dot, got.Glyph, got.Glyph, got.FG, got.BG, want, want)
		}
	}
}

// TestEncoderBrailleZeroMask checks the empty-mask branch emits a plain space
// even when two colors are present (not just the one-color shortcut).
func TestEncoderBrailleZeroMask(t *testing.T) {
	var d [8][8]float64
	d[6][6], d[6][1] = 1, 0
	d[1][1], d[1][6] = 0, 1
	r := encRenderer(d)

	var g [8]sample
	for i := range g {
		g[i] = encSample(1, roleFur)
	}
	g[0] = encSample(6, roleFur)
	got := r.encodeGroup(g, NeutralPose())
	if got.Glyph != ' ' || got.FG != 6 || got.BG != 1 {
		t.Errorf("zero mask should be a plain space with two colors, got %q fg=%d bg=%d", got.Glyph, got.FG, got.BG)
	}
}

func TestEncoderOneColorSpace(t *testing.T) {
	r := encRenderer(pairDist(1, 6))
	pose := NeutralPose()

	var background [8]sample
	for i := range background {
		background[i] = sample{color: 0, role: roleBackground}
	}
	if got := r.encodeGroup(background, pose); got.Glyph != ' ' || got.FG != 0 || got.BG != 0 {
		t.Errorf("all-background group: got %q fg=%d bg=%d, want space 0/0", got.Glyph, got.FG, got.BG)
	}

	// The one-color shortcut fires before face/mouth logic, even with a face role.
	var solid [8]sample
	for i := range solid {
		solid[i] = sample{color: 5, role: roleFur}
	}
	solid[0].role = roleEye
	if got := r.encodeGroup(solid, pose); got.Glyph != ' ' || got.FG != 5 || got.BG != 5 {
		t.Errorf("solid filled group: got %q fg=%d bg=%d, want space 5/5", got.Glyph, got.FG, got.BG)
	}

	var mouth [8]sample
	for i := range mouth {
		mouth[i] = sample{color: 7, role: roleMouth, under: 3, x: 0.2}
	}
	if got := r.encodeGroup(mouth, pose); got.Glyph != ' ' || got.FG != 7 || got.BG != 7 {
		t.Errorf("all-mouth single color: got %q fg=%d bg=%d, want space 7/7", got.Glyph, got.FG, got.BG)
	}
}

func TestEncoderForegroundCountSwap(t *testing.T) {
	r := encRenderer(pairDist(1, 6))
	pose := NeutralPose()

	// Equal counts (4 fg / 4 bg) must not swap: fg is the larger index.
	if got := r.encodeGroup(quadrantGroup(0b0011), pose); got.FG != 6 || got.BG != 1 {
		t.Errorf("4/4 counts must not swap, got fg=%d bg=%d", got.FG, got.BG)
	}

	// Strictly more foreground (5 vs 3) swaps.
	var g [8]sample
	for i := range g {
		g[i] = encSample(1, roleFur)
	}
	for i := 0; i < 5; i++ {
		g[i] = encSample(6, roleFur)
	}
	if got := r.encodeGroup(g, pose); got.FG != 1 || got.BG != 6 {
		t.Errorf("5/3 counts must swap, got fg=%d bg=%d", got.FG, got.BG)
	}
}

// TestEncoderDistanceTieChoosesBackground checks an equidistant sample leaves
// its Braille bit clear.
func TestEncoderDistanceTieChoosesBackground(t *testing.T) {
	var d [8][8]float64
	d[6][6], d[6][1] = 1, 1
	d[1][1], d[1][6] = 0, 2
	r := encRenderer(d)

	var g [8]sample
	for i := range g {
		g[i] = encSample(1, roleFur)
	}
	g[0] = encSample(6, roleFur)
	got := r.encodeGroup(g, NeutralPose())
	if got.Glyph != ' ' || got.FG != 6 || got.BG != 1 {
		t.Errorf("distance tie should clear the bit, got %q fg=%d bg=%d", got.Glyph, got.FG, got.BG)
	}
}

func TestRoleWeighting(t *testing.T) {
	for _, tc := range []struct {
		rl   role
		want int
	}{
		{roleBackground, 1}, {roleFur, 1}, {roleEye, 1},
		{rolePupil, 3}, {roleNose, 3}, {roleMouth, 3}, {roleTooth, 3},
	} {
		if got := weightOf(tc.rl); got != tc.want {
			t.Errorf("weightOf(%s) = %d, want %d", roleName(tc.rl), got, tc.want)
		}
	}
}

// TestEncoderFaceOverMouth checks a face role wins over mouth samples.
func TestEncoderFaceOverMouth(t *testing.T) {
	r := encRenderer(pairDist(6, 7))
	var g [8]sample
	for i := range g {
		g[i] = sample{color: 7, role: roleMouth, under: 2, x: 0.2}
	}
	// Quadrant 0 (indices 0 and 2) becomes the foreground color 6.
	g[0] = sample{color: 6, role: roleEye, under: 2}
	g[2] = sample{color: 6, role: roleMouth, under: 2, x: 0.2}
	got := r.encodeGroup(g, NeutralPose())
	if got.Glyph != '▘' || got.FG != 6 || got.BG != 7 {
		t.Errorf("face should win over mouth: got %q fg=%d bg=%d, want ▘ fg=6 bg=7", got.Glyph, got.FG, got.BG)
	}
}

// TestEncoderMouthGlyphsAndUnderShade checks the smile line/corner rule and that
// BG is the first visible mouth sample's underlying shade.
func TestEncoderMouthGlyphsAndUnderShade(t *testing.T) {
	r := encRenderer(pairDist(3, 7))
	group := func(xs [4]float64, unders [4]uint8) [8]sample {
		var g [8]sample
		for i := 0; i < 4; i++ {
			g[i] = sample{color: 7, role: roleMouth, under: unders[i], x: xs[i]}
		}
		for i := 4; i < 8; i++ {
			g[i] = encSample(3, roleFur)
		}
		return g
	}

	smile := Pose{Lift: 0.06}
	frown := Pose{Lift: -0.035}

	// Midline on both sides of zero averages to |x| <= .15.
	if got := r.encodeGroup(group([4]float64{0.05, 0.10, 0.10, 0.05}, [4]uint8{3, 5, 1, 2}), smile); got.Glyph != '─' || got.FG != 7 || got.BG != 3 {
		t.Errorf("midline mouth: got %q fg=%d bg=%d, want ─ fg=7 bg=3", got.Glyph, got.FG, got.BG)
	}
	// BG is the first sample's under shade (3), not the later 5/1/2.
	if got := r.encodeGroup(group([4]float64{0.2, 0.2, 0.2, 0.2}, [4]uint8{4, 4, 4, 4}), smile); got.Glyph != '╯' || got.BG != 4 {
		t.Errorf("right smile: got %q bg=%d, want ╯ bg=4", got.Glyph, got.BG)
	}
	if got := r.encodeGroup(group([4]float64{-0.2, -0.2, -0.2, -0.2}, [4]uint8{4, 4, 4, 4}), smile); got.Glyph != '╰' {
		t.Errorf("left smile: got %q, want ╰", got.Glyph)
	}
	if got := r.encodeGroup(group([4]float64{0.2, 0.2, 0.2, 0.2}, [4]uint8{4, 4, 4, 4}), frown); got.Glyph != '╮' {
		t.Errorf("right frown: got %q, want ╮", got.Glyph)
	}
	if got := r.encodeGroup(group([4]float64{-0.2, -0.2, -0.2, -0.2}, [4]uint8{4, 4, 4, 4}), frown); got.Glyph != '╭' {
		t.Errorf("left frown: got %q, want ╭", got.Glyph)
	}
}

// TestEncoderGlyphTables pins the quadrant and Braille lookup tables so a table
// edit cannot silently change the rendered art.
func TestEncoderGlyphTables(t *testing.T) {
	wantQuadrants := [16]rune{' ', '▘', '▝', '▀', '▖', '▌', '▞', '▛', '▗', '▚', '▐', '▜', '▄', '▙', '▟', '█'}
	if quadrantRunes != wantQuadrants {
		t.Errorf("quadrant rune table changed: %q", quadrantRunes)
	}
	wantBraille := [8]int{0, 3, 1, 4, 2, 5, 6, 7}
	if brailleBits != wantBraille {
		t.Errorf("braille bit order changed: %v", brailleBits)
	}
}
