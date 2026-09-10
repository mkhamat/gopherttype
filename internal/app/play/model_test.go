package play

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"gopherttype/internal/words"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func finishRound(t *testing.T, m *Model) {
	t.Helper()
	at := m.playUI.at.Add(time.Second)
	if m.roundConfig.Mode == engine.ModeTime {
		m.handlePlayKeyAt(tea.KeyPressMsg{Code: 'x', Text: "x"}, at)
		m.Update(tickMsg{game: m.game, at: at.Add(m.roundConfig.Duration)})
	} else {
		words := m.playUI.snapshot.Words
		for _, word := range words {
			m.handlePlayKeyAt(tea.KeyPressMsg{Text: word.Target}, at)
			at = at.Add(time.Second)
			if m.screen == play {
				m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at)
			}
		}
	}
	if m.screen != results || m.game.Status() != engine.Finished {
		t.Fatal("round did not finish")
	}
}

func newPlayModel(t *testing.T, config engine.Config, words []string) *Model {
	t.Helper()
	m := New(config, wordsGenerator())
	game, err := engine.New(config, words)
	if err != nil {
		t.Fatal(err)
	}
	m.game, m.roundConfig, m.screen = game, config, play
	m.syncGame(time.Unix(1000, 0), true)
	return m
}

func TestExtraCharactersStopAtLineEdge(t *testing.T) {
	m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 3}, []string{"one", "cat", "next"})
	m.Update(tea.WindowSizeMsg{Width: 10, Height: 24})
	at := m.playUI.at.Add(time.Second)
	m.handlePlayKeyAt(tea.KeyPressMsg{Text: "one"}, at)
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at)
	m.handlePlayKeyAt(tea.KeyPressMsg{Text: "catxxxx"}, at)
	if got := string(m.playUI.snapshot.Words[1].Typed); got != "catxx" {
		t.Fatalf("typed: got %q, want catxx", got)
	}
	if m.playUI.layout.cursorRow != 0 || m.playUI.layout.cursorColumn != 9 {
		t.Fatalf("cursor moved from line edge: %+v", m.playUI.layout)
	}
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeyBackspace}, at)
	m.handlePlayKeyAt(tea.KeyPressMsg{Text: "y"}, at)
	if got := string(m.playUI.snapshot.Words[1].Typed); got != "catxy" {
		t.Fatalf("replacement at edge: got %q", got)
	}
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at)
	if m.playUI.snapshot.CurrentWordIndex != 2 {
		t.Fatal("space did not advance from the line edge")
	}
}

