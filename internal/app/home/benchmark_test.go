package home

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func benchHomeModel() *Model {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

// BenchmarkHomeRebuild measures recomposing the two-block frame after a mascot
// frame change.
func BenchmarkHomeRebuild(b *testing.B) {
	m := benchHomeModel()
	b.ReportAllocs()
	for b.Loop() {
		m.rebuild()
	}
}

// TestHomeViewCachedAllocatesZero pins the cached-View contract.
func TestHomeViewCachedAllocatesZero(t *testing.T) {
	m := benchHomeModel()
	if allocs := testing.AllocsPerRun(100, func() { _ = m.View() }); allocs != 0 {
		t.Errorf("cached View allocs = %v, want 0", allocs)
	}
}
