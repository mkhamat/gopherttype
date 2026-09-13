package mascot

import (
	"math"
	"slices"
	"testing"
	"time"
)

func errAt(t time.Time) Attempt { return Attempt{At: t, Correct: false} }
func okAt(t time.Time) Attempt  { return Attempt{At: t, Correct: true} }

// paceAt builds one eligible single-rune attempt at a fixed time.
func paceAt(t time.Time, correct bool) Attempt {
	return Attempt{At: t, Correct: correct, Pace: true}
}

// paceSeq builds n pace-eligible attempts. Indices listed in wrong are errors.
func paceSeq(t0 time.Time, n int, step time.Duration, wrong ...int) []Attempt {
	out := make([]Attempt, n)
	for i := range out {
		out[i] = paceAt(t0.Add(time.Duration(i)*step), !slices.Contains(wrong, i))
	}
	return out
}

// TestReactionFlinchClosesAndReopens checks immediate closure, the 200 ms hold
// and the ~120 ms reopening envelope.
func TestReactionFlinchClosesAndReopens(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	r.observe(errAt(t0))

	if f := r.flinchFactor(t0); f != 1 {
		t.Errorf("impact must close immediately, factor %g", f)
	}
	if f := r.flinchFactor(t0.Add(flinchDuration / 2)); f != 1 {
		t.Errorf("flinch must hold for %v, factor %g", flinchDuration, f)
	}
	mid := t0.Add(flinchDuration + flinchReopen/2)
	if f := r.flinchFactor(mid); f <= 0 || f >= 1 {
		t.Errorf("mid-reopen factor %g, want (0,1)", f)
	}
	if f := r.flinchFactor(t0.Add(flinchDuration + flinchReopen)); f != 0 {
		t.Errorf("flinch must end after reopen window, factor %g", f)
	}
}

// TestReactionFlinchExtension checks a later mistake extends the envelope.
func TestReactionFlinchExtension(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	r.observe(errAt(t0))
	at := t0.Add(flinchDuration + flinchReopen)
	if r.flinchFactor(at) != 0 {
		t.Fatal("precondition: first flinch should have expired")
	}
	r.observe(errAt(t0.Add(100 * time.Millisecond)))
	if f := r.flinchFactor(at); f <= 0 {
		t.Error("a later mistake must extend the flinch")
	}
}

// TestReactionWorryLatchesOnThirdError checks worry latches with fewer than ten
// samples and clears after five consecutive correct attempts.
func TestReactionWorryLatchesOnThirdError(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	r.observe(errAt(t0))
	r.observe(errAt(t0))
	if r.worried {
		t.Fatal("two errors must not latch worry")
	}
	r.observe(errAt(t0))
	if !r.worried {
		t.Fatal("third window error must latch worry")
	}
	for i := 0; i < recoveryStreak-1; i++ {
		r.observe(okAt(t0))
	}
	if !r.worried {
		t.Fatalf("%d correct attempts must not clear worry yet", recoveryStreak-1)
	}
	r.observe(okAt(t0))
	if r.worried {
		t.Fatalf("%d consecutive correct attempts must clear worry", recoveryStreak)
	}
	r.step(t0.Add(moodHold))
	if r.worried {
		t.Error("evaluation alone must not re-latch worry from old errors")
	}
	r.observe(errAt(t0))
	if !r.worried {
		t.Error("a new mistake may re-latch worry")
	}
}

// TestReactionWindowPruneBoundary checks attempts strictly older than three
// seconds are dropped while the exact boundary is kept.
func TestReactionWindowPruneBoundary(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	r.observe(errAt(t0))
	r.observe(errAt(t0.Add(attemptWindow)))
	if len(r.attempts) != 2 {
		t.Fatalf("exact-boundary attempt must be kept, got %d", len(r.attempts))
	}
	r.observe(errAt(t0.Add(attemptWindow + time.Nanosecond)))
	if len(r.attempts) != 2 {
		t.Fatalf("strictly older attempt must be dropped, got %d", len(r.attempts))
	}
	if r.attempts[0].At != t0.Add(attemptWindow) {
		t.Errorf("oldest kept = %v, want %v", r.attempts[0].At, t0.Add(attemptWindow))
	}
}

