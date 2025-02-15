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
	"path/filepath"
	"time"
)

type SyncCmd struct {
	silentMode    bool
	contextValues map[string]string
}

func NewSyncCmd() *SyncCmd {
	return &SyncCmd{
		silentMode:    false,
		contextValues: make(map[string]string),
	}
}
func (sc *SyncCmd) Command(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync projects",
		Run: func(cmd *cobra.Command, args []string) {
			syncDir, _ := cmd.Flags().GetString("dir")
			if syncDir == "" {
				syncDir = cfg.WorkingDir
			}
			if !filepath.IsAbs(syncDir) {
				log.Println("dir should be full path:", syncDir)
				return
			}
			sc.contextValues[constant.TargetDir] = syncDir
			sc.contextValues[constant.ContextValueInclude], _ = cmd.Flags().GetString("include")
			sc.contextValues[constant.ContextValueExclude], _ = cmd.Flags().GetString("exclude")
			pullBranch, _ := cmd.Flags().GetString("pull-branch")
			if pullBranch != "" {
				sc.contextValues[constant.ContextValuePullDefault] = constant.ContextValueYES
				sc.contextValues[constant.ContextValuePullBranch] = pullBranch
			}
			if sc.silentMode {
				sc.contextValues[constant.SilentMode] = "YES"
				fmt.Printf("Syncing in SilentMode mode. Press Ctrl+C to stop.\nDir=%s\n", syncDir)
				sc.syncProjects(syncDir, cfg)
			} else {
				sc.contextValues[constant.SilentMode] = "NO"
				log.Printf("Syncing projects in %s...\n", syncDir)
				start := time.Now()
				sc.syncProjects(syncDir, cfg)
				elapsed := time.Since(start).Minutes()
				log.Printf("Syncing projects in %s...\ndone\nelapsedtime:%v minutes", syncDir, elapsed)
			}
		},
	}
	cmd.Flags().BoolVarP(&sc.silentMode, "silent", "", false, "Run in detached mode (like Docker’s -d)")
	cmd.Flags().String("dir", "", "Directory to sync projects and should be full path")
	cmd.Flags().String("include", "", "include project with patterns (comma-separated)")
	cmd.Flags().String("exclude", "", "exclude project with patterns (comma-separated)")
	cmd.Flags().String("pull-branch", "", "the target branch witch you want to just pull it")

	return cmd
}

func (sc *SyncCmd) syncProjects(syncDir string, cfg *config.Config) {
	gitClient, err := client.NewCLIGitClient(context.Background(), sc.contextValues, cfg)
	if err != nil {
		return
	}

	gitClient.InitPullRequestAutomationCLI(&syncDir)

}

func parseInterval(interval string) time.Duration {
	dur, err := time.ParseDuration(interval)
	if err != nil {
		return 5 * time.Minute // fallback
	}
	return dur
}
