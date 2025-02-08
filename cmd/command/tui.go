// Package command cmd/command/tui.go
package command

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sinaw369/Hermes/internal/config"
	"github.com/sinaw369/Hermes/internal/tui"
	"github.com/spf13/cobra"
	"log"
)

type HermesCmd struct{}

func (hc *HermesCmd) Command(cfg *config.Config) *cobra.Command {
	// rootCmd defines what happens when you run "hermes" with no subcommand.
	return &cobra.Command{
		Use:   "hermes",
		Short: "Hermes CLI",
		Run: func(cmd *cobra.Command, args []string) {
			// If the user just runs "hermes", we start the TUI.
			fmt.Println("Launching TUI...")
			if err := hc.startTUI(cfg); err != nil {
				log.Fatalf("Error running TUI: %v", err)
			}
		},
	}
}
func (hc *HermesCmd) startTUI(cfg *config.Config) error {

	// Initialize the application model.
	m := tui.InitialModel(cfg)
	m.LogWriter.InfoString("Welcome to Hermes!")
	m.LogWriter.InfoString("Application Version 0.0.1")

	// Start the Bubble Tea program.
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
	return nil
}
