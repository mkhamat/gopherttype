package home

import (
	"gopherttype/internal/app/ui"
)

type homeLayout struct {
	width  int
	height int
}

func (m *Model) homeLayout() homeLayout {
	width, height := ui.TerminalSize(m.width, m.height)
	return homeLayout{width: width, height: height}
}
