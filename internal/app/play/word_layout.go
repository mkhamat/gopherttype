package play

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

type wordLayout struct {
	lines          []string
	cursorRow      int
	activeColumn   int
	wordWidthLimit int
}

type wordCell struct {
	text   string
	width  int
	style  lipgloss.Style
	cursor bool
}

func layoutWords(words []engine.WordSnapshot, current, width int, styles *ui.Styles) wordLayout {
	layout := wordLayout{cursorRow: -1, wordWidthLimit: max(1, width-1)}
	var line strings.Builder
	column := 0
	flush := func() {
		layout.lines = append(layout.lines, line.String())
		line.Reset()
		column = 0
	}
	previousCursorSpace := false
	for i, word := range words {
		cells, wordWidth := wordCells(word, i == current, i < current, styles)
		separator := 0
		if i > 0 && !previousCursorSpace && column > 0 {
			separator = 1
		}
		if column > 0 && column+separator+wordWidth > layout.wordWidthLimit {
			flush()
			separator = 0
		}
		if separator > 0 {
			line.WriteByte(' ')
			column++
		}
		if i == current {
			layout.activeColumn = column
		}
		trailingCursor := i == current && len(word.Typed) >= utf8.RuneCountInString(word.Target)
		for cellIndex, cell := range cells {
			if cell.width > width {
				cell.text, cell.width = "�", 1
			}
			limit := layout.wordWidthLimit
			if trailingCursor && cellIndex == len(cells)-1 {
				limit = width
			}
			if column+cell.width > limit {
				flush()
			}
			if cell.cursor {
				layout.cursorRow = len(layout.lines)
				cell.style = styles.Cursor
			}
			line.WriteString(cell.style.Render(cell.text))
			column += cell.width
		}
		previousCursorSpace = i == current && len(word.Typed) >= utf8.RuneCountInString(word.Target)
	}
	flush()
	return layout
}

func (layout wordLayout) acceptsExtra(word engine.WordSnapshot, r rune, styles *ui.Styles) bool {
	word.Typed = append(word.Typed[:len(word.Typed):len(word.Typed)], r)
	_, width := wordCells(word, false, false, styles)
	return layout.activeColumn+width <= layout.wordWidthLimit
}

func wordCells(word engine.WordSnapshot, active, submitted bool, styles *ui.Styles) ([]wordCell, int) {
	target := []rune(word.Target)
	displayed := append([]rune(nil), target...)
	if len(word.Typed) > len(target) {
		displayed = append(displayed, word.Typed[len(target):]...)
	}
	for i, r := range displayed {
		if unicode.IsControl(r) {
			displayed[i] = '�'
		}
	}
	cells := make([]wordCell, 0, len(displayed)+1)
	submittedIncorrect := submitted && string(word.Typed) != word.Target
	graphemes := uniseg.NewGraphemes(string(displayed))
	position, width := 0, 0
	for graphemes.Next() {
		text := graphemes.Str()
		end := position + utf8.RuneCountInString(text)
		style := styles.Correct
		switch {
		case end > len(target):
			style = styles.Extra
		case end > len(word.Typed):
			style = styles.Pending
		}
		for i := position; i < min(end, len(word.Typed), len(target)); i++ {
			if word.Typed[i] != target[i] {
				style = styles.Incorrect
				break
			}
		}
		if submittedIncorrect {
			style = style.Underline(true).UnderlineColor(styles.Incorrect.GetForeground())
		}
		cellWidth := ansi.StringWidth(text)
		if cellWidth == 0 {
			text = "◌" + text
			cellWidth = ansi.StringWidth(text)
		}
		cells = append(cells, wordCell{
			text: text, width: cellWidth, style: style,
			cursor: active && len(word.Typed) >= position && len(word.Typed) < end,
		})
		width += cellWidth
		position = end
	}
	if active && len(word.Typed) >= len(target) {
		cells = append(cells, wordCell{text: " ", width: 1, cursor: true})
	}
	return cells, width
}

func (layout wordLayout) visibleLines(height int) string {
	first := max(0, layout.cursorRow-height/2)
	last := min(len(layout.lines), first+height)
	lines := make([]string, height)
	copy(lines, layout.lines[first:last])
	return strings.Join(lines, "\n")
}
