package words

import (
	_ "embed"
	"math/rand/v2"
	"strings"
)

//go:embed english.txt
var wordList string

type Generator struct {
	rng   *rand.Rand
	words []string
}

func New(seed1 uint64, seed2 uint64) *Generator {
	words := strings.Fields(wordList)
	rng := rand.New(rand.NewPCG(seed1, seed2))
	return &Generator{
		rng:   rng,
		words: words,
	}
}

func (g *Generator) Generate(count int) []string {
	words := make([]string, count)

	for i := range words {
		words[i] = g.words[g.rng.IntN(len(g.words))]
	}

	return words
}
