package app

import (
	"image/color"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/home"
	"gopherttype/internal/app/mascot"
	"gopherttype/internal/app/play"
	"gopherttype/internal/app/results"
	"gopherttype/internal/engine"
)

type fakeScreen struct{ calls []tea.Msg }

func (f *fakeScreen) Init() tea.Cmd { return f.Update("init") }
func (f *fakeScreen) Update(msg tea.Msg) tea.Cmd {
	f.calls = append(f.calls, msg)
	return func() tea.Msg { return msg }
}
func (f *fakeScreen) View() string { return "" }

func TestSwitchScreenConfiguresAndBatches(t *testing.T) {
	bg := tea.BackgroundColorMsg{Color: color.RGBA{R: 1, G: 2, B: 3, A: 0xff}}
	for _, background := range []*tea.BackgroundColorMsg{nil, &bg} {
		m := New()
		m.size = tea.WindowSizeMsg{Width: 100, Height: 40}
		m.background = background
		next := &fakeScreen{}
		cmd := m.switchScreen(next)
		want := []tea.Msg{"init", m.size}
		if background != nil {
			want = append(want, *background)
		}
		if m.active != next || !reflect.DeepEqual(next.calls, want) {
			t.Fatalf("screen configuration = %v, want %v on the new active screen", next.calls, want)
		}
		if cmd == nil {
			t.Fatal("configuration commands were lost")
		}
		batch, ok := cmd().(tea.BatchMsg)
		if !ok || len(batch) != len(want) {
			t.Fatalf("batch = %v, want %d commands", batch, len(want))
		}
		for i, command := range batch {
			if got := command(); !reflect.DeepEqual(got, want[i]) {
				t.Errorf("command %d = %v, want %v", i, got, want[i])
			}
		}
	}
}

// firstFrame unwraps a single or batched command to the mascot frame it arms.
func firstFrame(t *testing.T, cmd tea.Cmd) mascot.FrameMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command to extract a mascot frame from")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if f, ok := c().(mascot.FrameMsg); ok {
				return f
			}
		}
	}
	f, ok := msg.(mascot.FrameMsg)
	if !ok {
		t.Fatalf("command returned %T, want mascot.FrameMsg", msg)
	}
	return f
}

func TestAppDiscardsStaleFrameAfterNavigation(t *testing.T) {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := m.Update(home.StartMsg{Config: engine.Config{Mode: engine.ModeWords, WordCount: 3}})
	frame := firstFrame(t, cmd)
	m.Update(play.HomeMsg{})
	if _, ok := m.active.(*home.Model); !ok {
		t.Fatalf("after HomeMsg active = %T, want *home.Model", m.active)
	}
	before := m.active.View()
	if _, cmd := m.Update(frame); cmd != nil || m.active.View() != before {
		t.Error("a departed screen's frame must not change the view or reschedule")
	}
}

func TestAppNavigation(t *testing.T) {
	m := New()
	if _, ok := m.active.(*home.Model); !ok {
		t.Fatalf("initial screen = %T, want *home.Model", m.active)
	}
	m.Update(home.StartMsg{Config: engine.Config{Mode: engine.ModeWords, WordCount: 10}})
	first, ok := m.active.(*play.Model)
	if !ok {
		t.Fatalf("after StartMsg active = %T, want *play.Model", m.active)
	}
	m.Update(play.FinishedMsg{Metrics: engine.Metrics{Duration: 5 * time.Second, WPM: 60, Accuracy: 99}})
	if _, ok := m.active.(*results.Model); !ok {
		t.Fatalf("after FinishedMsg active = %T, want *results.Model", m.active)
	}
	view := ansi.Strip(m.active.View())
	if !strings.Contains(view, "99.00%") || !strings.Contains(view, "On fire!") {
		t.Errorf("results missing final metrics: %q", view)
	}
	m.Update(results.RetryMsg{})
	if retry, ok := m.active.(*play.Model); !ok || retry == first {
		t.Fatalf("retry must construct fresh play, got %T", m.active)
	}
	for _, msg := range []tea.Msg{play.HomeMsg{}, results.HomeMsg{}} {
		m.Update(msg)
		if _, ok := m.active.(*home.Model); !ok {
			t.Fatalf("after %T active = %T, want *home.Model", msg, m.active)
		}
	}
}

func TestAppCtrlCQuits(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("ctrl+c must return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c must quit")
	}
}
