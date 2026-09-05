package app

import (
	"strings"

	"charm.land/lipgloss/v2"

	"gopherttype/internal/engine"
)

var (
	correctStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	incorrectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	extraStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#800000"))
	pendingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cursorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("7"))
)

func renderWords(words []engine.WordSnapshot, currentWordIndex int) string {
	var output strings.Builder
	for i, word := range words {
		active := i == currentWordIndex
		output.WriteString(renderWord(word, active, i < currentWordIndex))
		if active && len(word.Typed) >= len([]rune(word.Target)) {
			output.WriteString(cursorStyle.Render(" "))
		} else if i < len(words)-1 {
			output.WriteByte(' ')
		}
	}
	return output.String()
}

func renderWord(word engine.WordSnapshot, active, submitted bool) string {
	target := []rune(word.Target)
	typed := word.Typed
	submittedIncorrect := submitted && string(typed) != word.Target
	var output strings.Builder

	for i := 0; i < max(len(target), len(typed)); i++ {
		style := pendingStyle
		var r rune

		switch {
		case i >= len(target):
			style = extraStyle
			r = typed[i]
		case i >= len(typed):
			r = target[i]
		case typed[i] == target[i]:
			style = correctStyle
			r = target[i]
		default:
			style = incorrectStyle
			r = target[i]
		}

		if submittedIncorrect {
			style = style.Underline(true).UnderlineColor(lipgloss.Color("1"))
		}
		if active && i == len(typed) {
			style = cursorStyle
		}
		output.WriteString(style.Render(string(r)))
	}

	return output.String()
}
