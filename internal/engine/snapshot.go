package engine

import "time"

type WordSnapshot struct {
	Target string
	Typed  []rune
}

type Snapshot struct {
	CurrentWordIndex int
	Words            []WordSnapshot
	Elapsed          time.Duration
}

func (g *Game) Snapshot(at time.Time) Snapshot {
	return Snapshot{
		CurrentWordIndex: g.current,
		Words:            g.snapshotWords(),
		Elapsed:          g.ElapsedAt(at),
	}
}

func (g *Game) ElapsedAt(at time.Time) time.Duration {
	switch g.status {
	case Ready:
		return 0
	case Finished:
		return g.finishedAt.Sub(g.startedAt)
	}
	elapsed := at.Sub(g.startedAt)
	if g.config.Mode == ModeTime {
		return min(elapsed, g.config.Duration)
	}
	return elapsed
}

func (g *Game) snapshotWords() []WordSnapshot {
	typedRuneCount := 0
	for i := range g.words {
		typedRuneCount += len(g.words[i].typedRunes)
	}

	words := make([]WordSnapshot, len(g.words))
	var typedRunes []rune
	if typedRuneCount > 0 {
		typedRunes = make([]rune, typedRuneCount)
	}

	offset := 0
	for i := range g.words {
		word := &g.words[i]
		next := offset + copy(typedRunes[offset:], word.typedRunes)
		var typed []rune
		if next > offset {
			typed = typedRunes[offset:next:next]
		}
		words[i] = WordSnapshot{
			Target: word.target,
			Typed:  typed,
		}
		offset = next
	}
	return words
}
