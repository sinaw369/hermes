// Package command cmd/command/sync.go
package command

import (
	"context"
	"fmt"
	"github.com/sinaw369/Hermes/internal/client"
	"github.com/sinaw369/Hermes/internal/config"
	"github.com/sinaw369/Hermes/internal/constant"
	"github.com/spf13/cobra"
	"log"
	"time"
)

type SyncCmd struct {
	detach        bool
	contextValues map[string]string
}

func NewSyncCmd() *SyncCmd {
	return &SyncCmd{
		detach:        false,
		contextValues: make(map[string]string),
	}
}
func (sc *SyncCmd) Command(cfg *config.Config) *cobra.Command {
	// Here, define the "sync" subcommand and its flags.
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync projects",
		Run: func(cmd *cobra.Command, args []string) {
			// Retrieve flag values or read from viper/env if not provided.
			syncDir, _ := cmd.Flags().GetString("dir")
			if syncDir == "" {
				syncDir = cfg.SyncDir
			}
			syncInterval, _ := cmd.Flags().GetString("interval")
			sc.contextValues[constant.TargetDir] = syncDir
			sc.contextValues[constant.SyncInterval] = syncInterval
			// Check if the user wants to detach
			if sc.detach {
				// DETACHED mode (like `docker run -d`).
				// We do NOT block the terminal — run sync in the background and return.
				sc.contextValues[constant.DetachMode] = "YES"
				fmt.Printf("Syncing in dettached mode. Press Ctrl+C to stop.\nDir=%s Interval=%s\n", syncDir, syncInterval)
				go sc.syncProjects(syncDir, syncInterval, cfg)
				select {}
			} else {
				// We block in this function, showing logs or any needed output.
				sc.contextValues[constant.DetachMode] = "NO"
				fmt.Printf("Syncing in attached mode. Press Ctrl+C to stop.\nDir=%s Interval=%s\n", syncDir, syncInterval)
				sc.syncProjects(syncDir, syncInterval, cfg) // blocks
			}
		},
	}
	cmd.Flags().BoolVarP(&sc.detach, "detach", "d", false, "Run in detached mode (like Docker’s -d)")
	cmd.Flags().String("dir", "", "Directory to sync projects")
	cmd.Flags().String("interval", "", "Sync interval")

	return cmd
}

// syncProjects is your actual sync logic.
func (sc *SyncCmd) syncProjects(syncDir, interval string, cfg *config.Config) {
	// Example infinite loop to show that attached mode blocks
	ticker := time.NewTicker(parseInterval(interval))
	defer ticker.Stop()
	gitClient, err := client.NewCLIGitClient(context.Background(), sc.contextValues, cfg)
	if err != nil {
		return
	}
	log.Printf("Syncing projects in %s...\n", syncDir)
	gitClient.InitPullRequestAutomationCLI(&syncDir)
	log.Printf("Syncing projects in %s...\n done", syncDir)

	for {
		select {
		case <-ticker.C:
			log.Printf("Syncing projects in %s...\n", syncDir)

			gitClient.InitPullRequestAutomationCLI(&syncDir)
			// Insert actual sync logic here
			log.Printf("Syncing projects in %s...\n done", syncDir)
		}
		// On attached mode, we keep printing logs. On detached, it’s silent in background.
	}
}

// parseInterval is a helper to parse a duration string into time.Duration.
func parseInterval(interval string) time.Duration {
	dur, err := time.ParseDuration(interval)
	if err != nil {
		return 5 * time.Minute // fallback
	}
	return dur
}
