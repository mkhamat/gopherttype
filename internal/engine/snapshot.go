package engine

import "time"

type WordSnapshot struct {
	Target string
	Typed  []rune
}

type Snapshot struct {
	Status           Status
	CurrentWordIndex int
	Words            []WordSnapshot
	Metrics          Metrics
}

func (g *Game) Snapshot(at time.Time) Snapshot {
	return Snapshot{
		Status:           g.status,
		CurrentWordIndex: g.current,
		Words:            g.snapshotWords(),
		Metrics:          g.metricsAt(at),
	}
}

// it's weird, but it basically copies each g.word[i].typedRunes to
// WordSnapshot.Typed doing only two allocations
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
