package client

import (
	"github.com/sinaw369/Hermes/constants"
	"github.com/sinaw369/Hermes/forms/progressScreen"
	"os"
	"path/filepath"
)

// InitPullRequestAutomation handles GitLab project automation tasks.
func (g *GitlabClient) InitPullRequestAutomation() {
	g.logWriter.InfoString("Starting GitLab project automation")

	// Determine the base directory
	baseDir := g.getBaseDir(constants.PullFieldPath)
	if baseDir == "" {
		return
	}

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
	g.processProjectsConcurrently(allProjects, baseDir)
}

// InitMergeAutomationFromDir walks the local directory, processes all Git repositories matching the pattern,
// and creates merge requests. Includes support for patterns like "backend/*" and exclusions.
func (g *GitlabClient) InitMergeAutomationFromDir() {
	g.logWriter.InfoString("Starting merge automation from directory...")

	// 1. Determine the base directory from configuration.
	baseDir := g.getBaseDir(constants.MergeFieldPath)
	if baseDir == "" {
		g.logWriter.ErrorString("Base directory is empty")
		return
	}

	// 2. Retrieve include and exclude patterns.
	includePatterns := g.getFieldValuesWithSeparator(constants.MergeFieldInclude, ",")
	excludePatterns := g.getFieldValuesWithSeparator(constants.MergeFieldExclude, ",")

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
		if err := checkoutAndResetBranch(path, currentBranch); err != nil {
			g.logWriter.ErrorString("Error handling branch for %s: %v", path, err)
			return filepath.SkipDir
		}

		// 10. Create a new branch from the current branch.
		branchName := g.getFieldValues(constants.MergeFieldBranch)
		if branchName == "" {
			g.logWriter.ErrorString("No branch name provided in context for repository: %s", path)
			return filepath.SkipDir
		}
		if err := CreateBranch(g.logWriter, path, branchName, currentBranch); err != nil {
			g.logWriter.ErrorString("Error creating branch in %s: %v", path, err)
			return nil
		}

		// 11. Retrieve and execute the command string from context.
		commandStr := g.getFieldValues(constants.MergeFieldCommand)
		if err := executeCommands(g.logWriter, path, commandStr); err != nil {
			g.logWriter.ErrorString("Error running commands for %s: %v", path, err)
			return filepath.SkipDir
		}

		// 12. Commit changes with the provided commit message.
		commitMsg := g.getFieldValues(constants.MergeFieldCommitMessage)
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
		targetBranch := g.getFieldValues(constants.MergeFieldMergeRequestTargetBranch)
		titleMsg := g.getFieldValues(constants.MergeFieldMergeRequestTitle)
		descriptionMsg := g.getFieldValues(constants.MergeFieldMergeRequestDescription)
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
