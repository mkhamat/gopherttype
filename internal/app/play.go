package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/engine"
)

func (m *Model) handlePlayKey(key tea.KeyPressMsg) tea.Cmd {
	now := time.Now()

	switch key.String() {
	case "backspace":
		m.game.Handle(engine.Event{
			Kind: engine.Backspace,
			At:   now,
		})
	case "ctrl+backspace", "alt+backspace":
		m.game.Handle(engine.Event{
			Kind: engine.DeleteWord,
			At:   now,
		})
	case "space":
		m.game.Handle(engine.Event{
			Kind: engine.Space,
			At:   now,
		})
	default:
		for _, r := range key.Key().Text {
			m.game.Handle(engine.Event{
				Kind: engine.Type,
				Rune: r,
				At:   now,
			})
		}
	}

	if m.game.Snapshot(now).Status == engine.Finished {
		m.screen = results
	}

	return nil
}
