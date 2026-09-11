package home

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func (m *Model) homeStatus(layout homeLayout) string {
	config := m.settings.config()
	status := fmt.Sprintf("%d words · English", config.WordCount)
	if config.Mode == engine.ModeTime {
		status = fmt.Sprintf("%.0f seconds · English", config.Duration.Seconds())
	}
	if !layout.compact {
		if config.Mode == engine.ModeTime {
			status += " · type to start the clock"
		} else {
			status += " · no time limit"
		}
	}
	return m.styles.Muted.Render(status)
}

func (m *Model) homeHints(width int) string {
	var hints []string
	for _, binding := range m.homeUI.form.KeyBinds() {
		help := binding.Help()
		if binding.Enabled() && help.Key != "" {
			hints = append(hints, help.Key+" "+help.Desc)
		}
	}
	return m.styles.Muted.Render(ansi.Wrap(strings.Join(hints, " · "), width, ""))
}

func (m *Model) render() string {
	layout := m.homeLayout()
	if layout.width < ui.MinimumWidth || layout.height < ui.MinimumHeight {
		return ui.ResizeView(layout.width, layout.height, "q / Esc quit")
	}
	focused := m.homeUI.form.GetFocusedField()
	title := m.styles.Accent.Render("gopherttype")
	status, hints := m.homeStatus(layout), m.homeHints(layout.inner)
	quit := m.styles.Muted.Render("q / Esc quit")
	var content string
	if layout.compact {
		step := 1
		switch focused.GetKey() {
		case lengthField:
			step = 2
		case startField:
			step = 3
		}
		title = m.styles.Accent.Render(fmt.Sprintf("gopherttype · %d/3", step))
		content = strings.Join([]string{title, focused.View(), status, hints, quit}, "\n")
	} else {
		fields := lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderCard(m.homeUI.mode.View(), layout.cardWidth, focused.GetKey() == modeField),
			strings.Repeat(" ", homeCardGap),
			m.renderCard(m.homeUI.length.View(), layout.cardWidth, focused.GetKey() == lengthField))
		content = title + "\n" + m.styles.Muted.Render("A little focus. A better rhythm.") +
			"\n\n" + fields + "\n\n" + m.homeUI.start.View() + "\n\n" + status + "\n\n" + hints + "\n" + quit
	}
	return ui.FitView(lipgloss.NewStyle().Width(layout.inner).Render(content), layout.width, layout.height)
}
