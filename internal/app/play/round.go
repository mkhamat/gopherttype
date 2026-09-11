package play

import (
	"fmt"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/engine"
)

const (
	tickInterval       = 100 * time.Millisecond
	replenishThreshold = 25
	replenishBatch     = 50
)

type tickMsg struct {
	game *engine.Game
	at   time.Time
}

func tick(game *engine.Game) tea.Cmd {
	return tea.Tick(tickInterval, func(at time.Time) tea.Msg {
		return tickMsg{game: game, at: at}
	})
}

func (m *Model) startGame() {
	count := m.roundConfig.WordCount
	if m.roundConfig.Mode == engine.ModeTime {
		count = replenishBatch
	}
	game, err := engine.New(m.roundConfig, m.generate(count))
	if err != nil {
		panic(fmt.Errorf("starting round: %w", err))
	}
	m.game = game
	m.refreshPlayState(time.Now(), true)
}

func (m *Model) handlePlayKeyAt(key tea.KeyPressMsg, at time.Time) tea.Cmd {
	previousStatus := m.game.Status()
	switch key.String() {
	case "backspace":
		m.game.Handle(engine.Event{Kind: engine.Backspace, At: at})
	case "ctrl+backspace", "alt+backspace":
		m.game.Handle(engine.Event{Kind: engine.DeleteWord, At: at})
	case "space", "shift+space":
		m.game.Handle(engine.Event{Kind: engine.Space, At: at})
	default:
		if key.Text == "" {
			return nil
		}
		snapshot := m.playUI.snapshot
		word := snapshot.Words[snapshot.CurrentWordIndex]
		for _, r := range key.Text {
			if len(word.Typed) >= utf8.RuneCountInString(word.Target) && !m.playUI.layout.acceptsExtra(word, r, m.styles) {
				continue
			}
			word.Typed = append(word.Typed, r)
			m.game.Handle(engine.Event{Kind: engine.Type, Rune: r, At: at})
			if m.game.Status() == engine.Finished {
				break
			}
		}
	}
	if cmd := m.refreshPlayState(at, true); cmd != nil {
		return cmd
	}
	if m.roundConfig.Mode == engine.ModeTime && previousStatus == engine.Ready && m.game.Status() == engine.Playing {
		return tick(m.game)
	}
	return nil
}

func (m *Model) handleTick(msg tickMsg) tea.Cmd {
	if m.roundConfig.Mode != engine.ModeTime || msg.game != m.game || m.game.Status() != engine.Playing {
		return nil
	}
	at := msg.at
	if at.Before(m.playUI.updatedAt) {
		at = m.playUI.updatedAt
	}
	m.game.Handle(engine.Event{Kind: engine.Tick, At: at})
	if cmd := m.refreshPlayState(at, false); cmd != nil {
		return cmd
	}
	return tick(m.game)
}

func (m *Model) refreshPlayState(at time.Time, wordsChanged bool) tea.Cmd {
	if m.game.Status() == engine.Finished {
		metrics := m.game.FinalMetrics()
		return func() tea.Msg { return FinishedMsg{Metrics: metrics} }
	}
	if m.roundConfig.Mode == engine.ModeTime && m.game.RemainingWords() < replenishThreshold {
		if err := m.game.AppendWords(m.generate(replenishBatch)); err != nil {
			panic(fmt.Errorf("adding round words: %w", err))
		}
		wordsChanged = true
	}
	m.playUI.updatedAt = at
	if wordsChanged {
		m.playUI.snapshot = m.game.Snapshot(at)
		m.layoutPlay()
	} else {
		m.playUI.snapshot.Elapsed = m.game.ElapsedAt(at)
	}
	return nil
}
