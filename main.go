// File: main.go
package main

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sinaw369/Hermes/client"
	"github.com/sinaw369/Hermes/constants"
	"github.com/sinaw369/Hermes/forms/logsScreen"
	"github.com/sinaw369/Hermes/forms/progressScreen"
	"github.com/sinaw369/Hermes/forms/screen"
	"github.com/sinaw369/Hermes/logWriter"
	"github.com/sinaw369/Hermes/messages"
	"log"
	"time"
)

// Screen defines the various screens in the application.
type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenList
	ScreenPull
	ScreenProgress
	ScreenLogs
	ScreenAutoMergeReq
	ScreenQuit
)

var (
	quitTextStyle = lipgloss.NewStyle().Margin(1, 0, 2, 4)
	logoStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF06B7")).Align(lipgloss.Center)
)

// Model defines the state of the entire application.
type Model struct {
	currentScreen      Screen
	optionList         *SyncerList.Model
	pullScreen         *screen.Model
	autoMergeReqScreen *screen.Model
	progressScreen     *progressScreen.Model
	logsScreen         *logsScreen.LogModel
	quitting           bool
	logWriter          *logWriter.Logger
	width              int // Window width
	height             int // Window height
}

// Init initializes the application; no initial command is needed.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the application state.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle quitting.
	if m.quitting {
		m.logWriter.InfoString("Quitting the application.")
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case messages.BackMsg:
		m.logWriter.InfoString("Received BackMsg. Handling back navigation.")
		return m.handleBack()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logWriter.InfoString("Window size changed: Width=%d, Height=%d", m.width, m.height)
		return m.updateCurrentScreenSize(msg)

	default:
		// Delegate message handling to the current screen.
		switch m.currentScreen {
		case ScreenWelcome:
			return m.updateWelcomeScreen(msg)
		case ScreenList:
			return m.updateListScreen(msg)
		case ScreenPull:
			return m.updatePullScreen(msg)
		case ScreenAutoMergeReq:
			return m.updateAutoMergeScreen(msg)
		case ScreenProgress:
			return m.updateProgressScreen(msg)
		case ScreenLogs:
			return m.updateLogsScreen(msg)
		}
	}

	return m, nil
}

// updateCurrentScreenSize delegates window size updates to the current screen.
func (m *Model) updateCurrentScreenSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case ScreenLogs:
		updatedLogsScreen, cmd := m.logsScreen.Update(msg)
		m.logsScreen = updatedLogsScreen.(*logsScreen.LogModel)
		return m, cmd
	case ScreenProgress:
		updatedProgressScreen, cmd := m.progressScreen.Update(msg)
		m.progressScreen = updatedProgressScreen.(*progressScreen.Model)
		return m, cmd
	default:
		return m, nil
	}
}

// handleBack manages the transition when a BackMsg is received.
func (m *Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case ScreenList:
		m.currentScreen = ScreenWelcome
	case ScreenPull, ScreenLogs, ScreenProgress, ScreenAutoMergeReq:
		m.currentScreen = ScreenList
	default:
		m.currentScreen = ScreenWelcome
	}
	m.logWriter.InfoString("Navigated back to screen: %v", m.currentScreen)
	return m, nil
}

// updateWelcomeScreen handles updates specific to the Welcome Screen.
func (m *Model) updateWelcomeScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			m.logWriter.InfoString("Enter pressed. Switching to List Screen.")
			m.currentScreen = ScreenList
			return m, nil
		case "q", "ctrl+c":
			m.logWriter.InfoString("Quit command received. Quitting application.")
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// updateListScreen handles updates specific to the List Screen.
func (m *Model) updateListScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedList, cmd := m.optionList.Update(msg)
	m.optionList = updatedList.(*SyncerList.Model)

	// Check if an option was selected.
	if m.optionList.Choice != "" {
		m.logWriter.InfoString("Option selected: %s", m.optionList.Choice)
		switch m.optionList.Choice {
		case constants.OptionListPullPr:
			m.currentScreen = ScreenPull
		case constants.OptionListAutoMergeReq:
			m.currentScreen = ScreenAutoMergeReq
		case constants.OptionListLogs:
			m.logWriter.YellowString("Switching to Logs Screen...")
			m.currentScreen = ScreenLogs
		case "Quit":
			m.logWriter.InfoString("Quit option selected. Quitting application.")
			m.quitting = true
			return m, tea.Quit
		}
		m.optionList.Choice = ""
	}

	return m, cmd
}

