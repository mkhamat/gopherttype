package engine

import (
	"reflect"
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

	if game.words[0].target != "cat" {
		t.Fatalf("target word changed to %q after caller mutated input", game.words[0].target)
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
	if got := string(game.words[0].typedRunes); got != "cxt!" {
		t.Fatalf("typed word = %q, want %q", got, "cxt!")
	}
	if game.stats.correctAttempts != 2 || game.stats.incorrectAttempts != 2 {
		t.Fatalf("counts = (%d correct, %d incorrect), want (2, 2)", game.stats.correctAttempts, game.stats.incorrectAttempts)
	}
}

func TestHandleTypeUsesRunes(t *testing.T) {
	game, err := New(Config{Mode: ModeWords, WordCount: 1}, []string{"é"})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	game.Handle(Event{Kind: Type, Rune: 'é', At: time.Now()})

	if game.stats.correctAttempts != 1 || game.stats.incorrectAttempts != 0 {
		t.Fatalf("counts = (%d correct, %d incorrect), want (1, 0)", game.stats.correctAttempts, game.stats.incorrectAttempts)
	}
}

func TestHandleIgnoresInputWhenFinished(t *testing.T) {
	game, err := New(Config{Mode: ModeWords, WordCount: 1}, []string{"cat"})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	game.status = Finished

	game.Handle(Event{Kind: Type, Rune: 'c', At: time.Now()})

	if len(game.words[0].typedRunes) != 0 || game.stats.correctAttempts != 0 || game.stats.incorrectAttempts != 0 {
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
	if got := string(game.words[0].typedRunes); got != "c" {
		t.Fatalf("typed word = %q, want deadline input to be ignored", got)
	}
	if got := game.FinalMetrics().Duration; got != 5*time.Second {
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

			if game.current != tt.wantIndex {
				t.Errorf("current word index = %d, want %d", game.current, tt.wantIndex)
			}
			gotTyped := make([]string, len(game.words))
			for i, word := range game.words {
				gotTyped[i] = string(word.typedRunes)
			}
			if !slices.Equal(gotTyped, tt.wantTyped) {
				t.Errorf("typed words = %q, want %q", gotTyped, tt.wantTyped)
			}
		})
	}
}

func TestAppendWordsValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config Config
		finish bool
		want   error
	}{
		{name: "words mode", config: Config{Mode: ModeWords, WordCount: 1}, want: ErrAppendWordsMode},
		{name: "finished time mode", config: Config{Mode: ModeTime, Duration: time.Minute}, finish: true, want: ErrGameFinished},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game, err := New(tt.config, []string{"cat"})
			if err != nil {
				t.Fatal(err)
			}
			if tt.finish {
				game.Handle(typed('c', 0))
				game.Handle(tick(time.Minute))
			}
			before := game.Snapshot(at(time.Minute))
			if err := game.AppendWords([]string{"dog"}); err != tt.want {
				t.Fatalf("AppendWords() error = %v, want %v", err, tt.want)
			}
			if after := game.Snapshot(at(time.Minute)); !reflect.DeepEqual(after, before) {
				t.Fatalf("rejected append changed snapshot: before %+v, after %+v", before, after)
			}
		})
	}
}

func TestAppendWordsAfterExhaustion(t *testing.T) {
	game, err := New(Config{Mode: ModeTime, Duration: time.Minute}, []string{"cat"})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []Event{typed('c', 0), typed('a', time.Second), typed('t', 2*time.Second), space(3 * time.Second)} {
		game.Handle(event)
	}
	if game.Status() != Playing || game.RemainingWords() != 0 {
		t.Fatalf("exhausted game: status %v, remaining %d", game.Status(), game.RemainingWords())
	}
	before := game.Snapshot(at(4 * time.Second))
	if err := game.AppendWords(nil); err != nil {
		t.Fatal(err)
	}
	if after := game.Snapshot(at(4 * time.Second)); !reflect.DeepEqual(after, before) {
		t.Fatal("empty append changed game")
	}
	words := []string{"dog"}
	if err := game.AppendWords(words); err != nil {
		t.Fatal(err)
	}
	words[0] = "changed"
	game.Handle(backspace(4 * time.Second))
	snapshot := game.Snapshot(at(4 * time.Second))
	if snapshot.CurrentWordIndex != 1 || snapshot.Words[1].Target != "dog" || game.RemainingWords() != 1 {
		t.Fatalf("unexpected state after append and backspace: %+v", snapshot)
	}
	for _, event := range []Event{typed('d', 5*time.Second), typed('o', 6*time.Second), typed('g', 7*time.Second), tick(time.Minute)} {
		game.Handle(event)
	}
	want := Metrics{Duration: time.Minute, WPM: 1.4, Raw: 1.4, Accuracy: 100, Correct: 7}
	if got := game.FinalMetrics(); got != want {
		t.Fatalf("FinalMetrics() = %+v, want %+v", got, want)
	}
}

func TestAppendWordsAndReopenIncorrectWord(t *testing.T) {
	for _, appendBefore := range []bool{true, false} {
		for _, kind := range []EventKind{Backspace, DeleteWord} {
			name := "append after reopen"
			if appendBefore {
				name = "append before reopen"
			}
			if kind == Backspace {
				name += "/backspace"
			} else {
				name += "/delete word"
			}
			t.Run(name, func(t *testing.T) {
				game, err := New(Config{Mode: ModeTime, Duration: time.Minute}, []string{"cat"})
				if err != nil {
					t.Fatal(err)
				}
				for _, event := range []Event{typed('c', 0), typed('x', time.Second), space(2 * time.Second)} {
					game.Handle(event)
				}
				if appendBefore {
					if err := game.AppendWords([]string{"dog"}); err != nil {
						t.Fatal(err)
					}
				}
				game.Handle(Event{Kind: kind, At: at(3 * time.Second)})
				snapshot := game.Snapshot(at(3 * time.Second))
				wantTyped := "cx"
				if kind == DeleteWord {
					wantTyped = ""
				}
				if snapshot.CurrentWordIndex != 0 || string(snapshot.Words[0].Typed) != wantTyped {
					t.Fatalf("unexpected reopened state: %+v", snapshot)
				}
				if !appendBefore {
					if err := game.AppendWords([]string{"dog"}); err != nil {
						t.Fatal(err)
					}
				}
				if kind == Backspace {
					game.Handle(deleteWord(4 * time.Second))
				}
				for _, event := range []Event{
					typed('c', 5*time.Second), typed('a', 6*time.Second), typed('t', 7*time.Second), space(8 * time.Second),
					typed('d', 9*time.Second), typed('o', 10*time.Second), typed('g', 11*time.Second), tick(time.Minute),
				} {
					game.Handle(event)
				}
				want := Metrics{Duration: time.Minute, WPM: 1.4, Raw: 1.4, Accuracy: 80, Correct: 7}
				if got := game.FinalMetrics(); got != want {
					t.Fatalf("FinalMetrics() = %+v, want %+v", got, want)
				}
			})
		}
	}
}