func TestExtraCharactersLayoutContract(t *testing.T) {
	for _, terminalWidth := range []int{10, 28, 40, 80, 200} {
		for _, extra := range []struct {
			name  string
			text  string
			width int
		}{
			{"ASCII", "x", 1},
			{"wide", "界", 2},
			{"zero-width-stub", "\u200b", 1},
		} {
			for _, batch := range []bool{false, true} {
				t.Run(fmt.Sprintf("width=%d/%s/batch=%t", terminalWidth, extra.name, batch), func(t *testing.T) {
					m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 3}, []string{"one", "cat", "next"})
					m.Update(tea.WindowSizeMsg{Width: terminalWidth, Height: 24})
					at := m.playUI.at.Add(time.Second)
					m.handlePlayKeyAt(tea.KeyPressMsg{Text: "one"}, at)
					m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at)
					m.handlePlayKeyAt(tea.KeyPressMsg{Text: "cat"}, at)
					width := m.playWidth()
					row := m.playUI.layout.cursorRow
					column := m.playUI.layout.cursorColumn
					checkLayout := func() {
						t.Helper()
						if m.playUI.layout.cursorRow != row || m.playUI.layout.activeColumn != 4 {
							t.Fatalf("extra moved the active word or cursor to another line: %+v", m.playUI.layout)
						}
						for _, line := range m.playUI.layout.lines {
							if got := ansi.StringWidth(line); got > width {
								t.Fatalf("rendered line width = %d, limit = %d", got, width)
							}
						}
					}
					if batch {
						m.handlePlayKeyAt(tea.KeyPressMsg{Text: strings.Repeat(extra.text, width)}, at)
						checkLayout()
					} else {
						for range width {
							m.handlePlayKeyAt(tea.KeyPressMsg{Text: extra.text}, at)
							checkLayout()
						}
					}
					want := "cat" + strings.Repeat(extra.text, (width-1-column)/extra.width)
					if got := string(m.playUI.snapshot.Words[1].Typed); got != want {
						t.Fatalf("typed = %q, want %q", got, want)
					}
					if m.playUI.layout.cursorColumn < width-1 {
						m.handlePlayKeyAt(tea.KeyPressMsg{Text: "x"}, at)
					}
					if m.playUI.layout.cursorColumn != width-1 {
						t.Fatalf("cursor column = %d, want %d", m.playUI.layout.cursorColumn, width-1)
					}
					before := string(m.playUI.snapshot.Words[1].Typed)
					m.handlePlayKeyAt(tea.KeyPressMsg{Text: "x界\u200b"}, at)
					if got := string(m.playUI.snapshot.Words[1].Typed); got != before {
						t.Fatalf("accepted overflowing extra: %q -> %q", before, got)
					}
					m.handlePlayKeyAt(tea.KeyPressMsg{Text: "\u0301"}, at)
					want = before + "\u0301"
					if extra.text == "\u200b" {
						want = before
					}
					if got := string(m.playUI.snapshot.Words[1].Typed); got != want {
						t.Fatalf("combining mark at edge: got %q, want %q", got, want)
					}
					checkLayout()
					m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at)
					if m.playUI.snapshot.CurrentWordIndex != 2 {
						t.Fatal("space did not advance from the line edge")
					}
				})
			}
		}
	}
}

func TestFinalWordCompletesAtLineEdge(t *testing.T) {
	m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 2}, []string{"one", "hello"})
	m.Update(tea.WindowSizeMsg{Width: 10, Height: 24})
	at := m.playUI.at.Add(time.Second)
	m.handlePlayKeyAt(tea.KeyPressMsg{Text: "one"}, at)
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at)
	m.handlePlayKeyAt(tea.KeyPressMsg{Text: "hello"}, at.Add(time.Second))
	if m.screen != results || m.game.Status() != engine.Finished {
		t.Fatal("final word did not complete at the line edge")
	}
}

func TestStartAllPresets(t *testing.T) {
	configs := []engine.Config{}
	for _, count := range []int{10, 25, 50, 100} {
		configs = append(configs, engine.Config{Mode: engine.ModeWords, WordCount: count})
	}
	for _, seconds := range []int{15, 30, 60, 120} {
		configs = append(configs, engine.Config{Mode: engine.ModeTime, Duration: time.Duration(seconds) * time.Second})
	}
	for _, config := range configs {
		m := New(config, wordsGenerator())
		cmd := m.Init()
		count := config.WordCount
		if config.Mode == engine.ModeTime {
			count = replenishBatch
		}
		if cmd != nil || m.screen != play || m.game == nil || m.game.Status() != engine.Ready || m.game.RemainingWords() != count || m.roundConfig != config {
			t.Fatalf("start failed: %+v", config)
		}
	}
}

