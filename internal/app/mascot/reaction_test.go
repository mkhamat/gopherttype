package mascot

import (
	"testing"
	"time"
)

func errAt(t time.Time) Attempt { return Attempt{At: t, Correct: false} }
func okAt(t time.Time) Attempt  { return Attempt{At: t, Correct: true} }

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
}

// TestReactionWorryCannotRelatchWithoutNewError checks that plain evaluation
// (timers/ticks) never re-arms worry from old errors, while a new mistake can.
func TestReactionWorryCannotRelatchWithoutNewError(t *testing.T) {
	r := newReaction()
	t0 := time.Unix(1000, 0)
	r.observe(errAt(t0))
	r.observe(errAt(t0))
	r.observe(errAt(t0))
	for i := 0; i < recoveryStreak; i++ {
		r.observe(okAt(t0))
	}
	if r.worried {
		t.Fatal("precondition: worry should be cleared")
	}
	for i := 0; i < 100; i++ {
		r.step(t0.Add(time.Duration(i) * time.Millisecond))
	}
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
	if len(r.attempts) != attemptCapacity {
		t.Fatalf("attempts = %d, want %d", len(r.attempts), attemptCapacity)
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
	m.Update(FrameMsg{owner: m, seq: m.seq, at: at}, at)
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
	m.setMoving(true)
	m.startBlink(t0)
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
