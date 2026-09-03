package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app"
)

func main() {
	model := app.New()
	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
