package app

import (
	"math/rand/v2"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/home"
	"gopherttype/internal/app/play"
	"gopherttype/internal/app/results"
	"gopherttype/internal/engine"
	"gopherttype/internal/words"
)

type screen interface {
	Init() tea.Cmd
	Update(tea.Msg) tea.Cmd
	View() string
}

type Model struct {
	active      screen
	roundConfig engine.Config
	generator   *words.Generator
	size        tea.WindowSizeMsg
	background  *tea.BackgroundColorMsg
}

func New() *Model {
	return &Model{
		active:    home.New(),
		generator: words.New(rand.Uint64(), rand.Uint64()),
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.active.Init(), tea.RequestBackgroundColor)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.size = msg
	case tea.BackgroundColorMsg:
		m.background = &msg
	case home.StartMsg:
		m.roundConfig = msg.Config
		return m, m.switchScreen(play.New(m.roundConfig, m.generator.Generate))
	case play.FinishedMsg:
		return m, m.switchScreen(results.New(msg.Metrics))
	case results.RetryMsg:
		if _, ok := m.active.(*results.Model); !ok {
			return m, nil
		}
		return m, m.switchScreen(play.New(m.roundConfig, m.generator.Generate))
	case results.HomeMsg:
		if _, ok := m.active.(*results.Model); !ok {
			return m, nil
		}
		return m, m.switchScreen(home.New())
	}
	return m, m.active.Update(msg)
}

func (m *Model) View() tea.View {
	view := tea.NewView(m.active.View())
	view.AltScreen = true
	return view
}

func (m *Model) switchScreen(next screen) tea.Cmd {
	m.active = next
	initCmd := m.active.Init()
	sizeCmd := m.active.Update(m.size)
	var backgroundCmd tea.Cmd
	if m.background != nil {
		backgroundCmd = m.active.Update(*m.background)
	}
	return tea.Batch(initCmd, sizeCmd, backgroundCmd)
}
