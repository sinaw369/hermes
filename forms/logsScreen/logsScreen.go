// File: forms/logsScreen/log_model.go
package logsScreen

import (
	"bytes"
	"fmt"
	"github.com/sinaw369/Hermes/messages"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	activeTabBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#7D56F4")).
				Padding(0, 1).
				Bold(true)

	inactiveTabBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#4B4E6D")).
				Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Align(lipgloss.Right)
)

type tab struct {
	name string
	buf  *bytes.Buffer
	mu   sync.Mutex
}

type LogModel struct {
	viewport       viewport.Model
	tabs           []tab // List of tabs
	activeTab      int   // Index of the active tab
	contentChanged bool  // Flag to indicate content update
	mu             sync.Mutex
}

func (m *LogModel) Init() tea.Cmd {
	// Set up a ticker to refresh logs every second
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return t
	})
}

func (m *LogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			return m, func() tea.Msg { return messages.BackMsg{} }
		case "right":
			m.mu.Lock()
			if len(m.tabs) > 0 {
				m.activeTab = (m.activeTab + 1) % len(m.tabs) // Next tab
				m.contentChanged = true
			}
			m.mu.Unlock()
		case "left":
			m.mu.Lock()
			if len(m.tabs) > 0 {
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs) // Previous tab
				m.contentChanged = true
			}
			m.mu.Unlock()
		case "up":
			m.viewport.LineUp(1)
		case "down":
			m.viewport.LineDown(1)
		case "pgup":
			m.viewport.ViewUp()
		case "pgdown":
			m.viewport.ViewDown()
		}
	case tea.WindowSizeMsg:
		headerHeight := calculateHeight(m.headerView())
		footerHeight := calculateHeight(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalMarginHeight
		// m.viewport.YPosition = headerHeight // Removed as viewport doesn't support YPosition

		m.contentChanged = true
	case time.Time:
		// Refresh logs and content on each tick
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return t
		})
	case messages.BackMsg:
		// Handle BackMsg if needed
	}

	// Update viewport content if needed
	m.mu.Lock()
	if m.contentChanged && len(m.tabs) > 0 {
		activeTab := &m.tabs[m.activeTab]
		activeTab.mu.Lock()
		content := activeTab.buf.String()
		activeTab.mu.Unlock()
		m.viewport.SetContent(content)
		m.contentChanged = false
	}
	m.mu.Unlock()

	// Handle viewport updates (like scrolling)
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *LogModel) View() string {
	body := m.viewport.View()
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444")).
		Render(strings.Repeat("─", m.viewport.Width))

	return fmt.Sprintf("%s\n%s\n%s\n%s", m.headerView(), separator, body, m.footerView())
}

func (m *LogModel) headerView() string {
	title := titleStyle.Render("Application Logs")

	// Render tabs dynamically based on the tab names
	var tabViews []string
	m.mu.Lock()
	for i, t := range m.tabs {
		if i == m.activeTab {
			tabViews = append(tabViews, activeTabBorderStyle.Render(t.name))
		} else {
			tabViews = append(tabViews, inactiveTabBorderStyle.Render(t.name))
		}
	}
	m.mu.Unlock()

	tabs := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	header := lipgloss.JoinVertical(lipgloss.Left, title, tabs)
	line := strings.Repeat("─", m.viewport.Width)

	return lipgloss.JoinVertical(lipgloss.Left, header, line)
}

func (m *LogModel) footerView() string {
	scrollPercent := m.viewport.ScrollPercent() * 100
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", scrollPercent))
	lineLength := m.viewport.Width - lipgloss.Width(info)
	if lineLength < 0 {
		lineLength = 0
	}
	line := strings.Repeat("─", lineLength)
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func calculateHeight(s string) int {
	return strings.Count(s, "\n") + 1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Updated AddTab function
func (m *LogModel) AddTab(name string) *bytes.Buffer {
	m.mu.Lock()
	defer m.mu.Unlock()

	buf := new(bytes.Buffer)
	if !m.isExist(name) {
		m.tabs = append(m.tabs, tab{name: name, buf: buf})
	}

	// Set activeTab to the first tab if this is the first tab added
	if len(m.tabs) == 1 {
		m.activeTab = 0
	}

	m.contentChanged = true
	return buf
}

func (m *LogModel) isExist(name string) bool {
	for _, t := range m.tabs {
		if t.name == name {
			return true
		}
	}
	return false
}

func InitialModel() *LogModel {
	return &LogModel{
		tabs: []tab{},
		viewport: viewport.Model{
			Width:  80, // Initial default width
			Height: 20, // Initial default height
		},
	}
}

// Append data to a specific tab by name
func (m *LogModel) AppendToTab(name, data string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, t := range m.tabs {
		if t.name == name {
			t.mu.Lock()
			t.buf.WriteString(data)
			t.mu.Unlock()
			if i == m.activeTab {
				m.contentChanged = true
			}
			break
		}
	}
}

func (m *LogModel) SetActiveTabByName(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, t := range m.tabs {
		if t.name == name {
			m.activeTab = i
			m.contentChanged = true
			return true
		}
	}
	return false
}
