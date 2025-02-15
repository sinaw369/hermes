package client

import (
	"bytes"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/sinaw369/Hermes/internal/constant"
	"github.com/sinaw369/Hermes/internal/form/progressScreen"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// InitPullRequestAutomationCLI InitPullRequestAutomation handles GitLab project automation tasks.
func (g *GitlabClient) InitPullRequestAutomationCLI(baseDir *string) {
	if baseDir == nil {
		// Determine the base director
		return
	}
	g.logWriter.InfoString("Starting GitLab project automation")

	// Initialize the GitLab client

	gitlabClient, err := g.createGitLabClient()
	if err != nil {
		return
	}

	// Fetch all projects
	allProjects, err := g.fetchGitLabProjects(gitlabClient)
	if err != nil {
		return
	}

	// Process projects concurrently
	g.processProjectsConcurrentlyCLI(allProjects, *baseDir)
}

func (g *GitlabClient) InitPullRequestAutomationTUI(baseDir *string) {
	if g.updatesChan == nil {
		// If there's no channel, we'd do nothing or return an error.
		g.logWriter.ErrorString("No updates channel provided for TUI automation.")
		return
	}

	var syncDir string
	if baseDir == nil {
		syncDir = g.getBaseDir(constant.PullFieldPath)
		if syncDir == "" {
			return
		}
	}
	g.logWriter.InfoString("Starting GitLab project automation")

	// Initialize the GitLab client
	gitlabClient, err := g.createGitLabClient()
	if err != nil {
		return
	}

	// Fetch all projects
	allProjects, err := g.fetchGitLabProjects(gitlabClient)
	if err != nil {
		return
	}

	// Process projects concurrently
	g.processProjectsConcurrentlyTUI(allProjects, syncDir)
}

// InitMergeAutomationFromDir walks the local directory, processes all Git repositories matching the pattern,
// and creates merge requests. Includes support for patterns like "backend/*" and exclusions.
func (g *GitlabClient) InitMergeAutomationFromDir() {
	g.logWriter.InfoString("Starting merge automation from directory...")

	// 1. Determine the base directory from configuration.
	baseDir := g.getBaseDir(constant.ContextValueDir)
	if baseDir == "" {
		g.logWriter.ErrorString("Base directory is empty")
		return
	}

	// 2. Retrieve include and exclude patterns.
	includePatterns := g.getFieldValuesWithSeparator(constant.ContextValueInclude, ",")
	excludePatterns := g.getFieldValuesWithSeparator(constant.ContextValueExclude, ",")

	// 3. Initialize a GitLab client for API operations.
	gitlabClient, err := g.createGitLabClient()
	if err != nil {
		g.logWriter.ErrorString("Error creating GitLab client: %v", err)
		return
	}

	// 4. Count the total number of Git repositories that match the pattern.
	totalPackages, err := countMatchingRepositories(baseDir, includePatterns, excludePatterns)
	if err != nil {
		g.logWriter.ErrorString("Error counting repositories: %v", err)
		return
	}

	// 5. Initialize a counter for the dynamic Index.
	index := 0

	// 6. Walk the base directory recursively.
	err = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // abort if there’s an error accessing a file
		}
		// Process only directories.
		if !info.IsDir() {
			return nil
		}

		// Check if the directory is a Git repository (by the existence of ".git").
		gitPath := filepath.Join(path, ".git")
		if _, err := os.Stat(gitPath); os.IsNotExist(err) {
			return nil // not a git repository; continue walking
		}

		// Compute a relative path for pattern matching.
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			relPath = path // fallback to full path
		}

		// 7. Validate repository against include/exclude rules.
		if !isValidRepo(relPath, includePatterns, excludePatterns) {
			g.logWriter.InfoString("Skipping repository (does not match patterns): %s", path)
			return filepath.SkipDir
		}

		g.logWriter.BlueString("Processing repository: %s", path)

		// 8. Get the current branch.
		currentBranch, err := getCurrentBranch(path)
		if err != nil {
			g.logWriter.ErrorString("Error getting current branch for %s: %v", path, err)
			return filepath.SkipDir
		}

		// 9. Handle checking out to "main" or "develop", if necessary, and resetting dirty repositories.
		if err := checkoutAndResetBranch(path, currentBranch, g.logWriter); err != nil {
			g.logWriter.ErrorString("Error handling branch for %s: %v", path, err)
			return filepath.SkipDir
		}

		// 10. Create a new branch from the current branch.
		branchName := g.getFieldValues(constant.MergeFieldBranch)
		if branchName == "" {
			g.logWriter.ErrorString("No branch name provided in context for repository: %s", path)
			return filepath.SkipDir
		}
		if err := CreateBranch(g.logWriter, path, branchName, currentBranch); err != nil {
			g.logWriter.ErrorString("Error creating branch in %s: %v", path, err)
			return nil
		}

		// 11. Retrieve and execute the command string from context.
		commandStr := g.getFieldValues(constant.MergeFieldCommand)
		if err := executeCommands(g.logWriter, path, commandStr); err != nil {
			g.logWriter.ErrorString("Error running commands for %s: %v", path, err)
			return filepath.SkipDir
		}

		// 12. Commit changes with the provided commit message.
		commitMsg := g.getFieldValues(constant.MergeFieldCommitMessage)
		if err := CommitChanges(g.logWriter, path, commitMsg); err != nil {
			g.logWriter.ErrorString("Error committing changes for %s: %v", path, err)
			return filepath.SkipDir
		}

		// 13. Push the new branch.
		if err := pushBranch(g.logWriter, path); err != nil {
			g.logWriter.ErrorString("Error pushing branch for %s: %v", path, err)
			return nil
		}

		// 14. Retrieve the GitLab project ID from the repository's remote URL.
		projectID, err := getProjectIDFromRepo(path, gitlabClient)
		if err != nil {
			g.logWriter.ErrorString("Error retrieving project ID for %s: %v", path, err)
			return nil
		}

		// 15. Create the merge request.
		targetBranch := g.getFieldValues(constant.MergeFieldMergeRequestTargetBranch)
		titleMsg := g.getFieldValues(constant.MergeFieldMergeRequestTitle)
		descriptionMsg := g.getFieldValues(constant.MergeFieldMergeRequestDescription)
		if err := createMergeRequest(g.logWriter, gitlabClient, projectID, targetBranch, branchName, titleMsg, descriptionMsg); err != nil {
			g.logWriter.ErrorString("Error creating merge request for %s: %v", path, err)
			return nil
		}

		// 16. Update the progress with dynamic index and total packages.
		index++
		g.updatesChan <- progressScreen.PackageUpdate{
			PackageName: path,
			Status:      true,
			TotalPkg:    totalPackages,
			Index:       index,
		}

		g.logWriter.GreenString("Merge request created successfully for %s", path)

		// Skip processing subdirectories inside this repository.
		return filepath.SkipDir
	})

	// Close the progress channel after processing all repositories.
	close(g.updatesChan)

	if err != nil {
		g.logWriter.ErrorString("Error walking directory: %v", err)
	}
}

