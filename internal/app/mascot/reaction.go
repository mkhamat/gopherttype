package mascot

import (
	"math"
	"time"
)

// Attempt is one accepted input fact observed by the play screen. Pace is true
// only for a fully single-rune message. The reaction
// tracker never reads the engine.
type Attempt struct {
	At      time.Time
	Correct bool
	Pace    bool
}

// Reaction tuning. Thresholds and envelopes stay centralized and named.
const (
	attemptWindow   = 3 * time.Second
	attemptCapacity = 256

	moodHold       = 500 * time.Millisecond
	flinchDuration = 200 * time.Millisecond
	flinchReopen   = 120 * time.Millisecond

	worryErrors    = 3
	worryWindow    = 10
	recoveryStreak = 5

	// Flow and idle policy. Excitement needs fast, accurate,
	// sustained single-rune evidence; sleep begins after a real pause.
	sleepAfter         = 3 * time.Second
	excitedWPM         = 60.0
	excitedAccuracy    = 95.0
	excitedPaceSamples = 10
	excitedPaceSeconds = 1.5
	excitedBobAmp      = 0.025
	excitedBobHz       = 2.0
)

// Results expression policy. Results classify the existing final
// metrics once and never use the play flinch, sleepiness or pace history.
const (
	resultCelebrate       = 2 * time.Second
	resultExcitedWPM      = 60.0
	resultExcitedAccuracy = 98.0
	resultProudAccuracy   = 95.0
	resultCalmAccuracy    = 85.0
)

// reactionMoods are the rig expression presets used by the play reaction.
var (
	calmMood    = Mood{EyeOpen: 0.95, Lift: 0.06}
	excitedMood = Mood{EyeOpen: 1.00, Lift: 0.10}
	worriedMood = Mood{EyeOpen: 0.80, Lift: -0.035}
	sleepyMood  = Mood{EyeOpen: 0.20, Lift: 0}
	flinchMood  = Mood{EyeOpen: 0.08, Lift: 0}
)

// Result is a one-shot results expression: the fixed mood the mascot holds and
// whether the bounded celebration bob plays before settling into the resting
// smile. It carries no flinch, sleep or pace policy.
type Result struct {
	Mood      Mood
	Celebrate bool
}

// ClassifyResult validates the consumed final metrics and maps them to a result
// expression. Only a finite WPM >= 0, a finite accuracy within 0..100 and a
// positive duration are valid; anything else is calm with no invented
// achievement. A valid 0 WPM is not invalid, and a slow accurate round is proud,
// never penalized for speed.
func ClassifyResult(wpm, accuracy float64, duration time.Duration) Result {
	calm := Result{Mood: calmMood}
	if duration <= 0 || !finite(wpm) || wpm < 0 || !finite(accuracy) || accuracy < 0 || accuracy > 100 {
		return calm
	}
	switch {
	case accuracy >= resultExcitedAccuracy && wpm >= resultExcitedWPM:
		return Result{Mood: excitedMood, Celebrate: true}
	case accuracy >= resultProudAccuracy:
		return Result{Mood: excitedMood}
	case accuracy >= resultCalmAccuracy:
		return calm
	default:
		return Result{Mood: worriedMood}
	}
}

// finite reports whether v is neither NaN nor an infinity.
func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// reaction is the bounded play tracker and mood machine. It keeps only fixed
// scalar state plus a preallocated attempt window; it schedules no commands and
// mutates no engine state.
type reaction struct {
	attempts []Attempt

	lastSeen     time.Time
	lastActivity time.Time
	started      bool

	clean   int
	worried bool

	flinchAt time.Time

	firstPaceAt time.Time
	excitedAt   time.Time
	asleep      bool
	woke        bool

	candidate      Mood
	candidateSince time.Time
	base           Mood

	// Result mode. A one-shot classification locks the base mood
	// and, for an excited result, a real-time celebration deadline.
	resultMode  bool
	resultBobAt time.Time
	resultEnd   time.Time
}

