package app

import (
	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/engine"
)

func (m *Model) handleHomeKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "q":
		return tea.Quit
	case "enter":
		m.startGame()
	}
	return nil
}

func (m *Model) startGame() {
	targetWords := m.generator.Generate(10)
	config := engine.Config{
		Mode:      engine.ModeWords,
		WordCount: 10,
	}

	newGame, err := engine.New(config, targetWords)
	if err != nil {
		m.errorMessage = "game creation: " + err.Error()
		return
	}

	m.game = newGame
	m.errorMessage = ""
	m.screen = play
}
