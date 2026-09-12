package play

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func plain(line string) string { return ansi.Strip(line) }

func TestLayoutWordsCursorMatchesHighlightedCell(t *testing.T) {
	styles := ui.StylesFor(true)
	words := []engine.WordSnapshot{
		{Target: "hello"},
		{Target: "world", Typed: []rune("wo")},
	}
	layout := layoutWords(words, 1, 40, styles)
	if layout.cursorRow != 0 {
		t.Fatalf("cursorRow = %d, want 0", layout.cursorRow)
	}
	if layout.cursorColumn != 8 || layout.cursorWidth != 1 {
		t.Fatalf("cursor = (%d,%d), want (8,1)", layout.cursorColumn, layout.cursorWidth)
	}
	line := []rune(plain(layout.lines[layout.cursorRow]))
	if got := line[layout.cursorColumn]; got != 'r' {
		t.Errorf("cell at cursor column = %q, want 'r' (line %q)", got, string(line))
	}
}

func TestLayoutWordsWideCursorWidth(t *testing.T) {
	styles := ui.StylesFor(true)
	layout := layoutWords([]engine.WordSnapshot{{Target: "日本"}}, 0, 40, styles)
	if layout.cursorColumn != 0 || layout.cursorWidth != 2 {
		t.Fatalf("cursor = (%d,%d), want (0,2)", layout.cursorColumn, layout.cursorWidth)
	}
}

func TestLayoutWordsFallbackWidth(t *testing.T) {
	styles := ui.StylesFor(true)
	layout := layoutWords([]engine.WordSnapshot{{Target: "日"}}, 0, 1, styles)
	if layout.cursorWidth != 1 {
		t.Fatalf("fallback cursorWidth = %d, want 1", layout.cursorWidth)
	}
	if !strings.Contains(plain(layout.lines[0]), "�") {
		t.Errorf("fallback line %q should contain the replacement glyph", plain(layout.lines[0]))
	}
}

func TestLayoutWordsDottedCircleFallback(t *testing.T) {
	styles := ui.StylesFor(true)
	layout := layoutWords([]engine.WordSnapshot{{Target: "\u0301"}}, 0, 40, styles)
	if layout.cursorWidth != 1 {
		t.Fatalf("zero-width grapheme cursorWidth = %d, want 1", layout.cursorWidth)
	}
	if !strings.Contains(plain(layout.lines[0]), "◌") {
		t.Errorf("dotted-circle fallback missing from %q", plain(layout.lines[0]))
	}
}

func TestLayoutWordsTrailingCursorSpace(t *testing.T) {
	styles := ui.StylesFor(true)
	layout := layoutWords([]engine.WordSnapshot{{Target: "go", Typed: []rune("go")}}, 0, 40, styles)
	if layout.cursorColumn != 2 || layout.cursorWidth != 1 {
		t.Fatalf("trailing cursor = (%d,%d), want (2,1)", layout.cursorColumn, layout.cursorWidth)
	}
	line := []rune(plain(layout.lines[layout.cursorRow]))
	if got := line[layout.cursorColumn]; got != ' ' {
		t.Errorf("trailing cursor should sit on a space, got %q", got)
	}
}

func TestLayoutWordsWrapsAndScrolls(t *testing.T) {
	styles := ui.StylesFor(true)
	words := make([]engine.WordSnapshot, 40)
	for i := range words {
		words[i] = engine.WordSnapshot{Target: "abcdefgh"}
	}
	layout := layoutWords(words, len(words)-1, 12, styles)
	if layout.cursorRow < playVisibleLines {
		t.Fatalf("cursorRow = %d, expected wrapped rows beyond the viewport", layout.cursorRow)
	}
	first := layout.firstVisibleRow(playVisibleLines)
	if first <= 0 {
		t.Fatalf("firstVisibleRow = %d, want > 0 for a deep cursor", first)
	}
	if last := first + playVisibleLines; layout.cursorRow < first || layout.cursorRow >= last {
		t.Errorf("cursor row %d must be inside viewport [%d,%d)", layout.cursorRow, first, last)
	}
}

func TestFirstVisibleRow(t *testing.T) {
	cases := []struct {
		row, height, want int
	}{
		{0, 3, 0},
		{1, 3, 0},
		{2, 3, 1},
		{10, 3, 9},
		{-1, 3, 0},
	}
	for _, tc := range cases {
		if got := (wordLayout{cursorRow: tc.row}).firstVisibleRow(tc.height); got != tc.want {
			t.Errorf("firstVisibleRow(row=%d,height=%d) = %d, want %d", tc.row, tc.height, got, tc.want)
		}
	}
}
