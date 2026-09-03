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

type Game struct {
	config           Config
	status           Status
	startedAt        time.Time
	finishedAt       time.Time
	targetWords      []string
	currentWordIndex int
	typedWords       [][]rune
	correctCount     int
	incorrectCount   int
}

func New(config Config, words []string) (*Game, error) {
	var selected []string

	switch config.Mode {
	case ModeWords:
		if config.WordCount <= 0 || config.WordCount > len(words) {
			return nil, ErrInvalidWordCount
		}
		selected = words[:config.WordCount]
	case ModeTime:
		if config.Duration <= 0 {
			return nil, ErrInvalidDuration
		}
		if len(words) == 0 {
			return nil, ErrNoWords
		}
		selected = words
	default:
		return nil, ErrInvalidMode
	}

	targetWords := slices.Clone(selected)
	return &Game{
		config:      config,
		targetWords: targetWords,
		typedWords:  make([][]rune, len(targetWords)),
		status:      Ready,
	}, nil
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

	g.targetWords = append(g.targetWords, words...)
	g.typedWords = append(g.typedWords, make([][]rune, len(words))...)
	return nil
}

func (g *Game) Handle(event Event) {
	if g.status == Finished {
		return
	}

	if g.status == Playing {
		if g.config.Mode == ModeTime {
			deadline := g.startedAt.Add(g.config.Duration)
			if !event.At.Before(deadline) {
				g.finish(deadline)
				return
			}
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
	if g.currentWordIndex >= len(g.targetWords) {
		return
	}
	if g.status == Ready {
		g.status = Playing
		g.startedAt = at
	}

	i := g.currentWordIndex
	position := len(g.typedWords[i])
	target := []rune(g.targetWords[i])
	g.typedWords[i] = append(g.typedWords[i], r)

	if position < len(target) && target[position] == r {
		g.correctCount++
	} else {
		g.incorrectCount++
	}

	if g.config.Mode == ModeWords && i == len(g.targetWords)-1 && slices.Equal(g.typedWords[i], target) {
		g.finish(at)
	}
}

func (g *Game) handleSpace(at time.Time) {
	i := g.currentWordIndex
	if i >= len(g.typedWords) || len(g.typedWords[i]) == 0 {
		return
	}

	hasSeparator := g.config.Mode == ModeTime || i < len(g.targetWords)-1
	if hasSeparator && slices.Equal(g.typedWords[i], []rune(g.targetWords[i])) {
		g.correctCount++
	} else {
		g.incorrectCount++
	}
	if g.config.Mode == ModeWords && i == len(g.targetWords)-1 {
		g.finish(at)
		return
	}
	g.currentWordIndex++
}

func (g *Game) handleBackspace() {
	i := g.currentWordIndex
	if i < len(g.typedWords) && len(g.typedWords[i]) > 0 {
		g.typedWords[i] = g.typedWords[i][:len(g.typedWords[i])-1]
		return
	}
	if i == 0 {
		return
	}

	previous := i - 1
	if slices.Equal(g.typedWords[previous], []rune(g.targetWords[previous])) {
		return
	}
	g.currentWordIndex = previous
}

func (g *Game) handleDeleteWord() {
	i := g.currentWordIndex
	if i < len(g.typedWords) && len(g.typedWords[i]) > 0 {
		g.typedWords[i] = nil
		return
	}
	if i == 0 {
		return
	}

	previous := i - 1
	if slices.Equal(g.typedWords[previous], []rune(g.targetWords[previous])) {
		return
	}
	g.currentWordIndex = previous
	g.typedWords[previous] = nil
}

func (g *Game) finish(at time.Time) {
	g.status = Finished
	g.finishedAt = at
}

func (g *Game) RemainingWords() int {
	if g.status == Finished {
		return 0
	}
	return max(len(g.targetWords)-g.currentWordIndex, 0)
}
