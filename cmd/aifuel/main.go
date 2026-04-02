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

var version = "1.1.0"

// Flags
var jsonOutput bool

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
		Long:  "Display a formatted one-line summary of current AI usage across all configured providers.\nUse --json for machine-readable output.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(jsonOutput)
		},
	}
	statusCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON for piping to other tools")

	// ── statusline ──────────────────────────────────────────────────────
	statuslineCmd := &cobra.Command{
		Use:   "statusline",
		Short: "Output compact status for Claude Code statusLine",
		Long: "Output a compact one-line string suitable for Claude Code's statusLine\n" +
			"integration. Configure in Claude Code settings as the statusLine command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusLine()
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

	// ── setup-chrome ────────────────────────────────────────────────
	setupChromeCmd := &cobra.Command{
		Use:   "setup-chrome",
		Short: "Configure Chrome extension and native messaging host",
		Long: "Detects the AIFuel Chrome extension, reads its ID, and configures\n" +
			"the native messaging host so live usage data flows to waybar.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupChrome()
		},
	}

	rootCmd.AddCommand(installCmd, checkCmd, dashboardCmd, statusCmd, statuslineCmd, uninstallCmd, versionCmd, setupChromeCmd)

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

// getProviderJSON runs aifuel-claude.sh and returns the parsed JSON
func getProviderJSON() (map[string]interface{}, error) {
	scriptPath := findScript("aifuel-claude.sh")
	if scriptPath == "" {
		return nil, fmt.Errorf("aifuel-claude.sh not found. Is aifuel installed? Run 'aifuel install' first")
	}

	cmd := exec.Command("bash", scriptPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run status script: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return nil, fmt.Errorf("no usage data available")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}
	return data, nil
}

// runStatus displays a styled status line or raw JSON
func runStatus(asJSON bool) error {
	data, err := getProviderJSON()
	if err != nil {
		if asJSON {
			fmt.Println("{}")
		} else {
			fmt.Println(ui.Dim.Render("No usage data available."))
		}
		return nil
	}

	if asJSON {
		out, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fiveHour := getFloat(data, "five_hour", 0)
	sevenDay := getFloat(data, "seven_day", 0)
	dailyCost := getFloat(data, "daily_cost", 0)
	msgs := getFloat(data, "session_messages", 0)
	source := getStr(data, "data_source", "?")
	burnRate := getFloat(data, "burn_rate_per_hour", 0)

	colorForPct := func(pct float64) lipgloss.Style {
		switch {
		case pct >= 85:
			return lipgloss.NewStyle().Bold(true).Foreground(ui.Red)
		case pct >= 60:
			return lipgloss.NewStyle().Bold(true).Foreground(ui.Yellow)
		default:
			return lipgloss.NewStyle().Bold(true).Foreground(ui.Green)
		}
	}

	fuel := lipgloss.NewStyle().Foreground(ui.Peach).Render("\u26fd")
	label := lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve).Render("Claude")
	fh := colorForPct(fiveHour).Render(fmt.Sprintf("%0.f%%", fiveHour))
	sd := colorForPct(sevenDay).Render(fmt.Sprintf("%0.f%%", sevenDay))
	cost := lipgloss.NewStyle().Foreground(ui.Yellow).Render(fmt.Sprintf("$%.2f", dailyCost))
	dim := ui.Dim

	fmt.Printf("%s %s  5h: %s  7d: %s  %s  %s msgs  %s/hr  via %s\n",
		fuel, label, fh, sd, cost,
		dim.Render(fmt.Sprintf("%.0f", msgs)),
		dim.Render(fmt.Sprintf("$%.2f", burnRate)),
		dim.Render(source))
	return nil
}

// runStatusLine outputs compact text for Claude Code's statusLine integration
func runStatusLine() error {
	data, err := getProviderJSON()
	if err != nil {
		fmt.Print("aifuel: no data")
		return nil
	}

	fiveHour := getFloat(data, "five_hour", 0)
	sevenDay := getFloat(data, "seven_day", 0)
	dailyCost := getFloat(data, "daily_cost", 0)
	msgs := getFloat(data, "session_messages", 0)

	// Compact format for Claude Code statusLine: "5h:14% 7d:2% $12.49 326msg"
	fmt.Printf("5h:%.0f%% 7d:%.0f%% $%.2f %.0fmsg", fiveHour, sevenDay, dailyCost, msgs)
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

func runSetupChrome() error {
	fuelIcon := lipgloss.NewStyle().Foreground(ui.Peach).Render("\u26fd")
	fmt.Printf("%s %s\n\n", fuelIcon, lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve).Render("Chrome Extension Setup"))

	// Step 1: Detect Chrome
	profilePath, variant := installer.DetectChrome()
	if variant == "" {
		return fmt.Errorf("no Chrome variant detected. Install Chrome, Chromium, or Brave first")
	}
	fmt.Printf("  %s Chrome variant: %s\n", ui.Success.Render("✓"), variant)

	// Step 2: Check extension is installed
	configDir, _, _ := installer.GetInstallDirs()
	extDir := filepath.Join(configDir, "chrome-extension")
	if _, err := os.Stat(filepath.Join(extDir, "manifest.json")); os.IsNotExist(err) {
		return fmt.Errorf("extension not found at %s. Run 'aifuel install' first", extDir)
	}
	fmt.Printf("  %s Extension files at %s\n", ui.Success.Render("✓"), extDir)

	// Step 3: Auto-detect extension ID from Chrome preferences
	extID := installer.DetectExtensionID(profilePath, "AIFuel Live Feed")
	if extID == "" {
		// Also try the legacy name
		extID = installer.DetectExtensionID(profilePath, "AI Usage Live Feed")
	}

	if extID == "" {
		fmt.Printf("\n  %s Extension not loaded in %s yet.\n\n", ui.Warning.Render("!"), variant)
		fmt.Printf("  %s\n", lipgloss.NewStyle().Bold(true).Foreground(ui.Text).Render("To load the extension:"))
		fmt.Printf("    1. Open %s and go to chrome://extensions\n", variant)
		fmt.Printf("    2. Enable \"Developer mode\" (top right toggle)\n")
		fmt.Printf("    3. Click \"Load unpacked\" and select:\n")
		fmt.Printf("       %s\n", ui.Code.Render(extDir))
		fmt.Printf("    4. Run this command again: %s\n\n", ui.Code.Render("aifuel setup-chrome"))
		return nil
	}

	fmt.Printf("  %s Extension ID: %s\n", ui.Success.Render("✓"), extID)

	// Step 4: Create/update native messaging host manifest
	err := installer.SetupNativeHost(profilePath, extID)
	if err != nil {
		return fmt.Errorf("failed to set up native host: %w", err)
	}
	fmt.Printf("  %s Native messaging host configured\n", ui.Success.Render("✓"))

	// Step 5: Verify
	nativeDir := installer.GetNativeMessagingHostDir(profilePath)
	manifestPath := filepath.Join(nativeDir, "com.aifuel.live_feed.json")
	fmt.Printf("  %s Manifest: %s\n", ui.Success.Render("✓"), manifestPath)

	fmt.Printf("\n  %s\n", lipgloss.NewStyle().Bold(true).Foreground(ui.Green).Render("Chrome extension is ready!"))
	fmt.Printf("  Live usage data will flow to waybar within 2 minutes.\n")
	fmt.Printf("  Make sure you're logged into %s\n\n", ui.Code.Render("claude.ai"))

	return nil
}
