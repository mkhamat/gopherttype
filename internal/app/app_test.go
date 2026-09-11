package app

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/home"
	"gopherttype/internal/app/play"
	"gopherttype/internal/app/results"
	"gopherttype/internal/engine"
	"gopherttype/internal/words"
)

func settle(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	budget := 1000
	var run func(tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		budget--
		if budget < 0 {
			t.Fatal("commands did not settle")
		}
		msg := cmd()
		if msg == nil {
			return
		}
		value := reflect.ValueOf(msg)
		if value.Kind() == reflect.Slice {
			for i := 0; i < value.Len(); i++ {
				next, ok := value.Index(i).Interface().(tea.Cmd)
				if !ok {
					t.Fatalf("unexpected command collection: %T", msg)
				}
				run(next)
			}
			return
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("unexpected quit")
		}
		_, next := m.Update(msg)
		run(next)
	}
	run(cmd)
}

func send(t *testing.T, m *Model, msg tea.Msg) {
	t.Helper()
	_, cmd := m.Update(msg)
	settle(t, m, cmd)
}

func TestScreenTransitions(t *testing.T) {
	m := New()
	m.generator = words.New(1, 2)
	targets := words.New(1, 2).Generate(25)
	initialHome := m.active
	settle(t, m, m.active.Init())
	send(t, m, tea.WindowSizeMsg{Width: 40, Height: 16})
	send(t, m, tea.BackgroundColorMsg{Color: color.White})
	before := m.View().Content
	send(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	send(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	send(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	for range 3 {
		send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	p, ok := m.active.(*play.Model)
	if !ok {
		t.Fatal("home did not start play")
	}
	view := m.View()
	if !view.AltScreen || lipgloss.Width(view.Content) != 40 || lipgloss.Height(view.Content) != 16 {
		t.Fatal("new screen lost terminal dimensions or alternate screen")
	}
	for _, word := range targets {
		send(t, m, tea.KeyPressMsg{Text: word})
		send(t, m, tea.KeyPressMsg{Code: tea.KeySpace})
	}
	if _, ok := m.active.(*results.Model); !ok || !strings.Contains(ansi.Strip(m.View().Content), "results") {
		t.Fatal("round did not reach the results screen")
	}
	if got := m.View().Content; lipgloss.Width(got) != 40 || lipgloss.Height(got) != 16 {
		t.Fatal("results lost terminal dimensions")
	}
	beforeTheme := m.View().Content
	send(t, m, tea.BackgroundColorMsg{Color: color.Black})
	if beforeTheme == m.View().Content {
		t.Fatal("results did not inherit the light theme")
	}
	send(t, m, tea.BackgroundColorMsg{Color: color.White})
	send(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if _, ok := m.active.(*home.Model); !ok || m.active == initialHome || m.View().Content != before {
		t.Fatal("return home did not create a fresh screen with default settings and the same size and theme")
	}
	for range 3 {
		send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	if _, ok := m.active.(*play.Model); !ok || m.active == p {
		t.Fatal("second start did not create a new play model")
	}
}

func TestResultsRetry(t *testing.T) {
	for _, config := range []engine.Config{
		{Mode: engine.ModeWords, WordCount: 10},
		{Mode: engine.ModeTime, Duration: 30 * time.Second},
	} {
		t.Run(fmt.Sprint(config.Mode), func(t *testing.T) {
			m := New()
			send(t, m, tea.WindowSizeMsg{Width: 40, Height: 16})
			send(t, m, tea.BackgroundColorMsg{Color: color.White})
			send(t, m, home.StartMsg{Config: config})
			previous := m.active
			metrics := engine.Metrics{WPM: 42, Accuracy: 98.5, Duration: 30 * time.Second}
			send(t, m, play.FinishedMsg{Metrics: metrics})
			if _, ok := m.active.(*results.Model); !ok {
				t.Fatal("completion did not switch to results")
			}
			plain := strings.Join(strings.Fields(ansi.Strip(m.View().Content)), " ")
			for _, want := range []string{"WPM: 42", "Accuracy: 98.50%", "Elapsed: 30s"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("results missing %q: %s", want, plain)
				}
			}
			send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
			if _, ok := m.active.(*play.Model); !ok || m.active == previous || m.roundConfig != config {
				t.Fatal("retry did not create a new play screen with the same configuration")
			}
			want := "Remaining: 10 words"
			if config.Mode == engine.ModeTime {
				want = "Remaining: 30s"
			}
			view := m.View().Content
			if !strings.Contains(ansi.Strip(view), want) || lipgloss.Width(view) != 40 || lipgloss.Height(view) != 16 {
				t.Fatalf("retry lost configuration or dimensions: %s", ansi.Strip(view))
			}
			beforeTheme := view
			send(t, m, tea.BackgroundColorMsg{Color: color.Black})
			if beforeTheme == m.View().Content {
				t.Fatal("retry did not inherit the light theme")
			}
		})
	}
}

func TestResultsNavigationIgnoresQueuedMessages(t *testing.T) {
	for _, first := range []tea.Msg{results.RetryMsg{}, results.HomeMsg{}} {
		t.Run(fmt.Sprintf("%T", first), func(t *testing.T) {
			m := New()
			m.roundConfig = engine.Config{Mode: engine.ModeWords, WordCount: 10}
			m.active = results.New(engine.Metrics{})
			send(t, m, first)
			active := m.active
			for _, queued := range []tea.Msg{results.RetryMsg{}, results.HomeMsg{}} {
				_, cmd := m.Update(queued)
				if cmd != nil || m.active != active {
					t.Fatal("queued results navigation changed screens")
				}
			}
		})
	}
}

func TestGlobalQuit(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{{Code: 'c', Mod: tea.ModCtrl}} {
		m := New()
		for _, active := range []screen{m.active, play.New(engine.Config{Mode: engine.ModeWords, WordCount: 10}, m.generator.Generate), results.New(engine.Metrics{})} {
			m.active = active
			_, cmd := m.Update(key)
			if cmd == nil {
				t.Fatal("global quit returned no command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("global quit did not quit")
			}
		}
	}
}
