package play

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/engine"
)

func TestSpaceKeySubmission(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"space", tea.KeyPressMsg{Code: tea.KeySpace}},
		{"space with text", tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}},
		{"shift space", tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModShift}},
		{"shift space with text", tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModShift, Text: " "}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 2}, []string{"cat", "dog"})
			at := m.playUI.updatedAt.Add(time.Second)
			if cmd := m.handlePlayKeyAt(tt.key, at); cmd != nil || m.game.Status() != engine.Ready {
				t.Fatal("space on an empty word started the round")
			}
			m.handlePlayKeyAt(tea.KeyPressMsg{Text: "cat"}, at)
			m.handlePlayKeyAt(tt.key, at.Add(time.Second))
			snapshot := m.playUI.snapshot
			if snapshot.CurrentWordIndex != 1 || string(snapshot.Words[0].Typed) != "cat" {
				t.Fatalf("space did not submit cleanly: %+v", snapshot)
			}
			cmd := m.handlePlayKeyAt(tea.KeyPressMsg{Text: "dog"}, at.Add(2*time.Second))
			assertFinished(t, m, cmd)
			if metrics := m.game.FinalMetrics(); metrics.Accuracy != 100 || metrics.Extra != 0 {
				t.Fatalf("space was scored as a mistake: %+v", metrics)
			}
		})
	}
}

func TestSpaceShortcutsDoNotSubmit(t *testing.T) {
	for _, mod := range []tea.KeyMod{tea.ModCtrl, tea.ModAlt, tea.ModCtrl | tea.ModShift} {
		key := tea.KeyPressMsg{Code: tea.KeySpace, Mod: mod}
		t.Run(key.String(), func(t *testing.T) {
			m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 2}, []string{"cat", "dog"})
			at := m.playUI.updatedAt.Add(time.Second)
			m.handlePlayKeyAt(tea.KeyPressMsg{Text: "cat"}, at)
			m.handlePlayKeyAt(key, at.Add(time.Second))
			if m.playUI.snapshot.CurrentWordIndex != 0 || string(m.playUI.snapshot.Words[0].Typed) != "cat" {
				t.Fatal("shortcut changed the current word")
			}
		})
	}
}

func TestInsufficientGeneratedWordsPanics(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config engine.Config
		words  []string
		want   error
	}{
		{"empty timed round", engine.Config{Mode: engine.ModeTime, Duration: time.Minute}, nil, engine.ErrNoWords},
		{"empty word round", engine.Config{Mode: engine.ModeWords, WordCount: 2}, nil, engine.ErrInvalidWordCount},
		{"short word round", engine.Config{Mode: engine.ModeWords, WordCount: 2}, []string{"cat"}, engine.ErrInvalidWordCount},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				err, ok := recover().(error)
				if !ok || !errors.Is(err, tt.want) {
					t.Fatalf("panic = %v, want %v", err, tt.want)
				}
			}()
			New(tt.config, func(int) []string { return tt.words })
		})
	}
}
