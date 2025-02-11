// File: client/gitlab_client.go
package client

import (
	"context"
	"fmt"
	"github.com/sinaw369/Hermes/internal/config"
	"github.com/sinaw369/Hermes/internal/constant"
	"github.com/sinaw369/Hermes/internal/form/logsScreen"
	"github.com/sinaw369/Hermes/internal/form/progressScreen"
	"github.com/sinaw369/Hermes/internal/logWriter"
	"gitlab.com/gitlab-org/api/client-go"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// GitlabClient manages GitLab interactions.
type GitlabClient struct {
	gitlabToken string
	gitlabURL   string
	updatesChan chan<- progressScreen.PackageUpdate
	contextMap  map[string]string
	logWriter   *logWriter.Logger
}

// NewTUIGitClient is for TUI usage: it accepts an updates channel and a TUI logs model.
func NewTUIGitClient(
	ctx context.Context,
	updatesChan chan<- progressScreen.PackageUpdate,
	contextMap map[string]string,
	cfg *config.Config,
	logsModel *logsScreen.LogModel,
) (*GitlabClient, error) {

	// We rely on TUI logs for output
	// Optionally store ctx for cancellation/timeouts
	return newGitlabClient(ctx, updatesChan, contextMap, cfg, logsModel)
}

// NewCLIGitClient is for CLI usage: no TUI logs, no progress channel.
// Typically, you don't need a channel at all, or you pass nil to skip sending.
func NewCLIGitClient(
	ctx context.Context,
	contextMap map[string]string,
	cfg *config.Config,
) (*GitlabClient, error) {
	return newGitlabClient(ctx, nil, contextMap, cfg, nil)
}

// newGitlabClient is the common internal constructor that reads GITLAB_TOKEN, etc.
func newGitlabClient(
	ctx context.Context,
	updatesChan chan<- progressScreen.PackageUpdate,
	contextMap map[string]string,
	cfg *config.Config,
	logsModel *logsScreen.LogModel,
) (*GitlabClient, error) {

	// Fetch the GitLab token and URL from environment variables
	gitlabToken := cfg.GitlabToken
	gitlabURL := cfg.GitlabBaseURL

	if gitlabToken == "" || gitlabURL == "" {
		return nil, fmt.Errorf("error: GITLAB_TOKEN or GITLAB_BASE_URL is not set in environment")
	}
	// Check if the detach mode flag is set
	disabled := false
	if contextMap[constant.DetachMode] == "YES" {
		disabled = true
	}

	var log *logWriter.Logger
	if logsModel != nil {
		tab := logsModel.AddTab(constant.LGitClient)
		log = logWriter.NewLogger(tab, true, disabled)
		log.InfoString("Starting the GitLab client for TUI usage...")
	} else {
		log = logWriter.NewLogger(os.Stdout, true, disabled)
		log.InfoString("Starting the GitLab client for CLI usage...")
	}

	client := &GitlabClient{
		gitlabToken: gitlabToken,
		gitlabURL:   gitlabURL,
		updatesChan: updatesChan,
		contextMap:  contextMap,
		logWriter:   log,
	}

	return client, nil
}

// getBaseDir returns the base directory for the project, either from context or default.
func (g *GitlabClient) getBaseDir(field string) string {

	if g.getFieldValues(field) == "" {
		homeDir, err := os.Getwd()
		if err != nil {
			g.logWriter.ErrorString("Error fetching home directory: %v", err)
			return ""
		}
		return filepath.Join(homeDir, "git-repos")
	}
	return g.getFieldValues(field)
}

// getFieldValuesWithSeparator retrieves a string from the context map using the provided key,
// splits it using the specified separator, and trims any surrounding whitespace from each value.
func (g *GitlabClient) getFieldValuesWithSeparator(field, separator string) []string {
	rawValue := g.contextMap[field]
	if rawValue == "" {
		// Optionally, you can log an error or return nil.
		return nil
	}
	values := strings.Split(rawValue, separator)
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	return values
}

// getFieldValuesWithSeparator retrieves a string from the context map using the provided key,
// splits it using the specified separator, and trims any surrounding whitespace from each value.
func (g *GitlabClient) getFieldValues(field string) string {
	rawValue := g.contextMap[field]
	if rawValue == "" {
		// Optionally, you can log an error or return nil.
		return ""
	}
	return rawValue
}

// createGitLabClient initializes a new GitLab client.
func (g *GitlabClient) createGitLabClient() (*gitlab.Client, error) {
	gitlabClient, err := gitlab.NewClient(g.gitlabToken, gitlab.WithBaseURL(g.gitlabURL))
	if err != nil {
		g.logWriter.RedString("Error creating GitLab client: %v", err)
	}
	return gitlabClient, err
}

// fetchGitLabProjects retrieves all projects from GitLab.
func (g *GitlabClient) fetchGitLabProjects(client *gitlab.Client) ([]*gitlab.Project, error) {
	listOptions := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
	}
	var allProjects []*gitlab.Project

	for {
		projects, resp, err := client.Projects.ListProjects(listOptions)
		if err != nil {
			g.logWriter.ErrorString("Error fetching GitLab projects: %v", err)
			return nil, err
		}

		for _, project := range projects {
			if g.shouldIncludeProject(project) {
				allProjects = append(allProjects, project)
				g.logWriter.YellowString("Appended project: %s", project.SSHURLToRepo)
			}
		}

		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		listOptions.Page = resp.NextPage
	}

	return allProjects, nil
}