// updatePullScreen handles updates specific to the Pull Screen.
func (m *Model) updatePullScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedPullScreen, cmd := m.pullScreen.Update(msg)
	m.pullScreen = updatedPullScreen.(*screen.Model)

	// If the form was submitted, begin GitLab processing.
	if m.pullScreen.Submitted {
		m.logWriter.BlueString("Form submission complete. Starting processing...")
		m.logWriter.YellowString("Switching to Progress Screen...")
		m.currentScreen = ScreenProgress

		// Create updates channel and a context for cancellation.
		updatesChan := make(chan progressScreen.PackageUpdate)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		// In a more advanced scenario, you might store cancel() to trigger cancellation later.
		defer cancel()

		// Collect form values.
		values := m.pullScreen.GetValue()

		// Initialize the GitLab client with the context.
		gClient, err := gitClient(ctx, updatesChan, values, m.logsScreen, m.logWriter)
		if err != nil {
			m.logWriter.RedString("GitClient Initialization Failed: %v", err)
			m.currentScreen = ScreenLogs
			m.logsScreen.SetActiveTabByName(constants.LGitClient)

			updatedLogsScreen, logCmd := m.logsScreen.Update(messages.BackMsg{})
			m.logsScreen = updatedLogsScreen.(*logsScreen.LogModel)
			m.logWriter.InfoString("Switched to Logs Screen due to GitClient initialization failure.")
			return m, logCmd
		}

		m.logWriter.YellowString("Pull Automation Starting...")
		// Launch the GitLab client processing in a separate goroutine.
		go gClient.InitPullRequestAutomation() // (Note: If desired, gClient can be enhanced to accept the context.)

		// Initialize the progress screen with the updates channel.
		m.progressScreen = progressScreen.NewModel(updatesChan, m.logWriter)
		return m, m.progressScreen.Init()
	}

	return m, cmd
}

// updateAutoMergeScreen handles updates specific to the Pull Screen.
func (m *Model) updateAutoMergeScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedAutoMergeReqScreen, cmd := m.autoMergeReqScreen.Update(msg)
	m.autoMergeReqScreen = updatedAutoMergeReqScreen.(*screen.Model)

	// If the form was submitted, begin GitLab processing.
	if m.autoMergeReqScreen.Submitted {
		m.logWriter.BlueString("Form submission complete. Starting processing...")
		m.logWriter.YellowString("Switching to Progress Screen...")
		m.currentScreen = ScreenProgress

		// Create updates channel and a context for cancellation.
		updatesChan := make(chan progressScreen.PackageUpdate)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		// In a more advanced scenario, you might store cancel() to trigger cancellation later.
		defer cancel()

		// Collect form values.
		values := m.autoMergeReqScreen.GetValue()

		// Initialize the GitLab client with the context.
		gClient, err := gitClient(ctx, updatesChan, values, m.logsScreen, m.logWriter)
		if err != nil {
			m.logWriter.RedString("GitClient Initialization Failed: %v", err)
			m.currentScreen = ScreenLogs
			m.logsScreen.SetActiveTabByName(constants.LGitClient)

			updatedLogsScreen, logCmd := m.logsScreen.Update(messages.BackMsg{})
			m.logsScreen = updatedLogsScreen.(*logsScreen.LogModel)
			m.logWriter.InfoString("Switched to Logs Screen due to GitClient initialization failure.")
			return m, logCmd
		}

		m.logWriter.YellowString("Pull Automation Starting...")
		// Launch the GitLab client processing in a separate goroutine.
		go gClient.InitMergeAutomationFromDir() // (Note: If desired, gClient can be enhanced to accept the context.)

		// Initialize the progress screen with the updates channel.
		m.progressScreen = progressScreen.NewModel(updatesChan, m.logWriter)
		return m, m.progressScreen.Init()
	}

	return m, cmd
}

// updateProgressScreen handles updates specific to the Progress Screen.
func (m *Model) updateProgressScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedProgressScreen, cmd := m.progressScreen.Update(msg)
	m.progressScreen = updatedProgressScreen.(*progressScreen.Model)

	// When processing is complete, transition to the Logs Screen.
	if m.progressScreen.Done() {
		m.logWriter.YellowString("Processing complete. Switching to Logs Screen...")
		m.currentScreen = ScreenLogs
		return m, nil
	}

	return m, cmd
}

// updateLogsScreen handles updates specific to the Logs Screen.
func (m *Model) updateLogsScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedLogsScreen, cmd := m.logsScreen.Update(msg)
	m.logsScreen = updatedLogsScreen.(*logsScreen.LogModel)
	return m, cmd
}

// View renders the UI based on the current screen.
func (m *Model) View() string {
	if m.quitting {
		return quitTextStyle.Render("Thanks for using Hermes!")
	}

	switch m.currentScreen {
	case ScreenWelcome:
		return m.viewWelcomeScreen()
	case ScreenList:
		return m.optionList.View()
	case ScreenPull:
		return m.pullScreen.View()
	case ScreenAutoMergeReq:
		return m.autoMergeReqScreen.View()
	case ScreenProgress:
		return m.progressScreen.View()
	case ScreenLogs:
		return m.logsScreen.View()
	default:
		return "Unknown Screen"
	}
}

