package client

import (
	"bufio"
	"fmt"
	"github.com/sinaw369/Hermes/internal/logWriter"
	"gitlab.com/gitlab-org/api/client-go"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// Helper function to execute shell commands
func runCommand(logger *logWriter.Logger, dir, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		logger.ErrorString("Error obtaining StdoutPipe: %v", err)
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		logger.ErrorString("Error obtaining StderrPipe: %v", err)
		return err
	}

	logger.BlueString("Running command: %s args: %v", command, args)

	if err := cmd.Start(); err != nil {
		logger.ErrorString("Error starting command: %v", err)
		return err
	}

	// Use WaitGroup to wait for both stdout and stderr to be processed
	var wg sync.WaitGroup
	wg.Add(2)

	// Function to read from a pipe and send logs
	readPipe := func(pipe io.ReadCloser, prefix string) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := scanner.Text()
			switch prefix {
			case "STDOUT":
				logger.InfoString(line)
			case "STDERR":
				logger.RedString(line) // Treat stderr as errors
			}
		}
		if err := scanner.Err(); err != nil {
			logger.ErrorString("Error reading %s: %v", prefix, err)
		}
	}

	// Read stdout
	go readPipe(stdoutPipe, "STDOUT")

	// Read stderr
	go readPipe(stderrPipe, "STDERR")

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		logger.ErrorString("Command execution failed: %v", err)
		// Even if there's an error, the logs have been captured
	}

	// Wait for both stdout and stderr to be processed
	wg.Wait()
	return nil
}

// Clone or pull repository
func CloneOrPullRepo(logger *logWriter.Logger, repoURL, baseDir string) error {
	// Ensure the base directory exists or create it if it doesn't
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			logger.RedString("Failed to create base directory: %v", err)
			return fmt.Errorf("failed to create base directory: %v", err)
		}
	}
	// Parse the URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return err
	}
	trimPrefix := strings.TrimPrefix(u.Path, "/")
	trimPrefix = strings.TrimSuffix(trimPrefix, ".git")
	repoPath := filepath.Join(baseDir, trimPrefix)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		logger.BlueString("Cloning repository: %s", repoURL)
		return runCommand(logger, "", "git", "clone", repoURL, repoPath)
	} else {
		// Pull the latest changes
		logger.MagentaString("Pulling latest changes for repository: %s", repoURL)
		return runCommand(logger, repoPath, "git", "pull")
	}
}

// pushBranch pushes the current branch to GitLab.
func pushBranch(logger *logWriter.Logger, repoDir string) error {
	return runCommand(logger, repoDir, "git", "push", "-u", "origin", "HEAD")
}

// CreateBranch creates a new branch and switches to it.
func CreateBranch(logger *logWriter.Logger, repoDir, branchName, defaultBranch string) error {
	currentBranch, err := getCurrentBranch(repoDir)
	if err != nil {
		return err
	}

	if currentBranch != defaultBranch {
		if err := runCommand(logger, repoDir, "git", "checkout", defaultBranch); err != nil {
			return err
		}
	}

	if err := runCommand(logger, repoDir, "git", "checkout", "-b", branchName); err != nil {
		return err
	}

	logger.GreenString("Successfully created new branch: %s", branchName)
	return nil
}

