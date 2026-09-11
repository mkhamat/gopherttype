package play

import (
	"fmt"
	"math"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

const (
	playContentWidth = 80
	playVisibleLines = 3
)

func (m *Model) playWidth() int {
	width, _ := m.terminalSize()
	padding := min(4, max(0, (width-24)/2))
	return min(playContentWidth, max(1, width-2*padding))
}

func (m *Model) terminalSize() (int, int) { return ui.TerminalSize(m.width, m.height) }

func (m *Model) layoutPlay() {
	snapshot := m.playUI.snapshot
	m.playUI.layout = layoutWords(snapshot.Words, snapshot.CurrentWordIndex, m.playWidth(), m.styles)
}

func (m *Model) renderPlay() string {
	width, height := m.terminalSize()
	if width < ui.MinimumWidth || height < ui.MinimumHeight {
		return ui.ResizeView(width, height, "Esc / Ctrl+C quit")
	}
	inner := m.playWidth()
	header := ansi.Wrap(renderRemaining(m.playUI.snapshot, m.roundConfig), inner, "")
	header = m.styles.Text.Width(inner).Align(lipgloss.Center).Render(header)
	body := m.playUI.layout.visibleLines(playVisibleLines)
	body = lipgloss.NewStyle().Width(inner).Align(lipgloss.Left).Render(body)
	return ui.FitView(header+"\n\n"+body, width, height)
}

func renderRemaining(snapshot engine.Snapshot, config engine.Config) string {
	if config.Mode == engine.ModeTime {
		remaining := config.Duration - snapshot.Elapsed
		return fmt.Sprintf("Remaining: %.0fs", math.Ceil(remaining.Seconds()))
	}
	remaining := config.WordCount - snapshot.CurrentWordIndex
	if remaining == 1 {
		return "Remaining: 1 word"
	}
	return fmt.Sprintf("Remaining: %d words", remaining)
}
