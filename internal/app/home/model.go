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

func (m *Model) toggleMode() {
	if m.settings.mode == engine.ModeTime {
		m.settings.mode = engine.ModeWords
		return
	}
	m.settings.mode = engine.ModeTime
}

func (m *Model) stepLength(delta int) {
	if m.settings.mode == engine.ModeTime {
		m.settings.durationIndex = wrapIndex(m.settings.durationIndex+delta, len(durationPresets))
		return
	}
	m.settings.wordIndex = wrapIndex(m.settings.wordIndex+delta, len(wordPresets))
}

func wrapIndex(index, length int) int {
	return ((index % length) + length) % length
}

type StartMsg struct{ Config engine.Config }

type Model struct {
	settings      settings
	styles        *ui.Styles
	width, height int

	mascot     *mascot.Model
	background color.Color
	content    string
	selection  ui.Point
	layout     ui.MascotLayout
	view       string
}

func New() *Model {
	m := &Model{
		settings: settings{mode: engine.ModeTime},
		styles:   ui.StylesFor(true),
		mascot:   mascot.New(),
	}
	m.refreshContent()
	m.rebuild()
	return m
}

func (m *Model) Init() tea.Cmd {
	return m.configure(time.Now())
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
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q":
			return tea.Quit
		case "enter":
			config := m.settings.config()
			return func() tea.Msg { return StartMsg{Config: config} }
		case "up", "down", "k", "j":
			m.toggleMode()
		case "left", "h":
			m.stepLength(-1)
		case "right", "l":
			m.stepLength(1)
		}
	}
	return m.configure(time.Now())
}

func (m *Model) configure(at time.Time) tea.Cmd {
	m.refreshContent()
	_, cmd := m.mascot.Configure(m.scene(), at)
	m.rebuild()
	return cmd
}

func (m *Model) refreshContent() {
	width, height := ui.TerminalSize(m.width, m.height)
	if width < ui.MinimumWidth || height < ui.MinimumHeight {
		m.content, m.selection, m.layout = "", ui.Point{}, ui.MascotLayout{}
		return
	}
	m.content, m.selection = m.renderContent()
	m.layout = ui.LayoutWithMascot(width, height, m.content)
}

func (m *Model) rebuild() {
	width, height := ui.TerminalSize(m.width, m.height)
	if width < ui.MinimumWidth || height < ui.MinimumHeight {
		m.view = ui.ResizeView(width, height, "q / Ctrl+C quit")
		return
	}
	m.view = ui.ComposeWithMascot(m.content, m.mascot.View(), m.layout, width, height)
}

func (m *Model) scene() mascot.Scene {
	background := m.background
	if background == nil {
		background = m.styles.Background
	}
	point, track := m.selectedPoint()
	return mascot.Scene{
		Slot:       m.layout.Mascot,
		Content:    m.layout.Content,
		Target:     point,
		Track:      track,
		Background: background,
	}
}
