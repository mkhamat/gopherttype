package engine

import (
	"math"
	"time"
)

type Metrics struct {
	Duration  time.Duration
	WPM       float64
	Raw       float64
	Accuracy  float64
	Correct   int
	Incorrect int
	Extra     int
	Missed    int
}

func (g *Game) FinalMetrics() Metrics {
	if g.status != Finished {
		panic("final metrics require a finished game")
	}
	policy := requireCompleteWord
	if g.config.Mode == ModeTime {
		policy = allowPartialWord
	}
	duration := g.finishedAt.Sub(g.startedAt)
	var score wordScore
	for i := range g.current {
		score.add(g.scoreSubmittedWord(i))
	}
	if g.current < len(g.words) {
		word := &g.words[g.current]
		score.add(scoreWord(word.typedRunes, word.targetRunes, policy))
	}

	return Metrics{
		Duration:  duration,
		WPM:       roundedWPM(score.creditedCharacters, duration),
		Raw:       roundedWPM(score.rawCharacters, duration),
		Accuracy:  g.stats.accuracy(),
		Correct:   score.creditedCharacters,
		Incorrect: score.incorrect,
		Extra:     score.extra,
		Missed:    score.missed,
	}
}

func (s *stats) accuracy() float64 {
	totalAttempts := s.correctAttempts + s.incorrectAttempts
	return roundToTwo(float64(s.correctAttempts) / float64(totalAttempts) * 100)
}

func roundedWPM(characterCount int, duration time.Duration) float64 {
	seconds := duration.Seconds()
	if seconds <= 0 {
		return 0
	}
	const charactersPerWord = 5
	wordsPerMinute := float64(characterCount) / charactersPerWord / (seconds / 60)
	return roundToTwo(wordsPerMinute)
}

func roundToTwo(value float64) float64 {
	return math.Round(value*100) / 100
}
