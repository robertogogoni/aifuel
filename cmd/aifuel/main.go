package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/robertogogoni/aifuel/internal/installer"
	"github.com/robertogogoni/aifuel/internal/ui"
	"github.com/spf13/cobra"
)

var version = "1.0.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "aifuel",
		Short: "AI Usage Fuel Gauge for Waybar",
		Long: lipgloss.NewStyle().Foreground(ui.Mauve).Bold(true).Render("\u26fd aifuel") +
			" " + lipgloss.NewStyle().Foreground(ui.Subtext0).Render("- AI Usage Fuel Gauge for Waybar") + "\n\n" +
			lipgloss.NewStyle().Foreground(ui.Text).Render(
				"Monitor your AI provider usage (Claude, Codex, Gemini) directly in your\n"+
					"waybar status bar. Beautiful TUI installer, systemd integration, and\n"+
					"optional Chrome extension for real-time tracking."),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default action: run the install wizard
			return installer.RunWizard()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// ── install ──────────────────────────────────────────────────────────
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Run the interactive installation wizard",
		Long:  "Launch the beautiful TUI wizard to install and configure aifuel on your system.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return installer.RunWizard()
		},
	}

	// ── check ────────────────────────────────────────────────────────────
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Run diagnostics to verify aifuel installation",
		Long:  "Execute the aifuel-check.sh diagnostic script to verify your installation is working correctly.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScript("aifuel-check.sh")
		},
	}

	// ── dashboard ────────────────────────────────────────────────────────
	dashboardCmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Open the TUI dashboard",
		Long:  "Launch the interactive terminal dashboard showing real-time AI usage across all configured providers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScript("dashboard.sh")
		},
	}

	// ── status ───────────────────────────────────────────────────────────
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show quick one-line usage status",
		Long:  "Display a formatted one-line summary of current AI usage across all configured providers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}

	// ── uninstall ────────────────────────────────────────────────────────
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove aifuel from your system",
		Long:  "Interactively remove aifuel, with the option to preserve your configuration for future reinstallation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return installer.RunUninstall()
		},
	}

	// ── version ──────────────────────────────────────────────────────────
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show aifuel version",
		Run: func(cmd *cobra.Command, args []string) {
			logo := lipgloss.NewStyle().Bold(true).Foreground(ui.Peach).Render("\u26fd aifuel")
			ver := lipgloss.NewStyle().Foreground(ui.Mauve).Render("v" + version)
			fmt.Printf("%s %s\n", logo, ver)
		},
	}

	rootCmd.AddCommand(installCmd, checkCmd, dashboardCmd, statusCmd, uninstallCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error.Render("Error: "+err.Error()))
		os.Exit(1)
	}
}

// runScript finds and executes a script from the aifuel lib directory
func runScript(name string) error {
	scriptPath := findScript(name)
	if scriptPath == "" {
		return fmt.Errorf("script %s not found. Is aifuel installed? Run 'aifuel install' first", name)
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// findScript looks for a script in known locations
func findScript(name string) string {
	_, _, libDir := installer.GetInstallDirs()

	// Check installed location
	installed := filepath.Join(libDir, name)
	if _, err := os.Stat(installed); err == nil {
		return installed
	}

	// Check local dev scripts/ directory
	local := filepath.Join("scripts", name)
	if _, err := os.Stat(local); err == nil {
		abs, err := filepath.Abs(local)
		if err == nil {
			return abs
		}
		return local
	}

	// Check relative to binary
	exe, err := os.Executable()
	if err == nil {
		binDir := filepath.Dir(exe)
		nearby := filepath.Join(binDir, "scripts", name)
		if _, err := os.Stat(nearby); err == nil {
			return nearby
		}
	}

	return ""
}

// runStatus executes aifuel-claude.sh and formats the output as a styled one-liner
func runStatus() error {
	scriptPath := findScript("aifuel-claude.sh")
	if scriptPath == "" {
		return fmt.Errorf("aifuel-claude.sh not found. Is aifuel installed? Run 'aifuel install' first")
	}

	cmd := exec.Command("bash", scriptPath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run status script: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		fmt.Println(ui.Dim.Render("No usage data available."))
		return nil
	}

	// Try to parse as JSON for nice formatting
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err == nil {
		return renderStatusJSON(data)
	}

	// Fallback: just print raw output
	fmt.Println(raw)
	return nil
}

func renderStatusJSON(data map[string]interface{}) error {
	provider := getStr(data, "provider", "unknown")
	usagePct := getFloat(data, "usage_pct", -1)
	resetDate := getStr(data, "reset_date", "")

	// Choose color based on usage
	var pctStyle lipgloss.Style
	switch {
	case usagePct >= 90:
		pctStyle = lipgloss.NewStyle().Bold(true).Foreground(ui.Red)
	case usagePct >= 70:
		pctStyle = lipgloss.NewStyle().Bold(true).Foreground(ui.Yellow)
	case usagePct >= 0:
		pctStyle = lipgloss.NewStyle().Bold(true).Foreground(ui.Green)
	default:
		pctStyle = lipgloss.NewStyle().Foreground(ui.Overlay0)
	}

	fuelIcon := lipgloss.NewStyle().Foreground(ui.Peach).Render("\u26fd")
	providerLabel := lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve).Render(capitalize(provider))

	line := fuelIcon + " " + providerLabel

	if usagePct >= 0 {
		line += " " + pctStyle.Render(fmt.Sprintf("%.0f%%", usagePct)) + " used"
	}

	if resetDate != "" {
		line += " " + ui.Dim.Render("(resets "+resetDate+")")
	}

	fmt.Println(line)
	return nil
}

func getStr(m map[string]interface{}, key, fallback string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return fallback
}

func getFloat(m map[string]interface{}, key string, fallback float64) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return fallback
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