func TestClockStartsWithTypingAndStopsAtDeadline(t *testing.T) {
	m := newPlayModel(t, engine.Config{Mode: engine.ModeTime, Duration: 15 * time.Second}, []string{"hello"})
	at := m.playUI.at.Add(time.Second)
	if cmd := m.handleTick(tickMsg{game: m.game, at: at}); cmd != nil || m.game.Status() != engine.Ready {
		t.Fatal("ready round must not tick")
	}
	if cmd := m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at); cmd != nil || m.game.Status() != engine.Ready {
		t.Fatal("empty space must not start the clock")
	}
	if !strings.Contains(ansi.Strip(m.playView()), "Remaining: 15s") {
		t.Fatal("ready timed round must show its duration")
	}
	if cmd := m.handlePlayKeyAt(tea.KeyPressMsg{Code: 'h', Text: "h"}, at); cmd == nil {
		t.Fatal("first character must schedule a tick")
	}
	if cmd := m.handlePlayKeyAt(tea.KeyPressMsg{Code: 'e', Text: "e"}, at.Add(time.Millisecond)); cmd != nil {
		t.Fatal("later characters must not create extra tick chains")
	}
	words, lines := &m.playUI.snapshot.Words[0], &m.playUI.layout.lines[0]
	if cmd := m.handleTick(tickMsg{game: m.game, at: at.Add(time.Second)}); cmd == nil {
		t.Fatal("playing round must continue ticking")
	}
	if m.playUI.snapshot.Metrics.Duration != time.Second || !strings.Contains(ansi.Strip(m.playView()), "Remaining: 14s") {
		t.Fatal("countdown did not advance")
	}
	if words != &m.playUI.snapshot.Words[0] || lines != &m.playUI.layout.lines[0] {
		t.Fatal("clock-only tick rebuilt words or their layout")
	}
	if got, want := m.playUI.snapshot.Metrics, m.game.Snapshot(m.playUI.at).Metrics; got != want {
		t.Fatalf("metrics-only update: got %+v, want %+v", got, want)
	}
	m.handleTick(tickMsg{game: m.game, at: at.Add(500 * time.Millisecond)})
	if m.playUI.snapshot.Metrics.Duration != time.Second {
		t.Fatal("delayed tick moved clock backward")
	}
	if cmd := m.handleTick(tickMsg{game: m.game, at: at.Add(15 * time.Second)}); cmd != nil {
		t.Fatal("finished round must stop ticking")
	}
	if m.screen != results || m.game.FinalMetrics().Duration != 15*time.Second {
		t.Fatal("timed round did not finish at its deadline")
	}
}

func TestTimedReplenishment(t *testing.T) {
	words := make([]string, replenishBatch)
	for i := range words {
		words[i] = "a"
	}
	m := newPlayModel(t, engine.Config{Mode: engine.ModeTime, Duration: time.Minute}, words)
	at := m.playUI.at
	for range replenishBatch - replenishThreshold + 1 {
		at = at.Add(time.Millisecond)
		m.handlePlayKeyAt(tea.KeyPressMsg{Code: 'a', Text: "a"}, at)
		m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at)
	}
	if m.screen != play || len(m.playUI.snapshot.Words) != 2*replenishBatch || m.game.RemainingWords() != 74 {
		t.Fatalf("replenishment failed: screen=%d words=%d remaining=%d", m.screen, len(m.playUI.snapshot.Words), m.game.RemainingWords())
	}
	if m.playUI.layout.cursorRow < 0 {
		t.Fatal("replenishment lost cursor")
	}
}

func TestRetryAndStaleTicks(t *testing.T) {
	m := newPlayModel(t, engine.Config{Mode: engine.ModeTime, Duration: 30 * time.Second}, []string{"hello"})
	oldGame, config := m.game, m.roundConfig
	finishRound(t, m)
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || m.screen != play || m.game == oldGame || m.game.Status() != engine.Ready || m.roundConfig != config {
		t.Fatal("retry did not create a ready round with the same configuration")
	}
	if cmd := m.handleTick(tickMsg{game: oldGame, at: time.Now().Add(time.Hour)}); cmd != nil || m.game.Status() != engine.Ready {
		t.Fatal("stale tick affected retry")
	}
}

