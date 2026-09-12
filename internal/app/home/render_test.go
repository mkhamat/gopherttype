package home

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
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
