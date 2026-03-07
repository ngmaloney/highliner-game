package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Load or create new game state
	gs := loadGame()
	if gs == nil {
		gs = newGame()
	}

	m := newModel(gs)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running highliner: %v\n", err)
		os.Exit(1)
	}
}
