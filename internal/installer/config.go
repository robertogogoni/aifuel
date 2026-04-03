package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/robertogogoni/aifuel/internal/ui"
)

// RunConfig presents an interactive form to edit the current aifuel configuration
func RunConfig() error {
	configDir, _, _ := GetInstallDirs()
	configPath := filepath.Join(configDir, "config.json")

	// Load existing config
	cfg := Config{
		DisplayMode:           "full",
		RefreshInterval:       30,
		CacheTTLSeconds:       55,
		NotificationsEnabled:  true,
		NotifyWarnThreshold:   80,
		NotifyCritThreshold:   95,
		NotifyCooldownMinutes: 15,
		HistoryEnabled:        true,
		HistoryRetentionDays:  7,
		Theme:                 "auto",
		Providers:             map[string]ProviderConfig{},
	}

	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	fuel := lipgloss.NewStyle().Foreground(ui.Peach).Bold(true).Render("\u26fd")
	header := lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve)

	fmt.Printf("\n%s %s\n", fuel, header.Render("Configuration Editor"))
	fmt.Printf("  %s\n\n", ui.Dim.Render(configPath))

	// Pre-fill form values from existing config
	displayMode := cfg.DisplayMode
	notifications := cfg.NotificationsEnabled
	cacheTTLStr := strconv.Itoa(cfg.CacheTTLSeconds)

	// Normalize TTL to a valid option
	switch cacheTTLStr {
	case "30", "55", "120":
		// valid
	default:
		cacheTTLStr = "55"
	}

	// Build selected providers list from config
	var selectedProviders []string
	allProviders := []string{"claude", "codex", "gemini", "antigravity"}
	for _, p := range allProviders {
		if pc, ok := cfg.Providers[p]; ok && pc.Enabled {
			selectedProviders = append(selectedProviders, p)
		}
	}
	if len(selectedProviders) == 0 {
		selectedProviders = []string{"claude"}
	}

	// Build provider options with pre-selection matching current config
	isEnabled := func(p string) bool {
		for _, sp := range selectedProviders {
			if sp == p {
				return true
			}
		}
		return false
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Enabled Providers").
				Options(
					huh.NewOption("Claude (Anthropic)", "claude").Selected(isEnabled("claude")),
					huh.NewOption("Codex (OpenAI)", "codex").Selected(isEnabled("codex")),
					huh.NewOption("Gemini (Google)", "gemini").Selected(isEnabled("gemini")),
					huh.NewOption("Antigravity", "antigravity").Selected(isEnabled("antigravity")),
				).
				Value(&selectedProviders).
				Filterable(false),
			huh.NewSelect[string]().
				Title("Display Mode").
				Description("How to show usage in waybar").
				Options(
					huh.NewOption("Icon only", "icon"),
					huh.NewOption("Compact (icon + percentage)", "compact"),
					huh.NewOption("Full (icon + percentage + label)", "full"),
				).
				Value(&displayMode),
			huh.NewConfirm().
				Title("Enable desktop notifications?").
				Description("Get notified when usage reaches thresholds").
				Affirmative("Yes").
				Negative("No").
				Value(&notifications),
			huh.NewSelect[string]().
				Title("Cache TTL").
				Description("How often to refresh usage data").
				Options(
					huh.NewOption("30 seconds (frequent)", "30"),
					huh.NewOption("55 seconds (recommended)", "55"),
					huh.NewOption("120 seconds (conservative)", "120"),
				).
				Value(&cacheTTLStr),
		),
	).WithTheme(catppuccinTheme())

	if err := form.Run(); err != nil {
		return fmt.Errorf("config editor cancelled: %w", err)
	}

	// Apply changes
	cacheTTL, _ := strconv.Atoi(cacheTTLStr)
	if cacheTTL == 0 {
		cacheTTL = 55
	}

	sel := WizardSelections{
		Providers:     selectedProviders,
		DisplayMode:   displayMode,
		Notifications: notifications,
		CacheTTL:      cacheTTL,
	}

	if err := WriteConfig(sel); err != nil {
		fmt.Printf("  %s %s\n", ui.CrossMark, ui.Error.Render(err.Error()))
		return err
	}

	fmt.Printf("\n  %s %s\n", ui.CheckMark,
		lipgloss.NewStyle().Bold(true).Foreground(ui.Green).Render("Configuration saved!"))
	fmt.Printf("  %s\n\n", ui.Dim.Render(configPath))

	return nil
}

