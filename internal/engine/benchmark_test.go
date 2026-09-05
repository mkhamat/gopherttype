package engine

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

var benchmarkStart = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

func BenchmarkHandleType(b *testing.B) {
	game, err := New(Config{Mode: ModeWords, WordCount: 1}, []string{strings.Repeat("a", 64)})
	if err != nil {
		b.Fatal(err)
	}
	event := Event{Kind: Type, Rune: 'x', At: benchmarkStart}
	backspaceEvent := Event{Kind: Backspace, At: benchmarkStart}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		game.Handle(event)
		game.Handle(backspaceEvent)
	}
}

func BenchmarkSnapshot(b *testing.B) {
	for _, wordCount := range []int{10, 1000} {
		b.Run(strconv.Itoa(wordCount)+"Words", func(b *testing.B) {
			game := benchmarkPlayingGame(b, wordCount)
			at := benchmarkStart.Add(time.Minute)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = game.Snapshot(at)
			}
		})
	}
}

func BenchmarkFinalMetrics1000Words(b *testing.B) {
	game := benchmarkFinishedGame(b, 1000)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = game.FinalMetrics()
	}
}

func benchmarkPlayingGame(b *testing.B, wordCount int) *Game {
	b.Helper()
	game := benchmarkGame(b, wordCount)
	for i := 0; i < wordCount-1; i++ {
		game.Handle(Event{Kind: Type, Rune: 'c', At: benchmarkStart})
		game.Handle(Event{Kind: Type, Rune: 'a', At: benchmarkStart})
		game.Handle(Event{Kind: Type, Rune: 't', At: benchmarkStart})
		game.Handle(Event{Kind: Space, At: benchmarkStart})
	}
	game.Handle(Event{Kind: Type, Rune: 'c', At: benchmarkStart})
	game.Handle(Event{Kind: Type, Rune: 'a', At: benchmarkStart})
	return game
}

func benchmarkFinishedGame(b *testing.B, wordCount int) *Game {
	b.Helper()
	game := benchmarkGame(b, wordCount)
	for i := range wordCount {
		game.Handle(Event{Kind: Type, Rune: 'c', At: benchmarkStart})
		game.Handle(Event{Kind: Type, Rune: 'a', At: benchmarkStart})
		game.Handle(Event{Kind: Type, Rune: 't', At: benchmarkStart})
		if i < wordCount-1 {
			game.Handle(Event{Kind: Space, At: benchmarkStart})
		}
	}
	return game
}

func benchmarkGame(b *testing.B, wordCount int) *Game {
	b.Helper()
	words := make([]string, wordCount)
	for i := range words {
		words[i] = "cat"
	}
	game, err := New(Config{Mode: ModeWords, WordCount: wordCount}, words)
	if err != nil {
		b.Fatal(err)
	}
	return game
}
