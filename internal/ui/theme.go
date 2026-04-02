package ui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette
const (
	Rosewater = lipgloss.Color("#f5e0dc")
	Flamingo  = lipgloss.Color("#f2cdcd")
	Pink      = lipgloss.Color("#f5c2e7")
	Mauve     = lipgloss.Color("#cba6f7")
	Red       = lipgloss.Color("#f38ba8")
	Maroon    = lipgloss.Color("#eba0ac")
	Peach     = lipgloss.Color("#fab387")
	Yellow    = lipgloss.Color("#f9e2af")
	Green     = lipgloss.Color("#a6e3a1")
	Teal      = lipgloss.Color("#94e2d5")
	Sky       = lipgloss.Color("#89dceb")
	Sapphire  = lipgloss.Color("#74c7ec")
	Blue      = lipgloss.Color("#89b4fa")
	Lavender  = lipgloss.Color("#b4befe")
	Text      = lipgloss.Color("#cdd6f4")
	Subtext1  = lipgloss.Color("#bac2de")
	Subtext0  = lipgloss.Color("#a6adc8")
	Overlay0  = lipgloss.Color("#6c7086")
	Surface0  = lipgloss.Color("#313244")
	Base      = lipgloss.Color("#1e1e2e")
	Mantle    = lipgloss.Color("#181825")
	Crust     = lipgloss.Color("#11111b")
)

var (
	// Title style for big headers
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Mauve).
		MarginBottom(1)

	// Subtitle for section headers
	Subtitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Blue)

	// Success for positive messages
	Success = lipgloss.NewStyle().
		Foreground(Green)

	// Warning for caution messages
	Warning = lipgloss.NewStyle().
		Foreground(Yellow)

	// Error for failure messages
	Error = lipgloss.NewStyle().
		Foreground(Red)

	// Dim for secondary information
	Dim = lipgloss.NewStyle().
		Foreground(Overlay0)

	// Bold text
	Bold = lipgloss.NewStyle().
		Bold(true).
		Foreground(Text)

	// Code for inline code snippets
	Code = lipgloss.NewStyle().
		Foreground(Peach).
		Background(Surface0).
		Padding(0, 1)

	// Banner for the main logo area
	Banner = lipgloss.NewStyle().
		Bold(true).
		Foreground(Mauve).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Lavender).
		Padding(1, 3).
		MarginBottom(1)

	// CheckMark styled green
	CheckMark = Success.Render("\u2714")

	// CrossMark styled red
	CrossMark = Error.Render("\u2718")

	// WarnMark styled yellow
	WarnMark = Warning.Render("\u26a0")

	// Highlight for important values
	Highlight = lipgloss.NewStyle().
			Bold(true).
			Foreground(Peach)

	// Box for info panels
	Box = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Surface0).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	// CodeBlock for multiline code
	CodeBlock = lipgloss.NewStyle().
			Foreground(Teal).
			Background(Surface0).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)
)

// RenderLogo returns the styled aifuel logo
func RenderLogo() string {
	logo := lipgloss.NewStyle().
		Bold(true).
		Foreground(Peach).
		Render("\u26fd aifuel")

	tagline := lipgloss.NewStyle().
		Foreground(Subtext0).
		Render("   AI Usage Fuel Gauge for Waybar")

	content := logo + "\n" + tagline

	return Banner.Render(content)
}

// RenderDetectionItem renders a single detection line
func RenderDetectionItem(name string, found bool, extra string) string {
	mark := CrossMark
	status := Error.Render("not found")
	if found {
		mark = CheckMark
		status = Success.Render("found")
	}

	line := mark + " " + Bold.Render(name) + " " + Dim.Render("...") + " " + status
	if extra != "" {
		line += " " + Dim.Render("("+extra+")")
	}
	return line
}

// RenderDetectionItemWarn renders a detection item with warning state
func RenderDetectionItemWarn(name string, message string) string {
	return WarnMark + " " + Bold.Render(name) + " " + Dim.Render("...") + " " + Warning.Render(message)
}

// RenderStepResult renders an installation step result
func RenderStepResult(step string, ok bool, detail string) string {
	mark := CrossMark
	if ok {
		mark = CheckMark
	}
	line := mark + " " + step
	if detail != "" {
		line += " " + Dim.Render(detail)
	}
	return line
}
