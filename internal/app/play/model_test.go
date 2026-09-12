package play

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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

func newPlayModel(config engine.Config) *Model {
	return New(config, func(n int) []string {
		words := make([]string, n)
		for i := range words {
			words[i] = "go"
		}
		return words
	})
}

func TestPlayShowsMascotAt80x24(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 3})
	cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd == nil {
		t.Fatal("configure must arm the mascot clock")
	}
	if m.layout.Mascot == (ui.Rect{}) {
		t.Fatalf("mascot should be visible at 80x24, layout %+v", m.layout)
	}
	if m.mascot.View() == "" {
		t.Fatal("a visible mascot should render a portrait")
	}
	if rows := len(strings.Split(m.View(), "\n")); rows != 24 {
		t.Fatalf("view rows = %d, want 24", rows)
	}
}

func TestPlayHidesBelowMinimum(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 3})
	m.Update(tea.WindowSizeMsg{Width: 27, Height: 11})
	if want := ui.ResizeView(27, 11, "Esc / Ctrl+C quit"); m.View() != want {
		t.Error("below minimum must keep the ResizeView")
	}
	if m.mascot.View() != "" {
		t.Error("mascot must be hidden below minimum")
	}
}

func TestPlayHidesWhenTooShort(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 3})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 17})
	if m.layout.Mascot != (ui.Rect{}) {
		t.Errorf("60x17 should hide the mascot, got %+v", m.layout.Mascot)
	}
	if m.scene().Track {
		t.Error("a hidden mascot must not track a target")
	}
}

func TestPlayFrameBypassesGame(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 3})
	cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd == nil {
		t.Fatal("configure must arm the mascot clock")
	}
	frame, ok := cmd().(mascot.FrameMsg)
	if !ok {
		t.Fatalf("armed command returned %T, want mascot.FrameMsg", cmd())
	}

	before := m.game.Snapshot(time.Unix(1000, 0))
	status := m.game.Status()

	if m.Update(frame) == nil {
		t.Error("a valid mascot frame must return its successor")
	}
	after := m.game.Snapshot(time.Unix(1000, 0))
	if m.game.Status() != status || !reflect.DeepEqual(before, after) {
		t.Error("a mascot frame must not touch the game")
	}
	if m.Update(frame) != nil {
		t.Error("a stale mascot frame must not reschedule")
	}
}

func TestPlayCursorTargetTracksRenderedCell(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 3})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.handlePlayKeyAt(key('g'), time.Unix(1000, 0))
	m.refreshContent(false)
	m.rebuild()

	scene := m.scene()
	if !scene.Track {
		t.Fatal("typing should track the caret")
	}
	cursor := m.playUI.layout
	first := cursor.firstVisibleRow(playVisibleLines)
	want := ui.Point{
		X: float64(m.layout.Content.X+cursor.cursorColumn) + float64(cursor.cursorWidth)/2,
		Y: float64(m.layout.Content.Y+m.headerHeight+1+cursor.cursorRow-first) + 0.5,
	}
	if scene.Target != want {
		t.Errorf("target = %+v, want %+v", scene.Target, want)
	}
}

func TestPlayEscReturnsHome(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 3})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	cmd := m.Update(key(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("Esc must return HomeMsg")
	}
	if _, ok := cmd().(HomeMsg); !ok {
		t.Fatalf("Esc returned %T, want HomeMsg", cmd())
	}
}

// TestPlayMascotFlowAndIdleExpressions drives the real play observation stream
// through the play handler, then steps the mascot's shared clock to a settled
// frame. Identical key sequences with different timing must render distinct
// calm, excited and sleepy expressions without touching the engine outcome.
func TestPlayMascotFlowAndIdleExpressions(t *testing.T) {
	t0 := time.Unix(1000, 0)

	render := func(keys []time.Time, drive time.Time, frames int) string {
		m := New(engine.Config{Mode: engine.ModeWords, WordCount: 1},
			func(int) []string { return []string{strings.Repeat("a", 20)} })
		m.width, m.height = 80, 24
		m.refreshContent(true)
		for _, at := range keys {
			m.handlePlayKeyAt(key('a'), at)
		}
		m.refreshContent(true)
		_, cmd := m.mascot.Configure(m.scene(), drive)
		if cmd == nil {
			t.Fatal("mascot configure must arm the clock")
		}
		at := drive
		for i := 0; i < frames; i++ {
			frame, ok := cmd().(mascot.FrameMsg)
			if !ok {
				t.Fatalf("frame %d: got %T, want mascot.FrameMsg", i, cmd())
			}
			at = at.Add(50 * time.Millisecond)
			_, cmd = m.mascot.Update(frame, at)
			if cmd == nil {
				t.Fatalf("frame %d: mascot clock chain ended", i)
			}
		}
		return m.mascot.View()
	}

	keys := func(step time.Duration) []time.Time {
		out := make([]time.Time, 15)
		for i := range out {
			out[i] = t0.Add(time.Duration(i) * step)
		}
		return out
	}

	calm := render(keys(10*time.Millisecond), t0.Add(200*time.Millisecond), 20)
	excited := render(keys(100*time.Millisecond), t0.Add(1500*time.Millisecond), 30)
	sleepy := render(keys(10*time.Millisecond), t0.Add(5*time.Second), 20)

	if excited == calm {
		t.Error("fast accurate flow must look different from calm")
	}
	if sleepy == calm {
		t.Error("a three-second pause must look different from calm")
	}
	if excited == sleepy {
		t.Error("excited and sleepy must differ")
	}
}
