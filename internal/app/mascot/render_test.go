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

	// A tiny pitch nudge should not move any of the four integrated quadrants.
	alt := base
	alt.Pitch = 0.005
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
// These drive the private encoder with explicit quadrant samples and distance
// tables, so every mask and tie rule is exercised without geometry. The
// end-to-end parity fixtures cover the real combinations.

func encRenderer(dist [9][9]float64) *Renderer {
	r := NewRenderer()
	r.dist = dist
	r.distSet = true
	return r
}

// pairDist builds the minimal table for two present colors: zero self distance
// and equal cross distance.
func pairDist(a, b uint8) [9][9]float64 {
	var d [9][9]float64
	d[a][a], d[b][b] = 0, 0
	d[a][b], d[b][a] = 1, 1
	return d
}

// quartersForMask builds a two-color group (fg 6, bg 1) whose quadrant mask is
// exactly `mask`. A set quadrant holds 7 fg + 1 bg (fg strictly closer);
// a clear quadrant holds 4 fg + 4 bg (a tie). Color 1 stays present throughout.
func quartersForMask(mask int) [4][quadSize]sample {
	var q [4][quadSize]sample
	for qi := 0; qi < 4; qi++ {
		n1 := 1
		if mask&(1<<qi) == 0 {
			n1 = quadSize / 2
		}
		for k := 0; k < quadSize; k++ {
			c := uint8(6)
			if k < n1 {
				c = 1
			}
			q[qi][k] = sample{color: c, role: roleFur}
		}
	}
	return q
}

func allFur(c uint8) [4][quadSize]sample {
	var q [4][quadSize]sample
	for qi := 0; qi < 4; qi++ {
		for k := 0; k < quadSize; k++ {
			q[qi][k] = sample{color: c, role: roleFur}
		}
	}
	return q
}

func TestEncoderQuadrantMasks(t *testing.T) {
	r := encRenderer(pairDist(1, 6))
	for mask := 0; mask < 16; mask++ {
		q := quartersForMask(mask)
		got := r.encodeCell(&q)
		if got.Glyph != quadrantRunes[mask] || got.FG != 6 || got.BG != 1 {
			t.Errorf("mask %d: got %q fg=%d bg=%d, want %q fg=6 bg=1", mask, got.Glyph, got.FG, got.BG, quadrantRunes[mask])
		}
	}
}

// TestEncoderStrictQuadrantTie checks the strict `<` comparison: an even split
// clears the bit, a strictly closer foreground sets it.
func TestEncoderStrictQuadrantTie(t *testing.T) {
	r := encRenderer(pairDist(1, 6))

	tie := allFur(1)
	for k := 0; k < 4; k++ {
		tie[0][k] = sample{color: 6, role: roleFur}
	}
	if got := r.encodeCell(&tie); got.Glyph != ' ' || got.FG != 6 || got.BG != 1 {
		t.Errorf("4/4 quadrant tie must clear the bit, got %q fg=%d bg=%d", got.Glyph, got.FG, got.BG)
	}

	win := allFur(1)
	for k := 0; k < 5; k++ {
		win[0][k] = sample{color: 6, role: roleFur}
	}
	if got := r.encodeCell(&win); got.Glyph != '▘' || got.FG != 6 || got.BG != 1 {
		t.Errorf("strictly closer foreground must set the bit, got %q fg=%d bg=%d", got.Glyph, got.FG, got.BG)
	}
}

// TestEncoderBackgroundZeroPreferred checks that when the background occurs,
// only pairs containing index 0 qualify and BG stays 0.
func TestEncoderBackgroundZeroPreferred(t *testing.T) {
	r := encRenderer(pairDist(0, 6))
	q := allFur(0)
	for qi := 1; qi < 4; qi += 2 {
		for k := 0; k < quadSize; k++ {
			q[qi][k] = sample{color: 6, role: roleFur}
		}
	}
	got := r.encodeCell(&q)
	if got.BG != 0 || got.FG != 6 {
		t.Errorf("a present background must stay BG=0, got fg=%d bg=%d", got.FG, got.BG)
	}
}

func TestEncoderOneColorSpace(t *testing.T) {
	r := encRenderer(pairDist(1, 6))

	q := allFur(0)
	for qi := 0; qi < 4; qi++ {
		q[qi][0] = sample{color: 0, role: roleBackground}
	}
	if got := r.encodeCell(&q); got.Glyph != ' ' || got.FG != 0 || got.BG != 0 {
		t.Errorf("all-background group: got %q fg=%d bg=%d, want space 0/0", got.Glyph, got.FG, got.BG)
	}

	solid := allFur(5)
	if got := r.encodeCell(&solid); got.Glyph != ' ' || got.FG != 5 || got.BG != 5 {
		t.Errorf("solid filled group: got %q fg=%d bg=%d, want space 5/5", got.Glyph, got.FG, got.BG)
	}

	mouth := allFur(7)
	for qi := 0; qi < 4; qi++ {
		for k := range mouth[qi] {
			mouth[qi][k].role = roleMouth
		}
	}
	if got := r.encodeCell(&mouth); got.Glyph != ' ' || got.FG != 7 || got.BG != 7 {
		t.Errorf("all-mouth single color: got %q fg=%d bg=%d, want space 7/7", got.Glyph, got.FG, got.BG)
	}
}

func TestRoleWeighting(t *testing.T) {
	for _, tc := range []struct {
		rl   role
		want float64
	}{
		{roleBackground, 1}, {roleFur, 1}, {roleEye, 1},
		{rolePupil, 2}, {roleNose, 2}, {roleMouth, 2}, {roleTooth, 2},
		{roleSeam, 2}, {roleEar, 2}, {roleMuzzle, 1.5},
	} {
		if got := weightOf(tc.rl); got != tc.want {
			t.Errorf("weightOf(%s) = %g, want %g", roleName(tc.rl), got, tc.want)
		}
	}
}

// TestEncoderGlyphTable pins the quadrant lookup table so a table edit cannot
// silently change the rendered art.
func TestEncoderGlyphTable(t *testing.T) {
	want := [16]rune{' ', '▘', '▝', '▀', '▖', '▌', '▞', '▛', '▗', '▚', '▐', '▜', '▄', '▙', '▟', '█'}
	if quadrantRunes != want {
		t.Errorf("quadrant rune table changed: %q", quadrantRunes)
	}
}
