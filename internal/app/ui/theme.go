package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Styles struct {
	Background color.Color
	Text       lipgloss.Style
	Accent     lipgloss.Style
	Muted      lipgloss.Style
	Warning    lipgloss.Style
	Correct    lipgloss.Style
	Incorrect  lipgloss.Style
	Extra      lipgloss.Style
	Pending    lipgloss.Style
	Cursor     lipgloss.Style
}

var (
	lightStyles = newStyles(false)
	darkStyles  = newStyles(true)
)

func StylesFor(dark bool) *Styles {
	if dark {
		return darkStyles
	}
	return lightStyles
}

func newStyles(dark bool) *Styles {
	text, muted, accent := "#24292f", "#57606a", "#006d77"
	warning, incorrect, extra, background := "#805500", "#b42318", "#9c36b5", "#ffffff"
	if dark {
		text, muted, accent = "#e6edf3", "#9da7b3", "#67d9e5"
		warning, incorrect, extra, background = "#eac45c", "#ff8080", "#e6a0f0", "#161b22"
	}
	fg := func(color string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	}
	return &Styles{
		Background: lipgloss.Color(background),
		Text:       fg(text),
		Accent:     fg(accent).Bold(true),
		Muted:      fg(muted),
		Warning:    fg(warning),
		Correct:    fg(text),
		Incorrect:  fg(incorrect),
		Extra:      fg(extra),
		Pending:    fg(muted),
		Cursor:     fg(background).Background(lipgloss.Color(text)),
	}
}
