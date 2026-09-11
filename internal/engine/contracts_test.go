package engine

import (
	"testing"
	"time"
)

func TestNewConfigValidation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{"words", Config{Mode: ModeWords, WordCount: 1}, false},
		{"time", Config{Mode: ModeTime, Duration: time.Second}, false},
		{"words ignore duration", Config{Mode: ModeWords, WordCount: 1, Duration: -time.Second}, false},
		{"time ignores count", Config{Mode: ModeTime, Duration: time.Second, WordCount: -1}, false},
		{"invalid mode", Config{Mode: -1}, true},
		{"zero words", Config{Mode: ModeWords}, true},
		{"negative words", Config{Mode: ModeWords, WordCount: -1}, true},
		{"zero duration", Config{Mode: ModeTime}, true},
		{"negative duration", Config{Mode: ModeTime, Duration: -time.Second}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				defer func() {
					if recover() == nil {
						t.Fatal("New() did not panic")
					}
				}()
				New(tt.config, []string{"cat"})
				return
			}
			New(tt.config, []string{"cat"})
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
			g := New(Config{Mode: ModeTime, Duration: time.Second}, []string{"cat"})
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
	g := New(Config{Mode: ModeTime, Duration: time.Second}, []string{"cat"})
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
