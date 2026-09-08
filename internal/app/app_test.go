package app

import (
	"image/color"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/home"
	"gopherttype/internal/app/play"
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
		m.Update(tea.KeyPressMsg{Text: word})
		m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "results") {
		t.Fatal("round did not reach results")
	}
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

func TestGlobalQuit(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEscape}, {Code: 'c', Mod: tea.ModCtrl}} {
		m := New()
		for _, active := range []screen{m.active, play.New(engine.Config{Mode: engine.ModeWords, WordCount: 10}, m.generator.Generate)} {
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
