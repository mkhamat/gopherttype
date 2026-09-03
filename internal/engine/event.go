package engine

import "time"

type EventKind int

const (
	Type EventKind = iota
	Space
	Backspace
	DeleteWord
	Tick
)

type Event struct {
	Kind EventKind
	Rune rune
	At   time.Time
}
