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

func TestResultsContent(t *testing.T) {
	m := New(engine.Metrics{Duration: 30 * time.Second, WPM: 42, Raw: 45, Accuracy: 87.5, Correct: 240, Incorrect: 12, Extra: 3})
	content := ansi.Strip(m.renderContent())

	for _, want := range []string{
		"Steady", "42 WPM", "87.50% accuracy", "30s",
		"raw 45 wpm", "12 wrong", "3 extra",
		"enter retry", "tab home", "q quit",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("content missing %q:\n%s", want, content)
		}
	}
	for _, unwanted := range []string{"missed", "correct"} {
		if strings.Contains(content, unwanted) {
			t.Errorf("content must drop %q:\n%s", unwanted, content)
		}
	}
}

func TestResultsHidesRawWhenEqual(t *testing.T) {
	m := New(engine.Metrics{Duration: 30 * time.Second, WPM: 42, Raw: 42, Accuracy: 87.5})
	if content := ansi.Strip(m.renderContent()); strings.Contains(content, "raw") {
		t.Errorf("raw must be hidden when it equals credited WPM:\n%s", content)
	}
}

func TestResultsCaptionMatchesTier(t *testing.T) {
	cases := []struct {
		name          string
		wpm, accuracy float64
		want          string
	}{
		{"celebrate", 80, 99, "On fire!"},
		{"proud", 20, 96, "Solid"},
		{"calm", 20, 90, "Steady"},
		{"worried", 20, 50, "Rough one"},
	}
	for _, tc := range cases {
		m := New(engine.Metrics{Duration: 30 * time.Second, WPM: tc.wpm, Accuracy: tc.accuracy})
		if got := ansi.Strip(m.renderContent()); !strings.Contains(got, tc.want) {
			t.Errorf("%s: content = %q, want caption %q", tc.name, got, tc.want)
		}
	}
}

func TestResultsMeasuresWrappedContent(t *testing.T) {
	m := New(engine.Metrics{Duration: 2 * time.Minute, WPM: 120, Raw: 120, Accuracy: 100})
	wide := contentRows(m)

	m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	narrow := contentRows(m)

	if narrow <= wide {
		t.Fatalf("narrow content must wrap to more rows: wide=%d narrow=%d", wide, narrow)
	}
	if m.layout.Content.Height != narrow {
		t.Errorf("layout content height = %d, want measured %d", m.layout.Content.Height, narrow)
	}
	if got := len(strings.Split(m.View(), "\n")); got != 24 {
		t.Errorf("composed rows = %d, want 24", got)
	}
}

func contentRows(m *Model) int {
	return len(strings.Split(ansi.Strip(m.renderContent()), "\n"))
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
