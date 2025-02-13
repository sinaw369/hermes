package client

import (
	"bufio"
	"fmt"
	"github.com/sinaw369/Hermes/internal/constant"
	"github.com/sinaw369/Hermes/internal/logWriter"
	"gitlab.com/gitlab-org/api/client-go"
	"golang.org/x/sync/errgroup"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// runCommand executes a shell command in the specified directory and logs its output.
// It uses a context to allow cancellation/timeouts and errgroup to run stdout and stderr reading concurrently.
func runCommand(logger *logWriter.Logger, dir, command string, args ...string) error {
	// Create the command with context support.
	cmd := exec.Command(command, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	logger.BlueString("Running command: %s args: %v", command, args)

	// Obtain pipes for stdout and stderr.
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

	// Start the command.
	if err := cmd.Start(); err != nil {
		logger.ErrorString("Error starting command: %v", err)
		return err
	}

	// Use errgroup to concurrently read from stdout and stderr.
	var g errgroup.Group

	g.Go(func() error {
		return scanAndLog(stdoutPipe, "STDOUT", logger)
	})
	g.Go(func() error {
		return scanAndLog(stderrPipe, "STDERR", logger)
	})

	// Wait for the output scanning goroutines.
	if err := g.Wait(); err != nil {
		logger.ErrorString("Error reading command output: %v", err)
		// Continue even if there's an error in scanning
	}

	// Wait for the command to complete.
	if err := cmd.Wait(); err != nil {
		logger.ErrorString("Command execution failed: %v", err)
		logger.ErrorString("dir:%v ,command:%v, args:%v", dir, command, args)
		return err
	}
	return nil
}

// scanAndLog reads from the provided pipe line-by-line and logs the output.
// It closes the pipe when done.
func scanAndLog(pipe io.ReadCloser, prefix string, logger *logWriter.Logger) error {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		switch prefix {
		case "STDOUT":
			logger.InfoString(line)
		case "STDERR":
			logger.MagentaString(line)
		}
	}
	if err := scanner.Err(); err != nil {
		// If the error indicates that the file is already closed, ignore it.
		if strings.Contains(err.Error(), "file already closed") {
			logger.InfoString("Ignored error reading %s: %v", prefix, err)
			return nil
		}
		return fmt.Errorf("error reading %s: %w", prefix, err)
	}
	return nil
}

// CloneOrPullRepo updates a repository by either pulling only the default branch (if configured)
// or pulling all remote branches.
// It stashes any uncommitted changes before pulling and then applies the stash using "git stash apply".
// If conflicts occur during stash apply, it aborts the merge and resets the repository to a safe commit.
func (g *GitlabClient) CloneOrPullRepo(logger *logWriter.Logger, repoURL, baseDir string) error {
	// Ensure the base directory exists.
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			logger.RedString("Failed to create base directory: %v", err)
			return fmt.Errorf("failed to create base directory: %v", err)
		}
	}

	// Parse the repository URL.
	u, err := url.Parse(repoURL)
	if err != nil {
		return err
	}
	trimPrefix := strings.TrimPrefix(u.Path, "/")
	trimPrefix = strings.TrimSuffix(trimPrefix, ".git")
	repoPath := filepath.Join(baseDir, trimPrefix)

	// If the repository doesn't exist locally, clone it.
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		logger.BlueString("Cloning repository: %s", repoURL)
		return runCommand(logger, "", "git", "clone", repoURL, repoPath)
	}

	// Repository exists; update it.
	logger.MagentaString("Updating repository: %s", repoURL)

	// Fetch all remote changes.
	if err := runCommand(logger, repoPath, "git", "fetch", "--all"); err != nil {
		return err
	}

	// If the context flag is set to pull only the default branch:
	if flag, ok := g.contextMap[constant.ContextValuePullDefault]; ok && flag == constant.ContextValueYES {
		branchToPull := g.contextMap[constant.ContextValuePullBranch]
		if branchToPull == "" {
			return fmt.Errorf("pull branch cant be empty")
		}
		logger.InfoString("Pulling only branch: %s", branchToPull)

		// Checkout the default branch.
		if err := runCommand(logger, repoPath, "git", "checkout", branchToPull); err != nil {
			logger.ErrorString("Error checking out branch %s: %v", branchToPull, err)
			return err
		}

		// Record the current commit as safe state.
		origHead, err := getCurrentCommit(repoPath)
		if err != nil {
			logger.ErrorString("Error getting current commit: %v", err)
			return err
		}

		// Check if repository is dirty and stash if needed.
		dirty, err := isRepoDirty(repoPath)
		if err != nil {
			logger.ErrorString("Error checking repository status: %v", err)
			return err
		}
		stashed := false
		if dirty {
			logger.InfoString("Stashing uncommitted changes on branch: %s", branchToPull)
			if err := runCommand(logger, repoPath, "git", "stash"); err != nil {
				logger.ErrorString("Error stashing changes: %v", err)
				return err
			}
			stashed = true
		}

		// Pull the latest changes.
		if err := runCommand(logger, repoPath, "git", "pull"); err != nil {
			logger.ErrorString("Error pulling branch %s: %v", branchToPull, err)
			return err
		}

		// If changes were stashed, attempt to apply them.
		if stashed {
			logger.InfoString("Applying stashed changes on branch: %s", branchToPull)
			if err := runCommand(logger, repoPath, "git", "stash", "apply"); err != nil {
				logger.ErrorString("Error applying stash on branch %s: %v", branchToPull, err)
				statusOutput, _ := getGitStatus(repoPath)
				if strings.Contains(statusOutput, "UU") {
					logger.ErrorString("Merge conflicts detected after stash apply on branch %s. Aborting pull...", branchToPull)
					abortPull(repoPath, logger)
					resetRepo(repoPath, logger, origHead)
					runCommand(logger, repoPath, "git", "stash", "apply")
					return fmt.Errorf("conflicts encountered when applying stash on branch %s", branchToPull)
				}
			} else {
				if err := runCommand(logger, repoPath, "git", "stash", "drop"); err != nil {
					logger.ErrorString("Error dropping stash on branch %s: %v", branchToPull, err)
				}
			}
		}
	} else {
		// Otherwise, pull all remote branches.
		currentBranch, err := getCurrentBranch(repoPath)
		if err != nil {
			logger.ErrorString("Error getting current branch: %v", err)
			return err
		}

		branches, err := getRemoteBranches(repoPath)
		if err != nil {
			logger.ErrorString("Error getting remote branches: %v", err)
			return err
		}

		for _, branch := range branches {
			// Check if repository is dirty.
			dirty, err := isRepoDirty(repoPath)
			if err != nil {
				logger.ErrorString("Error checking repository status: %v", err)
				continue
			}
			stashed := false
			if dirty {
				logger.InfoString("Stashing uncommitted changes for branch %s", branch)
				if err := runCommand(logger, repoPath, "git", "stash"); err != nil {
					logger.ErrorString("Error stashing changes: %v", err)
					continue
				}
				stashed = true
			}

			// Convert remote branch name to local branch name (e.g. "origin/feature" -> "feature").
			parts := strings.Split(branch, "/")
			localBranch := parts[len(parts)-1]

			logger.InfoString("Checking out branch: %s", localBranch)
			if err := runCommand(logger, repoPath, "git", "checkout", "-B", localBranch, branch); err != nil {
				logger.ErrorString("Error checking out branch %s: %v", localBranch, err)
				continue
			}

			origHead, err := getCurrentCommit(repoPath)
			if err != nil {
				logger.ErrorString("Error getting current commit: %v", err)
				continue
			}

			logger.InfoString("Pulling latest changes on branch: %s", localBranch)
			if err := runCommand(logger, repoPath, "git", "pull"); err != nil {
				logger.ErrorString("Error pulling branch %s: %v", localBranch, err)
				continue
			}

			if stashed {
				logger.InfoString("Applying stashed changes on branch: %s", localBranch)
				if err := runCommand(logger, repoPath, "git", "stash", "apply"); err != nil {
					logger.ErrorString("Error applying stash on branch %s: %v", localBranch, err)
					statusOutput, _ := getGitStatus(repoPath)
					if strings.Contains(statusOutput, "UU") {
						logger.ErrorString("Merge conflicts detected after stash apply on branch %s. Aborting pull...", localBranch)
						abortPull(repoPath, logger)
						resetRepo(repoPath, logger, origHead)
						runCommand(logger, repoPath, "git", "stash", "apply")
						continue
					}
				} else {
					if err := runCommand(logger, repoPath, "git", "stash", "drop"); err != nil {
						logger.ErrorString("Error dropping stash on branch %s: %v", localBranch, err)
					}
				}
			}
		}

		// Finally, switch back to the original branch.
		if err := runCommand(logger, repoPath, "git", "checkout", currentBranch); err != nil {
			logger.ErrorString("Error checking out branch %s: %v", currentBranch, err)
			return err
		}
	}

	return nil
}

