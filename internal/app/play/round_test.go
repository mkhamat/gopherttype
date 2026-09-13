package play

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mkhamat/gopherttype/internal/engine"
)

// textKey builds a key message carrying literal text; control keys have an
// empty Text and match by name.
func textKey(text string) tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Text: text}) }

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

func TestPlayInputScoring(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 2})
	base := time.Unix(1000, 0)
	steps := []tea.KeyPressMsg{key('g'), key('o'), key(' '), key('z'), key(tea.KeyBackspace), key('g'), key('o')}
	for i, s := range steps {
		m.handlePlayKeyAt(s, base.Add(time.Duration(i)*time.Second))
	}
	if m.game.Status() != engine.Finished {
		t.Fatal("final rune must finish the round")
	}
	want := engine.Metrics{Duration: 6 * time.Second, WPM: 10, Raw: 10, Accuracy: 83.33, Correct: 5}
	if got := m.game.FinalMetrics(); got != want {
		t.Errorf("metrics = %+v, want %+v", got, want)
	}
}

func TestPlayBatchAndNUL(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 2})
	base := time.Unix(1000, 0)
	m.handlePlayKeyAt(textKey("g\x00o"), base)
	m.handlePlayKeyAt(textKey("\x00"), base)
	m.handlePlayKeyAt(key('o'), base.Add(time.Second))
	want := engine.Snapshot{Elapsed: time.Second, Words: []engine.WordSnapshot{
		{Target: "go", Typed: []rune("goo")}, {Target: "go"},
	}}
	if got := m.game.Snapshot(base.Add(time.Second)); !reflect.DeepEqual(got, want) {
		t.Errorf("snapshot = %+v, want %+v", got, want)
	}
}

// TestPlayExtraWidthRejection checks that a rune rejected by the width guard
// never reaches the engine.
func TestPlayExtraWidthRejection(t *testing.T) {
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
	fin, ok := cmd().(FinishedMsg)
	want := engine.Metrics{Duration: 10 * time.Second, WPM: 1.2, Raw: 1.2, Accuracy: 100, Correct: 1}
	if !ok || fin.Metrics != want {
		t.Fatalf("deadline result = %+v, want %+v without the late rune", fin, want)
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

// TestPlayKeyFinishesBeforeTickRefresh checks a key that completes the final
// word finishes the round immediately and returns the existing FinalMetrics,
// without waiting for a snapshot refresh or a game tick.
func TestPlayKeyFinishesBeforeTickRefresh(t *testing.T) {
	m := newPlayModel(engine.Config{Mode: engine.ModeWords, WordCount: 1})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	base := time.Unix(1000, 0)
	m.handlePlayKeyAt(key('g'), base)
	cmd := m.handlePlayKeyAt(key('o'), base.Add(time.Second))

	if m.game.Status() != engine.Finished {
		t.Fatalf("status = %v, want Finished", m.game.Status())
	}
	if cmd == nil {
		t.Fatal("the finishing key must return FinishedMsg")
	}
	fin, ok := cmd().(FinishedMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want FinishedMsg", cmd())
	}
	want := engine.Metrics{Duration: time.Second, WPM: 24, Raw: 24, Accuracy: 100, Correct: 2}
	if fin.Metrics != want {
		t.Errorf("finished metrics = %+v, want %+v", fin.Metrics, want)
	}
	if m.handleTick(tickMsg{game: m.game, at: base.Add(2 * time.Second)}) != nil {
		t.Error("a tick for a finished round must be ignored")
	}
}
