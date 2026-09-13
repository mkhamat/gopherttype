package results

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"gopherttype/internal/app/mascot"
	"gopherttype/internal/app/ui"
)

const fieldSep = "  ·  "

func (m *Model) renderContent() string {
	width, _ := ui.TerminalSize(m.width, m.height)
	padding := min(4, (width-24)/2)
	inner := min(80, width-2*padding)

	lines := []string{
		centerBlock(m.tierStyle().Render(m.tier.Label()), inner),
		"",
		m.renderHero(inner),
	}
	if detail := m.renderDetail(inner); detail != "" {
		lines = append(lines, detail)
	}
	lines = append(lines, "", m.renderHelp(inner))
	return strings.Join(lines, "\n")
}

func (m *Model) tierStyle() lipgloss.Style {
	switch m.tier {
	case mascot.TierCelebrate, mascot.TierProud:
		return m.styles.Accent
	case mascot.TierWorried:
		return m.styles.Warning
	default:
		return m.styles.Text
	}
}

func (m *Model) renderHero(inner int) string {
	chip := m.styles.Highlight().Render(fmt.Sprintf(" %.0f WPM ", m.metrics.WPM))
	accuracy := m.tierStyle().Render(fmt.Sprintf("%.2f%%", m.metrics.Accuracy)) +
		m.styles.Muted.Render(" accuracy")
	elapsed := m.styles.Muted.Render(fmt.Sprintf("%.0fs", m.metrics.Duration.Seconds()))
	return m.centerClauses([]string{chip, accuracy, elapsed}, inner)
}

func (m *Model) renderDetail(inner int) string {
	var clauses []string
	wpm := fmt.Sprintf("%.0f", m.metrics.WPM)
	if raw := fmt.Sprintf("%.0f", m.metrics.Raw); raw != wpm {
		clauses = append(clauses, m.styles.Muted.Render("raw "+raw+" wpm"))
	}
	if errors := m.renderErrors(); errors != "" {
		clauses = append(clauses, errors)
	}
	if len(clauses) == 0 {
		return ""
	}
	return m.centerClauses(clauses, inner)
}

func (m *Model) renderErrors() string {
	parts := make([]string, 0, 3)
	if m.metrics.Incorrect > 0 {
		parts = append(parts, m.styles.Incorrect.Render(fmt.Sprintf("%d wrong", m.metrics.Incorrect)))
	}
	if m.metrics.Extra > 0 {
		parts = append(parts, m.styles.Extra.Render(fmt.Sprintf("%d extra", m.metrics.Extra)))
	}
	if m.metrics.Missed > 0 {
		parts = append(parts, m.styles.Incorrect.Render(fmt.Sprintf("%d missed", m.metrics.Missed)))
	}
	return strings.Join(parts, m.styles.Muted.Render(fieldSep))
}

func (m *Model) renderHelp(inner int) string {
	action := func(k, label string) string {
		return m.styles.Accent.Render(k) + m.styles.Muted.Render(" "+label)
	}
	return m.centerClauses([]string{action("enter", "retry"), action("tab", "home"), action("q", "quit")}, inner)
}

func (m *Model) centerClauses(clauses []string, inner int) string {
	if len(clauses) == 0 {
		return ""
	}
	sep := m.styles.Muted.Render(fieldSep)
	var lines []string
	current := clauses[0]
	for _, clause := range clauses[1:] {
		if joined := current + sep + clause; lipgloss.Width(joined) <= inner {
			current = joined
		} else {
			lines = append(lines, current)
			current = clause
		}
	}
	return centerBlock(strings.Join(append(lines, current), "\n"), inner)
}

func centerBlock(block string, inner int) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		lines[i] = lipgloss.PlaceHorizontal(inner, lipgloss.Center, line)
	}
	return strings.Join(lines, "\n")
}
