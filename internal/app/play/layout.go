package play

import "gopherttype/internal/app/ui"

const (
	playContentWidth = 80
	playVisibleLines = 3
)

func (m *Model) playWidth() int {
	width, _ := m.terminalSize()
	padding := min(4, max(0, (width-24)/2))
	return min(playContentWidth, max(1, width-2*padding))
}

func (m *Model) wordWidth() int { return max(1, m.playWidth()-1) }

func (m *Model) terminalSize() (int, int) { return ui.TerminalSize(m.width, m.height) }

func (m *Model) layoutPlay() {
	snapshot := m.playUI.snapshot
	m.playUI.layout = layoutWords(snapshot.Words, snapshot.CurrentWordIndex, m.playWidth(), m.styles)
}
