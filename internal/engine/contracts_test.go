package engine

import (
	"testing"
	"time"
)

func TestNewConfigValidation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		config  Config
		words   int
		wantErr bool
	}{
		{"exact word count", Config{Mode: ModeWords, WordCount: 2}, 2, false},
		{"extra supplied words", Config{Mode: ModeWords, WordCount: 1}, 2, false},
		{"time", Config{Mode: ModeTime, Duration: time.Second}, 2, false},
		{"words ignore duration", Config{Mode: ModeWords, WordCount: 1, Duration: -time.Second}, 1, false},
		{"time ignores count", Config{Mode: ModeTime, Duration: time.Second, WordCount: -1}, 1, false},
		{"invalid mode", Config{Mode: -1}, 1, true},
		{"zero words", Config{Mode: ModeWords}, 1, true},
		{"negative words", Config{Mode: ModeWords, WordCount: -1}, 1, true},
		{"too few words", Config{Mode: ModeWords, WordCount: 2}, 1, true},
		{"empty time buffer", Config{Mode: ModeTime, Duration: time.Second}, 0, true},
		{"zero duration", Config{Mode: ModeTime}, 1, true},
		{"negative duration", Config{Mode: ModeTime, Duration: -time.Second}, 1, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			words := []string{"cat", "dog"}[:tt.words]
			if tt.wantErr {
				defer func() {
					if recover() == nil {
						t.Fatal("New() did not panic")
					}
				}()
				New(tt.config, words)
				return
			}
			want := tt.words
			if tt.config.Mode == ModeWords {
				want = tt.config.WordCount
			}
			if got := New(tt.config, words).RemainingWords(); got != want {
				t.Errorf("selected %d words, want %d", got, want)
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
