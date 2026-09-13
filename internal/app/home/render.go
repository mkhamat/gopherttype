package home

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func (m *Model) modeIndex() int {
	if m.settings.mode == engine.ModeWords {
		return 1
	}
	return 0
}

func (m *Model) lengthRow() ([]string, int) {
	if m.settings.mode == engine.ModeTime {
		options := make([]string, len(durationPresets))
		for i, duration := range durationPresets {
			options[i] = fmt.Sprintf("%.0fs", duration.Seconds())
		}
		return options, m.settings.durationIndex
	}
	options := make([]string, len(wordPresets))
	for i, count := range wordPresets {
		options[i] = fmt.Sprint(count)
	}
	return options, m.settings.wordIndex
}

// The mascot follows length horizontally and mode vertically.
func (m *Model) renderContent() (string, ui.Point) {
	title := m.styles.Accent.Render("gopherttype")
	mode := padRight(m.renderOptions([]string{"time", "words"}, m.modeIndex()))
	lengthOptions, lengthIndex := m.lengthRow()
	length, selectedX := m.renderTrack(lengthOptions, lengthIndex)
	begin := m.styles.Accent.Render("enter") + m.styles.Muted.Render(" to begin")
	hints := m.styles.Accent.Render("↑↓") + m.styles.Muted.Render(" mode ") +
		m.styles.Accent.Render("←→") + m.styles.Muted.Render(" length ") +
		m.styles.Accent.Render("q") + m.styles.Muted.Render(" quit")
	lines := []string{title, ""}
	selectedY := float64(len(lines)+m.modeIndex()) + 0.5
	lines = append(lines, mode...)
	lines = append(lines, "", length, "", begin, "", hints)
	content := strings.Join(centerLines(lines), "\n")
	selectedX += float64((lipgloss.Width(content) - lipgloss.Width(length)) / 2)
	return content, ui.Point{X: selectedX, Y: selectedY}
}

func (m *Model) renderOptions(options []string, selected int) []string {
	cells := make([]string, len(options))
	for i, option := range options {
		if i == selected {
			cells[i] = m.highlight().Render(option)
		} else {
			cells[i] = m.styles.Muted.Render(option)
		}
	}
	return cells
}

func (m *Model) renderTrack(options []string, selected int) (string, float64) {
	selectedX := 1 + float64(ansi.StringWidth(options[selected]))/2
	for _, option := range options[:selected] {
		selectedX += float64(ansi.StringWidth(option) + 2)
	}
	unselected := m.styles.Track.Foreground(m.styles.Muted.GetForeground())
	var b strings.Builder
	b.WriteString(m.styles.Track.Render(" "))
	for i, option := range options {
		if i > 0 {
			b.WriteString(m.styles.Track.Render("  "))
		}
		if i == selected {
			b.WriteString(m.highlight().Render(option))
			continue
		}
		b.WriteString(unselected.Render(option))
	}
	b.WriteString(m.styles.Track.Render(" "))
	return b.String(), selectedX
}

func (m *Model) highlight() lipgloss.Style {
	return m.styles.Highlight()
}

func padRight(lines []string) []string {
	width := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > width {
			width = w
		}
	}
	for i, line := range lines {
		lines[i] = line + strings.Repeat(" ", width-lipgloss.Width(line))
	}
	return lines
}

func centerLines(lines []string) []string {
	width := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > width {
			width = w
		}
	}
	for i, line := range lines {
		if lipgloss.Width(line) > 0 {
			lines[i] = lipgloss.PlaceHorizontal(width, lipgloss.Center, line)
		}
	}
	return lines
}

func (m *Model) selectedPoint() (ui.Point, bool) {
	if m.layout.Mascot.Width <= 0 {
		return ui.Point{}, false
	}
	return ui.Point{
		X: float64(m.layout.Content.X) + m.selection.X,
		Y: float64(m.layout.Content.Y) + m.selection.Y,
	}, true
}
