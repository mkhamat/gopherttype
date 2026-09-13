package mascot

import (
	"image/color"
	"math"
	"testing"
	"time"
)

// sweepPose produces a deterministic arbitrary pose across the full valid pose
// box. A sweep is used instead of neutral alone so geometry and encoding
// benchmarks see every mood, limit and direction.
func sweepPose(i int) Pose {
	t := float64(i)
	return Pose{
		Yaw:     -35 + math.Mod(t*7.3, 70),
		Pitch:   math.Mod(t*3.1, 20),
		EyeOpen: math.Mod(t*0.13, 1),
		Lift:    -0.035 + math.Mod(t*0.017, 0.135),
		Bob:     -0.06 + math.Mod(t*0.011, 0.12),
	}
}

func BenchmarkSampleSceneSweep(b *testing.B) {
	var samples [SampleWidth * SampleHeight]sample
	i := 0
	b.ReportAllocs()
	for b.Loop() {
		sampleScene(sweepPose(i), &samples)
		i++
	}
}

func BenchmarkEncodeMixedFace(b *testing.B) {
	r := NewRenderer()
	r.Render(Pose{EyeOpen: 1, Lift: 0.10}, nil)
	b.ReportAllocs()
	for b.Loop() {
		_ = r.encode()
	}
}

func BenchmarkSerialize(b *testing.B) {
	r := NewRenderer()
	r.Render(NeutralPose(), nil)
	b.ReportAllocs()
	for b.Loop() {
		_ = r.serialize()
	}
}

func BenchmarkRenderChangedPose(b *testing.B) {
	r := NewRenderer()
	r.Render(sweepPose(0), nil)
	i := 1
	b.ReportAllocs()
	for b.Loop() {
		r.Render(sweepPose(i), nil)
		i++
	}
}

func BenchmarkRenderChangedPoseSameCells(b *testing.B) {
	r := NewRenderer()
	pose := NeutralPose()
	r.Render(pose, nil)
	i := 0
	b.ReportAllocs()
	for b.Loop() {
		pose.Pitch = float64(i%2) * 0.005
		r.Render(pose, nil)
		i++
	}
}

func BenchmarkRenderChangedBackground(b *testing.B) {
	r := NewRenderer()
	pose := Pose{Yaw: 35, Pitch: 14, EyeOpen: 0.95, Lift: 0.06}
	r.Render(pose, color.White)
	backgrounds := []color.Color{color.Black, color.White}
	i := 0
	b.ReportAllocs()
	for b.Loop() {
		r.Render(pose, backgrounds[i%2])
		i++
	}
}

func BenchmarkRenderCacheHit(b *testing.B) {
	r := NewRenderer()
	pose := NeutralPose()
	r.Render(pose, nil)
	b.ReportAllocs()
	for b.Loop() {
		r.Render(pose, nil)
	}
}

func BenchmarkNewRenderer(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = NewRenderer()
	}
}

func BenchmarkSpringAdvance(b *testing.B) {
	m := New()
	i := 0
	b.ReportAllocs()
	for b.Loop() {
		m.target = sweepPose(i)
		m.advance(frameInterval)
		i++
	}
}

func BenchmarkReactionStream(b *testing.B) {
	r := newReaction()
	at := time.Unix(1000, 0)
	b.ReportAllocs()
	for b.Loop() {
		r.observe(Attempt{At: at, Correct: true, Pace: true})
		r.step(at)
		at = at.Add(100 * time.Millisecond)
	}
}

func TestRendererAllocations(t *testing.T) {
	r := NewRenderer()
	pose := sweepPose(7)
	r.Render(pose, nil)
	for _, tc := range []struct {
		name string
		fn   func()
		max  float64
	}{
		{"sample", func() { sampleScene(pose, &r.samples) }, 0},
		{"encode", func() { _ = r.encode() }, 0},
		{"cache hit", func() { r.Render(pose, nil) }, 0},
		{"view", func() { _ = r.View() }, 0},
		{"serialize", func() { _ = r.serialize() }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := testing.AllocsPerRun(50, tc.fn); got > tc.max {
				t.Errorf("allocs = %v, want <= %v", got, tc.max)
			}
		})
	}
}

// TestRendererRetainedStateBounded checks the renderer's scratch and cached
// view stay within fixed structural bounds across many distinct frames rather
// than growing with frame count.
func TestRendererRetainedStateBounded(t *testing.T) {
	r := NewRenderer()
	for i := range 500 {
		r.Render(sweepPose(i), nil)
	}
	if cap(r.scratch) > 65536 || len(r.View()) > 65536 {
		t.Errorf("retained scratch/view exceed 64 KiB: %d/%d", cap(r.scratch), len(r.View()))
	}
}
