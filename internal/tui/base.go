package tui

import (
	"context"
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sinaw369/Hermes/internal/client"
	"github.com/sinaw369/Hermes/internal/config"
	"github.com/sinaw369/Hermes/internal/constant"
	"github.com/sinaw369/Hermes/internal/form/diffscreen"
	"github.com/sinaw369/Hermes/internal/form/logsScreen"
	"github.com/sinaw369/Hermes/internal/form/progressScreen"
	"github.com/sinaw369/Hermes/internal/form/screen"
	HermesList "github.com/sinaw369/Hermes/internal/list"
	"github.com/sinaw369/Hermes/internal/logWriter"
	HermesMsg "github.com/sinaw369/Hermes/internal/message"
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
	ScreenShowFile
	ScreenShowDiff

	ScreenQuit
)

var (
	quitTextStyle = lipgloss.NewStyle().Margin(1, 0, 2, 4)
	logoStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF06B7")).Align(lipgloss.Center)
)

// Model defines the state of the entire application.
type Model struct {
	currentScreen      Screen
	optionList         *HermesList.Model
	fileList           *HermesList.Model
	pullScreen         *screen.Model
	autoMergeReqScreen *screen.Model
	progressScreen     *progressScreen.Model
	logsScreen         *logsScreen.LogModel
	quitting           bool
	LogWriter          *logWriter.Logger
	width              int // Window width
	height             int // Window height
	cfg                *config.Config
	diffScreen         *diffscreen.Model
}

// Init initializes the application; no initial command is needed.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the application state.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle quitting.
	if m.quitting {
		m.LogWriter.InfoString("Quitting the application.")
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case HermesMsg.BackMsg:
		m.LogWriter.InfoString("Received BackMsg. Handling back navigation.")
		return m.handleBack()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.LogWriter.InfoString("Window size changed: Width=%d, Height=%d", m.width, m.height)
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
		case ScreenShowFile:
			return m.updateShowFileScreen(msg)
		case ScreenShowDiff:
			return m.updateShowDiffScreen(msg)
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
	case ScreenList:
		if m.fileList != nil {
			// Update the list's size to match the new terminal dimensions.
			m.fileList.List.SetSize(msg.Width, msg.Height)
			// Optionally adjust the pagination style (this centers the pagination, adjust as needed).
			newPaginationStyle := list.DefaultStyles().PaginationStyle.
				PaddingLeft((msg.Width - 40) / 2)
			m.fileList.List.Styles.PaginationStyle = newPaginationStyle
		}
		return m, nil
	default:
		return m, nil
	}
}

// handleBack manages the transition when a BackMsg is received.
func (m *Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case ScreenList:
		m.currentScreen = ScreenWelcome
	case ScreenPull, ScreenLogs, ScreenProgress, ScreenAutoMergeReq, ScreenShowFile:
		m.currentScreen = ScreenList
	case ScreenShowDiff:
		m.currentScreen = ScreenShowFile
	default:
		m.currentScreen = ScreenWelcome
	}
	m.LogWriter.InfoString("Navigated back to screen: %v", m.currentScreen)
	return m, nil
}

