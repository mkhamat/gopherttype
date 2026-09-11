package home

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

const optionsColumn = 13

func (m *Model) modeIndex() int {
	if m.settings.mode == engine.ModeWords {
		return 1
	}
	return 0
}

func (m *Model) lengthRow() (string, []string, int) {
	if m.settings.mode == engine.ModeTime {
		var options []string
		for _, duration := range durationPresets {
			options = append(options, fmt.Sprintf("%.0fs", duration.Seconds()))
		}
		return "duration", options, m.settings.durationIndex
	}
	var options []string
	for _, count := range wordPresets {
		options = append(options, fmt.Sprint(count))
	}
	return "words", options, m.settings.wordIndex
}

func (m *Model) renderRow(label string, options []string, selected int, focused bool) string {
	cursor, labelStyle := "  ", m.styles.Muted
	if focused {
		cursor, labelStyle = "› ", m.styles.Accent
	}
	var cells []string
	for i, option := range options {
		if i == selected {
			cells = append(cells, m.styles.Accent.Render(option))
		} else {
			cells = append(cells, m.styles.Muted.Render(option))
		}
	}
	padding := strings.Repeat(" ", max(0, optionsColumn-lipgloss.Width(cursor+label)))
	return cursor + labelStyle.Render(label) + padding + strings.Join(cells, "  ")
}

func (m *Model) render() string {
	layout := m.homeLayout()
	if layout.width < ui.MinimumWidth || layout.height < ui.MinimumHeight {
		return ui.ResizeView(layout.width, layout.height, "q / Esc quit")
	}
	focused := m.homeUI.form.GetFocusedField().GetKey()
	title := m.styles.Accent.Render("gopherttype")
	modeRow := m.renderRow("mode", []string{"time", "words"}, m.modeIndex(), focused == modeField)
	lengthLabel, lengthOptions, lengthIndex := m.lengthRow()
	lengthRow := m.renderRow(lengthLabel, lengthOptions, lengthIndex, focused == lengthField)
	begin := m.styles.Muted.Render("enter to begin")
	hints := m.styles.Muted.Render("↑↓ move   ←→ change   q quit")
	content := strings.Join([]string{title, "", "", modeRow, lengthRow, "", "", begin, "", hints}, "\n")
	return ui.FitView(lipgloss.NewStyle().Width(layout.inner).Render(content), layout.width, layout.height)
}
