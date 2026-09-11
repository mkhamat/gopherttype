package home

import (
	"charm.land/bubbles/v2/help"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"gopherttype/internal/app/ui"
)

func formTheme(styles *ui.Styles) huh.Theme {
	return huh.ThemeFunc(func(dark bool) *huh.Styles {
		form := huh.ThemeBase(dark)
		for _, field := range []*huh.FieldStyles{&form.Focused, &form.Blurred} {
			field.Base = lipgloss.NewStyle()
			field.Title = styles.Text.Bold(true).MarginBottom(1)
			field.SelectSelector = styles.Accent.SetString("● ")
			field.SelectedOption = styles.Accent
			field.UnselectedOption = styles.Muted
			field.ErrorIndicator = styles.Warning.SetString(" !")
			field.ErrorMessage = styles.Warning
			field.FocusedButton = lipgloss.NewStyle().Foreground(styles.Background).
				Background(styles.Accent.GetForeground()).Bold(true).Padding(0, 3)
			field.BlurredButton = styles.Accent.Padding(0, 3)
		}
		form.Focused.Title = styles.Accent.MarginBottom(1)
		form.Blurred.SelectSelector = styles.Muted.SetString("● ")
		form.Blurred.SelectedOption = styles.Text.Bold(true)
		form.Blurred.FocusedButton = form.Blurred.BlurredButton
		form.Help = help.DefaultStyles(dark)
		return form
	})
}

