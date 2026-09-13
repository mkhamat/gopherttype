package play

import (
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/mascot"
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

func (m *Model) handlePlayKeyAt(key tea.KeyPressMsg, at time.Time) tea.Cmd {
	previousStatus := m.game.Status()
	eligible := m.inputEligible(previousStatus, at)
	switch key.String() {
	case "backspace":
		if eligible {
			m.mascot.Activity(at)
		}
		m.game.Handle(engine.Event{Kind: engine.Backspace, At: at})
	case "ctrl+backspace", "alt+backspace":
		if eligible {
			m.mascot.Activity(at)
		}
		m.game.Handle(engine.Event{Kind: engine.DeleteWord, At: at})
	case "space", "shift+space":
		if eligible {
			m.observeSpace(at)
		}
		m.game.Handle(engine.Event{Kind: engine.Space, At: at})
	default:
		if key.Text == "" {
			return nil
		}
		snapshot := m.playUI.snapshot
		word := snapshot.Words[snapshot.CurrentWordIndex]
		target := []rune(word.Target)
		position := len(word.Typed)
		pace := utf8.RuneCountInString(key.Text) == 1
		for _, r := range key.Text {
			if len(word.Typed) >= len(target) && !m.playUI.layout.acceptsExtra(word, r, m.styles) {
				continue
			}
			if eligible && r != 0 {
				correct := position < len(target) && target[position] == r
				m.mascot.ObserveAttempt(mascot.Attempt{At: at, Correct: correct, Pace: pace})
				position++
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

// inputEligible reports whether a key event may be observed by the reaction
// tracker. Finished input is always ineligible; a timed event at or past the
// deadline is ineligible even though the engine still processes it through its
// original path.
func (m *Model) inputEligible(status engine.Status, at time.Time) bool {
	if status == engine.Finished {
		return false
	}
	if status == engine.Playing && m.roundConfig.Mode == engine.ModeTime && m.game.ElapsedAt(at) >= m.roundConfig.Duration {
		return false
	}
	return true
}

// observeSpace records one space submission from pre-event state. It only
// counts a nonempty word; correctness needs a matching whole word and an
// existing separator (timed mode or a nonfinal word-count word).
func (m *Model) observeSpace(at time.Time) {
	snapshot := m.playUI.snapshot
	if snapshot.CurrentWordIndex >= len(snapshot.Words) {
		return
	}
	word := snapshot.Words[snapshot.CurrentWordIndex]
	if len(word.Typed) == 0 {
		return
	}
	hasSeparator := m.roundConfig.Mode == engine.ModeTime || snapshot.CurrentWordIndex < len(snapshot.Words)-1
	m.mascot.ObserveAttempt(mascot.Attempt{At: at, Correct: hasSeparator && string(word.Typed) == word.Target})
}

func (m *Model) handleTick(msg tickMsg) tea.Cmd {
	if msg.game != m.game || m.game.Status() != engine.Playing {
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
		m.game.AppendWords(m.generate(replenishBatch))
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
