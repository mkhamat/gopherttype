package mascot

import (
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestOutputShapeAndDefaults(t *testing.T) {
	r := NewRenderer()
	if r.View() != "" {
		t.Error("View before Render must be empty")
	}
	r.Render(NeutralPose(), nil)
	lines := strings.Split(r.View(), "\n")
	if len(lines) != Height {
		t.Fatalf("want %d rows, got %d", Height, len(lines))
	}
	for y, line := range lines {
		if w := ansi.StringWidth(line); w != Width {
			t.Errorf("row %d width %d, want %d", y, w, Width)
		}
		if !strings.HasSuffix(line, ansi.ResetStyle) {
			t.Errorf("row %d missing reset suffix", y)
		}
	}
	if !strings.Contains(r.View(), "\x1b[39;49m") {
		t.Error("default channels must emit 39;49 to clear old art")
	}
	filled := 0
	for _, c := range r.Frame() {
		if c.Glyph == ' ' && c.FG == c.BG && c.BG != 0 {
			filled++
		}
	}
	if filled == 0 {
		t.Error("expected solid colored spaces")
	}
	if c := r.Frame()[0]; c != (Cell{Glyph: ' '}) {
		t.Errorf("outside corner must be terminal default, got %+v", c)
	}
}

func TestSanitizePoseFields(t *testing.T) {
	n := NeutralPose()
	for _, tc := range []struct{ input, want Pose }{
		{Pose{Yaw: math.NaN(), Pitch: math.Inf(1), EyeOpen: math.Inf(-1), Lift: math.NaN(), Bob: math.Inf(1)}, n},
		{Pose{Yaw: 10, Pitch: math.NaN()}, Pose{Yaw: 10, Pitch: n.Pitch}},
		{Pose{}, Pose{}},
		{Pose{Yaw: 100, Pitch: 50, EyeOpen: 2, Lift: 5, Bob: 1}, Pose{Yaw: 35, Pitch: 20, EyeOpen: 1, Lift: 0.10, Bob: 0.06}},
		{Pose{Yaw: -100, Pitch: -50, EyeOpen: -1, Lift: -5, Bob: -1}, Pose{Yaw: -35, Pitch: 0, EyeOpen: 0, Lift: -0.035, Bob: -0.06}},
	} {
		if got := sanitizePose(tc.input); got != tc.want {
			t.Errorf("sanitize(%+v) = %+v, want %+v", tc.input, got, tc.want)
		}
		a, b := NewRenderer(), NewRenderer()
		a.Render(tc.input, nil)
		b.Render(tc.want, nil)
		if a.Frame() != b.Frame() {
			t.Errorf("Render did not sanitize %+v", tc.input)
		}
	}
}

func TestRenderCacheTracksLatestFrame(t *testing.T) {
	r := NewRenderer()
	neutral := NeutralPose()
	nudge, turned := neutral, neutral
	nudge.Pitch = 0.005
	turned.Yaw, turned.Pitch = 35, 14
	var previous Frame
	for i, step := range []struct {
		pose Pose
		bg   color.Color
	}{
		{neutral, nil}, {neutral, nil}, {nudge, nil},
		{turned, nil}, {turned, color.Black}, {turned, color.White},
		{turned, color.White}, {neutral, nil},
	} {
		fresh := NewRenderer()
		fresh.Render(step.pose, step.bg)
		old := r.View()
		saved := strings.Clone(old)
		changed := r.Render(step.pose, step.bg)
		if want := i == 0 || fresh.Frame() != previous; changed != want {
			t.Errorf("step %d: changed = %t, want %t", i, changed, want)
		}
		if r.Frame() != fresh.Frame() || r.View() != fresh.View() {
			t.Errorf("step %d: cached renderer differs from fresh render", i)
		}
		if old != saved {
			t.Errorf("step %d: previously returned string was mutated", i)
		}
		previous = fresh.Frame()
	}
}

func TestIdenticalCellsAllocateZero(t *testing.T) {
	r := NewRenderer()
	base, nudge := NeutralPose(), NeutralPose()
	nudge.Pitch = 0.005
	r.Render(base, nil)
	first := r.View()
	if allocs := testing.AllocsPerRun(20, func() {
		if r.Render(nudge, nil) || r.Render(base, nil) {
			t.Fatal("sub-cell nudge changed appearance")
		}
	}); allocs != 0 {
		t.Errorf("identical-cell render allocs = %v, want 0", allocs)
	}
	if r.View() != first {
		t.Error("identical cells changed cached output")
	}
}

func pairDist(a, b uint8) [9][9]float64 {
	var d [9][9]float64
	d[a][b], d[b][a] = 1, 1
	return d
}

func allFur(c uint8) [4][quadSize]sample {
	var q [4][quadSize]sample
	for qi := range q {
		for k := range q[qi] {
			q[qi][k] = sample{color: c, role: roleFur}
		}
	}
	return q
}

func TestEncoderQuadrantMasksAndTies(t *testing.T) {
	r := &Renderer{dist: pairDist(1, 6)}
	for mask, glyph := range []rune(" ▘▝▀▖▌▞▛▗▚▐▜▄▙▟█") {
		q := allFur(1)
		for qi := range q {
			count := quadSize / 2
			if mask&(1<<qi) != 0 {
				count++
			}
			for k := range count {
				q[qi][k].color = 6
			}
		}
		if got := r.encodeCell(&q); got != (Cell{Glyph: glyph, FG: 6, BG: 1}) {
			t.Errorf("mask %d: got %+v, want %q 6/1", mask, got, glyph)
		}
	}
}

func TestEncoderBackgroundZeroRequired(t *testing.T) {
	r := &Renderer{dist: pairDist(1, 6)}
	r.dist[0][1], r.dist[1][0], r.dist[0][6], r.dist[6][0] = 100, 100, 100, 100
	q := allFur(1)
	q[0][0].color = 0
	for qi := 2; qi < 4; qi++ {
		for k := range q[qi] {
			q[qi][k].color = 6
		}
	}
	if got := r.encodeCell(&q); got != (Cell{Glyph: '█', FG: 6, BG: 0}) {
		t.Errorf("background must remain BG=0 even when a nonzero pair is cheaper, got %+v", got)
	}
}

func TestEncoderOneColorSpace(t *testing.T) {
	for _, s := range []sample{{color: 0, role: roleBackground}, {color: 5, role: roleFur}, {color: 7, role: roleMouth}} {
		q := allFur(s.color)
		for qi := range q {
			for k := range q[qi] {
				q[qi][k] = s
			}
		}
		if got := new(Renderer).encodeCell(&q); got != (Cell{Glyph: ' ', FG: s.color, BG: s.color}) {
			t.Errorf("single color %+v: got %+v", s, got)
		}
	}
}

func TestEncoderRoleWeighting(t *testing.T) {
	r := &Renderer{dist: pairDist(1, 6)}
	for _, tc := range []struct {
		role  role
		count int
		glyph rune
	}{
		{roleFur, 4, ' '}, {roleMuzzle, 4, '▘'}, {rolePupil, 3, '▘'},
	} {
		q := allFur(1)
		for k := range tc.count {
			q[0][k] = sample{color: 6, role: tc.role}
		}
		if got := r.encodeCell(&q); got != (Cell{Glyph: tc.glyph, FG: 6, BG: 1}) {
			t.Errorf("role %d: got %+v, want %q 6/1", tc.role, got, tc.glyph)
		}
	}
}
