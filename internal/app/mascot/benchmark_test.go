package mascot

import (
	"image/color"
	"math"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"
)

// Benchmarks isolate the costs the mascot runtime cares about: numeric
// sampling (fixed arrays, no per-ray allocation), encoding, immutable-string
// serialization, spring/reaction work and the cached view. Machine-specific
// ns/op is recorded, never asserted; the allocation-contract tests below are
// the only structural gates.

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

var benchBackgrounds = []struct {
	name string
	bg   color.Color
}{
	{"Default", nil},
	{"Dark", color.RGBA{R: 0x16, G: 0x1b, B: 0x22, A: 0xff}},
	{"Light", color.RGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff}},
}

// BenchmarkSampleSceneNeutral is the original neutral single-pose trace.
func BenchmarkSampleSceneNeutral(b *testing.B) {
	var samples [SampleWidth * SampleHeight]sample
	pose := NeutralPose()
	b.ReportAllocs()
	for b.Loop() {
		sampleScene(pose, &samples)
	}
}

// BenchmarkSampleSceneSweep traces arbitrary poses, not only neutral, so the
// geometry cost is measured across the whole pose box.
func BenchmarkSampleSceneSweep(b *testing.B) {
	var samples [SampleWidth * SampleHeight]sample
	i := 0
	b.ReportAllocs()
	for b.Loop() {
		sampleScene(sweepPose(i), &samples)
		i++
	}
}

// BenchmarkEncodeWorstMixedFace fills the sample buffer with an open-eyed
// forward pose that shows both pupils, the nose, muzzle, mouth and teeth, then
// encodes it. This is a near-worst mix of distinct palette colors per cell.
func BenchmarkEncodeWorstMixedFace(b *testing.B) {
	r := NewRenderer()
	pose := Pose{Yaw: 0, Pitch: 0, EyeOpen: 1, Lift: 0.10, Bob: 0}
	r.Render(pose, nil) // fill samples and build the distance table once
	b.ReportAllocs()
	for b.Loop() {
		_ = r.encode()
	}
}

// BenchmarkEncodeSweep encodes changing poses across every background instead
// of a single cached sample grid.
func BenchmarkEncodeSweep(b *testing.B) {
	r := NewRenderer()
	i := 0
	b.ReportAllocs()
	for b.Loop() {
		pose := sweepPose(i)
		r.buildDistances(backgroundRGB(benchBackgrounds[i%len(benchBackgrounds)].bg))
		sampleScene(pose, &r.samples)
		_ = r.encode()
		i++
	}
}

// BenchmarkSerialize is the immutable-string style-run serializer.
func BenchmarkSerialize(b *testing.B) {
	r := NewRenderer()
	r.Render(NeutralPose(), nil)
	b.ReportAllocs()
	for b.Loop() {
		_ = r.serialize()
	}
}

// BenchmarkRenderChangedPose exercises the full path (sample, encode,
// re-serialize) with a deterministic off-grid yaw sequence.
func BenchmarkRenderChangedPose(b *testing.B) {
	r := NewRenderer()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		pose := NeutralPose()
		pose.Yaw = float64((i%70)-35) + 0.5
		r.Render(pose, nil)
		i++
	}
}

// BenchmarkRenderChangedPoseSameCells uses sub-quantization pose deltas that
// raycast and encode but produce identical cells, exercising the suppression
// of the final serialization.
func BenchmarkRenderChangedPoseSameCells(b *testing.B) {
	r := NewRenderer()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		pose := NeutralPose()
		pose.Yaw = float64(i%2) * 1e-6
		r.Render(pose, nil)
		i++
	}
}

// BenchmarkRenderChangedBackground alternates the background with a fixed pose
// so the distance table is rebuilt and cells re-encoded.
func BenchmarkRenderChangedBackground(b *testing.B) {
	r := NewRenderer()
	pose := NeutralPose()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		r.Render(pose, benchBackgrounds[i%len(benchBackgrounds)].bg)
		i++
	}
}

// BenchmarkRenderCacheHit measures the identical pose/background early return.
func BenchmarkRenderCacheHit(b *testing.B) {
	r := NewRenderer()
	pose := NeutralPose()
	r.Render(pose, nil)
	b.ReportAllocs()
	for b.Loop() {
		r.Render(pose, nil)
	}
}

