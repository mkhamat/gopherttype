package home

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func TestRenderContentLayout(t *testing.T) {
	m := New()
	content, _ := m.renderContent()
	rows := strings.Split(ansi.Strip(content), "\n")
	want := []string{"gopherttype", "", "time", "words", "", "15s  30s  60s  120s", "", "enter to begin", "", "↑↓ mode ←→ length q quit"}
	if len(rows) != len(want) {
		t.Fatalf("content rows = %d, want %d", len(rows), len(want))
	}
	for i, row := range rows {
		if strings.TrimSpace(row) != want[i] {
			t.Errorf("row %d = %q, want %q", i, row, want[i])
		}
		if w := ansi.StringWidth(row); w > ui.MinimumWidth-4 {
			t.Errorf("row %d width = %d, want <= %d", i, w, ui.MinimumWidth-4)
		}
	}
	if strings.Index(rows[2], "time") != strings.Index(rows[3], "words") {
		t.Error("mode rows must be left aligned")
	}
}

func TestViewReturnsCachedComposite(t *testing.T) {
	m := New()
	cached := m.View()
	m.width, m.height = 40, 20
	if m.View() != cached {
		t.Error("View must return the cached composite without recomputing")
	}
}

func TestSelectedPointMatchesRenderedOptions(t *testing.T) {
	for _, tc := range []struct {
		token                string
		mode                 engine.Mode
		index, width, height int
		dark                 bool
	}{
		{"15s", engine.ModeTime, 0, 80, 24, true},
		{"30s", engine.ModeTime, 1, 32, 24, false},
		{"60s", engine.ModeTime, 2, 33, 25, true},
		{"120s", engine.ModeTime, 3, 81, 40, false},
		{"10", engine.ModeWords, 0, 81, 40, false},
		{"25", engine.ModeWords, 1, 33, 25, true},
		{"50", engine.ModeWords, 2, 32, 24, false},
		{"100", engine.ModeWords, 3, 80, 24, true},
	} {
		t.Run(tc.token, func(t *testing.T) {
			m := New()
			m.styles = ui.StylesFor(tc.dark)
			m.settings = settings{mode: tc.mode, wordIndex: tc.index, durationIndex: tc.index}
			m.Update(tea.WindowSizeMsg{Width: tc.width, Height: tc.height})
			rows := strings.Split(ansi.Strip(m.View()), "\n")
			var want ui.Point
			foundX, foundY := false, false
			mode := "time"
			if tc.mode == engine.ModeWords {
				mode = "words"
			}
			for y, row := range rows {
				if column := strings.Index(row, " "+tc.token+" "); column >= 0 {
					want.X = float64(ansi.StringWidth(row[:column+1])) + float64(len(tc.token))/2
					foundX = true
				}
				if strings.TrimSpace(row) == mode {
					want.Y, foundY = float64(y)+0.5, true
				}
			}
			if !foundX || !foundY {
				t.Fatal("selected option missing from rendered menu")
			}
			if point, ok := m.selectedPoint(); !ok || point != want {
				t.Errorf("target = %+v, valid=%t; rendered selection = %+v", point, ok, want)
			}
		})
	}
}

func TestSelectedPointHiddenHasNoTarget(t *testing.T) {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	if _, ok := m.selectedPoint(); ok || m.scene().Track {
		t.Error("a hidden slot must not track a target")
	}
}
