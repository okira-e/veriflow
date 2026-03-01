package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/okira-e/veriflow/app/config"
)

func Run(cfg *config.Cfg) {
	model := NewModel(cfg)

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