// viewWelcomeScreen renders the Welcome Screen.
func (m *Model) viewWelcomeScreen() string {
	return "\n" + logoStyle.Render(constants.AppLogo) + "\nPress Enter to continue."
}

// InitialModel sets up the initial state of the application.
func InitialModel() *Model {
	// Define the option list for the List Screen.
	oplist := SyncerList.ButtonList{
		ListItems:        []string{constants.OptionListPullPr, constants.OptionListAutoMergeReq, constants.OptionListLogs, "Quit"},
		Title:            "Hermes Options",
		ListWidth:        30,
		ListHeight:       14,
		ShowStatusBar:    false,
		FilteringEnabled: true,
	}

	// Define the form fields for the Pull Screen.
	pullFields := []screen.ButtonModel{
		{
			Label:       constants.PullFieldInclude,
			PlaceHolder: "Include patterns (comma-separated)",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       "SSH URL Include",
			PlaceHolder: "Can be author name",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constants.PullFieldExclude,
			PlaceHolder: "Exclude patterns",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constants.PullFieldPath,
			PlaceHolder: "Path to download",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
	}
	// Define the form fields for the Pull Screen.
	mergeRequestFields := []screen.ButtonModel{
		{
			Label:       constants.MergeFieldCommand,
			PlaceHolder: "go mod tidy;go get githubPkg",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constants.MergeFieldInclude,
			PlaceHolder: "Include patterns (comma-separated)",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constants.MergeFieldExclude,
			PlaceHolder: "Exclude patterns",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constants.MergeFieldPath,
			PlaceHolder: "project path",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constants.MergeFieldMergeRequestTargetBranch,
			PlaceHolder: "target branch (comma-separated)",
			Width:       50,
			Validate: func(s string) error {
				if s == "" {
					return fmt.Errorf("target branch cannot be empty")
				}
				return nil
			},
		},
		{
			Label:       constants.MergeFieldBranch,
			PlaceHolder: "branch name",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constants.MergeFieldCommitMessage,
			PlaceHolder: "commit message",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constants.MergeFieldMergeRequestTitle,
			PlaceHolder: "title",
			Width:       50,
			Validate: func(s string) error {
				if s == "" {
					return fmt.Errorf("title cannot be empty")
				}
				return nil
			},
		},
		{
			Label:       constants.MergeFieldMergeRequestDescription,
			PlaceHolder: "description",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
	}
	// Initialize the Logs Screen.
	logsScreenModel := logsScreen.InitialModel()
	// Add a tab for application logs and retrieve it's buffer.
	logBuf := logsScreenModel.AddTab(constants.LApplication)
	// Initialize the main logger to write to the application log buffer.
	mainLogger := logWriter.NewLogger(logBuf, true)
	mainLogger.InfoString("starting the application...")
	mainLogger.InfoString("performing initialization...")
	mainLogger.InfoString("application running.")

	// Add another tab for Pull Logs and initialize its logger.
	pullLogger := logWriter.NewLogger(logsScreenModel.AddTab("Pull Logs"), true)
	pullLogger.InfoString("starting pull operations...")
	// Add another tab for Pull Logs and initialize its logger.
	autoMergeLogger := logWriter.NewLogger(logsScreenModel.AddTab("Merge Logs"), true)
	autoMergeLogger.InfoString("starting auto merge operations...")
	// Initialize the option list model.
	optionListModel := SyncerList.InitialModel(oplist, mainLogger)
	// Initialize the Pull Screen with its logger.
	pullScreenModel := screen.NewModel(pullFields, pullLogger)
	mergeScreenModel := screen.NewModel(mergeRequestFields, autoMergeLogger)

	return &Model{
		currentScreen:      ScreenWelcome,
		optionList:         optionListModel,
		pullScreen:         pullScreenModel,
		autoMergeReqScreen: mergeScreenModel,
		progressScreen:     nil, // Will be initialized later with the updates channel.
		logsScreen:         logsScreenModel,
		quitting:           false,
		logWriter:          mainLogger,
		width:              0,
		height:             0,
	}
}

// main is the entry point of the application.
func main() {
	// Initialize the application model.
	m := InitialModel()
	m.logWriter.InfoString("Welcome to Hermes!")
	m.logWriter.InfoString("Application Version 0.0.1")

	// Start the Bubble Tea program.
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

// gitClient initializes and returns a new GitlabClient.
// The new version accepts a context, allowing for cancellation or timeouts if needed.
func gitClient(ctx context.Context, updatesChan chan<- progressScreen.PackageUpdate, values map[string]string,
	logsModel *logsScreen.LogModel, logger *logWriter.Logger) (*client.GitlabClient, error) {
	gitlabClient, err := client.New(updatesChan, values, logsModel)
	if err != nil {
		return nil, err
	}
	// (Optional) Enhance the client to use the provided context for any long-running operations.
	return gitlabClient, nil
}
