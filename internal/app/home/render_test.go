package home

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func TestRenderContentLayout(t *testing.T) {
	m := New()
	rows := strings.Split(ansi.Strip(m.renderContent()), "\n")
	want := []string{
		"gopherttype",
		"",
		"time",
		"words",
		"",
		"15s  30s  60s  120s",
		"",
		"enter to begin",
		"",
		"↑↓ mode ←→ length q quit",
	}
	if len(rows) != len(want) {
		t.Fatalf("content rows = %d, want %d", len(rows), len(want))
	}
	for i := range want {
		if got := strings.TrimSpace(rows[i]); got != want[i] {
			t.Errorf("row %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestRenderContentAlignsModeColumn(t *testing.T) {
	m := New()
	rows := strings.Split(m.renderContent(), "\n")
	left := func(row string) int { return len(row) - len(strings.TrimLeft(row, " ")) }
	if left(rows[2]) != left(rows[3]) {
		t.Errorf("mode rows not left aligned: %d vs %d", left(rows[2]), left(rows[3]))
	}
}

func TestRenderContentFitsMinimumWidth(t *testing.T) {
	m := New()
	for i, line := range strings.Split(m.renderContent(), "\n") {
		if w := ansi.StringWidth(line); w > ui.MinimumWidth-4 {
			t.Errorf("row %d width = %d, want <= %d", i, w, ui.MinimumWidth-4)
		}
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

func TestSelectedPointFollowsLengthAndMode(t *testing.T) {
	m := New()
	m.width, m.height = 80, 24
	m.configure(time.Now())

	m.settings.mode = engine.ModeTime
	for i, wantX := range []float64{31.5, 36.5, 41.5, 47} {
		m.settings.durationIndex = i
		m.configure(time.Now())
		got, ok := m.selectedPoint()
		if !ok {
			t.Fatalf("duration %d: no target", i)
		}
		if got.X != wantX || got.Y != 16.5 {
			t.Errorf("duration %d target = %+v, want {%.1f 16.5}", i, got, wantX)
		}
	}

	m.settings.mode = engine.ModeWords
	for i, wantX := range []float64{33, 37, 41, 45.5} {
		m.settings.wordIndex = i
		m.configure(time.Now())
		got, ok := m.selectedPoint()
		if !ok {
			t.Fatalf("word %d: no target", i)
		}
		if got.X != wantX || got.Y != 17.5 {
			t.Errorf("word %d target = %+v, want {%.1f 17.5}", i, got, wantX)
		}
	}
}

func TestSelectedPointDistinguishesTenFromHundred(t *testing.T) {
	m := New()
	m.width, m.height = 80, 24
	m.settings.mode = engine.ModeWords

	m.settings.wordIndex = 0
	m.configure(time.Now())
	ten, ok := m.selectedPoint()
	if !ok {
		t.Fatal("word 10: no target")
	}

	m.settings.wordIndex = 3
	m.configure(time.Now())
	hundred, ok := m.selectedPoint()
	if !ok {
		t.Fatal("word 100: no target")
	}

	if ten.X >= hundred.X {
		t.Errorf("word 10 x=%.1f must sit before word 100 x=%.1f", ten.X, hundred.X)
	}
}

func TestSelectedPointHiddenHasNoTarget(t *testing.T) {
	m := New()
	m.width, m.height = 40, 20
	m.configure(time.Now())
	if _, ok := m.selectedPoint(); ok {
		t.Error("a hidden slot must not produce a target")
	}
	if m.scene().Track {
		t.Error("a hidden slot must not track")
	}
}
