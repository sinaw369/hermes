// File: forms/screen/screen.go
package screen

import (
	"fmt"
	"github.com/sinaw369/Hermes/internal/logWriter"
	"github.com/sinaw369/Hermes/internal/message"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Define custom colors using Lip Gloss.
const (
	hotPink  = lipgloss.Color("#FF06B7")
	darkGray = lipgloss.Color("#767676")
	red      = lipgloss.Color("#FF0000")
)

// Define styles for various UI elements.
var (
	inputStyle    = lipgloss.NewStyle().Foreground(hotPink)
	continueStyle = lipgloss.NewStyle().Foreground(darkGray)
	errorStyle    = lipgloss.NewStyle().Foreground(red)
	labelStyle    = lipgloss.NewStyle().Bold(true)
)

// InputField represents a single input field with a label and the actual textinput.Model
type InputField struct {
	Label string          // Custom label for the input field
	Input textinput.Model // The actual input field model
}

// Model represents the state of the form/screen
type Model struct {
	Inputs    []InputField      // A slice to store multiple input fields with labels
	Focused   int               // The index of the currently focused input field
	Err       error             // Stores the current error (if any)
	logWriter *logWriter.Logger // Logger for logging events
	Submitted bool              // Field to track form submission
}

// ButtonModel represents the configuration for an input field
type ButtonModel struct {
	Label       string
	PlaceHolder string
	Width       int
	Validate    func(string) error
}

// NewModel initializes and returns a new form model with given input configurations
func NewModel(buttons []ButtonModel, logWriter *logWriter.Logger) *Model {
	model := &Model{
		Inputs:    []InputField{},
		Focused:   0,
		logWriter: logWriter,
	}

	for _, button := range buttons {
		model.addInput(button.Label, button.PlaceHolder, button.Width, button.Validate)
	}

	// Focus the first input if available
	if len(model.Inputs) > 0 {
		model.Inputs[0].Input.Focus()
	}

	return model
}

// addInput is a helper function to create new input fields programmatically with custom labels
func (m *Model) addInput(label string, placeholder string, width int, validateFunc func(string) error) {
	newInput := textinput.New()
	newInput.Placeholder = placeholder
	newInput.Width = width
	newInput.PromptStyle = labelStyle
	newInput.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("228")) // Light yellow for input text

	if validateFunc != nil {
		newInput.Validate = validateFunc
	}

	// Append new input to the model's input slice with the custom label
	m.Inputs = append(m.Inputs, InputField{
		Label: label,
		Input: newInput,
	})
}

// Init is the initialization command of the program
func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles all key-based interactions and updates the state accordingly
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Attempt to submit the form if the last input is focused
			if m.Focused == len(m.Inputs)-1 {
				// Validate all inputs before submission
				for i, inputField := range m.Inputs {
					if inputField.Input.Validate != nil {
						err := inputField.Input.Validate(inputField.Input.Value())
						if err != nil {
							m.Err = fmt.Errorf("error in '%s': %v", inputField.Label, err)
							m.Focused = i // Focus the input with the error
							return m, nil
						}
					}
				}

				// If validation passes, collect the values and log them
				values := m.GetValue()
				m.logWriter.GreenString("Collected values: %v", values)
				m.Submitted = true
				return m, nil
			}

			// Move to the next input
			m.nextInput()

		case "ctrl+c", "esc":
			if msg.String() == "esc" {
				return m, func() tea.Msg { return message.BackMsg{} }
			}
			return m, tea.Quit

		case "tab", "down":
			m.nextInput()

		case "shift+tab", "up":
			m.prevInput()
		}

	case tea.WindowSizeMsg:
		// Adjust input widths based on terminal size if necessary
		for i := range m.Inputs {
			labelLen := len(m.Inputs[i].Label)
			m.Inputs[i].Input.Width = msg.Width - labelLen - 6 // Adjust as needed
		}
	}

	// Handle focus: ensure only the focused input is active
	for i := range m.Inputs {
		if i == m.Focused && !m.Submitted {
			m.Inputs[i].Input.Focus()
		} else {
			m.Inputs[i].Input.Blur()
		}
	}

	// Update each input field and collect commands
	for i := range m.Inputs {
		var cmd tea.Cmd
		m.Inputs[i].Input, cmd = m.Inputs[i].Input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI layout
func (m *Model) View() string {
	var errorMessage string
	if m.Err != nil {
		errorMessage = errorStyle.Render(fmt.Sprintf("\n%s", m.Err.Error()))
	}

	// Render each input field with its label
	var view strings.Builder
	for _, inputField := range m.Inputs {
		view.WriteString(fmt.Sprintf("%s\n%s\n\n", inputStyle.Render(inputField.Label), inputField.Input.View()))
	}

	if m.Submitted {
		view.WriteString(continueStyle.Render("\nForm submitted successfully! Press 'Esc' to go back or 'Ctrl+C' to quit."))
	} else {
		view.WriteString(continueStyle.Render("Press 'Enter' to submit, 'Tab' to navigate, 'Esc' to go back, 'Ctrl+C' to quit."))
	}

	view.WriteString(errorMessage)
	return view.String()
}

// nextInput moves focus to the next input field
func (m *Model) nextInput() {
	if len(m.Inputs) == 0 {
		return
	}
	m.Focused = (m.Focused + 1) % len(m.Inputs)
}

// prevInput moves focus to the previous input field
func (m *Model) prevInput() {
	if len(m.Inputs) == 0 {
		return
	}
	if m.Focused > 0 {
		m.Focused--
	} else {
		m.Focused = len(m.Inputs) - 1
	}
}

// GetValue collects the values entered in the input fields and returns them as a map
func (m *Model) GetValue() map[string]string {
	values := make(map[string]string)

	// Loop through each input field in the model
	for _, inputField := range m.Inputs {
		values[inputField.Label] = inputField.Input.Value() // Add label and value to the map
	}

	return values
}
