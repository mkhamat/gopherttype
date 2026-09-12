package home

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/mascot"
	"gopherttype/internal/app/ui"
)

func key(code rune) tea.KeyPressMsg {
	k := tea.Key{Code: code}
	if code >= 0x20 && code != 0x7f {
		k.Text = string(code)
	}
	return tea.KeyPressMsg(k)
}

func TestHomeDefaultsFitsAt80x24(t *testing.T) {
	m := New()
	m.configure(time.Now())

	if want := (ui.Rect{X: 24, Y: 1, Width: 32, Height: 12}); m.layout.Mascot != want {
		t.Errorf("mascot slot = %+v, want %+v", m.layout.Mascot, want)
	}
	if rows := strings.Count(m.content, "\n") + 1; rows != 10 {
		t.Errorf("content rows = %d, want 10", rows)
	}
	if m.mascot.View() == "" {
		t.Fatal("configured mascot should render a portrait")
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 24 {
		t.Fatalf("view rows = %d, want 24", len(lines))
	}
	for i, pl := range strings.Split(m.mascot.View(), "\n") {
		if want := strings.Repeat(" ", m.layout.Mascot.X) + pl; lines[m.layout.Mascot.Y+i] != want {
			t.Errorf("portrait row %d misplaced", i)
		}
	}
	for i, cl := range strings.Split(m.content, "\n") {
		if want := strings.Repeat(" ", m.layout.Content.X) + cl; lines[m.layout.Content.Y+i] != want {
			t.Errorf("content row %d misplaced", i)
		}
	}
}

func TestHomeHidesAt40x20(t *testing.T) {
	m := New()
	m.width, m.height = 40, 20
	m.configure(time.Now())
	if m.layout.Mascot != (ui.Rect{}) {
		t.Errorf("40x20 should hide the mascot, got %+v", m.layout.Mascot)
	}
	if want := ui.FitView(m.content, 40, 20); m.View() != want {
		t.Error("hidden home must use the existing FitView result")
	}
	if m.mascot.View() != "" {
		t.Error("hidden mascot must render nothing")
	}
}

func TestHomeMinimumKeepsResizeView(t *testing.T) {
	m := New()
	m.width, m.height = 27, 11
	m.configure(time.Now())
	if want := ui.ResizeView(27, 11, "q / Esc quit"); m.View() != want {
		t.Error("below minimum must keep the ResizeView")
	}
}

func TestHomeConfigureArmsOneChain(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	if m.configure(t0) == nil {
		t.Fatal("first configure must arm the mascot clock")
	}
	if m.configure(t0.Add(time.Millisecond)) != nil {
		t.Error("a second configure must not arm another chain")
	}
}

func TestHomeFrameBypassesForm(t *testing.T) {
	m := New()
	cmd := m.configure(time.Unix(1000, 0))
	if cmd == nil {
		t.Fatal("configure did not arm")
	}
	frame, ok := cmd().(mascot.FrameMsg)
	if !ok {
		t.Fatalf("armed command returned %T", cmd())
	}

	settings := m.settings
	focused := m.homeUI.form.GetFocusedField().GetKey()
	if successor := m.Update(frame); successor == nil {
		t.Error("a valid mascot frame must return its successor")
	}
	if m.settings != settings {
		t.Error("a mascot frame must not touch form settings")
	}
	if m.homeUI.form.GetFocusedField().GetKey() != focused {
		t.Error("a mascot frame must not move form focus")
	}
	if successor := m.Update(frame); successor != nil {
		t.Error("a stale mascot frame must not reschedule")
	}
}

func TestHomeDepartureHidesMascot(t *testing.T) {
	enter := New()
	enter.configure(time.Now())
	cmd := enter.Update(key(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter must return StartMsg")
	}
	msg := cmd()
	if _, ok := msg.(StartMsg); !ok {
		t.Fatalf("Enter returned %T, want StartMsg", msg)
	}
	if enter.mascot.View() != "" {
		t.Error("Enter must hide the mascot")
	}

	quit := New()
	quit.configure(time.Now())
	if cmd := quit.Update(key('q')); cmd == nil {
		t.Fatal("q must return a quit command")
	}
	if quit.mascot.View() != "" {
		t.Error("q must hide the mascot")
	}
}
