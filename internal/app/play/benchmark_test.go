package play

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/engine"
)

func benchPlayModel() *Model {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 3})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

// BenchmarkPlayRebuild measures recomposing the two-block play frame after a
// mascot frame change, excluding the engine and word layout.
func BenchmarkPlayRebuild(b *testing.B) {
	m := benchPlayModel()
	b.ReportAllocs()
	for b.Loop() {
		m.rebuild()
	}
}

// TestPlayViewCachedAllocatesZero pins the cached-View contract.
func TestPlayViewCachedAllocatesZero(t *testing.T) {
	m := benchPlayModel()
	if allocs := testing.AllocsPerRun(100, func() { _ = m.View() }); allocs != 0 {
		t.Errorf("cached View allocs = %v, want 0", allocs)
	}
}
