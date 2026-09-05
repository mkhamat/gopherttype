package app

import (
	"math/rand/v2"
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/engine"
	"gopherttype/internal/words"
)

type screen int

const (
	home screen = iota
	play
	results
)

type Model struct {
	screen       screen
	game         *engine.Game
	generator    *words.Generator
	errorMessage string
}

func New() *Model {
	seed1 := rand.Uint64()
	seed2 := rand.Uint64()
	generator := words.New(seed1, seed2)
	return &Model{
		screen:    home,
		generator: generator,
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "esc", "ctrl+c":
		return m, tea.Quit
	}

	switch m.screen {
	case home:
		return m, m.handleHomeKey(key)
	case play:
		return m, m.handlePlayKey(key)
	case results:
		if key.String() == "q" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *Model) View() tea.View {
	switch m.screen {
	case home:
		text := "words 10\nenter to start, q to quit"
		if m.errorMessage != "" {
			text = m.errorMessage + "\n\n" + text
		}
		return tea.NewView(text)
	case play:
		snapshot := m.game.Snapshot(time.Now())
		return tea.NewView(renderWords(snapshot.Words, snapshot.CurrentWordIndex))
	case results:
		return tea.NewView("results screen")
	}
	return tea.View{}
}
