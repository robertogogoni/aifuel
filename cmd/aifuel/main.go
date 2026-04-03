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

var version = "1.3.0"

// Flags
var (
	jsonOutput bool
	fullOutput bool
)

// Themed Cobra usage template — Catppuccin-colored section headers and commands
const themedUsageTemplate = `{{header "Usage:"}}{{if .Runnable}}
  {{cmdStyle .UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{cmdStyle (printf "%s [command]" .CommandPath)}}{{end}}{{if gt (len .Aliases) 0}}

{{header "Aliases:"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{header "Examples:"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

{{header "Available Commands:"}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding | cmdStyle}}  {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{header "Flags:"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{header "Global Flags:"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

{{dimStyle (printf "Use \"%s [command] --help\" for more information about a command." .CommandPath)}}{{end}}
`

func main() {
	ui.Version = version

	// Register Catppuccin-themed template functions for Cobra help
	cobra.AddTemplateFunc("header", func(s string) string {
		return lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve).Render(s)
	})
	cobra.AddTemplateFunc("cmdStyle", func(s string) string {
		return lipgloss.NewStyle().Foreground(ui.Peach).Render(s)
	})
	cobra.AddTemplateFunc("dimStyle", func(s string) string {
		return lipgloss.NewStyle().Foreground(ui.Subtext0).Render(s)
	})

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
		Long:  "Display a formatted one-line summary of current AI usage across all configured providers.\nUse --full for a detailed dashboard view, or --json for machine-readable output.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fullOutput {
				return runStatusFull()
			}
			return runStatus(jsonOutput)
		},
	}
	statusCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON for piping to other tools")
	statusCmd.Flags().BoolVar(&fullOutput, "full", false, "Show detailed dashboard with progress bars")

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
			fmt.Print(ui.RenderRichLogo())
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

	// ── auth ─────────────────────────────────────────────────────────────
	authCmd := &cobra.Command{
		Use:   "auth [provider]",
		Short: "Authenticate with AI providers",
		Long: "Check authentication status for all providers, or authenticate with a specific one.\n\n" +
			"Providers: claude, codex, gemini, copilot, codexbar\n\n" +
			"Examples:\n  aifuel auth           # Show auth status for all providers\n" +
			"  aifuel auth claude   # Authenticate with Claude\n" +
			"  aifuel auth copilot  # Authenticate with GitHub Copilot",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuth(args)
		},
		ValidArgs: []string{"claude", "codex", "gemini", "copilot", "codexbar"},
	}

	// ── completion ──────────────────────────────────────────────────────
	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: "Generate shell completion scripts for your shell.\n\n" +
			"  Bash:  source <(aifuel completion bash)\n" +
			"  Zsh:   aifuel completion zsh > \"${fpath[1]}/_aifuel\"\n" +
			"  Fish:  aifuel completion fish | source",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			}
			return nil
		},
	}

	// ── config ──────────────────────────────────────────────────────────
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Interactively edit aifuel configuration",
		Long:  "Open an interactive form to modify display mode, notifications, cache TTL, and provider settings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return installer.RunConfig()
		},
	}

	rootCmd.AddCommand(installCmd, checkCmd, dashboardCmd, statusCmd, statuslineCmd,
		uninstallCmd, versionCmd, setupChromeCmd, authCmd, completionCmd, configCmd)
	rootCmd.SetUsageTemplate(themedUsageTemplate)

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

