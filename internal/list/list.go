// File: listview/listview.go
package listview

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sinaw369/Hermes/internal/logWriter"
	"github.com/sinaw369/Hermes/internal/message"
	"io"
	"os"
	"path/filepath"
)

// -------------------------------------------------------------------
// Common Styles
// -------------------------------------------------------------------
var (
	titleStyle      = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("#FF06B7"))
	itemStyle       = lipgloss.NewStyle().PaddingLeft(4)
	selectedStyle   = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle       = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	dirStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	fileStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
)

// -------------------------------------------------------------------
// Static Mode Types & Delegate
// -------------------------------------------------------------------

// Item is used for the static list mode.
type Item string

// FilterValue allows Item to be filtered.
func (i Item) FilterValue() string { return string(i) }

// itemDelegate renders static list items.
type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}
	str := fmt.Sprintf("%d. %s", index+1, i)
	if index == m.Index() {
		fmt.Fprint(w, selectedStyle.Render("> "+str))
	} else {
		fmt.Fprint(w, itemStyle.Render(str))
	}
}

// -------------------------------------------------------------------
// Directory Mode Types & Delegate
// -------------------------------------------------------------------

// FileItem represents a file or directory.
type FileItem struct {
	Name  string // Display name
	Path  string // Full path
	IsDir bool   // True if directory
}

// FilterValue enables filtering.
func (f FileItem) FilterValue() string { return f.Name }

// fileDelegate renders FileItem objects.
type fileDelegate struct{}

func (d fileDelegate) Height() int                               { return 1 }
func (d fileDelegate) Spacing() int                              { return 0 }
func (d fileDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d fileDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(FileItem)
	if !ok {
		return
	}
	// Use a folder icon for directories and a file icon for files.
	icon := "📄" // file icon
	if item.IsDir {
		icon = "📁" // folder icon
	}
	line := fmt.Sprintf("%s %s", icon, item.Name)
	if index == m.Index() {
		fmt.Fprint(w, selectedStyle.Render("> "+line))
	} else {
		if item.IsDir {
			fmt.Fprint(w, dirStyle.Render(line))
		} else {
			fmt.Fprint(w, fileStyle.Render(line))
		}
	}
}

// loadDirectory reads the directory at the given path and returns its entries.
func loadDirectory(path string) ([]list.Item, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var items []list.Item
	for _, entry := range entries {
		items = append(items, FileItem{
			Name:  entry.Name(),
			Path:  filepath.Join(path, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}
	return items, nil
}

// -------------------------------------------------------------------
// Unified Model & Config
// -------------------------------------------------------------------

// Config holds the settings to initialize the model.
type Config struct {
	// Set IsDir to true for directory mode; false for static mode.
	IsDir bool

	// For static mode:
	StaticList []string // list of items to display

	// For directory mode:
	// Set InitialPath to the starting directory.
	// If you want to load the directory at runtime instead, pass an empty string.
	InitialPath string

	// Shared settings:
	Title            string
	Width, Height    int
	ShowStatusBar    bool
	FilteringEnabled bool
}

// Model can operate in either static or directory mode.
type Model struct {
	List        list.Model
	isDir       bool
	CurrentPath string // used only in directory mode
	Choice      string
	logWriter   *logWriter.Logger
	InitialPath string
}

// NewModel creates a new Model based on the provided config.
// In directory mode, if InitialPath is empty, it falls back to <cwd>/git-repos.
func NewModel(config Config, logger *logWriter.Logger) (*Model, error) {

	if config.IsDir {
		var items []list.Item
		var title string
		if config.InitialPath != "" {
			var err error
			items, err = loadDirectory(config.InitialPath)
			if err != nil {
				return nil, err
			}
			title = fmt.Sprintf("Directory: %s", config.InitialPath)
		} else {
			homeDir, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			newPath := filepath.Join(homeDir, "git-repos")
			items, err = loadDirectory(newPath)
			if err != nil {
				return nil, err
			}
			title = fmt.Sprintf("Directory: %s", newPath)
			// Update config.InitialPath so that the model is aware of the fallback.
			config.InitialPath = newPath
		}

		l := list.New(items, fileDelegate{}, config.Width, config.Height)
		l.Title = title
		l.SetShowStatusBar(config.ShowStatusBar)
		l.SetFilteringEnabled(config.FilteringEnabled)
		l.Styles.Title = titleStyle
		l.Styles.PaginationStyle = paginationStyle
		l.Styles.HelpStyle = helpStyle

		return &Model{
			List:        l,
			isDir:       true,
			CurrentPath: config.InitialPath,
			Choice:      "",
			logWriter:   logger,
			InitialPath: config.InitialPath,
		}, nil
	}

	// Static mode
	var items []list.Item
	for _, s := range config.StaticList {
		items = append(items, Item(s))
	}
	l := list.New(items, itemDelegate{}, config.Width, config.Height)
	l.Title = config.Title
	l.SetShowStatusBar(config.ShowStatusBar)
	l.SetFilteringEnabled(config.FilteringEnabled)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	return &Model{
		List:      l,
		isDir:     false,
		Choice:    "",
		logWriter: logger,
	}, nil
}

// SetPath changes the current directory at runtime.
// It loads the directory at newPath and updates the list accordingly.
// This function works only when the model is in directory mode.
func (m *Model) SetPath(newPath string) error {
	if !m.isDir {
		m.logWriter.ErrorString("SetPath is only available in directory mode")
		return fmt.Errorf("SetPath is only available in directory mode")
	}
	if newPath == "" {
		homeDir, err := os.Getwd()
		if err != nil {
			m.logWriter.ErrorString("Error fetching home directory: %v", err)
			return err
		}
		newPath = filepath.Join(homeDir, "git-repos")
	}
	items, err := loadDirectory(newPath)
	if err != nil {
		return err
	}
	m.CurrentPath = newPath
	m.List.SetItems(items)
	m.List.Title = fmt.Sprintf("Directory: %s", m.CurrentPath)
	return nil
}

// Init is the Bubble Tea initialization command.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles key presses and other messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.List.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "esc":
			return m, func() tea.Msg { return message.BackMsg{} }
		}

		// If we're in directory mode, handle navigation keys.
		if m.isDir {
			switch msg.String() {
			case "enter":
				// Navigate into a directory if selected.
				selected, ok := m.List.SelectedItem().(FileItem)
				if !ok {
					return m, nil
				}
				if selected.IsDir {
					if err := m.SetPath(selected.Path); err != nil {
						// Optionally handle the error (for example, by showing an error message).
						return m, nil
					}
					m.Choice = selected.Path
				} else {
					m.Choice = selected.Path
					m.logWriter.InfoString("Opening file: %s", selected.Path)
				}

			case "backspace":
				// Navigate up one directory level.
				parent := filepath.Dir(m.CurrentPath)
				if err := m.SetPath(parent); err != nil {
					return m, nil
				}
			}

		} else {
			// In static mode, you can define additional key handling.
			if msg.String() == "enter" {
				selected, ok := m.List.SelectedItem().(Item)
				if ok {
					m.Choice = string(selected)
					m.logWriter.InfoString("Static item selected: %s", selected)
				}
			}
		}
	}

	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

// View renders the UI.
func (m *Model) View() string {
	return "\n" + m.List.View()
}
