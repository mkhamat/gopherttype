package results

import (
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func TestResultsResize(t *testing.T) {
	m := New(engine.Metrics{WPM: 42, Accuracy: 98.5, Duration: 30 * time.Second})
	if cmd := m.Init(); cmd != nil {
		t.Fatal("results must not schedule commands on initialization")
	}
	for _, size := range [][2]int{{100, 35}, {80, 24}, {28, 12}, {28, 5}, {10, 3}, {1, 1}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.View()
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatalf("results overflow at %dx%d", size[0], size[1])
		}
		plain := strings.Join(strings.Fields(ansi.Strip(view)), " ")
		if size[0] >= ui.MinimumWidth && size[1] >= ui.MinimumHeight {
			for _, want := range []string{"results", "WPM: 42", "Accuracy: 98.50%", "Elapsed: 30s", "Enter retry", "Tab home", "q / Esc quit"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("results missing %q: %s", want, plain)
				}
			}
		} else if size[0] >= ui.MinimumWidth && !strings.Contains(plain, "Resize to") {
			t.Fatal("short results screen must show resize fallback")
		}
	}
}

func TestBackgroundChangesUpdateResults(t *testing.T) {
	metrics := engine.Metrics{WPM: 42, Accuracy: 98.5, Duration: time.Minute}
	m := New(metrics)
	before := m.View()
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if before == m.View() || m.styles != ui.StylesFor(false) || m.metrics != metrics {
		t.Fatal("results theme change failed or changed final metrics")
	}
}

func TestResultsNavigation(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: tea.KeyTab},
		{Code: 'q', Text: "q"},
	} {
		t.Run(key.String(), func(t *testing.T) {
			m := New(engine.Metrics{})
			cmd := m.Update(key)
			if cmd == nil {
				t.Fatal("navigation did not emit a command")
			}
			msg := cmd()
			switch key.Code {
			case tea.KeyEnter:
				if _, ok := msg.(RetryMsg); !ok {
					t.Fatalf("got %T, want RetryMsg", msg)
				}
			case tea.KeyTab:
				if _, ok := msg.(HomeMsg); !ok {
					t.Fatalf("got %T, want HomeMsg", msg)
				}
			default:
				if _, ok := msg.(tea.QuitMsg); !ok {
					t.Fatalf("got %T, want tea.QuitMsg", msg)
				}
				return
			}
		})
	}
}

func TestResultsIgnoreTyping(t *testing.T) {
	m := New(engine.Metrics{WPM: 42})
	before := m.View()
	if cmd := m.Update(tea.KeyPressMsg{Text: "x"}); cmd != nil || m.View() != before {
		t.Fatal("typing changed results")
	}
}