// runStatusFull displays a rich dashboard-style status with progress bars
func runStatusFull() error {
	data, err := getProviderJSON()
	if err != nil {
		fmt.Println(ui.Dim.Render("No usage data available."))
		return nil
	}

	fiveHour := getFloat(data, "five_hour", 0)
	fiveReset := getStr(data, "five_hour_reset", "")
	sevenDay := getFloat(data, "seven_day", 0)
	sevenReset := getStr(data, "seven_day_reset", "")
	sonnet := getFloat(data, "seven_day_sonnet", 0)
	sonnetReset := getStr(data, "seven_day_sonnet_reset", "")
	dailyCost := getFloat(data, "daily_cost", 0)
	msgs := getFloat(data, "session_messages", 0)
	sessions := getFloat(data, "total_sessions", 0)
	burnRate := getFloat(data, "burn_rate_per_hour", 0)
	projected := getFloat(data, "projected_daily_cost", 0)
	source := getStr(data, "data_source", "?")
	plan := getStr(data, "plan", "?")

	fuel := lipgloss.NewStyle().Foreground(ui.Peach).Bold(true).Render("\u26fd")
	header := lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve)
	label := lipgloss.NewStyle().Foreground(ui.Subtext1).Width(14)
	dim := ui.Dim
	sep := lipgloss.NewStyle().Foreground(ui.Surface0).Render(strings.Repeat("\u2500", 42))

	fmt.Printf("\n%s %s\n%s\n\n", fuel, header.Render("aifuel Status"), "  "+sep)

	// ── Rate limits ──
	fmt.Printf("  %s\n\n", header.Render("Rate Limits"))

	renderLimit := func(name string, pct float64, reset string) {
		bar := ui.RenderProgressBar(pct, 20)
		pctStr := ui.ColorForPct(pct).Render(fmt.Sprintf("%3.0f%%", pct))
		countdown := ""
		if reset != "" {
			countdown = dim.Render("  resets in ") + dim.Render(formatCountdown(reset))
		}
		fmt.Printf("  %s %s %s%s\n", label.Render(name), bar, pctStr, countdown)
	}

	renderLimit("5-Hour", fiveHour, fiveReset)
	renderLimit("7-Day", sevenDay, sevenReset)
	if sonnet > 0 || sonnetReset != "" {
		renderLimit("7d Sonnet", sonnet, sonnetReset)
	}

	// Dynamic limits — only shown when active (non-null in API response)
	opusPct := getNullableFloat(data, "seven_day_opus")
	opusReset := getStr(data, "seven_day_opus_reset", "")
	if opusPct >= 0 {
		renderLimit("7d Opus", opusPct, opusReset)
	}
	oauthPct := getNullableFloat(data, "seven_day_oauth_apps")
	oauthReset := getStr(data, "seven_day_oauth_apps_reset", "")
	if oauthPct >= 0 {
		renderLimit("7d OAuth", oauthPct, oauthReset)
	}
	coworkPct := getNullableFloat(data, "seven_day_cowork")
	coworkReset := getStr(data, "seven_day_cowork_reset", "")
	if coworkPct >= 0 {
		renderLimit("7d Cowork", coworkPct, coworkReset)
	}

	// Extra usage credits
	extraEnabled := false
	if v, ok := data["extra_usage_enabled"]; ok {
		if b, ok := v.(bool); ok {
			extraEnabled = b
		}
	}
	if extraEnabled {
		extraCredits := getFloat(data, "extra_usage_credits", 0)
		extraLimit := getNullableFloat(data, "extra_usage_monthly_limit")
		extraUtil := getNullableFloat(data, "extra_usage_utilization")
		creditStr := lipgloss.NewStyle().Foreground(ui.Yellow).Render(fmt.Sprintf("$%.2f", extraCredits))
		if extraLimit >= 0 {
			creditStr += dim.Render(fmt.Sprintf(" / $%.2f", extraLimit))
		}
		if extraUtil >= 0 {
			fmt.Printf("\n  %s\n\n", header.Render("Extra Usage"))
			renderLimit("Credits", extraUtil, "")
			fmt.Printf("  %s %s\n", label.Render("Spent"), creditStr)
		} else if extraCredits > 0 {
			fmt.Printf("\n  %s\n\n", header.Render("Extra Usage"))
			fmt.Printf("  %s %s %s\n", label.Render("Credits used"), creditStr,
				dim.Render("(enabled)"))
		}
	}

	// ── Daily cost ──
	fmt.Printf("\n  %s\n\n", header.Render("Daily Cost"))
	fmt.Printf("  %s %s  %s\n",
		label.Render("Total"),
		lipgloss.NewStyle().Bold(true).Foreground(ui.Yellow).Render(fmt.Sprintf("$%.2f", dailyCost)),
		dim.Render(fmt.Sprintf("(%.0f messages, %.0f sessions)", msgs, sessions)))
	fmt.Printf("  %s %s\n",
		label.Render("Burn rate"),
		dim.Render(fmt.Sprintf("$%.2f/hr", burnRate)))
	fmt.Printf("  %s %s\n",
		label.Render("Projected"),
		dim.Render(fmt.Sprintf("$%.2f (16hr day)", projected)))

	// ── Model breakdown ──
	if models, ok := data["models"]; ok {
		if modelArr, ok := models.([]interface{}); ok && len(modelArr) > 0 {
			fmt.Printf("\n  %s\n\n", header.Render("Model Breakdown"))
			for _, mi := range modelArr {
				m, ok := mi.(map[string]interface{})
				if !ok {
					continue
				}
				name := getStr(m, "model", "?")
				cost := getFloat(m, "cost", 0)
				input := getFloat(m, "input", 0)
				output := getFloat(m, "output", 0)
				fmt.Printf("  %s %s  %s\n",
					label.Render(name),
					lipgloss.NewStyle().Foreground(ui.Yellow).Render(fmt.Sprintf("$%.2f", cost)),
					dim.Render(fmt.Sprintf("(%s in / %s out)", fmtTokens(input), fmtTokens(output))))
			}
		}
	}

	// ── Account info ──
	if acct, ok := data["account"].(map[string]interface{}); ok {
		tier := getStr(acct, "rate_limit_tier", "")
		billing := getStr(acct, "billing_type", "")
		if tier != "" || billing != "" {
			fmt.Printf("\n  %s\n\n", header.Render("Account"))
			if tier != "" {
				fmt.Printf("  %s %s\n", label.Render("Rate tier"),
					lipgloss.NewStyle().Foreground(ui.Peach).Bold(true).Render(tier))
			}
			if billing != "" {
				fmt.Printf("  %s %s\n", label.Render("Billing"), dim.Render(billing))
			}
			if caps, ok := acct["capabilities"].([]interface{}); ok && len(caps) > 0 {
				capStrs := make([]string, len(caps))
				for i, c := range caps {
					if s, ok := c.(string); ok {
						capStrs[i] = s
					}
				}
				fmt.Printf("  %s %s\n", label.Render("Capabilities"),
					dim.Render(strings.Join(capStrs, ", ")))
			}
			if models, ok := acct["models"].([]interface{}); ok && len(models) > 0 {
				var names []string
				for _, mi := range models {
					if m, ok := mi.(map[string]interface{}); ok {
						names = append(names, getStr(m, "name", ""))
					}
				}
				fmt.Printf("  %s %s\n", label.Render("Models"),
					dim.Render(strings.Join(names, ", ")))
			}
		}
	}

	// ── Footer ──
	fmt.Printf("\n  %s\n\n",
		dim.Render(fmt.Sprintf("Source: %s  |  Plan: %s", source, plan)))

	return nil
}

