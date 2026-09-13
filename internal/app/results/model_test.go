package results

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/mascot"
	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func key(code rune) tea.KeyPressMsg {
	k := tea.Key{Code: code}
	if code >= 0x20 && code != 0x7f {
		k.Text = string(code)
	}
	return tea.KeyPressMsg(k)
}

// proudMetrics is an excited/proud result (accuracy >= 98, WPM >= 60) used by
// tests that need a celebration-capable tier.
func proudMetrics() engine.Metrics {
	return engine.Metrics{Duration: 30 * time.Second, WPM: 80, Raw: 80, Accuracy: 99}
}

func TestResultsShowsMascotAt80x24(t *testing.T) {
	m := New(proudMetrics())
	if m.layout.Mascot == (ui.Rect{}) {
		t.Fatalf("mascot should be laid out at the default size, layout %+v", m.layout)
	}
	cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd == nil {
		t.Fatal("configure must arm the mascot clock")
	}
	if m.mascot.View() == "" {
		t.Fatal("a visible mascot should render a portrait")
	}
	if rows := len(strings.Split(m.View(), "\n")); rows != 24 {
		t.Fatalf("view rows = %d, want 24", rows)
	}
	if m.scene().Track {
		t.Error("results must never track a target")
	}
}

func TestResultsMinimumKeepsResizeView(t *testing.T) {
	m := New(proudMetrics())
	m.Update(tea.WindowSizeMsg{Width: 27, Height: 11})
	if want := ui.ResizeView(27, 11, "q / Ctrl+C quit"); m.View() != want {
		t.Error("below minimum must keep the ResizeView")
	}
	if m.mascot.View() != "" {
		t.Error("mascot must be hidden below minimum")
	}
}

func TestResultsHidesWhenTooShort(t *testing.T) {
	m := New(proudMetrics())
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 17})
	if m.layout.Mascot != (ui.Rect{}) {
		t.Errorf("60x17 should hide the mascot, got %+v", m.layout.Mascot)
	}
	if want := ui.FitView(m.content, 60, 17); m.View() != want {
		t.Error("hidden results must use the existing FitView result")
	}
}

func TestResultsContentUnchanged(t *testing.T) {
	m := New(engine.Metrics{Duration: 30 * time.Second, WPM: 42, Raw: 42, Accuracy: 87.5})
	rows := strings.Split(ansi.Strip(m.renderContent()), "\n")
	want := []string{
		"results",
		"",
		"WPM: 42  Accuracy: 87.50%  Elapsed: 30s",
		"",
		"Enter retry · Tab home · q quit",
	}
	if len(rows) != len(want) {
		t.Fatalf("content rows = %d, want %d (%q)", len(rows), len(want), rows)
	}
	for i := range want {
		if got := strings.TrimSpace(rows[i]); got != want[i] {
			t.Errorf("row %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestResultsMeasuresWrappedContent(t *testing.T) {
	m := New(engine.Metrics{Duration: 2 * time.Minute, WPM: 120, Raw: 120, Accuracy: 100})
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})

	rows := strings.Split(ansi.Strip(m.renderContent()), "\n")
	if len(rows) <= 5 {
		t.Fatalf("narrow stats/help should wrap, got %d rows: %q", len(rows), rows)
	}
	if m.layout.Content.Height != len(rows) {
		t.Errorf("layout content height = %d, want measured %d", m.layout.Content.Height, len(rows))
	}
	if got := len(strings.Split(m.View(), "\n")); got != 24 {
		t.Errorf("composed rows = %d, want 24", got)
	}
}

func TestResultsExpressionsDifferByTier(t *testing.T) {
	render := func(metrics engine.Metrics) string {
		m := New(metrics)
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		return m.mascot.View()
	}
	base := engine.Metrics{Duration: 30 * time.Second, Raw: 30}
	proud := render(engine.Metrics{Duration: base.Duration, WPM: 20, Accuracy: 96})
	calm := render(engine.Metrics{Duration: base.Duration, WPM: 20, Accuracy: 90})
	worried := render(engine.Metrics{Duration: base.Duration, WPM: 20, Accuracy: 50})

	if proud == "" || calm == "" || worried == "" {
		t.Fatal("every result tier must render a portrait")
	}
	if proud == calm {
		t.Error("proud and calm must differ")
	}
	if calm == worried {
		t.Error("calm and worried must differ")
	}
}

func TestResultsFrameBypassesMetrics(t *testing.T) {
	m := New(proudMetrics())
	cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd == nil {
		t.Fatal("configure must arm the mascot clock")
	}
	frame, ok := cmd().(mascot.FrameMsg)
	if !ok {
		t.Fatalf("armed command returned %T, want mascot.FrameMsg", cmd())
	}

	metrics := m.metrics
	content := m.content
	if m.Update(frame) == nil {
		t.Error("a valid mascot frame must return its successor")
	}
	if m.metrics != metrics || m.content != content {
		t.Error("a mascot frame must not touch the final metrics or content")
	}
	if m.Update(frame) != nil {
		t.Error("a stale mascot frame must not reschedule")
	}
}

func TestResultsDepartureKeys(t *testing.T) {
	if cmd := New(proudMetrics()).Update(key(tea.KeyEnter)); cmd == nil {
		t.Fatal("Enter must return RetryMsg")
	} else if _, ok := cmd().(RetryMsg); !ok {
		t.Fatalf("Enter returned %T, want RetryMsg", cmd())
	}
	if cmd := New(proudMetrics()).Update(key(tea.KeyTab)); cmd == nil {
		t.Fatal("Tab must return HomeMsg")
	} else if _, ok := cmd().(HomeMsg); !ok {
		t.Fatalf("Tab returned %T, want HomeMsg", cmd())
	}
	cmd := New(proudMetrics()).Update(key('q'))
	if cmd == nil {
		t.Fatal("q must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q must return QuitMsg")
	}
	if cmd := New(proudMetrics()).Update(key(tea.KeyEscape)); cmd != nil {
		t.Error("results must not handle Esc")
	}
}

func TestViewReturnsCachedComposite(t *testing.T) {
	m := New(proudMetrics())
	cached := m.View()
	m.width, m.height = 40, 20
	if m.View() != cached {
		t.Error("View must return the cached composite without recomputing")
	}
}
