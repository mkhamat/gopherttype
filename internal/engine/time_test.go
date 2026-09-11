package engine

import (
	"testing"
	"time"
)

func TestWordRoundElapsedAndFinalDuration(t *testing.T) {
	g, err := New(Config{Mode: ModeWords, WordCount: 1}, []string{"cat"})
	if err != nil {
		t.Fatal(err)
	}
	g.Handle(typed('c', time.Second))
	g.Handle(typed('a', 3*time.Second))
	if got := g.ElapsedAt(at(3 * time.Second)); got != 2*time.Second {
		t.Fatalf("elapsed = %v, want 2s", got)
	}
	g.Handle(typed('t', 3*time.Second))
	if g.Status() != Finished {
		t.Fatal("input did not complete the word")
	}
	if got := g.FinalMetrics().Duration; got != 2*time.Second {
		t.Fatalf("final duration = %v, want 2s", got)
	}
	if got := g.ElapsedAt(at(10 * time.Second)); got != 2*time.Second {
		t.Fatalf("finished elapsed = %v, want 2s", got)
	}
}

func TestLateTickFinishesAtDeadline(t *testing.T) {
	g, err := New(Config{Mode: ModeTime, Duration: time.Second}, []string{"cat"})
	if err != nil {
		t.Fatal(err)
	}
	g.Handle(typed('c', 0))
	if got := g.ElapsedAt(at(10 * time.Second)); got != time.Second {
		t.Fatalf("late query elapsed = %v, want 1s", got)
	}
	if g.Status() != Playing {
		t.Fatal("query finished the round")
	}
	g.Handle(tick(10 * time.Second))
	if got := g.FinalMetrics().Duration; got != time.Second {
		t.Fatalf("final duration = %v, want 1s", got)
	}
}

func TestWordIndexStaysWithinRound(t *testing.T) {
	g, err := New(Config{Mode: ModeWords, WordCount: 2}, []string{"cat", "dog"})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []Event{
		typed('x', 0), space(time.Second), backspace(2 * time.Second),
		deleteWord(3 * time.Second), typed('c', 4*time.Second),
		space(5 * time.Second), typed('d', 6*time.Second),
		typed('o', 7*time.Second), typed('g', 8*time.Second),
		space(9 * time.Second), typed('x', 10*time.Second),
	} {
		g.Handle(event)
		snapshot := g.Snapshot(event.At)
		if snapshot.CurrentWordIndex < 0 || snapshot.CurrentWordIndex >= len(snapshot.Words) {
			t.Fatalf("word index out of range: %d", snapshot.CurrentWordIndex)
		}
		if remaining := g.RemainingWords(); remaining < 0 || remaining > len(snapshot.Words) {
			t.Fatalf("remaining words out of range: %d", remaining)
		}
	}
	if g.Status() != Finished || g.RemainingWords() != 0 {
		t.Fatal("completed round must have no remaining words")
	}
}
