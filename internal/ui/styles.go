package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	titleStyle             = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	sectionStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).MarginBottom(1)
	selectedStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	promptStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	mutedStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle             = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	successStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	warningStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	codeStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	valueStyle             = lipgloss.NewStyle().Bold(true)
	helpContainerStyle     = lipgloss.NewStyle().Padding(1, 2)
	helpSectionBodyStyle   = lipgloss.NewStyle().MarginLeft(2)
	helpCommandNameStyle   = codeStyle.Copy().Bold(true)
	helpCommandDetailStyle = mutedStyle.Copy().MarginLeft(2)
	promptFrameStyle       = lipgloss.NewStyle().Padding(1, 2)
	promptHintStyle        = mutedStyle.Copy().MarginTop(1)
	optionDetailStyle      = mutedStyle.Copy().MarginLeft(4)
	inputRowStyle          = lipgloss.NewStyle().MarginLeft(1)
)

// RenderCommandHelp renders a styled help view for Cobra commands.
func RenderCommandHelp(cmd *cobra.Command) string {
	sections := []string{
		titleStyle.Render(cmd.CommandPath()),
	}

	if short := strings.TrimSpace(cmd.Short); short != "" {
		sections = append(sections, mutedStyle.Render(short))
	}

	sections = append(sections, renderHelpSection("Usage", codeStyle.Render(cmd.UseLine())))

	if len(cmd.Aliases) > 0 {
		sections = append(sections, renderHelpSection("Aliases", strings.Join(cmd.Aliases, ", ")))
	}

	if children := availableSubcommands(cmd); len(children) > 0 {
		lines := make([]string, 0, len(children))
		for _, child := range children {
			lines = append(lines, helpCommandNameStyle.Render(child.UseLine())+"\n"+helpCommandDetailStyle.Render(child.Short))
		}
		sections = append(sections, renderHelpSection("Commands", strings.Join(lines, "\n\n")))
	}

	if example := strings.TrimSpace(cmd.Example); example != "" {
		sections = append(sections, renderHelpSection("Examples", codeStyle.Render(example)))
	}

	if cmd.HasAvailableFlags() {
		flagLines := []string{}
		cmd.Flags().VisitAll(func(f *flag.Flag) {
			if f.Hidden {
				return
			}
			flagName := "--" + f.Name
			if f.Shorthand != "" {
				flagName = "-" + f.Shorthand + ", " + flagName
			}
			usage := f.Usage
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
				usage += " (default: " + f.DefValue + ")"
			}
			flagLines = append(flagLines, helpCommandNameStyle.Render(flagName)+"\n"+helpCommandDetailStyle.Render(usage))
		})
		if len(flagLines) > 0 {
			sections = append(sections, renderHelpSection("Flags", strings.Join(flagLines, "\n\n")))
		}
	}

	return helpContainerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// RenderInfo renders an informational message.
func RenderInfo(message string) string {
	return promptStyle.Render("→ " + message)
}

// RenderSuccess renders a success message.
func RenderSuccess(message string) string {
	return successStyle.Render("✓ " + message)
}

// RenderWarning renders a warning message.
func RenderWarning(message string) string {
	return warningStyle.Render("! " + message)
}

func renderHelpSection(title string, body string) string {
	return sectionStyle.Render(title) + "\n" + helpSectionBodyStyle.Render(body)
}

func availableSubcommands(cmd *cobra.Command) []*cobra.Command {
	children := make([]*cobra.Command, 0, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		if !child.IsAvailableCommand() || child.Hidden || child.IsAdditionalHelpTopicCommand() || child.Name() == "help" {
			continue
		}
		children = append(children, child)
	}

	return children
}
