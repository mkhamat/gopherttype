package mascot

import (
	"image/color"
	"math"
	"strings"
	"testing"

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