// FetchDiffCLI runs a git log command between two branches (from "origin/<branchFrom>" to "origin/<branchTo>")
// in the repository located at repoPath and returns a formatted summary along with a boolean flag indicating
// whether differences exist.
func (g *GitlabClient) FetchDiffCLI(repoPath, branchFrom, branchTo string) (string, bool, error) {
	// Prepare the git log command.
	cmd := exec.Command("git", "log", "--pretty=format:%H - %s - %ai - %ar", "origin/"+branchFrom+"..origin/"+branchTo)
	cmd.Dir = repoPath

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false, fmt.Errorf("error running git log: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		return "No differences between " + branchFrom + " and " + branchTo, false, nil
	}

	lines := strings.Split(output, "\n")

	// Define a header style using lipgloss (optional for CLI formatting).
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("85"))
	header := strings.TrimSpace(headerStyle.Render("Number of different commits: " + strconv.Itoa(len(lines)) + "\n"))

	// Format each commit line.
	var formattedLines []string
	for i, line := range lines {
		// Split the line into 4 parts: commit hash, message, full date, and relative time.
		parts := strings.SplitN(line, " - ", 4)
		if len(parts) < 4 {
			continue
		}
		commitHash := strings.TrimSpace(parts[0])
		commitMessage := strings.TrimSpace(parts[1])
		fullDate := strings.TrimSpace(parts[2])
		// commitRelativeTime := strings.TrimSpace(parts[3])
		// Format as "1. commitMessage , commitHash , fullDate"
		formattedLine := fmt.Sprintf("%d. %s , %s , %s", i+1, commitMessage, commitHash, fullDate)
		formattedLines = append(formattedLines, formattedLine)
	}

	return header + strings.Join(formattedLines, "\n"), true, nil
}

// FetchDiffCLI runs a git log command between two branches (from "origin/<branchFrom>" to "origin/<branchTo>")
// in the repository located at repoPath and returns a formatted summary along with a boolean flag indicating
// whether differences exist.
//func (g *GitlabClient) FetchDiffCLI(repoPath, branchFrom, branchTo string) (string, bool, error) {
//	// Prepare the git log command.
//	cmd := exec.Command("git", "log", "--pretty=format:%H - %s - %ai - %ar", fmt.Sprintf("origin/%s..origin/%s", branchFrom, branchTo))
//	cmd.Dir = repoPath
//
//	var out bytes.Buffer
//	cmd.Stdout = &out
//
//	if err := cmd.Run(); err != nil {
//		return "", false, fmt.Errorf("error running git log: %w", err)
//	}
//
//	// Trim and check output.
//	output := strings.TrimSpace(out.String())
//	if output == "" {
//		return fmt.Sprintf("No differences between %s and %s", branchFrom, branchTo), false, nil
//	}
//
//	// Use a scanner to iterate over each line.
//	scanner := bufio.NewScanner(&out)
//	var lines []string
//	for scanner.Scan() {
//		line := scanner.Text()
//		if strings.TrimSpace(line) != "" {
//			lines = append(lines, line)
//		}
//	}
//	if err := scanner.Err(); err != nil {
//		return "", false, fmt.Errorf("error scanning output: %w", err)
//	}
//
//	// Define a header style using Lip Gloss.
//	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
//	header := headerStyle.Render(fmt.Sprintf("Number of different commits: %d\n", len(lines)))
//
//	// Build the formatted output.
//	var sb strings.Builder
//	sb.WriteString(header)
//	separator := " , "
//	for i, line := range lines {
//		// Split the line into commit hash, message, full date, and relative time.
//		parts := strings.SplitN(line, " - ", 4)
//		if len(parts) < 4 {
//			continue
//		}
//		commitMessage := strings.TrimSpace(parts[1])
//		fullDate := strings.TrimSpace(parts[2])
//		// Format as: "1. commitMessage , fullDate"
//		formattedLine := fmt.Sprintf("%d. %s%s%s", i+1, commitMessage, separator, fullDate)
//		sb.WriteString(formattedLine)
//		if i < len(lines)-1 {
//			sb.WriteString("\n")
//		}
//	}
//
//	return sb.String(), true, nil
//}
