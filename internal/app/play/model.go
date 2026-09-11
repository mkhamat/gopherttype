package play

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

type FinishedMsg struct{ Metrics engine.Metrics }
type HomeMsg struct{}

type Model struct {
	game          *engine.Game
	roundConfig   engine.Config
	generate      func(int) []string
	playUI        playState
	styles        *ui.Styles
	width, height int
}

type playState struct {
	updatedAt time.Time
	snapshot  engine.Snapshot
	layout    wordLayout
}

func New(config engine.Config, generate func(int) []string) *Model {
	m := &Model{roundConfig: config, generate: generate, styles: ui.StylesFor(true)}
	m.startGame()
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) View() string { return m.renderPlay() }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if m.game.Status() == engine.Finished {
		return nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layoutPlay()
	case tea.BackgroundColorMsg:
		m.styles = ui.StylesFor(msg.IsDark())
		m.layoutPlay()
	case tickMsg:
		return m.handleTick(msg)
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return func() tea.Msg { return HomeMsg{} }
		}
		return m.handlePlayKeyAt(msg, time.Now())
	}
	return nil
}