// updateWelcomeScreen handles updates specific to the Welcome Screen.
func (m *Model) updateWelcomeScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			m.LogWriter.InfoString("Enter pressed. Switching to List Screen.")
			m.currentScreen = ScreenList
			return m, nil
		case "q", "ctrl+c":
			m.LogWriter.InfoString("Quit command received. Quitting application.")
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// updateListScreen handles updates specific to the List Screen.
func (m *Model) updateListScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedList, cmd := m.optionList.Update(msg)
	m.optionList = updatedList.(*HermesList.Model)

	if m.optionList.Choice != "" {
		m.LogWriter.InfoString("Option selected: %s", m.optionList.Choice)
		switch m.optionList.Choice {
		case constant.OptionListPullPr:
			m.currentScreen = ScreenPull
		case constant.OptionListAutoMergeReq:
			m.currentScreen = ScreenAutoMergeReq
		case constant.OptionListLogs:
			m.LogWriter.YellowString("Switching to Logs Screen...")
			m.currentScreen = ScreenLogs
		case constant.OptionListShowProject:
			m.LogWriter.YellowString("Switching to Show project Screen...")
			if m.fileList == nil {
				fileList := HermesList.Config{
					IsDir:            true,
					StaticList:       nil,
					InitialPath:      m.cfg.WorkingDir,
					Title:            "",
					Width:            m.width,
					Height:           m.height,
					ShowStatusBar:    false,
					FilteringEnabled: true,
				}
				existsBuf, err := m.logsScreen.GetTabBufferIfExists("File List Logs")
				if err != nil {
					// TODO: handle error Change Screen to log screen
					return nil, nil
				}
				fileListLogger := logWriter.NewLogger(existsBuf, true, false)
				fileListModel, _ := HermesList.NewModel(fileList, fileListLogger)
				// TODO : check err will be panic
				m.fileList = fileListModel
			} else {
				// If the file list model already exists, update its size with the current terminal dimensions.
				m.fileList.List.SetSize(m.width, m.height)
			}
			if err := m.fileList.SetPath(m.cfg.WorkingDir); err != nil {
				m.LogWriter.ErrorString("Error setting file list path: %v", err)
			}
			m.currentScreen = ScreenShowFile
		case "Quit":
			m.LogWriter.InfoString("Quit option selected. Quitting application.")
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
		m.LogWriter.BlueString("Form submission complete. Starting processing...")
		m.LogWriter.YellowString("Switching to Progress Screen...")
		m.currentScreen = ScreenProgress

		// Create updates channel and a context for cancellation.
		updatesChan := make(chan progressScreen.PackageUpdate)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		//TODO you might store cancel() to trigger cancellation later.
		defer cancel()

		// Collect form values.
		values := m.pullScreen.GetValue()

		gClient, err := client.NewTUIGitClient(ctx, updatesChan, values, m.cfg, m.logsScreen)
		if err != nil {
			m.LogWriter.RedString("GitClient Initialization Failed: %v", err)
			m.currentScreen = ScreenLogs
			m.logsScreen.SetActiveTabByName(constant.LGitClient)

			updatedLogsScreen, logCmd := m.logsScreen.Update(HermesMsg.BackMsg{})
			m.logsScreen = updatedLogsScreen.(*logsScreen.LogModel)
			m.LogWriter.InfoString("Switched to Logs Screen due to GitClient initialization failure.")
			return m, logCmd
		}

		m.LogWriter.YellowString("Pull Automation Starting...")
		// Launch the GitLab client processing in a separate goroutine.
		go gClient.InitPullRequestAutomationTUI(nil)

		m.progressScreen = progressScreen.NewModel(updatesChan, m.LogWriter)
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
		m.LogWriter.BlueString("Form submission complete. Starting processing...")
		m.LogWriter.YellowString("Switching to Progress Screen...")
		m.currentScreen = ScreenProgress

		// Create updates channel and a context for cancellation.
		updatesChan := make(chan progressScreen.PackageUpdate)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		// TODO you might store cancel() to trigger cancellation later.
		defer cancel()

		// Collect form values.
		values := m.autoMergeReqScreen.GetValue()

		// Initialize the GitLab client with the context.
		gClient, err := client.NewTUIGitClient(ctx, updatesChan, values, m.cfg, m.logsScreen)
		if err != nil {
			m.LogWriter.RedString("GitClient Initialization Failed: %v", err)
			m.currentScreen = ScreenLogs
			m.logsScreen.SetActiveTabByName(constant.LGitClient)

			updatedLogsScreen, logCmd := m.logsScreen.Update(HermesMsg.BackMsg{})
			m.logsScreen = updatedLogsScreen.(*logsScreen.LogModel)
			m.LogWriter.InfoString("Switched to Logs Screen due to GitClient initialization failure.")
			return m, logCmd
		}

		m.LogWriter.YellowString("Pull Automation Starting...")
		// Launch the GitLab client processing in a separate goroutine.
		go gClient.InitMergeAutomationFromDir()

		// Initialize the progress screen with the updates channel.
		m.progressScreen = progressScreen.NewModel(updatesChan, m.LogWriter)
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
		m.LogWriter.YellowString("Processing complete. Switching to Logs Screen...")
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

// updateLogsScreen handles updates specific to the Logs Screen.
func (m *Model) updateShowFileScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedFileListScreen, cmd := m.fileList.Update(msg)
	m.fileList = updatedFileListScreen.(*HermesList.Model)

	switch gitMsg := msg.(type) {
	case HermesMsg.GitRepoMsg:
		// Now you can access the Path field:
		repoPath := gitMsg.Path

		// If diffScreen is not yet initialized, create it.
		if m.diffScreen == nil {
			m.diffScreen = diffscreen.NewDiffModel(m.width, m.height, repoPath, m.cfg.DiffBranchFrom, m.cfg.DifBranchTO)
		} else {
			m.diffScreen.UpdateFetch(repoPath)
		}
		m.currentScreen = ScreenShowDiff
		return m, cmd
	}

	return m, cmd
}
func (m *Model) updateShowDiffScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedDiff, cmd := m.diffScreen.Update(msg)
	m.diffScreen = updatedDiff.(*diffscreen.Model)
	switch msg.(type) {
	case HermesMsg.BackToFolderMsg:

		m.fileList.SetPath(m.fileList.CurrentPath)
		m.currentScreen = ScreenShowFile
		return m, cmd
	}

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
	case ScreenShowFile:
		return m.fileList.View()
	case ScreenLogs:
		return m.logsScreen.View()
	case ScreenShowDiff:
		return m.diffScreen.View()

	default:
		return "Unknown Screen"
	}
}

