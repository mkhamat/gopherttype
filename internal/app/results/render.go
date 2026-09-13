package results

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func (m *Model) renderContent() string {
	width, _ := ui.TerminalSize(m.width, m.height)
	padding := min(4, (width-24)/2)
	inner := min(80, width-2*padding)
	message := ansi.Wrap(m.styles.Text.Render(renderStats(m.metrics)), inner, "")
	help := m.styles.Muted.Render(ansi.Wrap("Enter retry · Tab home · q quit", inner, ""))
	body := message + "\n\n" + help
	header := lipgloss.PlaceHorizontal(lipgloss.Width(body), lipgloss.Center, m.styles.Accent.Render("results"))
	return header + "\n\n" + body
}

func renderStats(metrics engine.Metrics) string {
	return fmt.Sprintf("WPM: %.0f  Accuracy: %.2f%%  Elapsed: %.0fs", metrics.WPM, metrics.Accuracy, metrics.Duration.Seconds())
}
