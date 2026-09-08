package home

import (
	"time"

	"gopherttype/internal/engine"
)

var (
	wordPresets     = [...]int{10, 25, 50, 100}
	durationPresets = [...]time.Duration{15 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second}
)

type settings struct {
	mode          engine.Mode
	wordIndex     int
	durationIndex int
}

func defaultSettings() settings {
	return settings{mode: engine.ModeTime, durationIndex: 1}
}

func (s settings) config() engine.Config {
	if s.mode == engine.ModeTime {
		return engine.Config{Mode: s.mode, Duration: durationPresets[s.durationIndex]}
	}
	return engine.Config{Mode: s.mode, WordCount: wordPresets[s.wordIndex]}
}