// formatCountdown converts an ISO 8601 timestamp to a human-friendly countdown
func formatCountdown(iso string) string {
	if iso == "" || iso == "null" {
		return ""
	}
	// Shell out to date for portability with ISO 8601 parsing
	cmd := exec.Command("date", "-d", iso, "+%s")
	out, err := cmd.Output()
	if err != nil {
		return iso
	}
	var resetEpoch int64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &resetEpoch)
	cmd2 := exec.Command("date", "+%s")
	out2, _ := cmd2.Output()
	var nowEpoch int64
	fmt.Sscanf(strings.TrimSpace(string(out2)), "%d", &nowEpoch)

	diff := resetEpoch - nowEpoch
	if diff <= 0 {
		return "expired"
	}
	d := diff / 86400
	h := (diff % 86400) / 3600
	m := (diff % 3600) / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return "< 1m"
	}
}

// fmtTokens formats a token count as human-readable: 842, 24.9K, 6.1M
func fmtTokens(n float64) string {
	switch {
	case n >= 1000000:
		return fmt.Sprintf("%.1fM", n/1000000)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", n/1000)
	default:
		return fmt.Sprintf("%.0f", n)
	}
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

// getNullableFloat returns the float value for a key, or -1 if null/missing.
// This distinguishes "0%" (active limit at 0) from "null" (limit not active).
func getNullableFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return -1
	}
	if f, ok := v.(float64); ok {
		return f
	}
	return -1
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