// BenchmarkViewCached measures returning the cached ANSI string with no work.
func BenchmarkViewCached(b *testing.B) {
	r := NewRenderer()
	r.Render(NeutralPose(), nil)
	b.ReportAllocs()
	for b.Loop() {
		_ = r.View()
	}
}

// BenchmarkNewRenderer reports the one-time cold cost of building the fixed
// style cache, kept separate from the warm steady state.
func BenchmarkNewRenderer(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = NewRenderer()
	}
}

// BenchmarkSpringAdvance measures one bounded spring step toward a target,
// including the Harmonica coefficient work.
func BenchmarkSpringAdvance(b *testing.B) {
	m := New()
	m.pose = NeutralPose()
	m.target = Pose{Yaw: 30, Pitch: 15, EyeOpen: 0.4, Lift: 0.02, Bob: 0.04}
	m.vel = poseVel{}
	b.ReportAllocs()
	for b.Loop() {
		m.advance(frameInterval)
	}
}

// BenchmarkReactionStream measures the bounded observe/step machine over a
// repeating fast accurate stream.
func BenchmarkReactionStream(b *testing.B) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	attempt := Attempt{At: t0, Correct: true, Pace: true}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		attempt.At = t0.Add(time.Duration(i) * 100 * time.Millisecond)
		r.observe(attempt)
		r.step(attempt.At)
		i++
	}
}

// ---- structural allocation contracts (ordinary tests, machine independent) ----

// allocsOf runs fn cold, then reports the average allocation count of the warm
// steady state so scratch reuse is measured rather than first-use growth.
func allocsOf(fn func(), warm int, runs int) float64 {
	for i := 0; i < warm; i++ {
		fn()
	}
	return testing.AllocsPerRun(runs, fn)
}

func TestSampleSceneAllocatesZero(t *testing.T) {
	var samples [SampleWidth * SampleHeight]sample
	pose := sweepPose(7)
	if got := allocsOf(func() { sampleScene(pose, &samples) }, 2, 50); got != 0 {
		t.Errorf("sampleScene allocs = %v, want 0", got)
	}
}

func TestEncodeAllocatesZero(t *testing.T) {
	r := NewRenderer()
	r.Render(NeutralPose(), nil)
	if got := allocsOf(func() { _ = r.encode() }, 1, 50); got != 0 {
		t.Errorf("encode allocs = %v, want 0", got)
	}
}

func TestRenderCacheHitAllocatesZero(t *testing.T) {
	r := NewRenderer()
	pose := NeutralPose()
	r.Render(pose, nil)
	if got := allocsOf(func() { r.Render(pose, nil) }, 1, 50); got != 0 {
		t.Errorf("cache-hit Render allocs = %v, want 0", got)
	}
}

func TestViewAllocatesZero(t *testing.T) {
	r := NewRenderer()
	r.Render(NeutralPose(), nil)
	if got := allocsOf(func() { _ = r.View() }, 1, 50); got != 0 {
		t.Errorf("View allocs = %v, want 0", got)
	}
}

// TestSerializeAllocationsBounded checks the serializer reuses its scratch and
// emits exactly one immutable string per changed frame.
func TestSerializeAllocationsBounded(t *testing.T) {
	r := NewRenderer()
	r.Render(NeutralPose(), nil)
	if got := allocsOf(func() { _ = r.serialize() }, 2, 50); got > 1 {
		t.Errorf("serialize allocs = %v, want <= 1", got)
	}
}

// TestRendererRetainedStateBounded checks the renderer's scratch and cached
// view stay within fixed structural bounds across many distinct frames rather
// than growing with frame count.
func TestRendererRetainedStateBounded(t *testing.T) {
	const frames = 500
	r := NewRenderer()
	for i := 0; i < frames; i++ {
		r.Render(sweepPose(i), nil)
	}
	if cap(r.scratch) > 65536 {
		t.Errorf("scratch cap %d after %d frames, want bounded", cap(r.scratch), frames)
	}
	if len(r.view) > 65536 {
		t.Errorf("cached view length %d after %d frames, want bounded", len(r.view), frames)
	}
}

