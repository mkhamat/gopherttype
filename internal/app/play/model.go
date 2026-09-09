package play

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

type screen int

const (
	play screen = iota
	results
	failed
	leaving
)

type HomeMsg struct{}

type Model struct {
	screen        screen
	game          *engine.Game
	err           error
	roundConfig   engine.Config
	generate      func(int) []string
	playUI        playState
	styles        *ui.Styles
	width, height int
}

type playState struct {
	at       time.Time
	snapshot engine.Snapshot
	layout   wordLayout
}

func New(config engine.Config, generate func(int) []string) *Model {
	m := &Model{roundConfig: config, generate: generate, styles: ui.StylesFor(true)}
	m.startGame()
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) fail(err error) {
	m.err = err
	m.screen = failed
}

func (m *Model) leave() tea.Cmd {
	m.screen = leaving
	return func() tea.Msg { return HomeMsg{} }
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.screen == play {
			m.layoutPlay()
		}
	case tea.BackgroundColorMsg:
		m.styles = ui.StylesFor(msg.IsDark())
		if m.screen == play {
			m.layoutPlay()
		}
	case tickMsg:
		return m.handleTick(msg)
	case tea.KeyPressMsg:
		switch m.screen {
		case play:
			return m.handlePlayKeyAt(msg, time.Now())
		case results, failed:
			switch msg.String() {
			case "enter":
				m.startGame()
			case "tab":
				return m.leave()
			case "q":
				return tea.Quit
			}
		}
	}
	return nil
}

func (m *Model) View() string {
	switch m.screen {
	case leaving:
		return ""
	case results:
		return m.resultsView()
	case failed:
		return m.errorView()
	default:
		return m.playView()
	}
}
