package engine

import (
	"testing"
	"time"
)

var testStart = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

func at(d time.Duration) time.Time { return testStart.Add(d) }

func typed(r rune, d time.Duration) Event { return Event{Kind: Type, Rune: r, At: at(d)} }

func space(d time.Duration) Event { return Event{Kind: Space, At: at(d)} }

func backspace(d time.Duration) Event { return Event{Kind: Backspace, At: at(d)} }

func deleteWord(d time.Duration) Event { return Event{Kind: DeleteWord, At: at(d)} }

func tick(d time.Duration) Event { return Event{Kind: Tick, At: at(d)} }

func TestFinalMetrics(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		words  []string
		events []Event
		want   Metrics
	}{
		{
			name:   "perfect run",
			config: Config{Mode: ModeWords, WordCount: 1},
			words:  []string{"cat"},
			events: []Event{typed('c', 0), typed('a', time.Second), typed('t', 2*time.Second)},
			want:   Metrics{Duration: 2 * time.Second, WPM: 18, Raw: 18, Accuracy: 100, Correct: 3},
		},
		{
			name:   "corrected typo costs accuracy but not wpm credit",
			config: Config{Mode: ModeWords, WordCount: 1},
			words:  []string{"cat"},
			events: []Event{
				typed('c', 0), typed('x', time.Second), backspace(2 * time.Second),
				typed('a', 3*time.Second), typed('t', 4*time.Second),
			},
			want: Metrics{Duration: 4 * time.Second, WPM: 9, Raw: 9, Accuracy: 75, Correct: 3},
		},
		{
			name:   "wrong submitted word is missed and not credited",
			config: Config{Mode: ModeWords, WordCount: 2},
			words:  []string{"cat", "dog"},
			events: []Event{
				typed('c', 0), typed('a', time.Second), typed('t', 2*time.Second), space(3 * time.Second),
				typed('x', 4*time.Second), space(5 * time.Second),
			},
			want: Metrics{Duration: 5 * time.Second, WPM: 9.6, Raw: 12, Accuracy: 66.67, Correct: 4, Incorrect: 1, Missed: 2},
		},
		{
			name:   "same-length wrong submission remains an error after repair",
			config: Config{Mode: ModeWords, WordCount: 2},
			words:  []string{"cat", "dog"},
			events: []Event{
				typed('c', 0), typed('x', time.Second), typed('t', 2*time.Second), space(3 * time.Second),
				backspace(4 * time.Second), backspace(5 * time.Second), backspace(6 * time.Second),
				typed('a', 7*time.Second), typed('t', 8*time.Second), space(9 * time.Second),
				typed('d', 10*time.Second), typed('o', 11*time.Second), typed('g', 12*time.Second),
			},
			want: Metrics{Duration: 12 * time.Second, WPM: 7, Raw: 7, Accuracy: 80, Correct: 7},
		},
		{
			name:   "word mode does not credit a partial final word",
			config: Config{Mode: ModeWords, WordCount: 2},
			words:  []string{"cat", "dog"},
			events: []Event{
				typed('c', 0), typed('a', time.Second), typed('t', 2*time.Second), space(3 * time.Second),
				typed('d', 4*time.Second), typed('o', 5*time.Second), space(6 * time.Second),
			},
			want: Metrics{Duration: 6 * time.Second, WPM: 8, Raw: 12, Accuracy: 85.71, Correct: 4, Missed: 1},
		},
		{
			name:   "time mode credits the correct prefix of the active word at timeout",
			config: Config{Mode: ModeTime, Duration: 5 * time.Second},
			words:  []string{"cat"},
			events: []Event{typed('c', 0), typed('a', time.Second), tick(5 * time.Second)},
			want:   Metrics{Duration: 5 * time.Second, WPM: 4.8, Raw: 4.8, Accuracy: 100, Correct: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := New(tt.config, tt.words)
			for _, event := range tt.events {
				game.Handle(event)
			}

			if got := game.FinalMetrics(); got != tt.want {
				t.Errorf("FinalMetrics() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
