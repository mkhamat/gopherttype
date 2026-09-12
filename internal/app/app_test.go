package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/home"
	"gopherttype/internal/app/play"
	"gopherttype/internal/app/results"
	"gopherttype/internal/engine"
)

// fakeScreen records construction calls so switchScreen propagation is
// observable.
type fakeScreen struct {
	init, update int
}

func (f *fakeScreen) Init() tea.Cmd          { f.init++; return nil }
func (f *fakeScreen) Update(tea.Msg) tea.Cmd { f.update++; return nil }
func (f *fakeScreen) View() string           { return "" }

func TestSwitchScreenInstallsAndConfigures(t *testing.T) {
	m := New()
	m.active = &fakeScreen{}
	next := &fakeScreen{}
	m.switchScreen(next)

	if m.active != next {
		t.Error("active screen was not replaced")
	}
	if next.init != 1 {
		t.Errorf("new screen Init called %d times, want 1", next.init)
	}
	if next.update != 1 {
		t.Errorf("new screen received size update %d times, want 1", next.update)
	}
}

func TestAppNavigationResetsScreens(t *testing.T) {
	m := New()
	if _, ok := m.active.(*home.Model); !ok {
		t.Fatalf("initial screen = %T, want *home.Model", m.active)
	}

	m.Update(home.StartMsg{Config: engine.Config{Mode: engine.ModeWords, WordCount: 10}})
	if _, ok := m.active.(*play.Model); !ok {
		t.Fatalf("after StartMsg active = %T, want *play.Model", m.active)
	}

	m.Update(play.FinishedMsg{Metrics: engine.Metrics{}})
	if _, ok := m.active.(*results.Model); !ok {
		t.Fatalf("after FinishedMsg active = %T, want *results.Model", m.active)
	}

	m.Update(results.RetryMsg{})
	if _, ok := m.active.(*play.Model); !ok {
		t.Fatalf("after RetryMsg active = %T, want *play.Model", m.active)
	}

	m.Update(play.HomeMsg{})
	if _, ok := m.active.(*home.Model); !ok {
		t.Fatalf("after HomeMsg active = %T, want *home.Model", m.active)
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

// TestPlayEscReturnsHome checks play's Esc still returns HomeMsg.
func TestPlayEscReturnsHome(t *testing.T) {
	p := play.New(engine.Config{Mode: engine.ModeWords, WordCount: 1}, func(int) []string {
		return []string{"go"}
	})
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if cmd == nil {
		t.Fatal("Esc must return HomeMsg")
	}
	if _, ok := cmd().(play.HomeMsg); !ok {
		t.Fatal("Esc must return HomeMsg")
	}
}

// TestResultsDepartureKeys checks results' existing navigation and that it adds
// no Esc handling.
func TestResultsDepartureKeys(t *testing.T) {
	enter := results.New(engine.Metrics{})
	cmd := enter.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Enter must return RetryMsg")
	}
	if _, ok := cmd().(results.RetryMsg); !ok {
		t.Fatal("Enter must return RetryMsg")
	}

	tab := results.New(engine.Metrics{})
	if cmd := tab.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})); cmd == nil {
		t.Fatal("Tab must return HomeMsg")
	} else if _, ok := cmd().(results.HomeMsg); !ok {
		t.Fatal("Tab must return HomeMsg")
	}

	quit := results.New(engine.Metrics{})
	if cmd := quit.Update(tea.KeyPressMsg(tea.Key{Code: 'q'})); cmd == nil {
		t.Fatal("q must quit")
	}

	if cmd := results.New(engine.Metrics{}).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})); cmd != nil {
		t.Error("results must not handle Esc")
	}
}
