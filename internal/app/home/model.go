package home

import (
	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

type StartMsg struct{ Config engine.Config }

type Model struct {
	settings      settings
	homeUI        formState
	styles        *ui.Styles
	width, height int
}

func New() *Model {
	m := &Model{
		settings: defaultSettings(),
		styles:   ui.StylesFor(true),
	}
	m.initForm()
	return m
}

func (m *Model) Init() tea.Cmd { return m.homeUI.form.Init() }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.styles = ui.StylesFor(msg.IsDark())
		m.homeUI.form.WithTheme(formTheme(m.styles))
	case tea.KeyPressMsg:
		if msg.String() == "q" {
			return tea.Quit
		}
	}
	return m.updateForm(msg)
}

func (m *Model) View() string { return m.homeView() }
