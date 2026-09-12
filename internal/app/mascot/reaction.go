package mascot

import "time"

// Attempt is one accepted input fact observed by the play screen. Pace is true
// only for a fully single-rune message; ticket 10 consumes it. The reaction
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
)

// reactionMoods are the rig expression presets used by the play reaction.
var (
	calmMood    = Mood{EyeOpen: 0.95, Lift: 0.06}
	worriedMood = Mood{EyeOpen: 0.80, Lift: -0.035}
	flinchMood  = Mood{EyeOpen: 0.08, Lift: 0}
)

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

	candidate      Mood
	candidateSince time.Time
	base           Mood
}

func newReaction() reaction {
	return reaction{
		attempts:  make([]Attempt, 0, attemptCapacity),
		candidate: calmMood,
		base:      calmMood,
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

// step advances base-mood candidate debounce to the current time. The candidate
// keeps progressing during a flinch; render overlays the transient.
func (r *reaction) step(at time.Time) {
	at = r.now(at)
	candidate := calmMood
	if r.worried {
		candidate = worriedMood
	}
	if candidate != r.candidate {
		r.candidate = candidate
		r.candidateSince = at
	}
	if r.candidateSince.IsZero() || at.Sub(r.candidateSince) >= moodHold {
		r.base = candidate
	}
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
