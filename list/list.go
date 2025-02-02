// File: packageList/packageList.go
package packageList

import (
	"fmt"
	"github.com/sinaw369/Hermes/logWriter"
	"github.com/sinaw369/Hermes/messages"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Define styles for various UI elements.
var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("#FF06B7"))
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

// Item represents a single list item.
type Item string

// FilterValue implements the list.Item interface for filtering.
func (i Item) FilterValue() string { return string(i) }

// itemDelegate handles rendering of list items.
type itemDelegate struct{}

// Height returns the height of a single item.
func (d itemDelegate) Height() int { return 1 }

// Spacing returns the spacing between items.
func (d itemDelegate) Spacing() int { return 0 }

// Update handles any updates to the item delegate.
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

// Render renders a single list item.
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	if index == m.Index() {
		// Item is selected
		fmt.Fprint(w, selectedItemStyle.Render("> "+str))
	} else {
		// Regular item
		fmt.Fprint(w, itemStyle.Render(str))
	}
}

// Model represents the state of the package list screen.
type Model struct {
	List      list.Model
	Choice    string
	logWriter *logWriter.Logger
}

// Init is the initialization command of the program.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the model accordingly.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.List.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, func() tea.Msg { return messages.BackMsg{} }
		case "enter":
			i, ok := m.List.SelectedItem().(Item)
			if ok {
				m.Choice = string(i)
				if m.logWriter != nil {
					m.logWriter.BlueString("User selected: %s\n", m.Choice)
				}
				return m, nil // Do not quit the program
			}
		}
	}

	var cmd tea.Cmd
	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

// View renders the package list screen.
func (m *Model) View() string {
	return "\n" + m.List.View()
}

// ButtonList represents the configuration for the package list.
type ButtonList struct {
	ListItems        []string
	Title            string
	ListWidth        int
	ListHeight       int
	ShowStatusBar    bool
	FilteringEnabled bool
}

// InitialModel sets up the package list screen with the given configurations.
func InitialModel(buttonList ButtonList, logWriter *logWriter.Logger) *Model {
	var items []list.Item
	for _, item := range buttonList.ListItems {
		items = append(items, Item(item))
	}

	l := list.New(items, itemDelegate{}, buttonList.ListWidth, buttonList.ListHeight)
	l.Title = buttonList.Title
	l.SetShowStatusBar(buttonList.ShowStatusBar)
	l.SetFilteringEnabled(buttonList.FilteringEnabled)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	return &Model{
		List:      l,
		Choice:    "",
		logWriter: logWriter,
	}
}
