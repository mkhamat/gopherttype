package engine

import (
	"math"
	"slices"
	"time"
)

type Result struct {
	Duration  time.Duration
	WPM       float64
	Raw       float64
	Accuracy  float64
	Correct   int
	Incorrect int
	Extra     int
	Missed    int
}

type WordSnapshot struct {
	Target string
	Typed  []rune
}

type Snapshot struct {
	Status           Status
	CurrentWordIndex int
	Words            []WordSnapshot
	Result           Result
}

type characterCounts struct {
	positionallyCorrect int
	wpmCredit           int
	incorrect           int
	extra               int
	missed              int
}

func (g *Game) Result() Result {
	if g.status != Finished {
		return Result{}
	}
	creditActiveWord := g.config.Mode == ModeTime
	return g.calculate(g.finishedAt.Sub(g.startedAt), creditActiveWord)
}

func (g *Game) snapshotMetrics(at time.Time) Result {
	if g.status == Ready {
		return Result{}
	}
	if g.status == Finished {
		return g.Result()
	}

	duration := at.Sub(g.startedAt)
	if g.config.Mode == ModeTime && duration > g.config.Duration {
		duration = g.config.Duration
	}
	return g.calculate(duration, true)
}

func (g *Game) Snapshot(at time.Time) Snapshot {
	words := make([]WordSnapshot, len(g.targetWords))
	for i, target := range g.targetWords {
		words[i] = WordSnapshot{
			Target: target,
			Typed:  slices.Clone(g.typedWords[i]),
		}
	}

	return Snapshot{
		Status:           g.status,
		CurrentWordIndex: g.currentWordIndex,
		Words:            words,
		Result:           g.snapshotMetrics(at),
	}
}

func (g *Game) calculate(duration time.Duration, creditActiveWord bool) Result {
	if duration < 0 {
		duration = 0
	}

	accuracy := 0.0
	totalAttempts := g.correctCount + g.incorrectCount
	if totalAttempts > 0 {
		accuracy = roundToTwo(float64(g.correctCount) / float64(totalAttempts) * 100)
	}

	total := characterCounts{}
	if len(g.targetWords) > 0 {
		lastWordIndex := min(g.currentWordIndex, len(g.targetWords)-1)
		for wordIndex := 0; wordIndex <= lastWordIndex; wordIndex++ {
			input := slices.Clone(g.typedWords[wordIndex])
			target := []rune(g.targetWords[wordIndex])
			wordWasSubmitted := wordIndex < g.currentWordIndex
			wordHasSeparator := g.config.Mode == ModeTime || wordIndex < len(g.targetWords)-1

			if wordWasSubmitted && wordHasSeparator {
				input = append(input, ' ')
			}
			if wordHasSeparator {
				target = append(target, ' ')
			}

			creditPartialWord := creditActiveWord && wordIndex == g.currentWordIndex
			word := classifyCharacters(input, target, creditPartialWord)
			total.positionallyCorrect += word.positionallyCorrect
			total.wpmCredit += word.wpmCredit
			total.incorrect += word.incorrect
			total.extra += word.extra
			total.missed += word.missed
		}
	}

	result := Result{
		Duration:  duration,
		Accuracy:  accuracy,
		Correct:   total.wpmCredit,
		Incorrect: total.incorrect,
		Extra:     total.extra,
		Missed:    total.missed,
	}

	seconds := duration.Seconds()
	if seconds > 0 {
		rawCharacterCount := total.positionallyCorrect + total.incorrect + total.extra
		result.WPM = roundToTwo(calculateWPM(total.wpmCredit, seconds))
		result.Raw = roundToTwo(calculateWPM(rawCharacterCount, seconds))
	}
	return result
}

func classifyCharacters(input, target []rune, creditPartialWord bool) characterCounts {
	counts := characterCounts{}
	wordIsCorrect := slices.Equal(input, target)
	inputIsCorrectPrefix := len(input) <= len(target) && slices.Equal(input, target[:len(input)])
	inputContainsSpace := slices.Contains(input, ' ')

	for position := 0; position < max(len(input), len(target)); position++ {
		hasInput := position < len(input)
		hasTarget := position < len(target)

		switch {
		case hasInput && hasTarget && input[position] == target[position]:
			if target[position] == ' ' && !wordIsCorrect {
				counts.extra++
			} else {
				counts.positionallyCorrect++
			}
			if wordIsCorrect || (creditPartialWord && inputIsCorrectPrefix) {
				counts.wpmCredit++
			}
		case !hasInput:
			if !creditPartialWord {
				counts.missed++
			}
		case !hasTarget:
			counts.extra++
		case target[position] == ' ' && input[position] != ' ' && !inputContainsSpace:
			counts.extra++
		default:
			counts.incorrect++
		}
	}
	return counts
}

func calculateWPM(characterCount int, durationSeconds float64) float64 {
	if durationSeconds <= 0 {
		return 0
	}
	return float64(characterCount) / 5 / (durationSeconds / 60)
}

func roundToTwo(value float64) float64 {
	return math.Round(value*100) / 100
}