func newReaction() reaction {
	return reaction{
		attempts:  make([]Attempt, 0, attemptCapacity),
		candidate: calmMood,
		base:      calmMood,
	}
}

// setResult locks the tracker into a single result expression, clearing the play
// history so nothing from a round can leak into the final face. An excited
// result starts a bounded celebration bob that expires after real time.
func (r *reaction) setResult(res Result, at time.Time) {
	r.resultMode = true
	r.base = res.Mood
	r.candidate = res.Mood
	r.candidateSince = at
	r.excitedAt = time.Time{}
	r.flinchAt = time.Time{}
	r.worried = false
	r.clean = 0
	r.asleep = false
	r.woke = false
	r.attempts = r.attempts[:0]
	r.resultBobAt = time.Time{}
	r.resultEnd = time.Time{}
	if res.Celebrate {
		r.resultBobAt = at
		r.resultEnd = at.Add(resultCelebrate)
	}
}

// ObserveAttempt records one accepted play attempt and its immediate feedback.
func (m *Model) ObserveAttempt(a Attempt) { m.reaction.observe(a) }

// Activity records eligible typing or edit activity without changing attempts.
func (m *Model) Activity(at time.Time) { m.reaction.activity(at) }

func (r *reaction) observe(a Attempt) {
	at := r.now(a.At)
	r.lastActivity = at
	r.started = true
	r.prune(at)
	r.append(a, at)
	if a.Pace && r.firstPaceAt.IsZero() {
		r.firstPaceAt = at
	}

	if a.Correct {
		r.clean++
		if r.clean >= recoveryStreak {
			r.worried = false
		}
		return
	}
	r.clean = 0
	r.flinchAt = at
	if r.recentErrors() >= worryErrors {
		r.worried = true
	}
}

func (r *reaction) activity(at time.Time) {
	r.lastActivity = r.now(at)
}

// step evaluates sleep, wake and ordinary base-mood debounce at the current
// time. Sleep bypasses the candidate delay; the candidate keeps progressing
// during a flinch; render overlays the transient.
func (r *reaction) step(at time.Time) {
	at = r.now(at)
	if r.resultMode {
		return
	}
	r.prune(at)
	r.woke = false

	if r.shouldSleep(at) {
		r.enterSleep(at)
		return
	}
	if r.asleep {
		r.wake(at)
	}

	candidate := calmMood
	if r.worried {
		candidate = worriedMood
	} else if r.excited(at) {
		candidate = excitedMood
	}
	if candidate != r.candidate {
		r.candidate = candidate
		r.candidateSince = at
	}
	if r.candidateSince.IsZero() || at.Sub(r.candidateSince) >= moodHold {
		r.base = candidate
	}

	// The excited bob phase starts at zero on entry and survives only while
	// the excited base holds; leaving resets it so the next entry restarts.
	if r.base == excitedMood {
		if r.excitedAt.IsZero() {
			r.excitedAt = at
		}
	} else {
		r.excitedAt = time.Time{}
	}
}

// shouldSleep reports whether play has been idle long enough to sleep. It can
// only trigger after the first accepted attempt, so ready/home wait forever.
func (r *reaction) shouldSleep(at time.Time) bool {
	return r.started && !r.lastActivity.IsZero() && at.Sub(r.lastActivity) >= sleepAfter
}

// enterSleep adopts the sleepy base immediately, clearing stale worry, recovery
// and candidate excitement. It is idempotent across repeated evaluation.
func (r *reaction) enterSleep(at time.Time) {
	if !r.asleep {
		r.worried = false
		r.clean = 0
		r.candidate = sleepyMood
		r.candidateSince = at
		r.excitedAt = time.Time{}
		r.flinchAt = time.Time{}
	}
	r.asleep = true
	r.base = sleepyMood
}

// wake leaves sleep immediately, starting calm before the caller reassesses
// fresh flow. It flags the transient so the model can cancel a stale blink.
func (r *reaction) wake(at time.Time) {
	r.asleep = false
	r.woke = true
	r.worried = false
	r.clean = 0
	r.candidate = calmMood
	r.candidateSince = at
	r.base = calmMood
	r.excitedAt = time.Time{}
	r.flinchAt = time.Time{}
}

