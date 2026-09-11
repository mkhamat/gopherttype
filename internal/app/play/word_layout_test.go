package play

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func cursorColumn(t *testing.T, layout wordLayout, text string) int {
	t.Helper()
	line := layout.lines[layout.cursorRow]
	before, _, found := strings.Cut(line, ui.StylesFor(true).Cursor.Render(text))
	if !found {
		t.Fatalf("missing cursor %q in %q", text, line)
	}
	return ansi.StringWidth(before)
}

func TestWordLayoutCursor(t *testing.T) {
	for _, tc := range []struct {
		name        string
		target      string
		typed       string
		width       int
		row, column int
		cursorText  string
	}{
		{"ready", "hello", "", 24, 0, 0, "h"},
		{"completed at edge", "hello", "hello", 6, 0, 5, " "},
		{"extra at edge", "cat", "catxx", 6, 0, 5, " "},
		{"wrapped end", strings.Repeat("a", 24), strings.Repeat("a", 24), 24, 1, 1, " "},
		{"overtyped end", "cat", strings.Repeat("x", 72), 24, 3, 3, " "},
		{"unicode mistakes", strings.Repeat("a", 30), strings.Repeat("界", 27), 10, 3, 0, "a"},
		{"wide extras", "a", "a界界", 4, 1, 2, " "},
		{"combining extra", "a", "á", 4, 0, 1, " "},
		{"emoji extra", "a", "a👩‍💻", 4, 0, 3, " "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layout := layoutWords([]engine.WordSnapshot{{Target: tc.target, Typed: []rune(tc.typed)}, {Target: "next"}}, 0, tc.width, ui.StylesFor(true))
			column := cursorColumn(t, layout, tc.cursorText)
			if layout.cursorRow != tc.row || column != tc.column {
				t.Fatalf("cursor: got (%d,%d), want (%d,%d)", layout.cursorRow, column, tc.row, tc.column)
			}
			for _, line := range layout.lines {
				if lipgloss.Width(line) > tc.width {
					t.Fatalf("overflow: %q", line)
				}
			}
			cursor := ui.StylesFor(true).Cursor.Render(tc.cursorText)
			if !strings.Contains(layout.lines[tc.row], cursor) || !strings.Contains(layout.visibleLines(playVisibleLines), cursor) {
				t.Fatal("cursor was lost during wrapping or scrolling")
			}
		})
	}
}

func TestWordLayoutWrapsWordsAndRepeatedText(t *testing.T) {
	words := []engine.WordSnapshot{
		{Target: "one", Typed: []rune("one")},
		{Target: "two", Typed: []rune("two")},
		{Target: "one", Typed: []rune("o")},
		{Target: "two"},
	}
	layout := layoutWords(words, 2, 8, ui.StylesFor(true))
	if layout.cursorRow != 1 || cursorColumn(t, layout, "n") != 1 {
		t.Fatal("repeated words confused cursor position")
	}
	if got := ansi.Strip(strings.Join(layout.lines, "\n")); got != "one two\none two" {
		t.Fatalf("unexpected wrapping: %q", got)
	}
}

func TestWordLayoutPreservesGraphemes(t *testing.T) {
	words := []engine.WordSnapshot{{Target: "a", Typed: []rune("a界é👩‍💻")}, {Target: "next"}}
	for width := 1; width <= 20; width++ {
		layout := layoutWords(words, 0, width, ui.StylesFor(true))
		for _, line := range layout.lines {
			if lipgloss.Width(line) > width {
				t.Fatalf("overflow at %d: %q", width, ansi.Strip(line))
			}
		}
		if width >= 2 {
			plain := ansi.Strip(strings.Join(layout.lines, "\n"))
			for _, cluster := range []string{"é", "👩‍💻"} {
				if !strings.Contains(plain, cluster) {
					t.Fatalf("split grapheme at %d: %q", width, plain)
				}
			}
		}
	}
}

func TestWordLayoutPreservesMistakeStyles(t *testing.T) {
	words := []engine.WordSnapshot{{Target: "cat", Typed: []rune("cxte")}, {Target: "dog"}}
	layout := layoutWords(words, 1, 24, ui.StylesFor(true))
	underline := func(style lipgloss.Style) lipgloss.Style {
		return style.Underline(true).UnderlineColor(ui.StylesFor(true).Incorrect.GetForeground())
	}
	for _, want := range []string{
		underline(ui.StylesFor(true).Correct).Render("c"),
		underline(ui.StylesFor(true).Incorrect).Render("a"),
		underline(ui.StylesFor(true).Extra).Render("e"),
		ui.StylesFor(true).Cursor.Render("d"),
	} {
		if !strings.Contains(layout.lines[0], want) {
			t.Fatalf("lost word styling: %q", want)
		}
	}
}

func TestWordLayoutNeutralizesControlCharacters(t *testing.T) {
	layout := layoutWords([]engine.WordSnapshot{{Target: "a", Typed: []rune("a\x1b[31m\n")}}, 0, 24, ui.StylesFor(true))
	plain := ansi.Strip(strings.Join(layout.lines, "\n"))
	if plain != "a�[31m� " {
		t.Fatalf("control characters were not displayed safely: %q", plain)
	}
}
