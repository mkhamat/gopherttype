package play

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app/mascot"
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

	mascot       *mascot.Model
	background   color.Color
	content      string
	headerHeight int
	layout       ui.MascotLayout
	view         string
}

type playState struct {
	updatedAt time.Time
	snapshot  engine.Snapshot
	layout    wordLayout
}

func New(config engine.Config, generate func(int) []string) *Model {
	m := &Model{roundConfig: config, generate: generate, styles: ui.StylesFor(true), mascot: mascot.New()}
	m.startGame()
	m.refreshContent(true)
	m.rebuild()
	return m
}

func (m *Model) Init() tea.Cmd { return m.configure(time.Now(), true, nil) }

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
		return m.configure(time.Now(), true, nil)
	case tea.BackgroundColorMsg:
		m.styles = ui.StylesFor(msg.IsDark())
		m.background = msg.Color
		return m.configure(time.Now(), true, nil)
	case tickMsg:
		return m.finishOrConfigure(m.handleTick(msg))
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return func() tea.Msg { return HomeMsg{} }
		}
		return m.finishOrConfigure(m.handlePlayKeyAt(msg, time.Now()))
	}
	return nil
}

func (m *Model) finishOrConfigure(cmd tea.Cmd) tea.Cmd {
	if m.game.Status() == engine.Finished {
		return cmd
	}
	return m.configure(time.Now(), false, cmd)
}

func (m *Model) configure(at time.Time, rebuildLayout bool, extra tea.Cmd) tea.Cmd {
	m.refreshContent(rebuildLayout)
	_, visual := m.mascot.Configure(m.scene(), at)
	m.rebuild()
	return tea.Batch(extra, visual)
}

func (m *Model) refreshContent(rebuildLayout bool) {
	width, height := ui.TerminalSize(m.width, m.height)
	if width < ui.MinimumWidth || height < ui.MinimumHeight {
		m.content, m.layout, m.headerHeight = "", ui.MascotLayout{}, 0
		return
	}
	if rebuildLayout {
		m.layoutPlay()
	}
	m.buildContent()
	m.layout = ui.LayoutWithMascot(width, height, m.content)
}

func (m *Model) rebuild() {
	width, height := ui.TerminalSize(m.width, m.height)
	if width < ui.MinimumWidth || height < ui.MinimumHeight {
		m.view = ui.ResizeView(width, height, "Esc / Ctrl+C quit")
		return
	}
	m.view = ui.ComposeWithMascot(m.content, m.mascot.View(), m.layout, width, height)
}

func (m *Model) scene() mascot.Scene {
	background := m.background
	if background == nil {
		background = m.styles.Background
	}
	point, track := m.cursorTarget()
	return mascot.Scene{
		Slot:       m.layout.Mascot,
		Content:    m.layout.Content,
		Target:     point,
		Track:      track,
		Background: background,
	}
}

func (m *Model) cursorTarget() (ui.Point, bool) {
	if m.layout.Mascot.Width <= 0 {
		return ui.Point{}, false
	}
	cursor := m.playUI.layout
	if cursor.cursorRow < 0 || cursor.cursorColumn < 0 || cursor.cursorWidth <= 0 {
		return ui.Point{}, false
	}
	first := cursor.firstVisibleRow(playVisibleLines)
	if cursor.cursorRow < first || cursor.cursorRow >= first+playVisibleLines {
		return ui.Point{}, false
	}
	x := m.layout.Content.X + cursor.cursorColumn
	y := m.layout.Content.Y + m.headerHeight + 1 + cursor.cursorRow - first
	if x < m.layout.Content.X || x+cursor.cursorWidth > m.layout.Content.X+m.layout.Content.Width {
		return ui.Point{}, false
	}
	if y < m.layout.Content.Y || y >= m.layout.Content.Y+m.layout.Content.Height {
		return ui.Point{}, false
	}
	width, height := ui.TerminalSize(m.width, m.height)
	if x < 0 || x+cursor.cursorWidth > width || y < 0 || y >= height {
		return ui.Point{}, false
	}
	return ui.Point{X: float64(x) + float64(cursor.cursorWidth)/2, Y: float64(y) + 0.5}, true
}
