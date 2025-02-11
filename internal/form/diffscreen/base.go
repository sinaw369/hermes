package diffscreen

import (
	"bytes"
	"fmt"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	HermesMsg "github.com/sinaw369/Hermes/internal/message"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DiffModel displays a unified diff between two branches.
type Model struct {
	viewport   viewport.Model
	branchFrom string
	branchTo   string
	content    string
	repoPath   string
	width      int
	height     int
	err        error
}

// NewDiffModel creates a new diff view.
func NewDiffModel(width, height int, repoPath, branchFrom, branchTo string) *Model {
	vp := viewport.New(width, height-3) // reserve some space for header
	model := Model{
		viewport:   vp,
		branchFrom: branchFrom,
		branchTo:   branchTo,
		repoPath:   repoPath,
		width:      width,
		height:     height,
		err:        nil,
	}
	model.fetchDiff() // populate diffContent
	model.viewport.SetContent(model.content)
	return &model
}

// fetchDiff runs "git diff branchFrom.branchTo" and stores the output.
func (m *Model) fetchDiff() {
	//cmd := exec.Command("git", "diff", m.branchFrom+"..origin/"+m.branchTo)
	//git log origin/production..origin/develop --oneline
	cmd := exec.Command("git", "log", "--pretty=format:\"%H - %s\"", "origin/"+m.branchFrom+"..origin/"+m.branchTo)

	cmd.Dir = m.repoPath
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		m.err = fmt.Errorf("error running git diff: %v", err)
		m.content = m.err.Error()
		return
	}
	output := strings.TrimSpace(out.String())

	if output == "" {
		m.content = "No differences between " + m.branchFrom + " and " + m.branchTo
	} else {
		lines := strings.Split(output, "\n")

		// Generate commit count text
		diffCount := "Number of different commits: " + strconv.Itoa(len(lines)) + "\n"
		diffCountStyled := strings.TrimSpace(lipgloss.NewStyle().Foreground(lipgloss.Color("201")).Render(diffCount))

		// Format the commits as "1. full_commit_hash commit_msg"
		var formattedLines []string
		for i, line := range lines {
			commitParts := strings.SplitN(line, " - ", 2) // Split commit hash and message using " - "
			if len(commitParts) > 1 {
				formattedLines = append(formattedLines, strconv.Itoa(i+1)+". "+commitParts[0]+" "+strings.TrimSpace(commitParts[1]))
			}
		}

		// Join commits properly
		m.content = diffCountStyled + strings.Join(formattedLines, "\n")
	}

}

// colorizeDiff applies basic colorization to a diff string.
func colorizeDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var coloredLines []string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			coloredLines = append(coloredLines, lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			coloredLines = append(coloredLines, lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(line))
		default:
			coloredLines = append(coloredLines, line)
		}
	}
	return strings.Join(coloredLines, "\n")
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	// Use a tick to update the view if needed.
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

// Update handles key events for scrolling.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.viewport.LineUp(1)
		case "down", "j":
			m.viewport.LineDown(1)
		case "pgup":
			m.viewport.ViewUp()
		case "pgdown":
			m.viewport.ViewDown()
		case "backspace":
			return m, func() tea.Msg { return HermesMsg.BackToFolderMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3
	}
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the diff screen.
func (m *Model) View() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF06B7")).
		Render(fmt.Sprintf("Diff: %s..%s (press q to quit,backspace to folder list)", m.branchFrom, m.branchTo))
	return fmt.Sprintf("%s\n%s", header, m.viewport.View())
}
