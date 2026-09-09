package play

import (
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
			at := m.playUI.at.Add(time.Second)
			if cmd := m.handlePlayKeyAt(tt.key, at); cmd != nil || m.game.Status() != engine.Ready {
				t.Fatal("space on an empty word started the round")
			}
			m.handlePlayKeyAt(tea.KeyPressMsg{Text: "cat"}, at)
			m.handlePlayKeyAt(tt.key, at.Add(time.Second))
			snapshot := m.playUI.snapshot
			if snapshot.CurrentWordIndex != 1 || string(snapshot.Words[0].Typed) != "cat" {
				t.Fatalf("space did not submit cleanly: %+v", snapshot)
			}
			if snapshot.Metrics.Accuracy != 100 || snapshot.Metrics.Extra != 0 {
				t.Fatalf("space was scored as a mistake: %+v", snapshot.Metrics)
			}
			m.handlePlayKeyAt(tea.KeyPressMsg{Text: "dog"}, at.Add(2*time.Second))
			if m.screen != results || m.game.FinalMetrics().Accuracy != 100 {
				t.Fatal("typing after space did not complete the next word")
			}
		})
	}
}

func TestSpaceShortcutsDoNotSubmit(t *testing.T) {
	for _, mod := range []tea.KeyMod{tea.ModCtrl, tea.ModAlt, tea.ModCtrl | tea.ModShift} {
		key := tea.KeyPressMsg{Code: tea.KeySpace, Mod: mod}
		t.Run(key.String(), func(t *testing.T) {
			m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 2}, []string{"cat", "dog"})
			at := m.playUI.at.Add(time.Second)
			m.handlePlayKeyAt(tea.KeyPressMsg{Text: "cat"}, at)
			m.handlePlayKeyAt(key, at.Add(time.Second))
			if m.playUI.snapshot.CurrentWordIndex != 0 || string(m.playUI.snapshot.Words[0].Typed) != "cat" {
				t.Fatal("shortcut changed the current word")
			}
		})
	}
}

func TestInvalidConfigShowsErrorBeforeGenerating(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config engine.Config
		want   error
	}{
		{"mode", engine.Config{Mode: -1}, engine.ErrInvalidMode},
		{"zero duration", engine.Config{Mode: engine.ModeTime}, engine.ErrInvalidDuration},
		{"negative duration", engine.Config{Mode: engine.ModeTime, Duration: -time.Second}, engine.ErrInvalidDuration},
		{"zero count", engine.Config{Mode: engine.ModeWords}, engine.ErrInvalidWordCount},
		{"negative count", engine.Config{Mode: engine.ModeWords, WordCount: -1}, engine.ErrInvalidWordCount},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.config, func(int) []string {
				t.Fatal("invalid config reached the generator")
				return nil
			})
			if m.screen != failed || !errors.Is(m.err, tt.want) {
				t.Fatalf("screen=%v error=%v, want %v", m.screen, m.err, tt.want)
			}
			m.Init()
			m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			m.Update(tea.BackgroundColorMsg{Color: color.White})
			for _, size := range [][2]int{{80, 24}, {28, 12}, {1, 1}} {
				m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				assertFits(t, m.View(), size[0], size[1])
			}
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			view := strings.Join(strings.Fields(ansi.Strip(m.View())), " ")
			if !strings.Contains(view, tt.want.Error()) || !strings.Contains(view, "Tab home") {
				t.Fatalf("error or recovery instructions missing: %s", view)
			}
			if cmd := m.handleTick(tickMsg{at: time.Now()}); cmd != nil {
				t.Fatal("failed round scheduled a tick")
			}
			cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			if cmd == nil {
				t.Fatal("error screen could not return home")
			}
			if _, ok := cmd().(HomeMsg); !ok {
				t.Fatal("error screen returned the wrong message")
			}
		})
	}
}

func TestFailedRetryIgnoresOldTicks(t *testing.T) {
	m := New(engine.Config{Mode: engine.ModeTime, Duration: time.Second}, wordsGenerator())
	finishRound(t, m)
	oldGame := m.game
	m.generate = func(int) []string { return nil }
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.screen != failed || !errors.Is(m.err, engine.ErrNoWords) {
		t.Fatal("retry did not show word generation error")
	}
	if cmd := m.handleTick(tickMsg{game: oldGame, at: time.Now()}); cmd != nil || m.screen != failed {
		t.Fatal("old tick affected the failed round")
	}
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("failed round could not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("failed round returned the wrong quit message")
	}
}

func TestRetryAfterWordGenerationFailure(t *testing.T) {
	for _, config := range []engine.Config{
		{Mode: engine.ModeWords, WordCount: 2},
		{Mode: engine.ModeTime, Duration: time.Minute},
	} {
		calls := 0
		m := New(config, func(count int) []string {
			calls++
			if calls == 1 {
				return nil
			}
			return wordsGenerator()(count)
		})
		if m.screen != failed || m.err == nil {
			t.Fatal("empty generated words did not show an error")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.screen != play || m.err != nil || m.game.Status() != engine.Ready {
			t.Fatal("retry did not recover")
		}
	}
}
