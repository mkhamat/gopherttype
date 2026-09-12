package play

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/engine"
)

// textKey builds a key message carrying literal text; control keys have an
// empty Text and match by name.
func textKey(text string) tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Text: text}) }

// rawEvents maps one key message to the events the app forwards to the engine,
// mirroring handlePlayKeyAt's accepted branch.
func rawEvents(k tea.KeyPressMsg, at time.Time) []engine.Event {
	switch k.String() {
	case "backspace":
		return []engine.Event{{Kind: engine.Backspace, At: at}}
	case "ctrl+backspace", "alt+backspace":
		return []engine.Event{{Kind: engine.DeleteWord, At: at}}
	case "space", "shift+space":
		return []engine.Event{{Kind: engine.Space, At: at}}
	}
	events := make([]engine.Event, 0, len(k.Text))
	for _, r := range k.Text {
		events = append(events, engine.Event{Kind: engine.Type, Rune: r, At: at})
	}
	return events
}

// TestInputEligibleStatusAndDeadline pins the observation gate: Finished is
// always ineligible and a timed event at or past the deadline is ineligible.
func TestInputEligibleStatusAndDeadline(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeTime, Duration: 10 * time.Second})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	start := time.Unix(1000, 0)
	m.handlePlayKeyAt(key('g'), start)

	cases := []struct {
		name   string
		status engine.Status
		at     time.Time
		want   bool
	}{
		{"ready always eligible", engine.Ready, start.Add(time.Hour), true},
		{"playing before deadline", engine.Playing, start.Add(9 * time.Second), true},
		{"playing at deadline", engine.Playing, start.Add(10 * time.Second), false},
		{"playing past deadline", engine.Playing, start.Add(11 * time.Second), false},
		{"finished always ineligible", engine.Finished, start, false},
	}
	for _, tc := range cases {
		if got := m.inputEligible(tc.status, tc.at); got != tc.want {
			t.Errorf("%s: inputEligible = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestPlayObservationPreservesEngineOutcome drives an identical sequence
// through the play handler and a bare engine and checks the final metrics
// match exactly.
func TestPlayObservationPreservesEngineOutcome(t *testing.T) {
	config := engine.Config{Mode: engine.ModeWords, WordCount: 2}
	m := newPlayModel(config)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	raw := engine.New(config, []string{"go", "go"})

	base := time.Unix(1000, 0)
	steps := []tea.KeyPressMsg{
		key('g'), key('o'), key(' '),
		key('z'), key(tea.KeyBackspace),
		key('g'), key('o'),
	}
	for _, s := range steps {
		m.handlePlayKeyAt(s, base)
		for _, e := range rawEvents(s, base) {
			raw.Handle(e)
		}
	}
	if m.game.Status() != raw.Status() {
		t.Fatalf("status = %v, want %v", m.game.Status(), raw.Status())
	}
	if got, want := m.game.FinalMetrics(), raw.FinalMetrics(); got != want {
		t.Errorf("metrics = %+v, want %+v", got, want)
	}
}

// TestPlayBatchAndNULObservationParity checks a multi-rune batch and an
// embedded NUL reach the same engine state through both paths. The NUL is a
// no-op for the engine and the observer even though the UI loop appends it
// locally.
func TestPlayBatchAndNULObservationParity(t *testing.T) {
	config := engine.Config{Mode: engine.ModeWords, WordCount: 2}
	m := newPlayModel(config)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	raw := engine.New(config, []string{"go", "go"})

	base := time.Unix(1000, 0)
	steps := []tea.KeyPressMsg{textKey("go"), textKey("\x00"), key('o')}
	for _, s := range steps {
		m.handlePlayKeyAt(s, base)
		for _, e := range rawEvents(s, base) {
			raw.Handle(e)
		}
	}
	if got, want := m.game.Snapshot(base), raw.Snapshot(base); !reflect.DeepEqual(got, want) {
		t.Errorf("snapshot mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// TestPlayExtraWidthRejectionSkipsEngineAndObserver checks that a rune rejected
// by the width guard never reaches the engine.
func TestPlayExtraWidthRejectionSkipsEngineAndObserver(t *testing.T) {
	target := strings.Repeat("a", 90)
	config := engine.Config{Mode: engine.ModeWords, WordCount: 2}
	m := New(config, func(int) []string { return []string{target, "x"} })
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	base := time.Unix(1000, 0)
	for i := 0; i < len(target); i++ {
		m.handlePlayKeyAt(key('a'), base)
	}
	if got := len(m.playUI.snapshot.Words[0].Typed); got != len(target) {
		t.Fatalf("typed %d runes, want %d", got, len(target))
	}
	m.handlePlayKeyAt(key('b'), base)
	if got := len(m.playUI.snapshot.Words[0].Typed); got != len(target) {
		t.Errorf("rejected rune reached the engine: typed %d, want %d", got, len(target))
	}
}

// TestPlayDeadlineInputStillFinishes checks an ineligible deadline event is
// still delivered so the engine finishes through its original path.
func TestPlayDeadlineInputStillFinishes(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeTime, Duration: 10 * time.Second})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	start := time.Unix(1000, 0)
	m.handlePlayKeyAt(key('g'), start)

	cmd := m.handlePlayKeyAt(key('o'), start.Add(10*time.Second))
	if m.game.Status() != engine.Finished {
		t.Fatalf("a deadline input must finish the round, status %v", m.game.Status())
	}
	if cmd == nil {
		t.Fatal("finishing must return FinishedMsg")
	}
	if _, ok := cmd().(FinishedMsg); !ok {
		t.Fatalf("cmd returned %T, want FinishedMsg", cmd())
	}
}

func TestPlayTimeModeArmsGameTick(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeTime, Duration: 15 * time.Second})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	cmd := m.handlePlayKeyAt(key('g'), time.Unix(1000, 0))
	if m.game.Status() != engine.Playing {
		t.Fatalf("first rune must start the round, status %v", m.game.Status())
	}
	if cmd == nil {
		t.Fatal("starting a timed round must arm the game tick")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Fatalf("start tick returned %T, want tickMsg", cmd())
	}
}

func TestPlayTickRejectsForeignAndNonPlaying(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeTime, Duration: 15 * time.Second})
	foreign := engine.New(m.roundConfig, []string{"go"})
	if cmd := m.handleTick(tickMsg{game: foreign, at: time.Unix(1000, 0)}); cmd != nil {
		t.Error("a foreign game tick must be ignored")
	}
	if cmd := m.handleTick(tickMsg{game: m.game, at: time.Unix(1000, 0)}); cmd != nil {
		t.Error("a tick while the round is Ready must be ignored")
	}
}

func TestPlayPlayingTickContinues(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeTime, Duration: 15 * time.Second})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	start := time.Unix(1000, 0)
	m.handlePlayKeyAt(key('g'), start)

	cmd := m.handleTick(tickMsg{game: m.game, at: start.Add(100 * time.Millisecond)})
	if cmd == nil {
		t.Fatal("a playing tick must reschedule")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Fatalf("tick chain returned %T, want tickMsg", cmd())
	}
}
