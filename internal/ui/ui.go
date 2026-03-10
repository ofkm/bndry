package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ErrCancelled is returned when the user exits an interactive prompt.
var ErrCancelled = errors.New("prompt cancelled")

// Validator validates text input.
type Validator func(value string) error

// Option is a single selectable item in the TUI.
type Option struct {
	Label       string
	Description string
	Value       string
}

// RunSelect renders a simple keyboard-driven selector.
func RunSelect(prompt string, options []Option, initialIndex int) (Option, error) {
	if len(options) == 0 {
		return Option{}, errors.New("no options available")
	}

	if initialIndex < 0 || initialIndex >= len(options) {
		initialIndex = 0
	}

	model := selectModel{
		prompt:  prompt,
		options: options,
		index:   initialIndex,
	}

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return Option{}, err
	}

	result, ok := finalModel.(selectModel)
	if !ok {
		return Option{}, errors.New("unexpected selection state")
	}
	if result.cancelled {
		return Option{}, ErrCancelled
	}

	return result.choice, nil
}

// RunTextInput renders a simple single-line text prompt.
func RunTextInput(prompt string, placeholder string, initialValue string, validate Validator) (string, error) {
	model := textModel{
		prompt:      prompt,
		placeholder: placeholder,
		value:       []rune(initialValue),
		validate:    validate,
	}

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", err
	}

	result, ok := finalModel.(textModel)
	if !ok {
		return "", errors.New("unexpected input state")
	}
	if result.cancelled {
		return "", ErrCancelled
	}

	return strings.TrimSpace(string(result.value)), nil
}

// RunConfirm renders a simple yes/no prompt.
func RunConfirm(prompt string, initialValue bool) (bool, error) {
	model := confirmModel{prompt: prompt, value: initialValue}

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return false, err
	}

	result, ok := finalModel.(confirmModel)
	if !ok {
		return false, errors.New("unexpected confirm state")
	}
	if result.cancelled {
		return false, ErrCancelled
	}

	return result.value, nil
}

type selectModel struct {
	prompt    string
	options   []Option
	index     int
	choice    Option
	cancelled bool
	selected  bool
}

func (m selectModel) Init() tea.Cmd {
	return nil
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "ctrl+c", "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.index > 0 {
			m.index--
		}
	case "down", "j":
		if m.index < len(m.options)-1 {
			m.index++
		}
	case "enter":
		m.choice = m.options[m.index]
		m.selected = true
		return m, tea.Quit
	}

	return m, nil
}

func (m selectModel) View() tea.View {
	var builder strings.Builder
	builder.WriteString(promptStyle.Render(m.prompt))
	builder.WriteString("\n\n")

	for idx, option := range m.options {
		cursor := mutedStyle.Render("  ")
		label := option.Label
		if idx == m.index {
			cursor = selectedStyle.Render("› ")
			label = selectedStyle.Render(label)
		} else {
			label = valueStyle.Render(label)
		}

		builder.WriteString(cursor)
		builder.WriteString(label)
		if option.Description != "" {
			builder.WriteString("\n")
			builder.WriteString(optionDetailStyle.Render(option.Description))
		}
		if idx < len(m.options)-1 {
			builder.WriteString("\n\n")
		}
	}

	builder.WriteString("\n")
	builder.WriteString(promptHintStyle.Render("Use ↑/↓ (or j/k), Enter to choose, q to cancel."))
	return tea.NewView(promptFrameStyle.Render(builder.String()))
}

type textModel struct {
	prompt      string
	placeholder string
	value       []rune
	validate    Validator
	errText     string
	cancelled   bool
}

func (m textModel) Init() tea.Cmd {
	return nil
}

func (m textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "enter":
		value := strings.TrimSpace(string(m.value))
		if m.validate != nil {
			if err := m.validate(value); err != nil {
				m.errText = err.Error()
				return m, nil
			}
		}
		m.value = []rune(value)
		return m, tea.Quit
	case "backspace", "ctrl+h":
		if len(m.value) > 0 {
			m.value = m.value[:len(m.value)-1]
		}
	default:
		if text := keyMsg.Key().Text; text != "" {
			m.value = append(m.value, []rune(text)...)
		}
	}

	return m, nil
}

func (m textModel) View() tea.View {
	var builder strings.Builder
	builder.WriteString(promptStyle.Render(m.prompt))
	builder.WriteString("\n\n")
	inputLine := selectedStyle.Render("› ")
	if len(m.value) == 0 && m.placeholder != "" {
		inputLine += mutedStyle.Render(m.placeholder)
	} else {
		inputLine += valueStyle.Render(string(m.value))
	}
	inputLine += selectedStyle.Render("█")
	builder.WriteString(inputRowStyle.Render(inputLine))

	if m.errText != "" {
		builder.WriteString("\n\n")
		builder.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.errText)))
	}

	builder.WriteString("\n\n")
	builder.WriteString(promptHintStyle.Render("Type your answer, Enter to continue, Esc to cancel."))
	return tea.NewView(promptFrameStyle.Render(builder.String()))
}

type confirmModel struct {
	prompt    string
	value     bool
	cancelled bool
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "ctrl+c", "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "left", "h", "y":
		m.value = true
	case "right", "l", "n":
		m.value = false
	case "enter":
		return m, tea.Quit
	}

	return m, nil
}

func (m confirmModel) View() tea.View {
	yes := selectedStyle.Render("[yes]")
	no := mutedStyle.Render(" no ")
	if !m.value {
		yes = mutedStyle.Render(" yes ")
		no = selectedStyle.Render("[no]")
	}

	var builder strings.Builder
	builder.WriteString(promptStyle.Render(m.prompt))
	builder.WriteString("\n\n")
	builder.WriteString(inputRowStyle.Render(yes + "     " + no))
	builder.WriteString("\n\n")
	builder.WriteString(promptHintStyle.Render("Use ←/→ (or y/n), Enter to continue, q to cancel."))

	return tea.NewView(promptFrameStyle.Render(builder.String()))
}
