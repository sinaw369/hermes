// File: cmd/app.go
package app

import (
	"fmt"
	"github.com/sinaw369/Hermes/cmd/command"
	"github.com/sinaw369/Hermes/internal/config"
	"github.com/spf13/cobra"
	"log"
)

// main is the entry point of the application.
func Run() {
	const description = "Hermes"
	root := &cobra.Command{Short: description}

	SyncCmd := command.NewSyncCmd()
	diffCmd := command.NewDiffCmd()
	var HermesCmd command.HermesCmd

	cfg, err := config.Load()
	if err != nil {
		return
	}

	root.AddCommand(
		SyncCmd.Command(cfg),
		HermesCmd.Command(cfg),
		diffCmd.Command(cfg),
	)

	if err := root.Execute(); err != nil {
		log.Fatalf(fmt.Sprintf("failed to execute root command: \n%v", err))
	}
}