func TestPlayAndResultsResize(t *testing.T) {
	m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 1}, []string{"hello"})
	for _, size := range [][2]int{{100, 35}, {80, 24}, {28, 12}, {28, 5}, {10, 3}, {1, 1}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		assertFits(t, m.playView(), size[0], size[1])
		if size[0] >= ui.MinimumWidth && size[1] >= ui.MinimumHeight {
			if !strings.Contains(ansi.Strip(m.playView()), "hello") {
				t.Fatal("typing area missing")
			}
		} else if size[0] >= ui.MinimumWidth && !strings.Contains(ansi.Strip(m.playView()), "Resize to") {
			t.Fatal("short play screen must show resize fallback")
		}
	}
	finishRound(t, m)
	for _, size := range [][2]int{{100, 35}, {80, 24}, {28, 12}, {28, 5}, {10, 3}, {1, 1}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.resultsView()
		assertFits(t, view, size[0], size[1])
		if size[0] >= ui.MinimumWidth && size[1] >= ui.MinimumHeight {
			for _, want := range []string{"results", "Elapsed:", "Enter retry", "Tab home", "q / Esc quit"} {
				if !strings.Contains(ansi.Strip(view), want) {
					t.Fatalf("results missing %q", want)
				}
			}
		}
	}
}

func TestBackgroundChangesUpdatePlay(t *testing.T) {
	m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 1}, []string{"hello"})
	words := &m.playUI.snapshot.Words[0]
	before := m.playView()
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if before == m.playView() || m.styles != ui.StylesFor(false) || words != &m.playUI.snapshot.Words[0] {
		t.Fatal("play theme change failed or recopied words")
	}
}

func TestEditingKeepsCursorOnDisplayedText(t *testing.T) {
	m := newPlayModel(t, engine.Config{Mode: engine.ModeWords, WordCount: 2}, []string{"hello", "world"})
	at := m.playUI.at.Add(time.Second)
	m.handlePlayKeyAt(tea.KeyPressMsg{Text: "界界"}, at)
	if m.playUI.layout.cursorRow != 0 || m.playUI.layout.cursorColumn != 2 {
		t.Fatal("Unicode mistakes moved cursor away from target text")
	}
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeyBackspace}, at)
	if m.playUI.layout.cursorColumn != 1 {
		t.Fatal("backspace did not move cursor")
	}
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModAlt}, at)
	if m.playUI.layout.cursorColumn != 0 {
		t.Fatal("delete word did not move cursor")
	}
	m.handlePlayKeyAt(tea.KeyPressMsg{Text: "xx"}, at)
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeySpace}, at)
	if m.playUI.snapshot.CurrentWordIndex != 1 {
		t.Fatal("space did not advance")
	}
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeyBackspace}, at)
	if m.playUI.snapshot.CurrentWordIndex != 0 || m.playUI.layout.cursorColumn != 2 {
		t.Fatal("reopening incorrect word lost cursor")
	}
	m.handlePlayKeyAt(tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModCtrl}, at)
	if m.playUI.layout.cursorColumn != 0 {
		t.Fatal("ctrl+backspace did not clear reopened word")
	}
}

func TestCountdownRoundsUp(t *testing.T) {
	config := engine.Config{Mode: engine.ModeTime, Duration: 15 * time.Second}
	for _, tc := range []struct {
		elapsed time.Duration
		want    string
	}{{0, "15s"}, {100 * time.Millisecond, "15s"}, {time.Second, "14s"}, {14900 * time.Millisecond, "1s"}, {16 * time.Second, "0s"}} {
		got := renderPlayStats(engine.Metrics{Duration: tc.elapsed}, config)
		if !strings.Contains(got, "Remaining: "+tc.want) {
			t.Fatalf("%s: %s", tc.elapsed, got)
		}
	}
}

func wordsGenerator() func(int) []string { return words.New(1, 2).Generate }

func assertFits(t *testing.T, view string, width, height int) {
	t.Helper()
	if lipgloss.Width(view) > width || lipgloss.Height(view) > height {
		t.Fatalf("overflow at %dx%d: %dx%d", width, height, lipgloss.Width(view), lipgloss.Height(view))
	}
}