// excited reports whether the active window holds fast, accurate, sustained
// single-rune evidence. The denominator is capped at the window, never shrunk
// to a burst, and firstPaceAt is the first eligible single-rune message.
func (r *reaction) excited(at time.Time) bool {
	if r.firstPaceAt.IsZero() {
		return false
	}
	seconds := at.Sub(r.firstPaceAt).Seconds()
	if seconds <= 0 {
		return false
	}
	if seconds > attemptWindow.Seconds() {
		seconds = attemptWindow.Seconds()
	}
	if seconds < excitedPaceSeconds {
		return false
	}

	total, correct, samples, correctPace := 0, 0, 0, 0
	for _, a := range r.attempts {
		total++
		if a.Correct {
			correct++
		}
		if a.Pace {
			samples++
			if a.Correct {
				correctPace++
			}
		}
	}
	if samples < excitedPaceSamples {
		return false
	}
	if float64(correctPace)*12/seconds < excitedWPM {
		return false
	}
	return float64(correct)/float64(total)*100 >= excitedAccuracy
}

// bobTarget is the restrained excited head bob: zero unless excited, phase
// starting at zero on entry, bounded by excitedBobAmp world units.
func (r *reaction) bobTarget(at time.Time) float64 {
	if r.resultMode {
		return r.resultBob(at)
	}
	if r.base != excitedMood || r.excitedAt.IsZero() {
		return 0
	}
	t := at.Sub(r.excitedAt).Seconds()
	return excitedBobAmp * math.Sin(2*math.Pi*excitedBobHz*t)
}

// resultBob is the results celebration: the same restrained 2 Hz bob, starting
// at zero and returning to zero once the real-time deadline passes. Hidden time
// expires it, so showing again does not replay it.
func (r *reaction) resultBob(at time.Time) float64 {
	if r.resultBobAt.IsZero() || !at.Before(r.resultEnd) {
		return 0
	}
	t := at.Sub(r.resultBobAt).Seconds()
	return excitedBobAmp * math.Sin(2*math.Pi*excitedBobHz*t)
}

// flinchFactor returns the 0..1 blend toward the flinch expression: it closes
// immediately at the latest mistake and reopens over flinchReopen.
func (r *reaction) flinchFactor(at time.Time) float64 {
	if r.flinchAt.IsZero() {
		return 0
	}
	d := at.Sub(r.flinchAt)
	if d < 0 {
		return 0
	}
	if d < flinchDuration {
		return 1
	}
	d -= flinchDuration
	if d >= flinchReopen {
		return 0
	}
	return 1 - float64(d)/float64(flinchReopen)
}

// now monotonically clamps the tracker's own visual timestamps.
func (r *reaction) now(at time.Time) time.Time {
	if at.Before(r.lastSeen) {
		return r.lastSeen
	}
	r.lastSeen = at
	return at
}

// prune drops attempts strictly older than the active window.
func (r *reaction) prune(now time.Time) {
	cutoff := now.Add(-attemptWindow)
	drop := 0
	for drop < len(r.attempts) && r.attempts[drop].At.Before(cutoff) {
		drop++
	}
	if drop > 0 {
		r.attempts = append(r.attempts[:0], r.attempts[drop:]...)
	}
}

// append stores an attempt in the bounded slice, dropping the oldest on
// overflow.
func (r *reaction) append(a Attempt, at time.Time) {
	a.At = at
	if len(r.attempts) == cap(r.attempts) {
		copy(r.attempts, r.attempts[1:])
		r.attempts[len(r.attempts)-1] = a
		return
	}
	r.attempts = append(r.attempts, a)
}

// recentErrors counts errors among the last at-most-ten active-window attempts.
func (r *reaction) recentErrors() int {
	errors := 0
	start := max(0, len(r.attempts)-worryWindow)
	for _, a := range r.attempts[start:] {
		if !a.Correct {
			errors++
		}
	}
	return errors
}