// TestReactionAttemptOverflow checks the bounded slice caps at capacity and
// drops the oldest samples.
func TestReactionAttemptOverflow(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	const total = attemptCapacity + 20
	for i := 0; i < total; i++ {
		r.observe(okAt(t0.Add(time.Duration(i) * time.Millisecond)))
	}
	if len(r.attempts) != attemptCapacity || cap(r.attempts) != attemptCapacity {
		t.Fatalf("attempts len/cap = %d/%d, want %d", len(r.attempts), cap(r.attempts), attemptCapacity)
	}
	if want := t0.Add(time.Duration(total-attemptCapacity) * time.Millisecond); r.attempts[0].At != want {
		t.Errorf("oldest kept = %v, want %v", r.attempts[0].At, want)
	}
}

// TestReactionClampsBackwardTimestamps checks observation timestamps never move
// backwards.
func TestReactionClampsBackwardTimestamps(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	r.observe(okAt(t0.Add(2 * time.Second)))
	r.observe(okAt(t0.Add(time.Second)))
	if r.attempts[1].At != t0.Add(2*time.Second) {
		t.Errorf("backward observation not clamped: %v", r.attempts[1].At)
	}
	r.activity(t0)
	if r.lastActivity != t0.Add(2*time.Second) {
		t.Errorf("backward activity not clamped: %v", r.lastActivity)
	}
}

// TestReactionActivityWakesWithoutStarting checks editing activity updates the
// activity stamp but records no attempt and does not start play.
func TestReactionActivityWakesWithoutStarting(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	r.activity(t0)
	if len(r.attempts) != 0 {
		t.Errorf("activity must not add attempts, got %d", len(r.attempts))
	}
	if r.started {
		t.Error("activity alone must not start play")
	}
	if r.lastActivity != t0 {
		t.Errorf("lastActivity = %v, want %v", r.lastActivity, t0)
	}
	r.observe(okAt(t0))
	if !r.started {
		t.Error("an accepted attempt must start play")
	}
}

// TestReactionCandidateHold checks the base mood waits for a stable candidate
// and that repeated evaluation does not reset candidateSince.
func TestReactionCandidateHold(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	r.worried = true

	r.step(t0)
	if r.base != calmMood {
		t.Errorf("base changed before hold elapsed: %+v", r.base)
	}
	since := r.candidateSince
	r.step(t0)
	if r.candidateSince != since {
		t.Errorf("repeated evaluation reset candidateSince to %v, want %v", r.candidateSince, since)
	}
	r.step(t0.Add(moodHold - time.Millisecond))
	if r.base != calmMood {
		t.Errorf("base changed just before hold elapsed: %+v", r.base)
	}
	r.step(t0.Add(moodHold))
	if r.base != worriedMood {
		t.Errorf("base = %+v, want worried after hold", r.base)
	}
}

// TestModelFlinchClosesImmediately checks the rendered pose snaps to the flinch
// expression the instant a mistake is recorded.
func TestModelFlinchClosesImmediately(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.ObserveAttempt(errAt(t0))
	m.Configure(visibleScene(), t0)
	if got := m.renderer.lastPose.EyeOpen; got != flinchMood.EyeOpen {
		t.Errorf("rendered eye = %g, want %g", got, flinchMood.EyeOpen)
	}
	if got := m.renderer.lastPose.Lift; got != flinchMood.Lift {
		t.Errorf("rendered lift = %g, want %g", got, flinchMood.Lift)
	}
}

// TestModelWorriedAfterHold checks the base mood folds into the pose target only
// after the hold, driven by the frame clock.
func TestModelWorriedAfterHold(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.ObserveAttempt(errAt(t0))
	m.ObserveAttempt(errAt(t0))
	m.ObserveAttempt(errAt(t0))
	m.Configure(visibleScene(), t0)
	if m.target.EyeOpen != calmMood.EyeOpen {
		t.Fatalf("base must hold before %v, target eye %g", moodHold, m.target.EyeOpen)
	}
	at := t0.Add(moodHold)
	m.Update(FrameMsg{owner: m, seq: m.seq}, at)
	if m.target.EyeOpen != worriedMood.EyeOpen || m.target.Lift != worriedMood.Lift {
		t.Errorf("target = %+v, want worried %+v", m.target, worriedMood)
	}
}

// TestModelBlinkSuppressedDuringFlinch checks a blink does not darken the eye
// while a flinch overlay is visible.
func TestModelBlinkSuppressedDuringFlinch(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.setVisible(true, t0)
	m.moving = true
	m.manualAt = t0
	m.ObserveAttempt(errAt(t0))
	m.render(t0.Add(blinkDuration / 2))
	if got := m.renderer.lastPose.EyeOpen; got != flinchMood.EyeOpen {
		t.Errorf("rendered eye = %g, want flinch %g (blink must be suppressed)", got, flinchMood.EyeOpen)
	}
}

