package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

const (
	boundaryRed       = "#D14B57"
	boundaryRedDark   = "#A63A46"
	boundaryGray      = "#A1A1AA"
	boundaryGrayDark  = "#52525B"
	boundaryGrayLight = "#E4E4E7"
)

var (
	titleStyle             = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(boundaryRed))
	sectionStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(boundaryRedDark)).MarginBottom(1)
	selectedStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(boundaryRed))
	promptStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(boundaryRed))
	mutedStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color(boundaryGray))
	errorStyle             = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(boundaryRedDark))
	successStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(boundaryGrayLight))
	warningStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(boundaryRed))
	codeStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color(boundaryGrayLight))
	valueStyle             = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(boundaryGrayLight))
	helpContainerStyle     = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(boundaryGrayDark))
	helpSectionBodyStyle   = lipgloss.NewStyle().MarginLeft(2)
	helpCommandNameStyle   = codeStyle.Bold(true)
	helpCommandDetailStyle = mutedStyle.MarginLeft(2)
	promptFrameStyle       = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(boundaryGrayDark))
	promptHintStyle        = mutedStyle.MarginTop(1)
	optionDetailStyle      = mutedStyle.MarginLeft(4)
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