// branchExists returns true if the given branch exists in the repository.
func branchExists(repoDir, branch string) bool {
	cmd := exec.Command("git", "branch", "--list", branch)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// isRepoDirty returns true if there are any uncommitted changes in the repository.
func isRepoDirty(repoDir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(string(out)) != "" {
		return true, nil
	}
	return false, nil
}

// isValidRepo checks whether the repository’s relative path matches the include/exclude criteria.
func isValidRepo(repoPath string, includePatterns, excludePatterns []string) bool {
	// Use path.Match for wildcard matching.
	for _, includePattern := range includePatterns {
		matched, err := path.Match(includePattern, repoPath)
		if err != nil {
			continue
		}
		if matched {
			// If an exclude pattern matches, skip this repository.
			for _, excludePattern := range excludePatterns {
				matched, err := path.Match(excludePattern, repoPath)
				if err != nil {
					continue
				}
				if matched {
					return false
				}
			}
			return true
		}
	}
	return false
}

// getCurrentBranch returns the current branch of the repository.
func getCurrentBranch(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getProjectIDFromRepo retrieves the project ID by parsing the remote URL.
// It handles both SSH URL formats:
//   - "ssh://git@git.*.app:2222/s.hatami/test.git"
//   - "git@git.*.app:s.hatami/test.git"
func getProjectIDFromRepo(repoDir string, client *gitlab.Client) (interface{}, error) {
	// Get the remote URL using git config.
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting remote URL: %v", err)
	}
	remoteURL := strings.TrimSpace(string(out))

	// Parse the remote URL to extract the project path.
	var projectPath string
	if strings.HasPrefix(remoteURL, "ssh://") {
		u, err := url.Parse(remoteURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing remote URL: %v", err)
		}
		projectPath = strings.TrimPrefix(u.Path, "/")
		projectPath = strings.TrimSuffix(projectPath, ".git")
	} else {
		parts := strings.Split(remoteURL, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("cannot parse remote URL: %s", remoteURL)
		}
		projectPath = strings.TrimSuffix(parts[1], ".git")
	}
	project, _, err := client.Projects.GetProject(projectPath, nil)
	if err != nil {
		return nil, fmt.Errorf("project not found for remote URL: %s; error: %v", remoteURL, err)
	}
	return project.ID, nil
}

// createMergeRequest creates a merge request on GitLab.
func createMergeRequest(logger *logWriter.Logger, gitlabClient *gitlab.Client, projectID interface{}, targetBranch, branchName, titleMsg, descriptionMsg string) error {
	user, _, err := gitlabClient.Users.CurrentUser()
	if err != nil {
		logger.ErrorString("Failed to fetch current user: %v", err)
		return err
	}

	mrOptions := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       &branchName,
		TargetBranch:       gitlab.Ptr(targetBranch),
		Title:              gitlab.Ptr(fmt.Sprintf("Merge branch '%s' into %s", branchName, targetBranch)),
		Description:        gitlab.Ptr("Automatically created merge request after running `go mod tidy`."),
		AssigneeID:         &user.ID,
		RemoveSourceBranch: gitlab.Ptr(true),
	}
	if titleMsg != "" {
		mrOptions.Title = gitlab.Ptr(titleMsg)
	}
	if descriptionMsg != "" {
		mrOptions.Description = gitlab.Ptr(descriptionMsg)
	}
	_, _, err = gitlabClient.MergeRequests.CreateMergeRequest(projectID, mrOptions)
	if err != nil {
		logger.ErrorString("Failed to create merge request: %v", err)
		return fmt.Errorf("failed to create merge request: %v", err)
	}

	logger.GreenString("Merge request created for branch: %s", branchName)
	return nil
}

// CommitChanges and pushes them to GitLab.
func CommitChanges(logger *logWriter.Logger, repoDir, commitMsg string) error {
	if err := runCommand(logger, repoDir, "git", "add", "."); err != nil {
		return err
	}

	err := runCommand(logger, repoDir, "git", "commit", "-m", commitMsg)
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return nil
		}
		return err
	}
	return nil
}

// countMatchingRepositories counts the total number of Git repositories
// that match the include/exclude patterns.
func countMatchingRepositories(baseDir string, includePatterns, excludePatterns []string) (int, error) {
	var count int
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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

		// Validate repository against include/exclude rules.
		if isValidRepo(relPath, includePatterns, excludePatterns) {
			count++
		}
		return nil
	})
	return count, err
}

// executeCommands splits the command string and executes each command in the repository directory.
func executeCommands(logger *logWriter.Logger, repoDir, commandStr string) error {
	// Split the command string by semicolon to separate commands.
	commands := strings.Split(commandStr, ";")
	for _, cmdStr := range commands {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}
		// Split the command string by spaces to separate the command from its arguments.
		parts := strings.Split(cmdStr, " ")
		cmdName := parts[0]
		cmdArgs := parts[1:]

		// Log and run the command.
		logger.BlueString("Running command: %s with args %v in %s", cmdName, cmdArgs, repoDir)
		if err := runCommand(logger, repoDir, cmdName, cmdArgs...); err != nil {
			logger.ErrorString("Error running command '%s' in %s: %v", cmdStr, repoDir, err)
			// Continue with the next command, if any.
		}
	}
	return nil
}

// checkoutAndResetBranch checks out to 'develop' or 'main' if not already on either.
// If the branch is dirty, it will reset all uncommitted changes.
func checkoutAndResetBranch(path, currentBranch string) error {
	if currentBranch != "main" && currentBranch != "develop" {
		if branchExists(path, "develop") {
			if err := runCommand(nil, path, "git", "checkout", "develop"); err != nil {
				return fmt.Errorf("error checking out 'develop': %v", err)
			}
		} else if branchExists(path, "main") {
			if err := runCommand(nil, path, "git", "checkout", "main"); err != nil {
				return fmt.Errorf("error checking out 'main': %v", err)
			}
		} else {
			return fmt.Errorf("no 'main' or 'develop' branch available")
		}
	}

	// Ensure the branch is clean.
	if dirty, err := isRepoDirty(path); err != nil {
		return fmt.Errorf("error checking repo cleanliness: %v", err)
	} else if dirty {
		if err := runCommand(nil, path, "git", "reset", "--hard"); err != nil {
			return fmt.Errorf("error resetting dirty repository: %v", err)
		}
	}

	return nil
}
