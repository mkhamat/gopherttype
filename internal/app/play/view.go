package play

import (
	"fmt"
	"math"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

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