// viewWelcomeScreen renders the Welcome Screen.
func (m *Model) viewWelcomeScreen() string {
	return "\n" + logoStyle.Render(constant.AppLogo) + "\nPress Enter to continue."
}

// InitialModel sets up the initial state of the application.
func InitialModel(cfg *config.Config) *Model {
	// Define the option list for the List Screen.
	oplist := HermesList.Config{
		IsDir:            false,
		StaticList:       []string{constant.OptionListPullPr, constant.OptionListAutoMergeReq, constant.OptionListShowProject, constant.OptionListLogs, "Quit"},
		InitialPath:      "./",
		Title:            "Hermes Options",
		Width:            30,
		Height:           14,
		ShowStatusBar:    false,
		FilteringEnabled: true,
	}

	// Define the form fields for the Pull Screen.
	pullFields := []screen.ButtonModel{
		{
			Label:       constant.ContextValueInclude,
			PlaceHolder: "Include patterns (comma-separated)",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constant.ContextValueExclude,
			PlaceHolder: "Exclude patterns",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constant.PullFieldPath,
			PlaceHolder: "Path to download",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
	}
	// Define the form fields for the Pull Screen.
	mergeRequestFields := []screen.ButtonModel{
		{
			Label:       constant.MergeFieldCommand,
			PlaceHolder: "go mod tidy;go get githubPkg",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constant.ContextValueInclude,
			PlaceHolder: "Include patterns (comma-separated)",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constant.ContextValueExclude,
			PlaceHolder: "Exclude patterns",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constant.ContextValueDir,
			PlaceHolder: "project path",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constant.MergeFieldMergeRequestTargetBranch,
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
			Label:       constant.MergeFieldBranch,
			PlaceHolder: "branch name",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constant.MergeFieldCommitMessage,
			PlaceHolder: "commit message",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
		{
			Label:       constant.MergeFieldMergeRequestTitle,
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
			Label:       constant.MergeFieldMergeRequestDescription,
			PlaceHolder: "description",
			Width:       50,
			Validate:    func(s string) error { return nil },
		},
	}
	// Initialize the Logs Screen.
	logsScreenModel := logsScreen.InitialModel()
	// Add a tab for application logs and retrieve it's buffer.
	logBuf := logsScreenModel.AddTab(constant.LApplication)
	// Initialize the main logger to write to the application log buffer.
	mainLogger := logWriter.NewLogger(logBuf, true, false)
	mainLogger.InfoString("starting the application...")
	mainLogger.InfoString("performing initialization...")
	mainLogger.InfoString("application running.")

	// Add another tab for Pull Logs and initialize its logger.
	pullLogger := logWriter.NewLogger(logsScreenModel.AddTab("Pull Logs"), true, false)
	pullLogger.InfoString("starting pull operations...")
	// Add another tab for Pull Logs and initialize its logger.
	autoMergeLogger := logWriter.NewLogger(logsScreenModel.AddTab("Merge Logs"), true, false)
	autoMergeLogger.InfoString("starting auto merge operations...")
	// Initialize the option list model.
	fileListLogger := logWriter.NewLogger(logsScreenModel.AddTab("File List Logs"), true, false)
	fileListLogger.InfoString("starting file list operations...")
	optionListModel, _ := HermesList.NewModel(oplist, mainLogger)
	// Initialize the Pull Screen with its logger.
	pullScreenModel := screen.NewModel(pullFields, pullLogger)
	mergeScreenModel := screen.NewModel(mergeRequestFields, autoMergeLogger)

	return &Model{
		currentScreen:      ScreenWelcome,
		optionList:         optionListModel,
		fileList:           nil, // Will be initialized later with the updates channel.
		pullScreen:         pullScreenModel,
		autoMergeReqScreen: mergeScreenModel,
		progressScreen:     nil, // Will be initialized later with the updates channel.
		logsScreen:         logsScreenModel,
		quitting:           false,
		LogWriter:          mainLogger,
		width:              0,
		height:             0,
		cfg:                cfg,
	}
}
