package home

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

var (
	wordPresets     = [...]int{10, 25, 50, 100}
	durationPresets = [...]time.Duration{15 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second}
)

type settings struct {
	mode          engine.Mode
	wordIndex     int
	durationIndex int
}

func defaultSettings() settings {
	return settings{mode: engine.ModeTime, durationIndex: 1}
}

func (s settings) config() engine.Config {
	if s.mode == engine.ModeTime {
		return engine.Config{Mode: s.mode, Duration: durationPresets[s.durationIndex]}
	}
	return engine.Config{Mode: s.mode, WordCount: wordPresets[s.wordIndex]}
}

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
