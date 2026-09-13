package play

import (
	"fmt"
	"math"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mkhamat/gopherttype/internal/app/ui"
	"github.com/mkhamat/gopherttype/internal/engine"
)

const (
	playContentWidth = 80
	playVisibleLines = 3
)

func (m *Model) playWidth(width int) int {
	padding := min(4, max(0, (width-24)/2))
	return min(playContentWidth, max(1, width-2*padding))
}

func (m *Model) layoutPlay() {
	snapshot := m.playUI.snapshot
	width, _ := ui.TerminalSize(m.width, m.height)
	m.playUI.layout = layoutWords(snapshot.Words, snapshot.CurrentWordIndex, m.playWidth(width), m.styles)
}

func (m *Model) buildContent() {
	width, _ := ui.TerminalSize(m.width, m.height)
	inner := m.playWidth(width)
	header := ansi.Wrap(renderRemaining(m.playUI.snapshot, m.roundConfig), inner, "")
	header = m.styles.Text.Width(inner).Align(lipgloss.Center).Render(header)
	m.headerHeight = lipgloss.Height(header)
	body := m.playUI.layout.visibleLines(playVisibleLines)
	body = lipgloss.NewStyle().Width(inner).Align(lipgloss.Left).Render(body)
	m.content = header + "\n\n" + body
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
