package command

import (
	"context"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/sinaw369/Hermes/internal/client"
	"github.com/sinaw369/Hermes/internal/config"
	"github.com/sinaw369/Hermes/internal/logWriter"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type DiffCmd struct {
	baseDir       string
	pathPattern   string
	branchFrom    string
	branchTo      string
	onlyWithDiff  bool
	contextValues map[string]string
}

func NewDiffCmd() *DiffCmd {
	return &DiffCmd{
		contextValues: make(map[string]string),
	}
}

func (sc *DiffCmd) Command(cfg *config.Config) *cobra.Command {
	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Show diff summary between two branches for one or more repositories",
		Run: func(cmd *cobra.Command, args []string) {
			err := sc.startApp(cfg)
			if err != nil {
				log.Println("err is :", err)
				return
			}
		},
	}

	// Define flags for the diff command.
	diffCmd.Flags().StringVar(&sc.baseDir, "basedir", "", "Base directory where repositories are located")
	diffCmd.Flags().StringVar(&sc.pathPattern, "path", "", "Glob pattern to select repositories (e.g., 'backend/*')")
	diffCmd.Flags().StringVar(&sc.branchFrom, "branch-from", "", "Source branch for diff (e.g., develop)")
	diffCmd.Flags().StringVar(&sc.branchTo, "branch-to", "", "Target branch for diff (e.g., production)")
	diffCmd.Flags().BoolVar(&sc.onlyWithDiff, "only-with-diff", false, "Show only projects that have differences between branches")

	return diffCmd
}
func (sc *DiffCmd) FetchFromEnvironment(cfg *config.Config) {
	if sc.baseDir == "" {
		sc.baseDir = cfg.FileDir
	}
	if sc.branchTo == "" {
		sc.branchTo = cfg.DifBranchTO
	}
	if sc.branchFrom == "" {
		sc.branchFrom = cfg.DiffBranchFrom
	}
}
func (sc *DiffCmd) startApp(cfg *config.Config) error {
	sc.FetchFromEnvironment(cfg)
	logger := logWriter.NewLogger(os.Stdout, false, false)
	gitClient, err := client.NewCLIGitClient(context.Background(), sc.contextValues, cfg)
	if err != nil {
		return err
	}

	var repos []string

	// Check if baseDir contains glob characters.
	if strings.ContainsAny(sc.baseDir, "*?[]") {
		matches, err := filepath.Glob(sc.baseDir)
		if err != nil {
			return fmt.Errorf("error parsing baseDir glob: %v", err)
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || !info.IsDir() {
				continue
			}
			// Check if the directory contains a .git folder.
			gitPath := filepath.Join(m, ".git")
			if gitInfo, err := os.Stat(gitPath); err == nil && gitInfo.IsDir() {
				repos = append(repos, m)
			}
		}
		if len(repos) == 0 {
			return fmt.Errorf("no git repositories found matching baseDir pattern: %s", sc.baseDir)
		}
	} else if sc.pathPattern != "" {
		// Use baseDir and pathPattern together as a glob.
		fullPattern := filepath.Join(sc.baseDir, sc.pathPattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return fmt.Errorf("error parsing glob pattern: %v", err)
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || !info.IsDir() {
				continue
			}
			// Check if the directory contains a .git folder.
			gitPath := filepath.Join(m, ".git")
			if gitInfo, err := os.Stat(gitPath); err == nil && gitInfo.IsDir() {
				repos = append(repos, m)
			}
		}
		if len(repos) == 0 {
			return fmt.Errorf("no git repositories found matching pattern: %s", fullPattern)
		}
	} else {
		// If no glob or pathPattern is provided, assume baseDir itself is a repository.
		repos = []string{sc.baseDir}
	}

	// Dynamically obtain the terminal width.
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 120 // fallback width if error occurs
	} else {
		// Optionally subtract 20 columns.
		width = width - 20
	}
	//separator := strings.Repeat("-", width)

	// Create a diff box style once.
	diffBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Width(width)

	// Iterate over repositories and show diff summaries.
	for _, repoPath := range repos {
		// Determine repository name relative to base directory.
		repoName := repoPath
		if rel, err := filepath.Rel(cfg.FileDir, repoPath); err == nil {
			repoName = rel
		}

		// Format header using Lip Gloss.
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("85"))
		header := headerStyle.Render("Repository: " + repoName)

		// Fetch diff summary from the Git client.
		diff, hasChange, err := gitClient.FetchDiffCLI(repoPath, sc.branchFrom, sc.branchTo)
		if err != nil {
			logger.RedString("Error fetching diff for %s: it seems branch %s or %s does not exist\n", repoName, sc.branchFrom, sc.branchTo)
			continue
		}

		// Only print the diff if either we want all or we want only repos with diffs and there is a diff.
		if !sc.onlyWithDiff || (sc.onlyWithDiff && hasChange) {
			diffBox := diffBoxStyle.Render(diff)
			fmt.Println(header)
			fmt.Println(diffBox)
			fmt.Println()
		}
	}
	return nil
}
