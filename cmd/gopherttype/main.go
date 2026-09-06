package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"gopherttype/internal/app"
)

func main() {
	model := app.New()
	app := tea.NewProgram(model)
	if _, err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
