package mascot

import (
	"math"
	"testing"
	"time"

	"gopherttype/internal/app/ui"
)

// TestClassifyResultTiers pins every consumed-metric boundary: the 98/95/85
// accuracy tiers, the 60 WPM excitement gate, a valid 0 WPM, and the invalid
// fallbacks. Slow accurate rounds are proud, never penalized for speed.
func TestClassifyResultTiers(t *testing.T) {
	const sec = time.Second
	cases := []struct {
		name     string
		wpm      float64
		accuracy float64
		duration time.Duration
		want     Result
	}{
		{"excited at both boundaries", 60, 98, sec, Result{Mood: excitedMood, Celebrate: true}},
		{"excited above", 200, 100, sec, Result{Mood: excitedMood, Celebrate: true}},
		{"fast, accuracy just below", 200, 97.99, sec, Result{Mood: excitedMood}},
		{"accuracy 98, speed just below", 59.99, 98, sec, Result{Mood: excitedMood}},
		{"slow accurate is proud", 0, 100, sec, Result{Mood: excitedMood}},
		{"proud at 95", 12, 95, sec, Result{Mood: excitedMood}},
		{"calm just below 95", 12, 94.99, sec, Result{Mood: calmMood}},
		{"calm at 85", 12, 85, sec, Result{Mood: calmMood}},
		{"worried just below 85", 12, 84.99, sec, Result{Mood: worriedMood}},
		{"worried at zero accuracy", 80, 0, sec, Result{Mood: worriedMood}},
		{"negative wpm is invalid", -0.01, 100, sec, Result{Mood: calmMood}},
		{"nan wpm is invalid", math.NaN(), 100, sec, Result{Mood: calmMood}},
		{"inf wpm is invalid", math.Inf(1), 100, sec, Result{Mood: calmMood}},
		{"negative infinity wpm is invalid", math.Inf(-1), 100, sec, Result{Mood: calmMood}},
		{"accuracy above 100 is invalid", 10, 100.01, sec, Result{Mood: calmMood}},
		{"negative accuracy is invalid", 10, -0.01, sec, Result{Mood: calmMood}},
		{"nan accuracy is invalid", 10, math.NaN(), sec, Result{Mood: calmMood}},
		{"inf accuracy is invalid", 10, math.Inf(1), sec, Result{Mood: calmMood}},
		{"zero duration is invalid", 100, 100, 0, Result{Mood: calmMood}},
		{"negative duration is invalid", 100, 100, -sec, Result{Mood: calmMood}},
	}
	for _, tc := range cases {
		if got := ClassifyResult(tc.wpm, tc.accuracy, tc.duration); got != tc.want {
			t.Errorf("%s: ClassifyResult(%g, %g, %v) = %+v, want %+v", tc.name, tc.wpm, tc.accuracy, tc.duration, got, tc.want)
		}
	}
}

// TestReactionResultOverridesPlay checks result mode locks the classified mood
// and ignores the play flinch/worry/sleep/excitement policy entirely.
func TestReactionResultOverridesPlay(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := newReaction()
	r.observe(errAt(t0))
	r.observe(errAt(t0))
	r.observe(errAt(t0))
	if !r.worried {
		t.Fatal("precondition: three errors should latch worry")
	}
	r.setResult(Result{Mood: excitedMood}, t0)
	r.step(t0.Add(10 * time.Second))
	if r.base != excitedMood {
		t.Errorf("result base = %+v, want excited", r.base)
	}
	if r.worried || r.asleep || r.clean != 0 || len(r.attempts) != 0 {
		t.Errorf("result mode must clear play state: worried=%v asleep=%v clean=%d attempts=%d",
			r.worried, r.asleep, r.clean, len(r.attempts))
	}
}

// TestResultClassificationHappensOnce checks repeated evaluation and a refresh
// do not restart or reclassify the result expression or its celebration.
func TestResultClassificationHappensOnce(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := newReaction()
	r.setResult(Result{Mood: excitedMood, Celebrate: true}, t0)
	end := r.resultEnd
	start := r.resultBobAt
	for i := 0; i < 10; i++ {
		r.step(t0.Add(time.Duration(i) * time.Second))
	}
	if r.resultEnd != end || r.resultBobAt != start {
		t.Errorf("celebration deadline restarted: start %v end %v, want %v/%v", r.resultBobAt, r.resultEnd, start, end)
	}
}

