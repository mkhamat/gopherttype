package results

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/mascot"
	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

type RetryMsg struct{}
type HomeMsg struct{}

type Model struct {
	metrics       engine.Metrics
	styles        *ui.Styles
	width, height int

	mascot     *mascot.Model
	background color.Color
	content    string
	layout     ui.MascotLayout
	view       string
}

func New(metrics engine.Metrics) *Model {
	m := &Model{metrics: metrics, styles: ui.StylesFor(true), mascot: mascot.New()}
	m.SetResult(metrics, time.Now())
	m.refreshContent()
	m.rebuild()
	return m
}

// SetResult classifies the finished round once and locks the mascot into the
// matching result expression. A fresh results screen calls it exactly once;
// Init, resize and background updates must not reclassify or restart it.
func (m *Model) SetResult(metrics engine.Metrics, at time.Time) {
	m.mascot.SetResult(mascot.ClassifyResult(metrics.WPM, metrics.Accuracy, metrics.Duration), at)
}

func (m *Model) Init() tea.Cmd { return m.configure(time.Now()) }

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
		case "enter":
			return func() tea.Msg { return RetryMsg{} }
		case "tab":
			return func() tea.Msg { return HomeMsg{} }
		case "q":
			return tea.Quit
		}
		return nil
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
		m.content, m.layout = "", ui.MascotLayout{}
		return
	}
	m.content = m.renderContent()
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
	return mascot.Scene{
		Slot:       m.layout.Mascot,
		Content:    m.layout.Content,
		Track:      false,
		Background: background,
	}
}
