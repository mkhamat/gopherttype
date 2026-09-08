package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app"
)

func main() {
	program := tea.NewProgram(app.New())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
