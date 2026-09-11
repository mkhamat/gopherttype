package engine

import (
	"errors"
	"testing"
	"time"
)

func TestNewConfigValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config Config
		want   error
	}{
		{"words", Config{Mode: ModeWords, WordCount: 1}, nil},
		{"time", Config{Mode: ModeTime, Duration: time.Second}, nil},
		{"words ignore duration", Config{Mode: ModeWords, WordCount: 1, Duration: -time.Second}, nil},
		{"time ignores count", Config{Mode: ModeTime, Duration: time.Second, WordCount: -1}, nil},
		{"invalid mode", Config{Mode: -1}, ErrInvalidMode},
		{"zero words", Config{Mode: ModeWords}, ErrInvalidWordCount},
		{"negative words", Config{Mode: ModeWords, WordCount: -1}, ErrInvalidWordCount},
		{"zero duration", Config{Mode: ModeTime}, ErrInvalidDuration},
		{"negative duration", Config{Mode: ModeTime, Duration: -time.Second}, ErrInvalidDuration},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config, []string{"cat"})
			if !errors.Is(err, tt.want) {
				t.Fatalf("New() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestFinalMetricsRequiresFinishedGame(t *testing.T) {
	for _, playing := range []bool{false, true} {
		name := "ready"
		if playing {
			name = "playing"
		}
		t.Run(name, func(t *testing.T) {
			g, err := New(Config{Mode: ModeTime, Duration: time.Second}, []string{"cat"})
			if err != nil {
				t.Fatal(err)
			}
			if playing {
				g.Handle(typed('c', 0))
			}
			defer func() {
				if recover() == nil {
					t.Fatal("FinalMetrics did not panic before completion")
				}
			}()
			g.FinalMetrics()
		})
	}
}

func TestElapsedAndSnapshotDoNotFinishRound(t *testing.T) {
	g, err := New(Config{Mode: ModeTime, Duration: time.Second}, []string{"cat"})
	if err != nil {
		t.Fatal(err)
	}
	g.Handle(typed('c', 0))
	if got := g.ElapsedAt(at(2 * time.Second)); got != time.Second {
		t.Fatalf("duration = %v, want 1s", got)
	}
	snapshot := g.Snapshot(at(2 * time.Second))
	if g.Status() != Playing {
		t.Fatal("query finished the round")
	}
	snapshot.Words[0].Target = "changed"
	snapshot.Words[0].Typed[0] = 'x'
	unchanged := g.Snapshot(at(2 * time.Second))
	if unchanged.Words[0].Target != "cat" || string(unchanged.Words[0].Typed) != "c" {
		t.Fatal("snapshot mutation changed the game")
	}
	g.Handle(tick(time.Second))
	if g.Status() != Finished {
		t.Fatal("deadline tick did not finish the round")
	}
}
