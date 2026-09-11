package engine

type scoringPolicy int

const (
	requireCompleteWord scoringPolicy = iota
	allowPartialWord
)

type wordScore struct {
	creditedCharacters int
	rawCharacters      int
	incorrect          int
	extra              int
	missed             int
}

func scoreWord(input, target []rune, policy scoringPolicy) wordScore {
	score := wordScore{
		rawCharacters: len(input),
		extra:         max(len(input)-len(target), 0),
	}
	for position := 0; position < min(len(input), len(target)); position++ {
		if input[position] != target[position] {
			score.incorrect++
		}
	}

	allTypedCharactersMatch := score.incorrect == 0 && score.extra == 0
	if allTypedCharactersMatch && (len(input) == len(target) || policy == allowPartialWord) {
		score.creditedCharacters = len(input)
	}
	if policy == requireCompleteWord {
		score.missed = max(len(target)-len(input), 0)
	}
	return score
}

func (s *wordScore) add(other wordScore) {
	s.creditedCharacters += other.creditedCharacters
	s.rawCharacters += other.rawCharacters
	s.incorrect += other.incorrect
	s.extra += other.extra
	s.missed += other.missed
}

func (g *Game) scoreSubmittedWord(index int) wordScore {
	word := &g.words[index]
	score := scoreWord(word.typedRunes, word.targetRunes, requireCompleteWord)
	score.rawCharacters++
	if score.incorrect == 0 && len(word.typedRunes) == len(word.targetRunes) {
		score.creditedCharacters++
	}
	return score
}
