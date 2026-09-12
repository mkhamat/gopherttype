package results

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func benchResultsModel() *Model {
	m := New(proudMetrics())
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

func BenchmarkResultsViewCached(b *testing.B) {
	m := benchResultsModel()
	b.ReportAllocs()
	for b.Loop() {
		_ = m.View()
	}
}

// BenchmarkResultsRebuild measures recomposing the two-block frame, which runs
// once per changed mascot frame, never in View.
func BenchmarkResultsRebuild(b *testing.B) {
	m := benchResultsModel()
	b.ReportAllocs()
	for b.Loop() {
		m.rebuild()
	}
}

// TestResultsViewCachedAllocatesZero pins the cached-View contract: returning
// the composed string allocates nothing.
func TestResultsViewCachedAllocatesZero(t *testing.T) {
	m := benchResultsModel()
	if allocs := testing.AllocsPerRun(100, func() { _ = m.View() }); allocs != 0 {
		t.Errorf("cached View allocs = %v, want 0", allocs)
	}
}
