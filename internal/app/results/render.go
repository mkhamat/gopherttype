package results

import (
	"fmt"

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
	return m.styles.Accent.Render("results") + "\n\n" + message + "\n\n" + help
}

func renderStats(metrics engine.Metrics) string {
	return fmt.Sprintf("WPM: %.0f  Accuracy: %.2f%%  Elapsed: %.0fs", metrics.WPM, metrics.Accuracy, metrics.Duration.Seconds())
}
