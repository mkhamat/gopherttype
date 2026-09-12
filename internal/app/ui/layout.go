package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	DefaultWidth  = 80
	DefaultHeight = 24
	MinimumWidth  = 28
	MinimumHeight = 12
)

type Point struct {
	X, Y float64
}

type Rect struct {
	X, Y          int
	Width, Height int
}

const (
	mascotWidth      = 32
	mascotHeight     = 12
	mascotGap        = 1
	mascotContentTop = 1 + mascotHeight + mascotGap
)

type MascotLayout struct {
	Mascot  Rect
	Content Rect
}

func TerminalSize(width, height int) (int, int) {
	if width <= 0 {
		width = DefaultWidth
	}
	if height <= 0 {
		height = DefaultHeight
	}
	return width, height
}

func FitView(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "")
	}
	content = strings.Join(lines[:min(height, len(lines))], "\n")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func ResizeView(width, height int, quitting string) string {
	content := fmt.Sprintf("gopherttype\nResize to %d x %d\n%s", MinimumWidth, MinimumHeight, quitting)
	return FitView(content, width, height)
}

func LayoutWithMascot(width, height int, content string) MascotLayout {
	contentWidth, contentHeight := measure(content)

	if width >= mascotWidth && contentWidth <= width && mascotContentTop+contentHeight <= height {
		contentY := mascotContentTop + (height-mascotContentTop-contentHeight)/2
		return MascotLayout{
			Mascot: Rect{
				X:      (width - mascotWidth) / 2,
				Y:      contentY - mascotHeight - mascotGap,
				Width:  mascotWidth,
				Height: mascotHeight,
			},
			Content: Rect{
				X:      (width - contentWidth) / 2,
				Y:      contentY,
				Width:  contentWidth,
				Height: contentHeight,
			},
		}
	}

	if contentWidth <= width && contentHeight <= height {
		return MascotLayout{Content: Rect{
			X:      (width - contentWidth) / 2,
			Y:      (height - contentHeight) / 2,
			Width:  contentWidth,
			Height: contentHeight,
		}}
	}
	return MascotLayout{}
}

func ComposeWithMascot(content, portrait string, layout MascotLayout, width, height int) string {
	if layout.Mascot.Width <= 0 || layout.Mascot.Height <= 0 {
		return FitView(content, width, height)
	}

	rows := make([]string, height)
	for i, line := range strings.Split(portrait, "\n") {
		if i >= layout.Mascot.Height {
			break
		}
		if y := layout.Mascot.Y + i; y >= 0 && y < height {
			rows[y] = strings.Repeat(" ", layout.Mascot.X) + line
		}
	}
	for i, line := range strings.Split(content, "\n") {
		if i >= layout.Content.Height {
			break
		}
		if y := layout.Content.Y + i; y >= 0 && y < height {
			rows[y] = strings.Repeat(" ", layout.Content.X) + line
		}
	}
	return strings.Join(rows, "\n")
}

func measure(content string) (int, int) {
	lines := strings.Split(content, "\n")
	width := 0
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			width = w
		}
	}
	return width, len(lines)
}