// runAuth handles the auth command logic
func runAuth(args []string) error {
	fuelIcon := lipgloss.NewStyle().Foreground(ui.Peach).Render("\u26fd")

	if len(args) == 0 {
		// ── Status table for all providers ────────────────────────────────
		fmt.Printf("\n%s %s\n\n",
			fuelIcon,
			lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve).Render("Authentication Status"))

		results := installer.AuthAll()

		// Provider display metadata
		type providerMeta struct {
			icon string
			name string
		}
		meta := map[string]providerMeta{
			"claude":   {icon: "\U0001f9e0", name: "Claude"},
			"codex":    {icon: "\U0001f4bb", name: "Codex"},
			"gemini":   {icon: "\u2728", name: "Gemini"},
			"copilot":  {icon: "\U0001f419", name: "Copilot"},
			"codexbar": {icon: "\U0001f4ca", name: "CodexBar"},
		}

		nameStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Text).Width(12)
		cliStyle := lipgloss.NewStyle().Foreground(ui.Subtext0).Width(12)
		pathStyle := lipgloss.NewStyle().Foreground(ui.Overlay0)

		// Header
		headerName := lipgloss.NewStyle().Bold(true).Foreground(ui.Blue).Width(14).Render("Provider")
		headerCLI := lipgloss.NewStyle().Bold(true).Foreground(ui.Blue).Width(12).Render("CLI")
		headerInstall := lipgloss.NewStyle().Bold(true).Foreground(ui.Blue).Width(12).Render("Installed")
		headerAuth := lipgloss.NewStyle().Bold(true).Foreground(ui.Blue).Width(12).Render("Authed")
		headerCred := lipgloss.NewStyle().Bold(true).Foreground(ui.Blue).Render("Credentials")
		fmt.Printf("  %s%s%s%s%s\n", headerName, headerCLI, headerInstall, headerAuth, headerCred)
		fmt.Printf("  %s\n", lipgloss.NewStyle().Foreground(ui.Surface0).Render(strings.Repeat("\u2500", 72)))

		for _, r := range results {
			m := meta[r.Provider]

			installMark := ui.CrossMark
			if r.Installed {
				installMark = ui.CheckMark
			}

			authMark := ui.CrossMark
			if r.NowAuthed {
				authMark = ui.CheckMark
			}

			credPath := installer.CredentialPath(r.Provider)
			if !r.Installed {
				credPath = "CLI not installed"
			} else if !r.NowAuthed {
				credPath = "not authenticated"
			}

			fmt.Printf("  %s %s%s%s    %s    %s\n",
				m.icon,
				nameStyle.Render(m.name),
				cliStyle.Render(r.CliTool),
				installMark,
				authMark,
				pathStyle.Render(credPath),
			)
		}

		fmt.Printf("\n  %s\n\n",
			ui.Dim.Render("Run 'aifuel auth <provider>' to authenticate with a specific provider."))
		return nil
	}

	// ── Single provider auth flow ────────────────────────────────────────
	provider := strings.ToLower(args[0])
	fmt.Printf("\n%s %s %s\n\n",
		fuelIcon,
		lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve).Render("Authenticating:"),
		lipgloss.NewStyle().Bold(true).Foreground(ui.Peach).Render(capitalize(provider)))

	result := installer.RunAuthFlow(provider)

	if result.Error != nil && !result.Installed {
		fmt.Printf("  %s %s\n\n", ui.CrossMark, ui.Error.Render(result.Error.Error()))
		return nil
	}

	if result.WasAuthed {
		fmt.Printf("  %s Already authenticated with %s\n",
			ui.CheckMark,
			lipgloss.NewStyle().Bold(true).Foreground(ui.Text).Render(capitalize(provider)))
		credPath := installer.CredentialPath(provider)
		fmt.Printf("  %s Credentials: %s\n\n", ui.Dim.Render("\u2022"), ui.Dim.Render(credPath))
		return nil
	}

	if result.NowAuthed {
		fmt.Printf("\n  %s %s\n",
			ui.CheckMark,
			lipgloss.NewStyle().Bold(true).Foreground(ui.Green).Render("Authentication successful!"))
		credPath := installer.CredentialPath(provider)
		fmt.Printf("  %s Credentials: %s\n\n", ui.Dim.Render("\u2022"), ui.Dim.Render(credPath))
		return nil
	}

	if result.Error != nil {
		fmt.Printf("\n  %s %s\n\n", ui.CrossMark, ui.Error.Render(result.Error.Error()))
	} else {
		fmt.Printf("\n  %s %s\n\n", ui.CrossMark, ui.Error.Render("Authentication was not completed."))
	}
	return nil
}
