// File: forms/progressScreen/progress_screen.go
package progressScreen

import (
	"fmt"
	"github.com/sinaw369/Hermes/logWriter"
	"github.com/sinaw369/Hermes/messages"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PackageUpdate represents an update for a package's processing status.
type PackageUpdate struct {
	PackageName string
	Status      bool
	TotalPkg    int
	Index       int
}

// tickMsg is a custom message type for ticker ticks.
type tickMsg time.Time

// Model defines the state of the progress screen.
type Model struct {
	width       int
	spinner     spinner.Model
	progress    progress.Model
	done        bool
	checkmarks  []string             // List of processed packages with checkmarks or crosses
	updatesChan <-chan PackageUpdate // Read-only channel for package updates
	currentPkg  string
	totalPkg    int
	index       int
	showLine    int
	logWriter   *logWriter.Logger // Logger for debugging

}

// Styles for different UI elements.
var (
	// Pre-rendered styled symbols
	checkMark = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")). // Green color
			Render("✓")

	crossMark = lipgloss.NewStyle().
			Foreground(lipgloss.Color("160")). // Red color
			Render("✗")

	currentPkgNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("211")).
				Italic(true)

	doneStyle = lipgloss.NewStyle().
			Margin(1, 2).
			Bold(true).
			Foreground(lipgloss.Color("42"))
)

// NewModel initializes and returns a new progress screen model.
func NewModel(updatesChan <-chan PackageUpdate, logger *logWriter.Logger) *Model {
	// Initialize the progress bar without percentage display.
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)

	// Initialize the spinner.
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

	return &Model{
		spinner:     s,
		progress:    p,
		checkmarks:  []string{},
		updatesChan: updatesChan,
		currentPkg:  "",
		totalPkg:    0,
		index:       0,
		showLine:    0,
		done:        false,
		logWriter:   logger,
	}
}

// Init starts the spinner and sets up periodic ticks using tea.Tick.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startTick(),
	)
}

// startTick sets up a periodic tick every 50ms for smoother progress updates.
func (m *Model) startTick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles incoming messages and updates the model accordingly.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Label the loop for controlled breaking
Loop:
	for {
		select {
		case update, ok := <-m.updatesChan:
			if !ok {
				// Channel closed, set progress to 100% and mark as done.
				m.logWriter.InfoString("updatesChan closed. Marking processing as done.")
				m.progress.SetPercent(1.0)
				m.done = true
				break Loop // Exit the loop to prevent infinite cycling
			}

			// Process the package update.
			m.logWriter.InfoString("Processing package: %s (Status: %v)", update.PackageName, update.Status)
			m.processPackageUpdate(update)
			m.totalPkg = update.TotalPkg
			m.index = update.Index

			// Calculate the new target percentage.
			if m.totalPkg > 0 {
				m.progress.SetPercent(float64(m.index+1) / float64(m.totalPkg))
				m.logWriter.InfoString("Progress set to %.2f%%", float64(m.index+1)/float64(m.totalPkg)*100)
			} else {
				m.progress.SetPercent(0.0)
				m.logWriter.RedString("2->", float64(m.index+1)/float64(m.totalPkg))
				m.logWriter.InfoString("Total packages is zero. Progress set to 0%%")
			}

		default:
			// No more updates to process.
			break Loop // Exit the loop as there are no more updates
		}
	}

	// Handle other messages.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.logWriter.InfoString("Esc key pressed. Sending BackMsg.")
			return m, func() tea.Msg { return messages.BackMsg{} }
		case "ctrl+c":
			m.logWriter.InfoString("Ctrl+C pressed. Quitting.")
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tickMsg:
		// Schedule the next tick.
		cmds = append(cmds, m.startTick())

		// Update the spinner.
		cmds = append(cmds, m.spinner.Tick)

	default:
		// Pass through any messages to progress.
	}

	// Handle progress updates.
	updatedProgressModel, pCmd := m.progress.Update(msg)
	if pModel, ok := updatedProgressModel.(progress.Model); ok {
		m.progress = pModel
	} else {
		m.logWriter.ErrorString("Failed to assert progress.Model")
	}

	cmds = append(cmds, pCmd)

	return m, tea.Batch(cmds...)
}

// processPackageUpdate appends a checkmark or cross to the checkmarks list based on package status.
func (m *Model) processPackageUpdate(update PackageUpdate) {
	var symbol string
	if update.Status {
		symbol = checkMark
	} else {
		symbol = crossMark
	}
	// Append the package with the symbol to the checkmarks list.
	m.checkmarks = append(m.checkmarks, fmt.Sprintf("%s %s", symbol, update.PackageName))
	// Update current package name.
	m.currentPkg = update.PackageName
}

// View renders the progress screen.
func (m *Model) View() string {
	if m.done {
		m.logWriter.InfoString("Processing complete. Rendering done message.")
		return doneStyle.Render(fmt.Sprintf("Done! Processed %d packages.\n", m.totalPkg))
	}
	m.showLine++

	// Spinner view
	spin := m.spinner.View() + " "

	// Current package info
	pkgName := currentPkgNameStyle.Render(m.currentPkg)
	info := lipgloss.NewStyle().Render("Processing " + pkgName)

	// Progress bar view
	prog := m.progress.ViewAs(float64(m.index+1) / float64(m.totalPkg))

	// Package count
	pkgCount := fmt.Sprintf(" %d/%d", m.index+1, m.totalPkg)

	// Combine all elements with spacing
	line := fmt.Sprintf("%s%s %s %s", spin, info, prog, pkgCount)

	// Render processed packages above the main line
	processed := ""
	if len(m.checkmarks) < 10 {
		processed = strings.Join(m.checkmarks, "\n") + "\n"
	} else {
		processed = strings.Join(m.checkmarks[len(m.checkmarks)-10:], "\n") + "\n"
	}

	return processed + line
}

// Done indicates whether the processing is complete.
func (m *Model) Done() bool {
	return m.done
}
