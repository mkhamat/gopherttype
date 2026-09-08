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
