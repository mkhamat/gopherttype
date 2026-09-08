package app

import (
	"math/rand/v2"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/home"
	"gopherttype/internal/app/play"
	"gopherttype/internal/words"
)

type screen interface {
	Init() tea.Cmd
	Update(tea.Msg) tea.Cmd
	View() string
}

type Model struct {
	active     screen
	generator  *words.Generator
	size       tea.WindowSizeMsg
	background *tea.BackgroundColorMsg
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
		if msg.String() == "esc" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.size = msg
	case tea.BackgroundColorMsg:
		m.background = &msg
	case home.StartMsg:
		p := play.New(msg.Config, m.generator.Generate)
		m.active = p
		cmd := p.Init()
		return m, tea.Batch(cmd, m.syncScreen())
	case play.HomeMsg:
		m.active = home.New()
		cmd := m.active.Init()
		return m, tea.Batch(cmd, m.syncScreen())
	}
	return m, m.active.Update(msg)
}

func (m *Model) syncScreen() tea.Cmd {
	sizeCmd := m.active.Update(m.size)
	var backgroundCmd tea.Cmd
	if m.background != nil {
		backgroundCmd = m.active.Update(*m.background)
	}
	return tea.Batch(sizeCmd, backgroundCmd)
}

func (m *Model) View() tea.View {
	view := tea.NewView(m.active.View())
	view.AltScreen = true
	return view
}