// shouldIncludeProject checks if a GitLab project matches the include criteria.
func (g *GitlabClient) shouldIncludeProject(project *gitlab.Project) bool {
	// If Include fields are empty, they should be ignored.
	includeSSHURL := g.contextMap[constant.PullFieldSSHURLInclude]
	includeField := g.contextMap[constant.PullFieldInclude]
	excludeField := g.contextMap[constant.PullFieldExclude]

	// If all include fields are empty, return true (not filtering)
	if includeSSHURL == "" && includeField == "" && excludeField == "" {
		return true
	}

	// Check SSH URL includes the desired SSH URL pattern if not empty
	if includeSSHURL != "" && !strings.Contains(project.SSHURLToRepo, includeSSHURL) {
		return false
	}

	// Check if the project contains the specified "include" field, if not empty
	if includeField != "" && !strings.Contains(project.SSHURLToRepo, includeField) {
		return false
	}

	// Check if the project contains the specified "exclude" field, if not empty
	if excludeField != "" && strings.Contains(project.SSHURLToRepo, excludeField) {
		return false
	}

	return true
}

// processProjectsConcurrently processes the projects with concurrency.
func (g *GitlabClient) processProjectsConcurrentlyTUI(projects []*gitlab.Project, baseDir string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Limit to 10 concurrent operations

	for idx, project := range projects {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		go func(idx int, project *gitlab.Project) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			repoURL := project.SSHURLToRepo
			g.logWriter.BlueString("Processing repository: %s", repoURL)

			err := CloneOrPullRepo(g.logWriter, repoURL, baseDir)
			if err != nil {
				g.logWriter.ErrorString("Error cloning/pulling repository: %v", err)
				g.updatesChan <- progressScreen.PackageUpdate{
					PackageName: repoURL,
					Status:      false,
					TotalPkg:    len(projects),
					Index:       idx,
				}
				return
			}

			g.updatesChan <- progressScreen.PackageUpdate{
				PackageName: repoURL,
				Status:      true,
				TotalPkg:    len(projects),
				Index:       idx,
			}
		}(idx, project)
	}

	wg.Wait()
	g.logWriter.GreenString("Finished processing all repositories.")
	close(g.updatesChan)
}

// processProjectsConcurrently processes the projects with concurrency.
func (g *GitlabClient) processProjectsConcurrentlyCLI(projects []*gitlab.Project, baseDir string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Limit to 10 concurrent operations

	for idx, project := range projects {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		go func(idx int, project *gitlab.Project) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			repoURL := project.SSHURLToRepo
			g.logWriter.BlueString("Processing repository: %s", repoURL)

			err := CloneOrPullRepo(g.logWriter, repoURL, baseDir)
			if err != nil {
				g.logWriter.ErrorString("Error cloning/pulling repository: %v", err)
				return
			}
		}(idx, project)
	}

	wg.Wait()
	g.logWriter.GreenString("Finished processing all repositories.")
}
