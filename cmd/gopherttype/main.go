package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/mkhamat/gopherttype/internal/app"
)

func main() {
	program := tea.NewProgram(app.New(), tea.WithFPS(120))
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
