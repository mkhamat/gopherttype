package home

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"gopherttype/internal/engine"
)

const (
	modeField   = "mode"
	lengthField = "length"
)

type formState struct {
	form   *huh.Form
	mode   *huh.Select[engine.Mode]
	length *huh.Select[int]
}

type lengthAccessor struct{ settings *settings }

func (a lengthAccessor) Get() int {
	if a.settings.mode == engine.ModeTime {
		return a.settings.durationIndex
	}
	return a.settings.wordIndex
}

func (a lengthAccessor) Set(value int) {
	if a.settings.mode == engine.ModeTime {
		a.settings.durationIndex = value
	} else {
		a.settings.wordIndex = value
	}
}

func (m *Model) configureLength() {
	var options []huh.Option[int]
	var title string
	if m.settings.mode == engine.ModeTime {
		title = "02  DURATION"
		for i, duration := range durationPresets {
			options = append(options, huh.NewOption(fmt.Sprintf("%.0fs", duration.Seconds()), i))
		}
	} else {
		title = "02  WORD COUNT"
		for i, count := range wordPresets {
			options = append(options, huh.NewOption(fmt.Sprint(count), i))
		}
	}
	m.homeUI.length.Title(title).Options(options...)
}

func (m *Model) initForm() {
	m.homeUI.mode = huh.NewSelect[engine.Mode]().Key(modeField).Title("01  MODE").
		Options(huh.NewOption("Time", engine.ModeTime), huh.NewOption("Words", engine.ModeWords)).
		Value(&m.settings.mode).Inline(true)
	m.homeUI.length = huh.NewSelect[int]().Key(lengthField).Accessor(lengthAccessor{&m.settings}).Inline(true)
	m.configureLength()
	keys := huh.NewDefaultKeyMap()
	keys.Select.Filter.SetEnabled(false)
	keys.Select.Up.SetEnabled(false)
	keys.Select.Down.SetEnabled(false)
	keys.Select.Left.SetHelp("←/h", "change")
	keys.Select.Right.SetHelp("→/l", "change")
	keys.Select.Next.SetKeys("down", "j", "tab")
	keys.Select.Next.SetHelp("↓/j", "next")
	keys.Select.Prev.SetKeys("up", "k", "shift+tab")
	keys.Select.Prev.SetHelp("↑/k", "back")
	keys.Select.Submit.SetHelp("enter", "start")
	m.homeUI.form = huh.NewForm(huh.NewGroup(m.homeUI.mode, m.homeUI.length)).
		WithTheme(formTheme(m.styles)).WithKeyMap(keys)
	m.resizeForm()
}

func (m *Model) updateForm(msg tea.Msg) tea.Cmd {
	if m.homeUI.form.State != huh.StateNormal {
		return nil
	}
	previous := m.settings
	_, cmd := m.homeUI.form.Update(msg)
	if m.settings.mode != previous.mode {
		m.configureLength()
	}
	m.resizeForm()
	return cmd
}
