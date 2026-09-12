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

func (m *Model) renderContent() string {
	title := m.styles.Accent.Render("gopherttype")
	mode := padRight(m.renderOptions([]string{"time", "words"}, m.modeIndex()))
	lengthOptions, lengthIndex := m.lengthRow()
	length := m.renderTrack(lengthOptions, lengthIndex)
	begin := m.styles.Accent.Render("enter") + m.styles.Muted.Render(" to begin")
	hints := m.styles.Accent.Render("↑↓") + m.styles.Muted.Render(" mode ") +
		m.styles.Accent.Render("←→") + m.styles.Muted.Render(" length ") +
		m.styles.Accent.Render("q") + m.styles.Muted.Render(" quit")
	lines := []string{title, ""}
	lines = append(lines, mode...)
	lines = append(lines, "", length, "", begin, "", hints)
	return strings.Join(centerLines(lines), "\n")
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

func (m *Model) renderTrack(options []string, selected int) string {
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
	return b.String()
}

func (m *Model) highlight() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(m.styles.Background).
		Background(m.styles.Accent.GetForeground()).
		Bold(true)
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
	modeToken := []string{"time", "words"}[m.modeIndex()]
	lengthOptions, lengthIndex := m.lengthRow()
	lengthToken := lengthOptions[lengthIndex]

	lines := strings.Split(ansi.Strip(m.content), "\n")
	modeLine, _, okMode := findToken(lines, modeToken)
	_, lengthCol, okLength := findToken(lines, lengthToken)
	if !okMode || !okLength {
		return ui.Point{}, false
	}
	return ui.Point{
		X: float64(m.layout.Content.X+lengthCol) + float64(ansi.StringWidth(lengthToken))/2,
		Y: float64(m.layout.Content.Y+modeLine) + 0.5,
	}, true
}

func findToken(lines []string, token string) (int, int, bool) {
	for y, line := range lines {
		for from := 0; ; {
			i := strings.Index(line[from:], token)
			if i < 0 {
				break
			}
			i += from
			end := i + len(token)
			if (i == 0 || !tokenByte(line[i-1])) && (end == len(line) || !tokenByte(line[end])) {
				return y, ansi.StringWidth(line[:i]), true
			}
			from = i + 1
		}
	}
	return 0, 0, false
}

func tokenByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
