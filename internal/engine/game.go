package engine

import (
	"errors"
	"slices"
	"time"
)

type Mode int

type Status int

const (
	ModeTime Mode = iota
	ModeWords
)

const (
	Ready Status = iota
	Playing
	Finished
)

var (
	ErrInvalidMode      = errors.New("invalid game mode")
	ErrInvalidDuration  = errors.New("duration must be positive in time mode")
	ErrInvalidWordCount = errors.New("word count must be positive and no greater than the supplied words")
	ErrNoWords          = errors.New("at least one word is required")
	ErrAppendWordsMode  = errors.New("words can only be appended in time mode")
	ErrGameFinished     = errors.New("game is finished")
)

type Config struct {
	Mode      Mode
	Duration  time.Duration
	WordCount int
}

func (c Config) Validate() error {
	switch c.Mode {
	case ModeWords:
		if c.WordCount <= 0 {
			return ErrInvalidWordCount
		}
	case ModeTime:
		if c.Duration <= 0 {
			return ErrInvalidDuration
		}
	default:
		return ErrInvalidMode
	}
	return nil
}

type word struct {
	target      string
	targetRunes []rune
	typedRunes  []rune
}

type stats struct {
	submitted         wordScore
	correctAttempts   int
	incorrectAttempts int
}

func (s *stats) recordAttempt(correct bool) {
	if correct {
		s.correctAttempts++
	} else {
		s.incorrectAttempts++
	}
}

type Game struct {
	config     Config
	status     Status
	startedAt  time.Time
	finishedAt time.Time
	words      []word
	current    int
	stats      stats
}

func New(config Config, words []string) (*Game, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	var selected []string

	switch config.Mode {
	case ModeWords:
		if config.WordCount > len(words) {
			return nil, ErrInvalidWordCount
		}
		selected = words[:config.WordCount]
	case ModeTime:
		if len(words) == 0 {
			return nil, ErrNoWords
		}
		selected = words
	}

	gameWords := make([]word, len(selected))
	for i, target := range selected {
		gameWords[i] = makeWord(target)
	}

	return &Game{
		config: config,
		status: Ready,
		words:  gameWords,
	}, nil
}

func makeWord(target string) word {
	targetRunes := []rune(target)
	return word{
		target:      target,
		targetRunes: targetRunes,
		typedRunes:  make([]rune, 0, len(targetRunes)),
	}
}

func (g *Game) AppendWords(words []string) error {
	if g.config.Mode != ModeTime {
		return ErrAppendWordsMode
	}
	if g.status == Finished {
		return ErrGameFinished
	}
	if len(words) == 0 {
		return nil
	}

	g.words = slices.Grow(g.words, len(words))
	for _, target := range words {
		g.words = append(g.words, makeWord(target))
	}
	return nil
}

func (g *Game) Handle(event Event) {
	if g.status == Finished {
		return
	}

	if g.status == Playing && g.config.Mode == ModeTime {
		deadline := g.startedAt.Add(g.config.Duration)
		if !event.At.Before(deadline) {
			g.finish(deadline)
			return
		}
	}

	switch event.Kind {
	case Type:
		if event.Rune == 0 {
			return
		}
		g.handleType(event.Rune, event.At)
	case Space:
		g.handleSpace(event.At)
	case Backspace:
		g.handleBackspace()
	case DeleteWord:
		g.handleDeleteWord()
	case Tick:
	}
}

func (g *Game) handleType(r rune, at time.Time) {
	if g.current >= len(g.words) {
		return
	}
	if g.status == Ready {
		g.status = Playing
		g.startedAt = at
	}

	word := &g.words[g.current]
	position := len(word.typedRunes)
	word.typedRunes = append(word.typedRunes, r)
	correct := position < len(word.targetRunes) && word.targetRunes[position] == r
	g.stats.recordAttempt(correct)

	isLastWord := g.current == len(g.words)-1
	if g.config.Mode == ModeWords && isLastWord && slices.Equal(word.typedRunes, word.targetRunes) {
		g.finish(at)
	}
}

func (g *Game) handleSpace(at time.Time) {
	if g.current >= len(g.words) || len(g.words[g.current].typedRunes) == 0 {
		return
	}

	word := &g.words[g.current]
	hasSeparator := g.wordHasSeparator(g.current)
	correct := hasSeparator && slices.Equal(word.typedRunes, word.targetRunes)
	g.stats.recordAttempt(correct)

	if g.config.Mode == ModeWords && g.current == len(g.words)-1 {
		g.finish(at)
		return
	}

	g.stats.submitted.add(g.scoreSubmittedWord(g.current))
	g.current++
}

func (g *Game) handleBackspace() {
	if g.current < len(g.words) && len(g.words[g.current].typedRunes) > 0 {
		word := &g.words[g.current]
		word.typedRunes = word.typedRunes[:len(word.typedRunes)-1]
		return
	}
	g.reopenPreviousWord()
}

func (g *Game) handleDeleteWord() {
	if g.current < len(g.words) && len(g.words[g.current].typedRunes) > 0 {
		g.words[g.current].typedRunes = g.words[g.current].typedRunes[:0]
		return
	}
	if g.reopenPreviousWord() {
		g.words[g.current].typedRunes = g.words[g.current].typedRunes[:0]
	}
}

func (g *Game) reopenPreviousWord() bool {
	if g.current == 0 {
		return false
	}

	previous := g.current - 1
	word := &g.words[previous]
	if slices.Equal(word.typedRunes, word.targetRunes) {
		return false
	}

	g.stats.submitted.subtract(g.scoreSubmittedWord(previous))
	g.current = previous
	return true
}

func (g *Game) wordHasSeparator(current int) bool {
	return g.config.Mode == ModeTime || current < len(g.words)-1
}

func (g *Game) finish(at time.Time) {
	g.status = Finished
	g.finishedAt = at
}

func (g *Game) Status() Status {
	return g.status
}

func (g *Game) RemainingWords() int {
	if g.status == Finished {
		return 0
	}
	return max(len(g.words)-g.current, 0)
}
