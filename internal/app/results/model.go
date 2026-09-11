package results

import (
	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

type RetryMsg struct{}
type HomeMsg struct{}

type Model struct {
	metrics       engine.Metrics
	styles        *ui.Styles
	width, height int
}

func New(metrics engine.Metrics) *Model {
	return &Model{metrics: metrics, styles: ui.StylesFor(true)}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) View() string { return m.render() }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
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
	}
	return nil
}