// getGitStatus runs "git status --porcelain" and returns its output.
func getGitStatus(repoPath string) (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// getCurrentCommit returns the current commit hash of the repository.
func getCurrentCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting current commit: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// abortPull attempts to abort any merge in progress.
func abortPull(repoPath string, logger *logWriter.Logger) {
	logger.InfoString("Aborting merge/pull in repository.")
	if err := runCommand(logger, repoPath, "git", "merge", "--abort"); err != nil {
		logger.ErrorString("Error aborting merge: %v", err)
	}
}

// resetRepo resets the repository to the given commit.
func resetRepo(repoPath string, logger *logWriter.Logger, commit string) {
	logger.InfoString("Resetting repository to commit: %s", commit)
	if err := runCommand(logger, repoPath, "git", "reset", "--hard", commit); err != nil {
		logger.ErrorString("Error resetting repository: %v", err)
	}
}

// getRemoteBranches runs "git branch -r" and returns a slice of remote branch names.
func getRemoteBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "branch", "-r")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "->") {
			continue
		}
		branches = append(branches, line)
	}
	return branches, nil
}

// getCurrentBranch returns the current branch name.
func getCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting current branch: %v", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("empty branch name")
	}
	return branch, nil
}

// isRepoDirty returns true if there are uncommitted changes.
func isRepoDirty(repoPath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("error checking repository status: %v", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
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
func checkoutAndResetBranch(path, currentBranch string, logger *logWriter.Logger) error {
	//TODO : this branch should be from cfg
	if currentBranch != "main" && currentBranch != "develop" {
		if branchExists(path, "develop") {
			if err := runCommand(logger, path, "git", "checkout", "develop"); err != nil {
				return fmt.Errorf("error checking out 'develop': %v", err)
			}
		} else if branchExists(path, "main") {
			if err := runCommand(logger, path, "git", "checkout", "main"); err != nil {
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
		if err := runCommand(logger, path, "git", "reset", "--hard"); err != nil {
			return fmt.Errorf("error resetting dirty repository: %v", err)
		}
	}

	return nil
}
