package ui

import (
	"testing"
)

// The composition helpers build a full 80x24 (or 120x40) frame from a cached
// portrait and cached content. They run once per changed mascot frame, never in
// View, so their cost is measured directly.

func benchPortrait() string {
	return block(mascotHeight, mascotWidth, "█")
}

func BenchmarkLayoutWithMascot(b *testing.B) {
	content := block(10, 36, "c")
	b.ReportAllocs()
	for b.Loop() {
		_ = LayoutWithMascot(80, 24, content)
	}
}

func BenchmarkComposeWithMascotVisible80x24(b *testing.B) {
	content := block(10, 36, "c")
	layout := LayoutWithMascot(80, 24, content)
	portrait := benchPortrait()
	b.ReportAllocs()
	for b.Loop() {
		_ = ComposeWithMascot(content, portrait, layout, 80, 24)
	}
}

func BenchmarkComposeWithMascotVisible120x40(b *testing.B) {
	content := block(10, 36, "c")
	layout := LayoutWithMascot(120, 40, content)
	portrait := benchPortrait()
	b.ReportAllocs()
	for b.Loop() {
		_ = ComposeWithMascot(content, portrait, layout, 120, 40)
	}
}

// BenchmarkComposeWithMascotHidden measures the hidden fallback, which composes
// only the content block.
func BenchmarkComposeWithMascotHidden(b *testing.B) {
	content := block(10, 36, "c")
	b.ReportAllocs()
	for b.Loop() {
		_ = ComposeWithMascot(content, "", MascotLayout{}, 80, 24)
	}
}
