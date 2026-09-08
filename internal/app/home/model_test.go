package home

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gopherttype/internal/app/ui"
	"gopherttype/internal/engine"
)

func settleHome(t *testing.T, m *Model, cmd tea.Cmd) *StartMsg {
	t.Helper()
	var started *StartMsg
	budget := 1000
	var run func(tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		budget--
		if budget < 0 {
			t.Fatal("home commands did not settle")
		}
		msg := cmd()
		if msg == nil {
			return
		}
		value := reflect.ValueOf(msg)
		if value.Kind() == reflect.Slice {
			for i := 0; i < value.Len(); i++ {
				next, ok := value.Index(i).Interface().(tea.Cmd)
				if !ok {
					t.Fatalf("unexpected command collection: %T", msg)
				}
				run(next)
			}
			return
		}
		if start, ok := msg.(StartMsg); ok {
			started = &start
			if start.Config != m.settings.config() {
				t.Fatal("start message lost settings")
			}
			return
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("unexpected quit")
		}
		run(m.Update(msg))
	}
	run(cmd)
	return started
}

func homeMessage(t *testing.T, m *Model, msg tea.Msg) *StartMsg {
	t.Helper()
	return settleHome(t, m, m.Update(msg))
}

func press(t *testing.T, m *Model, code rune, mod tea.KeyMod) *StartMsg {
	t.Helper()
	return homeMessage(t, m, tea.KeyPressMsg{Code: code, Mod: mod})
}

