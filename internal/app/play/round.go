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
	m.err = nil
	if err := m.roundConfig.Validate(); err != nil {
		m.fail(fmt.Errorf("starting round: %w", err))
		return
	}
	count := m.roundConfig.WordCount
	if m.roundConfig.Mode == engine.ModeTime {
		count = replenishBatch
	}
	game, err := engine.New(m.roundConfig, m.generate(count))
	if err != nil {
		m.fail(fmt.Errorf("starting round: %w", err))
		return
	}
	m.game = game
	m.screen = play
	m.playUI = playState{}
	m.syncGame(time.Now(), true)
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
		if snapshot.CurrentWordIndex >= len(snapshot.Words) {
			break
		}
		word := snapshot.Words[snapshot.CurrentWordIndex]
		for _, r := range key.Text {
			extra := len(word.Typed) >= utf8.RuneCountInString(word.Target)
			word.Typed = append(word.Typed, r)
			if extra {
				_, width := wordCells(word, false, false, m.styles)
				if m.playUI.layout.activeColumn+width > m.wordWidth() {
					word.Typed = word.Typed[:len(word.Typed)-1]
					continue
				}
			}
			m.game.Handle(engine.Event{Kind: engine.Type, Rune: r, At: at})
			if m.game.Status() == engine.Finished {
				break
			}
		}
	}
	m.syncGame(at, true)
	if m.screen == play && previousStatus == engine.Ready && m.game.Status() == engine.Playing {
		return tick(m.game)
	}
	return nil
}

func (m *Model) handleTick(msg tickMsg) tea.Cmd {
	if m.screen != play || msg.game != m.game || m.game.Status() != engine.Playing {
		return nil
	}
	at := msg.at
	if at.Before(m.playUI.at) {
		at = m.playUI.at
	}
	m.game.Handle(engine.Event{Kind: engine.Tick, At: at})
	m.syncGame(at, false)
	if m.screen == play {
		return tick(m.game)
	}
	return nil
}

func (m *Model) syncGame(at time.Time, wordsChanged bool) {
	if m.game.Status() == engine.Finished {
		m.screen = results
		return
	}
	if m.roundConfig.Mode == engine.ModeTime && m.game.RemainingWords() < replenishThreshold {
		if err := m.game.AppendWords(m.generate(replenishBatch)); err != nil {
			m.fail(fmt.Errorf("adding round words: %w", err))
			return
		}
		wordsChanged = true
	}
	m.playUI.at = at
	if wordsChanged {
		m.playUI.snapshot = m.game.Snapshot(at)
		m.layoutPlay()
	} else {
		m.playUI.snapshot.Metrics = m.game.MetricsAt(at)
	}
}
