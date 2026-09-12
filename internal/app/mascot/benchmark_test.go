package mascot

import (
	"testing"
)

// Benchmarks isolate the two costs the renderer cares about: numeric sampling
// (fixed arrays, no per-ray allocation) and immutable-string serialization.
// Machine-specific ns/op is recorded, never asserted.

func BenchmarkSampleScene(b *testing.B) {
	var samples [SampleWidth * SampleHeight]sample
	pose := NeutralPose()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sampleScene(pose, &samples)
	}
}

func BenchmarkEncode(b *testing.B) {
	r := NewRenderer()
	pose := NeutralPose()
	r.Render(pose, nil) // fill samples once
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = r.encode(pose)
	}
}

func BenchmarkSerialize(b *testing.B) {
	r := NewRenderer()
	r.Render(NeutralPose(), nil)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = r.serialize()
	}
}

// BenchmarkRenderChangedPose exercises the full path (sample, encode, re-serialize)
// with a deterministic off-grid yaw sequence.
func BenchmarkRenderChangedPose(b *testing.B) {
	r := NewRenderer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pose := NeutralPose()
		pose.Yaw = float64((i%70)-35) + 0.5
		r.Render(pose, nil)
	}
}

// BenchmarkRenderCacheHit measures the identical pose/background early return.
func BenchmarkRenderCacheHit(b *testing.B) {
	r := NewRenderer()
	pose := NeutralPose()
	r.Render(pose, nil)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Render(pose, nil)
	}
}
