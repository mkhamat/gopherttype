package engine

import (
	"slices"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		words   []string
		wantErr bool
	}{
		{
			name:    "valid words mode",
			config:  Config{Mode: ModeWords, WordCount: 2},
			words:   []string{"cat", "dog", "bird"},
			wantErr: false,
		},
		{
			name:    "exactly enough target words",
			config:  Config{Mode: ModeWords, WordCount: 2},
			words:   []string{"cat", "dog"},
			wantErr: false,
		},
		{
			name:    "more target words than requested",
			config:  Config{Mode: ModeWords, WordCount: 1},
			words:   []string{"cat", "dog"},
			wantErr: false,
		},
		{
			name:    "zero word count rejected",
			config:  Config{Mode: ModeWords, WordCount: 0},
			words:   []string{"cat"},
			wantErr: true,
		},
		{
			name:    "negative word count rejected",
			config:  Config{Mode: ModeWords, WordCount: -1},
			words:   []string{"cat"},
			wantErr: true,
		},
		{
			name:    "too few target words rejected",
			config:  Config{Mode: ModeWords, WordCount: 2},
			words:   []string{"cat"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config, tt.words)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCopiesTargetWords(t *testing.T) {
	words := []string{"cat", "dog"}
	game, err := New(Config{Mode: ModeWords, WordCount: 2}, words)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	words[0] = "changed"

	if game.targetWords[0] != "cat" {
		t.Fatalf("target word changed to %q after caller mutated input", game.targetWords[0])
	}
}

func TestHandleTypeTracksInputAndCounts(t *testing.T) {
	game, err := New(Config{Mode: ModeWords, WordCount: 1}, []string{"cat"})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	startedAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	events := []Event{
		{Kind: Type, Rune: 'c', At: startedAt},
		{Kind: Type, Rune: 'x', At: startedAt.Add(time.Second)},
		{Kind: Type, Rune: 't', At: startedAt.Add(2 * time.Second)},
		{Kind: Type, Rune: '!', At: startedAt.Add(3 * time.Second)},
	}
	for _, event := range events {
		game.Handle(event)
	}

	if game.status != Playing {
		t.Fatalf("status = %v, want Playing", game.status)
	}
	if !game.startedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", game.startedAt, startedAt)
	}
	if got := string(game.typedWords[0]); got != "cxt!" {
		t.Fatalf("typed word = %q, want %q", got, "cxt!")
	}
	if game.correctCount != 2 || game.incorrectCount != 2 {
		t.Fatalf("counts = (%d correct, %d incorrect), want (2, 2)", game.correctCount, game.incorrectCount)
	}
}

func TestHandleTypeUsesRunes(t *testing.T) {
	game, err := New(Config{Mode: ModeWords, WordCount: 1}, []string{"é"})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	game.Handle(Event{Kind: Type, Rune: 'é', At: time.Now()})

	if game.correctCount != 1 || game.incorrectCount != 0 {
		t.Fatalf("counts = (%d correct, %d incorrect), want (1, 0)", game.correctCount, game.incorrectCount)
	}
}

func TestHandleIgnoresInputWhenFinished(t *testing.T) {
	game, err := New(Config{Mode: ModeWords, WordCount: 1}, []string{"cat"})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	game.status = Finished

	game.Handle(Event{Kind: Type, Rune: 'c', At: time.Now()})

	if len(game.typedWords[0]) != 0 || game.correctCount != 0 || game.incorrectCount != 0 {
		t.Fatal("finished game accepted typed input")
	}
}

func TestTimeModeFinishesAtExactDeadline(t *testing.T) {
	game, err := New(Config{Mode: ModeTime, Duration: 5 * time.Second}, []string{"cat"})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	game.Handle(typed('c', 0))
	game.Handle(tick(5*time.Second - time.Nanosecond))
	if game.status != Playing {
		t.Fatalf("status before deadline = %v, want Playing", game.status)
	}

	game.Handle(typed('a', 5*time.Second))
	if game.status != Finished {
		t.Fatalf("status at deadline = %v, want Finished", game.status)
	}
	if got := string(game.typedWords[0]); got != "c" {
		t.Fatalf("typed word = %q, want deadline input to be ignored", got)
	}
	if got := game.Result().Duration; got != 5*time.Second {
		t.Fatalf("duration = %v, want %v", got, 5*time.Second)
	}
}

func TestHandleNavigation(t *testing.T) {
	tests := []struct {
		name      string
		words     []string
		events    []Event
		wantIndex int
		wantTyped []string
	}{
		{
			name:      "space on an empty word is ignored",
			words:     []string{"cat", "dog"},
			events:    []Event{space(0)},
			wantIndex: 0,
			wantTyped: []string{"", ""},
		},
		{
			name:      "backspace at the start is ignored",
			words:     []string{"cat"},
			events:    []Event{backspace(0)},
			wantIndex: 0,
			wantTyped: []string{""},
		},
		{
			name:      "backspace pops the last rune",
			words:     []string{"cat"},
			events:    []Event{typed('c', 0), typed('x', time.Second), backspace(2 * time.Second)},
			wantIndex: 0,
			wantTyped: []string{"c"},
		},
		{
			name:      "space submits the current word",
			words:     []string{"cat", "dog"},
			events:    []Event{typed('c', 0), typed('a', time.Second), typed('t', 2*time.Second), space(3 * time.Second)},
			wantIndex: 1,
			wantTyped: []string{"cat", ""},
		},
		{
			name:      "backspace stays when the previous word is correct",
			words:     []string{"cat", "dog"},
			events:    []Event{typed('c', 0), typed('a', time.Second), typed('t', 2*time.Second), space(3 * time.Second), backspace(4 * time.Second)},
			wantIndex: 1,
			wantTyped: []string{"cat", ""},
		},
		{
			name:      "backspace returns into an incorrect previous word",
			words:     []string{"cat", "dog"},
			events:    []Event{typed('c', 0), typed('x', time.Second), space(2 * time.Second), backspace(3 * time.Second)},
			wantIndex: 0,
			wantTyped: []string{"cx", ""},
		},
		{
			name:      "delete word clears the incorrect previous word",
			words:     []string{"cat", "dog"},
			events:    []Event{typed('c', 0), typed('x', time.Second), space(2 * time.Second), deleteWord(3 * time.Second)},
			wantIndex: 0,
			wantTyped: []string{"", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game, err := New(Config{Mode: ModeWords, WordCount: len(tt.words)}, tt.words)
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}
			for _, event := range tt.events {
				game.Handle(event)
			}

			if game.currentWordIndex != tt.wantIndex {
				t.Errorf("currentWordIndex = %d, want %d", game.currentWordIndex, tt.wantIndex)
			}
			gotTyped := make([]string, len(game.typedWords))
			for i, w := range game.typedWords {
				gotTyped[i] = string(w)
			}
			if !slices.Equal(gotTyped, tt.wantTyped) {
				t.Errorf("typed words = %q, want %q", gotTyped, tt.wantTyped)
			}
		})
	}
}