// TestResultBobStartsAtZeroAndExpires checks the celebration starts at zero,
// stays bounded and returns exactly zero after the real-time deadline.
func TestResultBobStartsAtZeroAndExpires(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := newReaction()
	r.setResult(Result{Mood: excitedMood, Celebrate: true}, t0)

	if got := r.bobTarget(t0); got != 0 {
		t.Errorf("celebration must start at zero, got %g", got)
	}
	quarter := t0.Add(resultCelebrate / 16) // quarter period at 2 Hz
	if got := r.bobTarget(quarter); math.Abs(got-excitedBobAmp) > 1e-9 {
		t.Errorf("quarter-period bob = %g, want %g", got, excitedBobAmp)
	}
	for i := 0; i < 400; i++ {
		if v := math.Abs(r.bobTarget(t0.Add(time.Duration(i) * 5 * time.Millisecond))); v > excitedBobAmp+1e-9 {
			t.Fatalf("celebration bob %g exceeds amplitude %g", v, excitedBobAmp)
		}
	}
	if got := r.bobTarget(t0.Add(resultCelebrate)); got != 0 {
		t.Errorf("celebration must expire at %v, got %g", resultCelebrate, got)
	}
	if got := r.bobTarget(t0.Add(resultCelebrate + time.Second)); got != 0 {
		t.Errorf("celebration must stay zero after expiry, got %g", got)
	}
}

// TestResultNoCelebrationStaysFlat checks a resting result never bobs, even at
// the phase where a celebration would peak.
func TestResultNoCelebrationStaysFlat(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := newReaction()
	r.setResult(Result{Mood: excitedMood}, t0)
	if got := r.bobTarget(t0.Add(resultCelebrate / 16)); got != 0 {
		t.Errorf("resting proud must not bob, got %g", got)
	}
}

// TestModelResultFacesForward checks result mode forces yaw/pitch 0 with zero
// angle velocities and folds the classified mood, even when Configure is handed
// a tracking scene.
func TestModelResultFacesForward(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.SetResult(Result{Mood: calmMood}, t0)
	scene := Scene{
		Slot:    ui.Rect{X: 24, Y: 1, Width: 32, Height: 12},
		Content: ui.Rect{X: 28, Y: 14, Width: 24, Height: 10},
		Target:  ui.Point{X: 52, Y: 24},
		Track:   true,
	}
	m.Configure(scene, t0)
	if m.target.Yaw != 0 || m.target.Pitch != 0 {
		t.Fatalf("result target = %+v, want yaw/pitch 0", m.target)
	}
	if m.pose.Yaw != 0 || m.pose.Pitch != 0 {
		t.Fatalf("first result pose = %+v, want yaw/pitch 0", m.pose)
	}
	if m.vel.yaw != 0 || m.vel.pitch != 0 {
		t.Fatalf("result velocity = %+v, want zero angles", m.vel)
	}
	if m.target.EyeOpen != calmMood.EyeOpen || m.target.Lift != calmMood.Lift {
		t.Errorf("result mood not folded: %+v", m.target)
	}

	// A frame and a reconfiguration must never pull the head off forward.
	at := t0.Add(frameInterval)
	m.Update(FrameMsg{owner: m, seq: m.seq, at: at}, at)
	scene.Target = ui.Point{X: -400, Y: -400}
	m.Configure(scene, at.Add(frameInterval))
	if m.target.Yaw != 0 || m.target.Pitch != 0 || m.pose.Yaw != 0 || m.pose.Pitch != 0 {
		t.Errorf("result head drifted: target %+v pose %+v", m.target, m.pose)
	}
}

// TestModelResultCelebrationExpiresWhileHidden checks an excited result folds
// the proud mood and bob, and that elapsing the deadline while hidden does not
// replay the celebration on show.
func TestModelResultCelebrationExpiresWhileHidden(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.SetResult(Result{Mood: excitedMood, Celebrate: true}, t0)
	m.Configure(visibleScene(), t0)
	if m.target.EyeOpen != excitedMood.EyeOpen {
		t.Fatalf("result mood = %+v, want excited", m.target)
	}

	quarter := t0.Add(resultCelebrate / 16)
	m.Update(FrameMsg{owner: m, seq: m.seq, at: quarter}, quarter)
	if math.Abs(m.target.Bob-excitedBobAmp) > 1e-9 {
		t.Errorf("celebration bob target = %g, want %g", m.target.Bob, excitedBobAmp)
	}

	m.Configure(Scene{}, t0.Add(resultCelebrate))
	show := t0.Add(resultCelebrate + time.Second)
	m.Configure(visibleScene(), show)
	if m.target.Bob != 0 {
		t.Errorf("expired celebration must not replay, bob = %g", m.target.Bob)
	}
	if m.target.EyeOpen != excitedMood.EyeOpen {
		t.Errorf("resting proud must persist, got %+v", m.target)
	}
}
