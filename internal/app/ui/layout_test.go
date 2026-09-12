package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func block(rows, cols int, fill string) string {
	line := strings.Repeat(fill, cols)
	lines := make([]string, rows)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func TestLayoutWithMascotFits80x24(t *testing.T) {
	content := block(10, 36, "c")
	got := LayoutWithMascot(80, 24, content)
	if want := (Rect{X: 24, Y: 1, Width: 32, Height: 12}); got.Mascot != want {
		t.Errorf("mascot = %+v, want %+v", got.Mascot, want)
	}
	if want := (Rect{X: 22, Y: 14, Width: 36, Height: 10}); got.Content != want {
		t.Errorf("content = %+v, want %+v", got.Content, want)
	}
}

func TestLayoutWithMascotWidthThreshold(t *testing.T) {
	content := block(11, 28, "c")
	if got := LayoutWithMascot(31, 25, content); got.Mascot.Width != 0 {
		t.Errorf("width 31 must hide, got %+v", got.Mascot)
	}
	if got := LayoutWithMascot(32, 25, content); got.Mascot != (Rect{X: 0, Y: 1, Width: 32, Height: 12}) {
		t.Errorf("width 32 slot = %+v", got.Mascot)
	}
	if got := LayoutWithMascot(33, 25, content); got.Mascot != (Rect{X: 0, Y: 1, Width: 32, Height: 12}) {
		t.Errorf("width 33 slot = %+v", got.Mascot)
	}
}

func TestLayoutWithMascotHeightThreshold(t *testing.T) {
	content := block(11, 28, "c")
	if got := LayoutWithMascot(32, 24, content); got.Mascot.Width != 0 {
		t.Errorf("height 24 must hide, got %+v", got.Mascot)
	}
	if got := LayoutWithMascot(32, 25, content); got.Mascot.Width == 0 {
		t.Errorf("height 25 must show, got %+v", got.Mascot)
	}
}

func TestLayoutWithMascotHiddenReclaimsRows(t *testing.T) {
	content := block(10, 36, "c")
	got := LayoutWithMascot(40, 20, content)
	if got.Mascot != (Rect{}) {
		t.Errorf("expected hidden mascot, got %+v", got.Mascot)
	}
	if want := (Rect{X: 2, Y: 5, Width: 36, Height: 10}); got.Content != want {
		t.Errorf("hidden content = %+v, want %+v", got.Content, want)
	}
}

func TestLayoutWithMascotMinimumStaysHidden(t *testing.T) {
	content := block(11, 24, "c")
	got := LayoutWithMascot(28, 12, content)
	if got.Mascot != (Rect{}) {
		t.Errorf("below the slot a mascot must hide, got %+v", got.Mascot)
	}
	if got.Content.Width != 24 || got.Content.Height != 11 {
		t.Errorf("content should still measure, got %+v", got.Content)
	}
}

func TestLayoutWithMascotClippedContentHasNoCoordinates(t *testing.T) {
	content := block(10, 40, "c")
	if got := LayoutWithMascot(32, 30, content); got.Content != (Rect{}) {
		t.Errorf("clipped content must not invent coordinates, got %+v", got.Content)
	}
}

func TestLayoutWithMascotSticksToContent(t *testing.T) {
	content := block(10, 36, "c")
	got := LayoutWithMascot(80, 40, content)
	if got.Mascot.Y <= 1 {
		t.Errorf("mascot must not stay at the top on tall terminals: %+v", got.Mascot)
	}
	if gap := got.Content.Y - (got.Mascot.Y + got.Mascot.Height); gap != 1 {
		t.Errorf("mascot/content gap = %d, want 1", gap)
	}
	if got.Mascot.Y != got.Content.Y-13 {
		t.Errorf("mascot Y = %d, want content.Y-13 = %d", got.Mascot.Y, got.Content.Y-13)
	}
	top := got.Mascot.Y
	bottom := 40 - (got.Content.Y + got.Content.Height)
	if diff := top - bottom; diff < -1 || diff > 1 {
		t.Errorf("block not centered: top margin %d, bottom margin %d", top, bottom)
	}
}

func TestLayoutWithMascotOddCentering(t *testing.T) {
	content := block(10, 35, "c")
	got := LayoutWithMascot(81, 25, content)
	if want := (Rect{X: 24, Y: 1, Width: 32, Height: 12}); got.Mascot != want {
		t.Errorf("odd width mascot = %+v, want %+v", got.Mascot, want)
	}
	if want := (Rect{X: 23, Y: 14, Width: 35, Height: 10}); got.Content != want {
		t.Errorf("odd width content = %+v, want %+v", got.Content, want)
	}
}

func TestComposeWithMascotHiddenMatchesFitView(t *testing.T) {
	content := block(10, 36, "c")
	got := ComposeWithMascot(content, "", MascotLayout{}, 40, 20)
	if want := FitView(content, 40, 20); got != want {
		t.Errorf("hidden compose must equal FitView")
	}
}

func TestComposeWithMascotPlacesBothBlocks(t *testing.T) {
	portrait := block(12, 32, "p")
	content := block(10, 36, "c")
	layout := LayoutWithMascot(80, 24, content)

	got := ComposeWithMascot(content, portrait, layout, 80, 24)
	lines := strings.Split(got, "\n")
	if len(lines) != 24 {
		t.Fatalf("want 24 rows, got %d", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > 80 {
			t.Fatalf("row %d width %d exceeds terminal", i, w)
		}
	}
	for i, pl := range strings.Split(portrait, "\n") {
		want := strings.Repeat(" ", layout.Mascot.X) + pl
		if lines[layout.Mascot.Y+i] != want {
			t.Errorf("portrait row %d = %q, want %q", i, lines[layout.Mascot.Y+i], want)
		}
	}
	for i, cl := range strings.Split(content, "\n") {
		want := strings.Repeat(" ", layout.Content.X) + cl
		if lines[layout.Content.Y+i] != want {
			t.Errorf("content row %d = %q, want %q", i, lines[layout.Content.Y+i], want)
		}
	}
}

func TestComposeWithMascotPreservesColoredTrailingSpaces(t *testing.T) {
	row := "\x1b[41m  \x1b[0m"
	portrait := strings.Join([]string{row, row, row, row, row, row, row, row, row, row, row, row}, "\n")
	content := block(1, 16, "c")
	layout := LayoutWithMascot(64, 15, content)
	got := ComposeWithMascot(content, portrait, layout, 64, 15)
	lines := strings.Split(got, "\n")
	if want := strings.Repeat(" ", layout.Mascot.X) + row; lines[layout.Mascot.Y] != want {
		t.Errorf("colored spaces not preserved: %q", lines[layout.Mascot.Y])
	}
}