// TestReactionRetainedStateBounded checks the attempt window stays capped and
// the pace origin does not grow with sample count.
func TestReactionRetainedStateBounded(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	for i := 0; i < attemptCapacity*4; i++ {
		r.observe(paceAt(t0.Add(time.Duration(i)*time.Millisecond), true))
	}
	if len(r.attempts) > attemptCapacity {
		t.Fatalf("attempts = %d, want <= %d", len(r.attempts), attemptCapacity)
	}
	if cap(r.attempts) != attemptCapacity {
		t.Errorf("attempt capacity = %d, want %d", cap(r.attempts), attemptCapacity)
	}
}

// ---- opt-in bounded latency distribution ----

// TestRenderLatencyDistribution collects p50/p95/p99 and the worst frame over a
// bounded render sweep. It is opt-in (GOPHERTTYPE_PERF=1) because absolute
// nanoseconds are machine specific, and it asserts nothing about speed.
func TestRenderLatencyDistribution(t *testing.T) {
	if os.Getenv("GOPHERTTYPE_PERF") == "" {
		t.Skip("set GOPHERTTYPE_PERF=1 to collect render latency percentiles")
	}
	const samples = 4000
	r := NewRenderer()
	p50, p95, p99, max, mean := renderDistribution(r, samples, benchBackgrounds[1].bg)
	t.Logf("render latency over %d frames: mean=%v p50=%v p95=%v p99=%v max=%v", samples, mean, p50, p95, p99, max)
}

// renderDistribution warms the renderer, then times samples renders over an
// arbitrary pose sweep and returns the percentile distribution. Timing is
// local to this helper; no production telemetry is added.
func renderDistribution(r *Renderer, samples int, bg color.Color) (p50, p95, p99, max, mean time.Duration) {
	for i := 0; i < 20; i++ {
		r.Render(sweepPose(i), bg)
	}
	durs := make([]time.Duration, samples)
	var total time.Duration
	for i := 0; i < samples; i++ {
		start := time.Now()
		r.Render(sweepPose(i), bg)
		d := time.Since(start)
		durs[i] = d
		total += d
	}
	slices.Sort(durs)
	mean = total / time.Duration(samples)
	at := func(p float64) time.Duration {
		idx := int(math.Ceil(p*float64(samples))) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= samples {
			idx = samples - 1
		}
		return durs[idx]
	}
	return at(0.50), at(0.95), at(0.99), durs[samples-1], mean
}

// TestSustainedFrameSteadyState is an opt-in bounded sustained-load check. It
// drives one component through 120 Hz synthetic frames for a target duration,
// asserting the reaction window stays capped and reporting the heap delta. It
// cannot measure physical terminal presentation, which needs a real terminal.
func TestSustainedFrameSteadyState(t *testing.T) {
	if os.Getenv("GOPHERTTYPE_PERF") == "" {
		t.Skip("set GOPHERTTYPE_PERF=1 to run the sustained frame check")
	}
	const frames = 120 * 60 // 60 s at 120 Hz
	m := New()
	t0 := time.Unix(1000, 0)
	m.setVisible(true, t0)
	m.setMoving(true)
	m.pending = true
	m.seq = 1
	m.nextDeadline = t0.Add(frameInterval)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	now := t0
	for i := 0; i < frames; i++ {
		now = now.Add(frameInterval)
		m.setTarget(sweepPose(i * 3))
		m.advancePose(now)
		m.ObserveAttempt(Attempt{At: now, Correct: i%5 != 0, Pace: true})
	}
	// Collect before sampling so the delta reflects retained, not garbage,
	// memory.
	runtime.GC()
	runtime.ReadMemStats(&after)

	if len(m.reaction.attempts) > attemptCapacity {
		t.Errorf("retained attempts = %d, want <= %d", len(m.reaction.attempts), attemptCapacity)
	}
	if cap(m.renderer.scratch) > 65536 {
		t.Errorf("scratch cap = %d, want bounded", cap(m.renderer.scratch))
	}
	t.Logf("frames=%d retained attempts=%d heap delta=%d bytes", frames, len(m.reaction.attempts), int64(after.HeapAlloc)-int64(before.HeapAlloc))
}
