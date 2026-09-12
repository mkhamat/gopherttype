package home

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/mascot"
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

	mascot     *mascot.Model
	background color.Color
	content    string
	layout     ui.MascotLayout
	view       string
}

func New() *Model {
	m := &Model{
		settings: settings{mode: engine.ModeTime, durationIndex: 1},
		styles:   ui.StylesFor(true),
		mascot:   mascot.New(),
	}
	m.initForm()
	m.refreshContent()
	m.rebuild()
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.configure(time.Now()), m.homeUI.form.Init())
}

func (m *Model) View() string { return m.view }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case mascot.FrameMsg:
		changed, cmd := m.mascot.Update(msg, time.Now())
		if changed {
			m.rebuild()
		}
		return cmd
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.background = msg.Color
		m.styles = ui.StylesFor(msg.IsDark())
		m.homeUI.form.WithTheme(formTheme(m.styles))
	case tea.KeyPressMsg:
		if msg.String() == "q" {
			m.mascot.Hide()
			return tea.Quit
		}
		if msg.String() == "enter" {
			m.mascot.Hide()
			config := m.settings.config()
			return func() tea.Msg { return StartMsg{Config: config} }
		}
	}
	return tea.Batch(m.updateForm(msg), m.configure(time.Now()))
}

func (m *Model) configure(at time.Time) tea.Cmd {
	m.refreshContent()
	_, cmd := m.mascot.Configure(m.scene(), at)
	m.rebuild()
	return cmd
}

func (m *Model) refreshContent() {
	layout := m.homeLayout()
	if layout.width < ui.MinimumWidth || layout.height < ui.MinimumHeight {
		m.content, m.layout = "", ui.MascotLayout{}
		return
	}
	m.content = m.renderContent(layout.inner)
	m.layout = ui.LayoutWithMascot(layout.width, layout.height, m.content)
}

func (m *Model) rebuild() {
	layout := m.homeLayout()
	if layout.width < ui.MinimumWidth || layout.height < ui.MinimumHeight {
		m.view = ui.ResizeView(layout.width, layout.height, "q / Esc quit")
		return
	}
	m.view = ui.ComposeWithMascot(m.content, m.mascot.View(), m.layout, layout.width, layout.height)
}

func (m *Model) scene() mascot.Scene {
	background := m.background
	if background == nil {
		background = m.styles.Background
	}
	return mascot.Scene{Slot: m.layout.Mascot, Content: m.layout.Content, Background: background}
}
