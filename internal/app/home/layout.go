package home

import (
	"charm.land/lipgloss/v2"

	"gopherttype/internal/app/ui"
)

const (
	homeContentWidth  = 68
	homeCompactWidth  = 58
	homeCompactHeight = 23
	homeCardGap       = 2
	homeCardFrame     = 4
)

type homeLayout struct {
	width      int
	height     int
	inner      int
	cardWidth  int
	fieldWidth int
	compact    bool
}

func (m *Model) homeLayout() homeLayout {
	width, height := m.terminalSize()
	inner := min(homeContentWidth, max(1, width-4))
	compact := width < homeCompactWidth || height < homeCompactHeight
	cardWidth := (inner - homeCardGap) / 2
	fieldWidth := inner
	if !compact {
		fieldWidth = cardWidth - homeCardFrame
	}
	return homeLayout{
		width:      width,
		height:     height,
		inner:      inner,
		cardWidth:  cardWidth,
		fieldWidth: fieldWidth,
		compact:    compact,
	}
}

func (m *Model) terminalSize() (int, int) { return ui.TerminalSize(m.width, m.height) }

func (m *Model) resizeForm() {
	layout := m.homeLayout()
	m.homeUI.form.WithWidth(layout.inner).WithHeight(layout.height)
	m.homeUI.mode.WithWidth(layout.fieldWidth)
	m.homeUI.length.WithWidth(layout.fieldWidth)
	m.homeUI.start.WithWidth(layout.inner)
	fieldHeight := 0
	if layout.compact {
		reserved := 2 + lipgloss.Height(m.homeStatus(layout)) + lipgloss.Height(m.homeHints(layout.inner))
		fieldHeight = max(3, layout.height-reserved)
	}
	m.homeUI.mode.WithHeight(min(4, fieldHeight))
	m.homeUI.length.WithHeight(min(2+max(len(wordPresets), len(durationPresets)), fieldHeight))
	m.homeUI.start.WithHeight(0)
}
