package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Version can be set by the main package to display in the logo
var Version = "dev"

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

// RenderRichLogo returns a rich, pixel-art fuel pump logo with gradient coloring.
// Designed to match the visual quality of the Claude Code startup banner.
//
// The pump uses a warm-to-shadow vertical gradient:
//   - Flamingo (highlight) cap/top
//   - Peach (main body)
//   - Green (fuel gauge window)
//   - Mauve (display panel)
//   - Maroon (shadow) base
func RenderRichLogo() string {
	p := lipgloss.NewStyle().Foreground(Peach).Bold(true)
	ph := lipgloss.NewStyle().Foreground(Flamingo)
	g := lipgloss.NewStyle().Foreground(Green).Bold(true)
	m := lipgloss.NewStyle().Foreground(Mauve)
	d := lipgloss.NewStyle().Foreground(Overlay0)
	sh := lipgloss.NewStyle().Foreground(Maroon)

	// Fuel pump pixel art — pill-shaped body with uniform-width lines
	// ▄ top cap and ▀ bottom cap create rounded edges
	// Explicit 2-space indent per line (more reliable than MarginLeft with JoinHorizontal)
	pump := strings.Join([]string{
		"  " + ph.Render("▄██████▄"),
		"  " + p.Render("████████") + d.Render("╮"),
		"  " + p.Render("██") + g.Render("████") + p.Render("██") + d.Render("│"),
		"  " + p.Render("██") + g.Render("████") + p.Render("██") + d.Render("╯"),
		"  " + p.Render("████████"),
		"  " + p.Render("██") + m.Render("████") + p.Render("██"),
		"  " + sh.Render("████████"),
		"  " + sh.Render("▀██████▀"),
	}, "\n")

	// Text info block (aligned to pump lines via JoinHorizontal)
	fuel := lipgloss.NewStyle().Foreground(Peach).Bold(true).Render("\u26fd")
	name := lipgloss.NewStyle().Foreground(Peach).Bold(true).Render("aifuel")
	ver := lipgloss.NewStyle().Foreground(Mauve).Bold(true).Render("v" + Version)
	tag := lipgloss.NewStyle().Foreground(Subtext0).Render("AI Usage Fuel Gauge for Waybar")

	// Gradient fuel bar: Peach → Yellow → Green (warm to cool, like a real gauge)
	bar := lipgloss.NewStyle().Foreground(Peach).Render("\u25b0\u25b0\u25b0\u25b0") +
		lipgloss.NewStyle().Foreground(Yellow).Render("\u25b0\u25b0\u25b0\u25b0\u25b0") +
		lipgloss.NewStyle().Foreground(Green).Render("\u25b0\u25b0\u25b0\u25b0\u25b0") +
		lipgloss.NewStyle().Foreground(Surface0).Render("\u25b1\u25b1\u25b1\u25b1\u25b1\u25b1")

	info := strings.Join([]string{
		"",
		"",
		fuel + " " + name + " " + ver,
		tag,
		"",
		bar,
		"",
		"",
	}, "\n")

	combined := lipgloss.JoinHorizontal(lipgloss.Top, pump, "   ", info)
	return "\n" + combined + "\n"
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

// ColorForPct returns a lipgloss style colored by usage percentage threshold
func ColorForPct(pct float64) lipgloss.Style {
	switch {
	case pct >= 85:
		return lipgloss.NewStyle().Bold(true).Foreground(Red)
	case pct >= 60:
		return lipgloss.NewStyle().Bold(true).Foreground(Yellow)
	default:
		return lipgloss.NewStyle().Bold(true).Foreground(Green)
	}
}

// RenderProgressBar returns a colored ▰▱ bar. Width is total cells, pct is 0-100.
func RenderProgressBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	color := Green
	switch {
	case pct >= 85:
		color = Red
	case pct >= 60:
		color = Yellow
	}

	filledStr := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("\u25b0", filled))
	emptyStr := lipgloss.NewStyle().Foreground(Surface0).Render(strings.Repeat("\u25b1", width-filled))
	return filledStr + emptyStr
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
