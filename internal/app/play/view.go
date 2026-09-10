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

func (m *Model) playView() string {
	width, height := m.terminalSize()
	if width < ui.MinimumWidth || height < ui.MinimumHeight {
		return ui.ResizeView(width, height, "Esc / Ctrl+C quit")
	}
	inner := m.playWidth()
	header := ansi.Wrap(renderPlayStats(m.playUI.snapshot.Metrics, m.roundConfig), inner, "")
	header = m.styles.Text.Width(inner).Align(lipgloss.Center).Render(header)
	body := m.playUI.layout.visibleLines(playVisibleLines)
	body = lipgloss.NewStyle().Width(inner).Align(lipgloss.Left).Render(body)
	return ui.FitView(header+"\n\n"+body, width, height)
}

func (m *Model) resultsView() string {
	width, height := m.terminalSize()
	if width < ui.MinimumWidth || height < ui.MinimumHeight {
		return ui.ResizeView(width, height, "q / Esc quit")
	}
	inner := m.playWidth()
	stats := m.styles.Text.Render(ansi.Wrap(renderStats(m.game.FinalMetrics()), inner, ""))
	help := m.styles.Muted.Render(ansi.Wrap("Enter retry · Tab home · q / Esc quit", inner, ""))
	content := m.styles.Accent.Render("results") + "\n\n" + stats + "\n\n" + help
	return ui.FitView(content, width, height)
}

func (m *Model) errorView() string {
	width, height := m.terminalSize()
	if width < ui.MinimumWidth || height < ui.MinimumHeight {
		return ui.ResizeView(width, height, "q / Esc quit")
	}
	inner := m.playWidth()
	message := m.styles.Warning.Render(ansi.Wrap(m.err.Error(), inner, ""))
	help := m.styles.Muted.Render(ansi.Wrap("Enter retry · Tab home · q / Esc quit", inner, ""))
	content := m.styles.Accent.Render("round unavailable") + "\n\n" + message + "\n\n" + help
	return ui.FitView(content, width, height)
}

func renderStats(metrics engine.Metrics) string {
	return renderMetrics(metrics, "Elapsed", metrics.Duration.Seconds())
}

func renderPlayStats(metrics engine.Metrics, config engine.Config) string {
	if config.Mode == engine.ModeTime {
		remaining := max(0, config.Duration-metrics.Duration)
		return renderMetrics(metrics, "Remaining", math.Ceil(remaining.Seconds()))
	}
	return renderStats(metrics)
}

func renderMetrics(metrics engine.Metrics, clock string, seconds float64) string {
	return fmt.Sprintf("WPM: %.0f  Accuracy: %.2f%%  %s: %.0fs", metrics.WPM, metrics.Accuracy, clock, seconds)
}
