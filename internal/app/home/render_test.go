package home

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
)

func TestRenderContentMatchesPreChange(t *testing.T) {
	m := New()
	content := m.renderContent(m.homeLayout().inner)
	if rows := strings.Count(content, "\n") + 1; rows != 10 {
		t.Errorf("content rows = %d, want 10", rows)
	}
	for i, line := range strings.Split(content, "\n") {
		if w := ansi.StringWidth(line); w != 36 {
			t.Errorf("row %d width = %d, want 36", i, w)
		}
	}
	got := ui.ComposeWithMascot(content, "", ui.MascotLayout{}, 80, 24)
	if want := ui.FitView(content, 80, 24); got != want {
		t.Error("hidden compose changed the pre-existing content placement")
	}
}

func TestRenderContentDurationWrapsAtNarrowWidth(t *testing.T) {
	m := New()
	content := m.renderContent(28)
	if rows := strings.Count(content, "\n") + 1; rows != 11 {
		t.Errorf("narrow duration rows = %d, want 11", rows)
	}
	if !strings.Contains(ansi.Strip(content), "120s") {
		t.Error("wrapping must not drop the 120s option")
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
