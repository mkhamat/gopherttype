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
	startField  = "start"
)

type formState struct {
	form   *huh.Form
	mode   *huh.Select[engine.Mode]
	length *huh.Select[int]
	start  *huh.Confirm
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
		Options(huh.NewOption("Words", engine.ModeWords), huh.NewOption("Time", engine.ModeTime)).Value(&m.settings.mode)
	m.homeUI.length = huh.NewSelect[int]().Key(lengthField).Accessor(lengthAccessor{&m.settings})
	m.configureLength()
	m.homeUI.start = huh.NewConfirm().Key(startField).Affirmative("Start round →").Negative("")
	keys := huh.NewDefaultKeyMap()
	keys.Select.Filter.SetEnabled(false)
	keys.Select.Up.SetHelp("↑/↓", "choose")
	keys.Select.Down.SetHelp("", "")
	keys.Select.Next.SetHelp("Tab/Enter", "next")
	keys.Select.Prev.SetHelp("Shift+Tab", "back")
	keys.Confirm.Toggle.SetEnabled(false)
	keys.Confirm.Accept.SetEnabled(false)
	keys.Confirm.Reject.SetEnabled(false)
	keys.Confirm.Prev.SetHelp("Shift+Tab", "back")
	keys.Confirm.Submit.SetHelp("Enter", "start")
	m.homeUI.form = huh.NewForm(huh.NewGroup(m.homeUI.mode, m.homeUI.length, m.homeUI.start)).
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
	switch m.homeUI.form.State {
	case huh.StateCompleted:
		config := m.settings.config()
		return func() tea.Msg { return StartMsg{Config: config} }
	case huh.StateAborted:
		return tea.Quit
	}
	m.resizeForm()
	return cmd
}
