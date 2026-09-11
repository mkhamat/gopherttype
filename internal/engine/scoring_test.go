package engine

import (
	"testing"
	"time"
)

func TestScoreWord(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    string
		target   string
		complete wordScore
		partial  wordScore
	}{
		{
			name:     "empty input",
			target:   "cat",
			complete: wordScore{missed: 3},
		},
		{
			name: "empty words",
		},
		{
			name:     "empty target",
			input:    "x",
			complete: wordScore{rawCharacters: 1, extra: 1},
			partial:  wordScore{rawCharacters: 1, extra: 1},
		},
		{
			name:     "matching word",
			input:    "cat",
			target:   "cat",
			complete: wordScore{rawCharacters: 3, creditedCharacters: 3},
			partial:  wordScore{rawCharacters: 3, creditedCharacters: 3},
		},
		{
			name:     "mismatched characters are incorrect",
			input:    "cxt",
			target:   "cat",
			complete: wordScore{rawCharacters: 3, incorrect: 1},
			partial:  wordScore{rawCharacters: 3, incorrect: 1},
		},
		{
			name:     "matching literal spaces",
			input:    "cat ",
			target:   "cat ",
			complete: wordScore{rawCharacters: 4, creditedCharacters: 4},
			partial:  wordScore{rawCharacters: 4, creditedCharacters: 4},
		},
		{
			name:     "replacing a literal space is incorrect",
			input:    "catx",
			target:   "cat ",
			complete: wordScore{rawCharacters: 4, incorrect: 1},
			partial:  wordScore{rawCharacters: 4, incorrect: 1},
		},
		{
			name:     "a later space does not repair a mismatch",
			input:    "catx ",
			target:   "cat ",
			complete: wordScore{rawCharacters: 5, incorrect: 1, extra: 1},
			partial:  wordScore{rawCharacters: 5, incorrect: 1, extra: 1},
		},
		{
			name:     "literal spaces are ordinary characters",
			input:    "cxt ",
			target:   "cat ",
			complete: wordScore{rawCharacters: 4, incorrect: 1},
			partial:  wordScore{rawCharacters: 4, incorrect: 1},
		},
		{
			name:     "overflow never earns credit",
			input:    "catx",
			target:   "cat",
			complete: wordScore{rawCharacters: 4, extra: 1},
			partial:  wordScore{rawCharacters: 4, extra: 1},
		},
		{
			name:     "missing characters only count on completion",
			input:    "cx",
			target:   "cat",
			complete: wordScore{rawCharacters: 2, incorrect: 1, missed: 1},
			partial:  wordScore{rawCharacters: 2, incorrect: 1},
		},
		{
			name:     "correct prefix earns partial credit",
			input:    "ca",
			target:   "cat",
			complete: wordScore{rawCharacters: 2, missed: 1},
			partial:  wordScore{rawCharacters: 2, creditedCharacters: 2},
		},
		{
			name:     "untyped literal space counts as missed on completion",
			input:    "ca",
			target:   "cat ",
			complete: wordScore{rawCharacters: 2, missed: 2},
			partial:  wordScore{rawCharacters: 2, creditedCharacters: 2},
		},
		{
			name:     "prefix containing a literal space",
			input:    "ca ",
			target:   "ca t",
			complete: wordScore{rawCharacters: 3, missed: 1},
			partial:  wordScore{rawCharacters: 3, creditedCharacters: 3},
		},
		{
			name:     "characters are runes rather than bytes",
			input:    "猫",
			target:   "猫犬",
			complete: wordScore{rawCharacters: 1, missed: 1},
			partial:  wordScore{rawCharacters: 1, creditedCharacters: 1},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input, target := []rune(tt.input), []rune(tt.target)
			if got := scoreWord(input, target, requireCompleteWord); got != tt.complete {
				t.Errorf("complete score = %+v, want %+v", got, tt.complete)
			}
			if got := scoreWord(input, target, allowPartialWord); got != tt.partial {
				t.Errorf("partial score = %+v, want %+v", got, tt.partial)
			}
		})
	}
}

func TestSubmissionDoesNotReclassifyCharacters(t *testing.T) {
	for _, tt := range []struct {
		input  string
		before Metrics
		after  Metrics
	}{
		{
			input:  "catx",
			before: Metrics{Duration: time.Minute, Raw: 0.8, Accuracy: 75, Extra: 1},
			after:  Metrics{Duration: time.Minute, Raw: 1, Accuracy: 60, Extra: 1},
		},
		{
			input:  "cxt",
			before: Metrics{Duration: time.Minute, Raw: 0.6, Accuracy: 66.67, Incorrect: 1},
			after:  Metrics{Duration: time.Minute, Raw: 0.8, Accuracy: 50, Incorrect: 1},
		},
		{
			input:  "ca",
			before: Metrics{Duration: time.Minute, WPM: 0.4, Raw: 0.4, Accuracy: 100, Correct: 2},
			after:  Metrics{Duration: time.Minute, Raw: 0.6, Accuracy: 66.67, Missed: 1},
		},
		{
			input:  "cat",
			before: Metrics{Duration: time.Minute, WPM: 0.6, Raw: 0.6, Accuracy: 100, Correct: 3},
			after:  Metrics{Duration: time.Minute, WPM: 0.8, Raw: 0.8, Accuracy: 100, Correct: 4},
		},
	} {
		t.Run(tt.input, func(t *testing.T) {
			game := New(Config{Mode: ModeWords, WordCount: 2}, []string{"cat", "dog"})
			for _, r := range tt.input {
				game.Handle(typed(r, 0))
			}
			finalMetrics := func() Metrics {
				finished := *game
				finished.config.Mode = ModeTime
				finished.finish(at(time.Minute))
				return finished.FinalMetrics()
			}
			if got := finalMetrics(); got != tt.before {
				t.Fatalf("before submission = %+v, want %+v", got, tt.before)
			}

			game.Handle(space(time.Minute))
			if got := finalMetrics(); got != tt.after {
				t.Fatalf("after submission = %+v, want %+v", got, tt.after)
			}

			if tt.input != "cat" {
				game.Handle(backspace(time.Minute))
				want := tt.before
				want.Accuracy = tt.after.Accuracy
				if got := finalMetrics(); got != want {
					t.Fatalf("after reopening = %+v, want %+v", got, want)
				}
			}
		})
	}
}

func TestTypedSpaceCountsAsExtra(t *testing.T) {
	game := New(Config{Mode: ModeTime, Duration: time.Minute}, []string{"cat"})
	for _, r := range "cat " {
		game.Handle(typed(r, 0))
	}
	game.Handle(tick(time.Minute))

	want := Metrics{Duration: time.Minute, Raw: 0.8, Accuracy: 75, Extra: 1}
	if got := game.FinalMetrics(); got != want {
		t.Fatalf("FinalMetrics() = %+v, want %+v", got, want)
	}
}
