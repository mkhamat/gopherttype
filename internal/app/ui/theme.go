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
	Track      lipgloss.Style
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

func (s *Styles) Highlight() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(s.Background).
		Background(s.Accent.GetForeground()).
		Bold(true)
}

func newStyles(dark bool) *Styles {
	text, muted, accent := "#24292f", "#57606a", "#006d77"
	warning, incorrect, background := "#805500", "#b42318", "#ffffff"
	track := "#eef1f4"
	if dark {
		text, muted, accent = "#e6edf3", "#9da7b3", "#67d9e5"
		warning, incorrect, background = "#eac45c", "#ff8080", "#161b22"
		track = "#202830"
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
		Extra:      fg(incorrect).Faint(true),
		Pending:    fg(muted),
		Cursor:     fg(background).Background(lipgloss.Color(accent)),
		Track:      lipgloss.NewStyle().Background(lipgloss.Color(track)),
	}
}