// TestModelHiddenFlinchDoesNotReplay checks that real time expires a flinch
// recorded while hidden, so showing again renders the base mood.
func TestModelHiddenFlinchDoesNotReplay(t *testing.T) {
	m := New()
	t0 := time.Unix(1000, 0)
	m.ObserveAttempt(errAt(t0))
	m.Configure(visibleScene(), t0.Add(flinchDuration+flinchReopen+time.Second))
	if got := m.renderer.lastPose.EyeOpen; got == flinchMood.EyeOpen {
		t.Error("an expired hidden flinch must not replay on show")
	}
}

// TestReactionExcitedThresholds pins every evidence boundary: sample minimum,
// elapsed evidence, WPM and accuracy. Slow accurate and insufficient evidence
// never excite; a burst cannot shrink the capped denominator.
func TestReactionExcitedThresholds(t *testing.T) {
	t0 := time.Unix(1000, 0)
	cases := []struct {
		name string
		att  []Attempt
		at   time.Duration
		want bool
	}{
		{"no evidence", nil, 2 * time.Second, false},
		{"below sample minimum", paceSeq(t0, 9, 100*time.Millisecond), 1500 * time.Millisecond, false},
		{"below pace seconds", paceSeq(t0, 10, 100*time.Millisecond), 1499 * time.Millisecond, false},
		{"at pace seconds", paceSeq(t0, 10, 100*time.Millisecond), 1500 * time.Millisecond, true},
		{"slow accurate", paceSeq(t0, 10, 100*time.Millisecond), 3 * time.Second, false},
		{"exact 60 wpm", paceSeq(t0, 15, 200*time.Millisecond), 3 * time.Second, true},
		{"accuracy exactly 95", paceSeq(t0, 20, 150*time.Millisecond, 0), 3 * time.Second, true},
		{"accuracy below 95", paceSeq(t0, 20, 150*time.Millisecond, 0, 1), 3 * time.Second, false},
	}
	for _, tc := range cases {
		r := newReaction()
		for _, a := range tc.att {
			r.observe(a)
		}
		if got := r.excited(t0.Add(tc.at)); got != tc.want {
			t.Errorf("%s: excited = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestReactionBatchesAndSpacesNeverPace checks multi-rune batches and spaces
// still count for accuracy but never the pace numerator or the sample minimum.
func TestReactionBatchesAndSpacesNeverPace(t *testing.T) {
	t0 := time.Unix(1000, 0)

	// Only non-pace accepted attempts: no firstPaceAt, so no excitement even
	// at a perfect rate.
	barren := newReaction()
	for i := 0; i < 30; i++ {
		barren.observe(Attempt{At: t0.Add(time.Duration(i) * 50 * time.Millisecond), Correct: true})
	}
	if !barren.firstPaceAt.IsZero() {
		t.Error("non-single-rune attempts must not set firstPaceAt")
	}
	if barren.excited(t0.Add(2 * time.Second)) {
		t.Error("batches/spaces alone must not excite")
	}

	// Nine pace samples plus many correct batches still miss the ten-sample
	// minimum.
	mixed := newReaction()
	for _, a := range paceSeq(t0, 9, 100*time.Millisecond) {
		mixed.observe(a)
	}
	for i := 0; i < 20; i++ {
		mixed.observe(Attempt{At: t0.Add(time.Second + time.Duration(i)*10*time.Millisecond), Correct: true})
	}
	if mixed.excited(t0.Add(1500 * time.Millisecond)) {
		t.Error("batches must not fill the pace sample minimum")
	}
}

// TestReactionFirstPaceAtSurvivesPruneAndWake checks the pace origin is the
// first eligible single-rune message, not the oldest remaining sample, and
// that a later burst cannot shrink the capped denominator.
func TestReactionFirstPaceAtSurvivesPruneAndWake(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := newReaction()
	r.observe(paceAt(t0, true))
	if r.firstPaceAt != t0 {
		t.Fatalf("firstPaceAt = %v, want %v", r.firstPaceAt, t0)
	}
	r.observe(paceAt(t0.Add(5*time.Second), true))
	r.step(t0.Add(10 * time.Second))
	if r.firstPaceAt != t0 {
		t.Errorf("firstPaceAt changed to %v after prune", r.firstPaceAt)
	}
	r.observe(paceAt(t0.Add(11*time.Second), true))
	r.step(t0.Add(11 * time.Second))
	if r.firstPaceAt != t0 {
		t.Errorf("firstPaceAt changed to %v after wake", r.firstPaceAt)
	}

	burst := newReaction()
	burst.observe(paceAt(t0, true))
	for i := 0; i < 10; i++ {
		burst.observe(paceAt(t0.Add(10*time.Second+time.Duration(i)*10*time.Millisecond), true))
	}
	if burst.excited(t0.Add(10 * time.Second)) {
		t.Error("a late burst must not shrink the pace denominator")
	}
}

// TestReactionSleepThreshold checks sleep begins exactly at the pause
// threshold and only after the first accepted attempt.
func TestReactionSleepThreshold(t *testing.T) {
	t0 := time.Unix(1000, 0)

	edit := newReaction()
	edit.activity(t0)
	edit.step(t0.Add(10 * time.Second))
	if edit.base == sleepyMood {
		t.Error("ready editing must never sleep just because time passed")
	}

	r := newReaction()
	r.observe(okAt(t0))
	r.step(t0.Add(sleepAfter - time.Millisecond))
	if r.base == sleepyMood {
		t.Error("must not sleep before the inactivity threshold")
	}
	r.step(t0.Add(sleepAfter))
	if r.base != sleepyMood || !r.asleep {
		t.Errorf("must sleep immediately at %v, base %+v", sleepAfter, r.base)
	}
}

// TestReactionSleepClearsAndWakeStartsCalm checks sleep drops stale worry and
// recovery, and waking on activity resumes calm before fresh evaluation.
func TestReactionSleepClearsAndWakeStartsCalm(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := newReaction()
	r.observe(errAt(t0))
	r.observe(errAt(t0))
	r.observe(errAt(t0))
	r.clean = 4
	if !r.worried {
		t.Fatal("precondition: worry should be latched")
	}

	r.step(t0.Add(sleepAfter))
	if r.base != sleepyMood {
		t.Fatalf("base = %+v, want sleepy", r.base)
	}
	if r.worried || r.clean != 0 {
		t.Errorf("sleep must clear worry and recovery, worried=%v clean=%d", r.worried, r.clean)
	}

	wake := t0.Add(sleepAfter + time.Second)
	r.activity(wake)
	r.step(wake)
	if r.base != calmMood || r.asleep {
		t.Errorf("wake must start calm awake, base %+v asleep %v", r.base, r.asleep)
	}
	if !r.woke {
		t.Error("wake must flag the transient for blink cancellation")
	}
}

// TestReactionPriorityWorriedOverExcited checks worry outranks excitement even
// while the pace evidence still qualifies.
func TestReactionPriorityWorriedOverExcited(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := newReaction()
	for _, a := range paceSeq(t0, 60, 30*time.Millisecond) {
		r.observe(a)
	}
	for i := range 3 {
		r.observe(errAt(t0.Add(1800*time.Millisecond + time.Duration(i)*50*time.Millisecond)))
	}
	if !r.worried || !r.excited(t0.Add(2500*time.Millisecond)) {
		t.Fatal("precondition: worry and excitement must both qualify")
	}
	r.step(t0.Add(2 * time.Second))
	r.step(t0.Add(2500 * time.Millisecond))
	if r.base != worriedMood {
		t.Errorf("base = %+v, want worried over excited", r.base)
	}
}

// TestReactionExcitedBob checks the bob target starts at zero on entry, stays
// within the named amplitude, and returns to zero when excitement ends.
func TestReactionExcitedBob(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := newReaction()
	if got := r.bobTarget(t0); got != 0 {
		t.Errorf("calm bob target = %g, want 0", got)
	}
	r.base = excitedMood
	r.excitedAt = t0
	if got := r.bobTarget(t0); got != 0 {
		t.Errorf("phase must start at zero on entry, got %g", got)
	}
	quarter := t0.Add(125 * time.Millisecond) // quarter period at 2 Hz
	if got := r.bobTarget(quarter); math.Abs(got-excitedBobAmp) > 1e-9 {
		t.Errorf("quarter-period bob = %g, want %g", got, excitedBobAmp)
	}
	for i := 0; i < 200; i++ {
		if v := math.Abs(r.bobTarget(t0.Add(time.Duration(i) * 5 * time.Millisecond))); v > excitedBobAmp+1e-9 {
			t.Fatalf("bob %g exceeds amplitude %g", v, excitedBobAmp)
		}
	}
	r.base = calmMood
	if got := r.bobTarget(quarter); got != 0 {
		t.Errorf("exiting excitement must target zero, got %g", got)
	}
}
