package play

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/engine"
)

func TestPlayTimeModeArmsGameTick(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeTime, Duration: 15 * time.Second})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	cmd := m.handlePlayKeyAt(key('g'), time.Unix(1000, 0))
	if m.game.Status() != engine.Playing {
		t.Fatalf("first rune must start the round, status %v", m.game.Status())
	}
	if cmd == nil {
		t.Fatal("starting a timed round must arm the game tick")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Fatalf("start tick returned %T, want tickMsg", cmd())
	}
}

func TestPlayTickRejectsForeignAndNonPlaying(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeTime, Duration: 15 * time.Second})
	foreign := engine.New(m.roundConfig, []string{"go"})
	if cmd := m.handleTick(tickMsg{game: foreign, at: time.Unix(1000, 0)}); cmd != nil {
		t.Error("a foreign game tick must be ignored")
	}
	if cmd := m.handleTick(tickMsg{game: m.game, at: time.Unix(1000, 0)}); cmd != nil {
		t.Error("a tick while the round is Ready must be ignored")
	}
}

func TestPlayPlayingTickContinues(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeTime, Duration: 15 * time.Second})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	start := time.Unix(1000, 0)
	m.handlePlayKeyAt(key('g'), start)

	cmd := m.handleTick(tickMsg{game: m.game, at: start.Add(100 * time.Millisecond)})
	if cmd == nil {
		t.Fatal("a playing tick must reschedule")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Fatalf("tick chain returned %T, want tickMsg", cmd())
	}
}