func newHomeModel(t *testing.T, width, height int) *Model {
	t.Helper()
	m := New()
	settleHome(t, m, m.Init())
	homeMessage(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	return m
}

func focusedKey(m *Model) string {
	return m.homeUI.form.GetFocusedField().GetKey()
}

func TestHomeNavigation(t *testing.T) {
	m := newHomeModel(t, 80, 24)
	if got := m.settings.config(); got.Mode != engine.ModeTime || got.Duration != durationPresets[1] {
		t.Fatalf("default: %+v", got)
	}
	press(t, m, tea.KeyTab, tea.ModShift)
	if focusedKey(m) != modeField {
		t.Fatal("back at the first field must stay at the first field")
	}
	if m.settings.config().Duration != durationPresets[1] {
		t.Fatal("default duration")
	}
	press(t, m, tea.KeyEnter, 0)
	if focusedKey(m) != lengthField {
		t.Fatal("enter must advance to length")
	}
	press(t, m, tea.KeyDown, 0)
	if m.settings.config().Duration != durationPresets[2] {
		t.Fatal("duration change")
	}
	press(t, m, tea.KeyTab, tea.ModShift)
	press(t, m, tea.KeyUp, 0)
	press(t, m, tea.KeyTab, 0)
	press(t, m, tea.KeyUp, 0)
	if m.settings.config().WordCount != 100 {
		t.Fatal("preset wrap")
	}
	press(t, m, tea.KeyTab, tea.ModShift)
	press(t, m, tea.KeyDown, 0)
	if m.settings.config().Duration != durationPresets[2] || m.homeUI.length.GetValue() != 2 {
		t.Fatal("duration not preserved")
	}
	if m.homeUI.form.State == huh.StateCompleted {
		t.Fatal("navigation started a game")
	}
}

func TestStartAllPresets(t *testing.T) {
	for _, mode := range []engine.Mode{engine.ModeWords, engine.ModeTime} {
		for i := range len(wordPresets) {
			t.Run(fmt.Sprintf("%d/%d", mode, i), func(t *testing.T) {
				m := newHomeModel(t, 80, 24)
				if mode == engine.ModeWords {
					press(t, m, tea.KeyUp, 0)
				}
				press(t, m, tea.KeyEnter, 0)
				if mode == engine.ModeTime {
					press(t, m, tea.KeyUp, 0)
				}
				for range i {
					press(t, m, tea.KeyDown, 0)
				}
				config := m.settings.config()
				press(t, m, tea.KeyEnter, 0)
				if focusedKey(m) != startField || m.homeUI.form.State == huh.StateCompleted {
					t.Fatal("length submission must focus start without launching")
				}
				started := press(t, m, tea.KeyEnter, 0)
				if started == nil || started.Config != config || m.homeUI.form.State != huh.StateCompleted {
					t.Fatalf("start failed: %+v", config)
				}
			})
		}
	}
}

func TestHomeResize(t *testing.T) {
	for _, size := range [][2]int{{100, 35}, {80, 24}, {80, 12}, {58, 23}, {54, 22}, {40, 16}, {28, 12}, {24, 9}, {10, 3}, {1, 1}} {
		for _, mode := range []engine.Mode{engine.ModeWords, engine.ModeTime} {
			t.Run(fmt.Sprintf("%v/%d", size, mode), func(t *testing.T) {
				m := newHomeModel(t, size[0], size[1])
				if mode == engine.ModeWords {
					press(t, m, tea.KeyUp, 0)
				}
				for _, field := range []string{modeField, lengthField, startField} {
					if focusedKey(m) != field {
						t.Fatalf("focus: want %s, got %s", field, focusedKey(m))
					}
					view := m.homeView()
					assertFits(t, view, size[0], size[1])
					plain := ansi.Strip(view)
					if size[0] >= ui.MinimumWidth && size[1] >= ui.MinimumHeight {
						for _, want := range []string{"gopherttype", "q / Esc quit"} {
							if !strings.Contains(plain, want) {
								t.Fatalf("missing %q:\n%s", want, plain)
							}
						}
						if strings.Contains(plain, "toggle") {
							t.Fatal("help advertises an unavailable toggle")
						}
						if m.homeLayout().compact && field != startField && strings.Contains(plain, "Enter start") {
							t.Fatal("help advertises starting from a selector")
						}
					} else if size[0] >= 24 && !strings.Contains(plain, "Resize to") {
						t.Fatal("missing resize fallback")
					}
					if field != startField {
						press(t, m, tea.KeyEnter, 0)
					}
				}
			})
		}
	}
}

func TestHomeResizeRestoresOptions(t *testing.T) {
	for _, mode := range []engine.Mode{engine.ModeWords, engine.ModeTime} {
		m := newHomeModel(t, 80, 24)
		if mode == engine.ModeWords {
			press(t, m, tea.KeyUp, 0)
		}
		before := m.homeView()
		for _, size := range [][2]int{{80, 12}, {1, 1}, {28, 12}, {80, 24}} {
			homeMessage(t, m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		}
		if after := m.homeView(); before != after {
			t.Fatalf("resize did not restore all options:\nbefore:\n%s\nafter:\n%s", ansi.Strip(before), ansi.Strip(after))
		}
	}
}

func TestBackgroundChangesUpdateHome(t *testing.T) {
	m := newHomeModel(t, 80, 24)
	before := m.homeView()
	homeMessage(t, m, tea.BackgroundColorMsg{Color: color.White})
	if before == m.homeView() || m.styles != ui.StylesFor(false) {
		t.Fatal("home did not adopt light theme")
	}
}

func TestStartOnlyUsesAdvertisedSubmitKey(t *testing.T) {
	m := newHomeModel(t, 28, 12)
	press(t, m, tea.KeyEnter, 0)
	press(t, m, tea.KeyEnter, 0)
	for _, code := range []rune{tea.KeyLeft, tea.KeyRight, 'y', 'n', tea.KeyTab} {
		press(t, m, code, 0)
		if m.homeUI.form.State == huh.StateCompleted || focusedKey(m) != startField {
			t.Fatalf("unexpected start or navigation from %q", code)
		}
	}
	press(t, m, tea.KeyEnter, 0)
	if m.homeUI.form.State != huh.StateCompleted {
		t.Fatal("advertised start key failed")
	}
}

func assertFits(t *testing.T, view string, width, height int) {
	t.Helper()
	if lipgloss.Width(view) > width || lipgloss.Height(view) > height {
		t.Fatalf("overflow at %dx%d: %dx%d", width, height, lipgloss.Width(view), lipgloss.Height(view))
	}
}
