package home

import (
	"gopherttype/internal/app/ui"
)

const homeContentWidth = 36

type homeLayout struct {
	width  int
	height int
	inner  int
}

func (m *Model) homeLayout() homeLayout {
	width, height := m.terminalSize()
	return homeLayout{
		width:  width,
		height: height,
		inner:  min(homeContentWidth, max(1, width-4)),
	}
}

func (m *Model) terminalSize() (int, int) { return ui.TerminalSize(m.width, m.height) }

func (m *Model) resizeForm() {
	layout := m.homeLayout()
	m.homeUI.form.WithWidth(layout.inner).WithHeight(layout.height)
	m.homeUI.mode.WithWidth(layout.inner)
	m.homeUI.length.WithWidth(layout.inner)
}