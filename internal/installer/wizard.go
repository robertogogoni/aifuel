package installer

import (
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/robertogogoni/aifuel/internal/ui"
)

// RunWizard executes the interactive installation wizard
func RunWizard() error {
	// ── Page 1: Welcome & Detection ──────────────────────────────────────
	fmt.Println()
	fmt.Println(ui.RenderLogo())
	fmt.Println()
	fmt.Println(ui.Subtitle.Render("System Detection"))
	fmt.Println()

	det := DetectAll()

	fmt.Println(ui.RenderDetectionItem("waybar", det.WaybarFound, det.WaybarPath))
	fmt.Println(ui.RenderDetectionItem("jq", det.JqFound, det.JqPath))
	fmt.Println(ui.RenderDetectionItem("curl", det.CurlFound, det.CurlPath))
	fmt.Println(ui.RenderDetectionItem("gum", det.GumFound, det.GumPath))
	fmt.Println(ui.RenderDetectionItem("ccusage", det.CcusageFound, det.CcusagePath))
	fmt.Println(ui.RenderDetectionItem("notify-send", det.NotifyFound, det.NotifyPath))

	if det.ChromeFound {
		fmt.Println(ui.RenderDetectionItem("Chrome", true, det.ChromeVariant))
	} else {
		fmt.Println(ui.RenderDetectionItemWarn("Chrome", "no variant detected"))
	}

	fmt.Println()

	if !det.WaybarFound {
		fmt.Println(ui.Warning.Render("  waybar not found. aifuel requires waybar to display usage."))
		fmt.Println(ui.Dim.Render("  Install it with your package manager and re-run the installer."))
		fmt.Println()
	}

	// Pause before continuing
	var continueInstall bool
	continueForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Continue with installation?").
				Affirmative("Yes, let's go").
				Negative("Cancel").
				Value(&continueInstall),
		),
	).WithTheme(catppuccinTheme())

	if err := continueForm.Run(); err != nil {
		return fmt.Errorf("wizard cancelled: %w", err)
	}

	if !continueInstall {
		fmt.Println(ui.Dim.Render("Installation cancelled."))
		return nil
	}

	// ── Page 2: Provider Selection ───────────────────────────────────────
	fmt.Println()
	fmt.Println(ui.Subtitle.Render("Provider Selection"))
	fmt.Println(ui.Dim.Render("Choose which AI providers to monitor"))
	fmt.Println()

	var selectedProviders []string

	providerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("AI Providers").
				Options(
					huh.NewOption("Claude (Anthropic)", "claude").Selected(true),
					huh.NewOption("Codex (OpenAI)", "codex"),
					huh.NewOption("Gemini (Google)", "gemini"),
					huh.NewOption("Antigravity", "antigravity"),
				).
				Value(&selectedProviders).
				Filterable(false),
		),
	).WithTheme(catppuccinTheme())

	if err := providerForm.Run(); err != nil {
		return fmt.Errorf("provider selection failed: %w", err)
	}

	if len(selectedProviders) == 0 {
		selectedProviders = []string{"claude"}
	}

	// ── Page 3: Options ──────────────────────────────────────────────────
	fmt.Println()
	fmt.Println(ui.Subtitle.Render("Configuration"))
	fmt.Println(ui.Dim.Render("Customize your aifuel setup"))
	fmt.Println()

	var (
		displayMode     string
		notifications   bool
		cacheTTLStr     string
		chromeExtension bool
	)

	optionsGroup := []*huh.Group{
		huh.NewGroup(
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
	}

	// Only offer Chrome extension if Chrome was detected
	if det.ChromeFound {
		optionsGroup = append(optionsGroup,
			huh.NewGroup(
				huh.NewConfirm().
					Title("Install Chrome extension?").
					Description(fmt.Sprintf("Detected: %s", det.ChromeVariant)).
					Affirmative("Yes").
					Negative("No").
					Value(&chromeExtension),
			),
		)
	}

	optionsForm := huh.NewForm(optionsGroup...).WithTheme(catppuccinTheme())

	if err := optionsForm.Run(); err != nil {
		return fmt.Errorf("options configuration failed: %w", err)
	}

	// Parse cache TTL
	cacheTTL := 55
	switch cacheTTLStr {
	case "30":
		cacheTTL = 30
	case "120":
		cacheTTL = 120
	}

	selections := WizardSelections{
		Providers:       selectedProviders,
		DisplayMode:     displayMode,
		Notifications:   notifications,
		CacheTTL:        cacheTTL,
		ChromeExtension: chromeExtension,
		ChromeVariant:   det.ChromeVariant,
		ChromeProfile:   det.ChromeProfile,
	}

	// ── Page 4: Installation ─────────────────────────────────────────────
	fmt.Println()
	fmt.Println(ui.Subtitle.Render("Installing"))
	fmt.Println()

	var installErr error
	steps := buildInstallSteps(selections)

	for i, step := range steps {
		stepLabel := fmt.Sprintf("[%d/%d] %s", i+1, len(steps), step.label)

		action := func() {
			// Small artificial delay so spinner is visible
			time.Sleep(300 * time.Millisecond)
			if err := step.fn(); err != nil {
				installErr = err
			}
		}

		if err := spinner.New().
			Title(stepLabel).
			Action(action).
			Style(lipgloss.NewStyle().Foreground(ui.Mauve)).
			Run(); err != nil {
			return fmt.Errorf("spinner error: %w", err)
		}

		if installErr != nil {
			fmt.Println(ui.RenderStepResult(step.label, false, installErr.Error()))
			return fmt.Errorf("installation failed at step '%s': %w", step.label, installErr)
		}

		fmt.Println(ui.RenderStepResult(step.label, true, ""))
	}

	// ── Page 5: Done ─────────────────────────────────────────────────────
	fmt.Println()
	printSuccessBanner(selections)

	return nil
}

type installStep struct {
	label string
	fn    func() error
}

func buildInstallSteps(sel WizardSelections) []installStep {
	steps := []installStep{
		{
			label: "Creating directories",
			fn:    CreateDirectories,
		},
		{
			label: "Installing scripts",
			fn:    InstallScripts,
		},
		{
			label: "Writing configuration",
			fn: func() error {
				return WriteConfig(sel)
			},
		},
		{
			label: "Setting up systemd service",
			fn:    SetupSystemd,
		},
	}

	if sel.ChromeExtension {
		steps = append(steps, installStep{
			label: "Installing Chrome extension",
			fn:    InstallChromeExtension,
		})
		steps = append(steps, installStep{
			label: "Setting up native messaging host",
			fn: func() error {
				return SetupNativeHost(sel.ChromeProfile)
			},
		})
	}

	return steps
}

func printSuccessBanner(sel WizardSelections) {
	checkStyle := lipgloss.NewStyle().Foreground(ui.Green).Bold(true)
	successBanner := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Green).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(ui.Green).
		Padding(1, 3).
		MarginBottom(1).
		Render("\u2714  aifuel installed successfully!")

	fmt.Println(successBanner)
	fmt.Println()

	// Summary of what was installed
	fmt.Println(ui.Subtitle.Render("What was installed:"))
	fmt.Println()

	configDir, _, libDir := GetInstallDirs()

	items := []string{
		checkStyle.Render("\u2714") + " Scripts installed to " + ui.Dim.Render(libDir),
		checkStyle.Render("\u2714") + " Configuration at " + ui.Dim.Render(configDir+"/config.json"),
		checkStyle.Render("\u2714") + " systemd timer enabled " + ui.Dim.Render("(aifuel-feed.timer)"),
	}

	if sel.ChromeExtension {
		items = append(items,
			checkStyle.Render("\u2714")+" Chrome extension at "+ui.Dim.Render(configDir+"/chrome-extension/"),
			checkStyle.Render("\u2714")+" Native messaging host configured",
		)
	}

	for _, item := range items {
		fmt.Println("  " + item)
	}

	fmt.Println()

	// Next steps
	fmt.Println(ui.Subtitle.Render("Next steps:"))
	fmt.Println()

	stepsText := []string{
		"1. Add the waybar module to your config:",
	}
	for _, s := range stepsText {
		fmt.Println("  " + ui.Bold.Render(s))
	}

	snippet := CreateWaybarSnippet(sel.DisplayMode)
	fmt.Println()
	fmt.Println(ui.CodeBlock.Render(snippet))
	fmt.Println()

	moreSteps := []string{
		"2. Run " + ui.Code.Render("aifuel check") + " to verify everything works",
		"3. Run " + ui.Code.Render("aifuel dashboard") + " to open the TUI dashboard",
		"4. Restart waybar to load the new module",
	}
	for _, s := range moreSteps {
		fmt.Println("  " + s)
	}

	fmt.Println()
	fmt.Println(ui.Dim.Render("  Happy monitoring! \u26fd"))
	fmt.Println()
}

// catppuccinTheme returns a huh theme using Catppuccin Mocha colors
func catppuccinTheme() *huh.Theme {
	t := huh.ThemeCatppuccin()

	// Override key styles to use our palette
	t.Focused.Title = t.Focused.Title.Foreground(ui.Mauve)
	t.Focused.Description = t.Focused.Description.Foreground(ui.Subtext0)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(ui.Green)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(ui.Text)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(ui.Green).SetString("[x] ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(ui.Overlay0).SetString("[ ] ")
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Foreground(ui.Base).
		Background(ui.Mauve)
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Foreground(ui.Text).
		Background(ui.Surface0)

	return t
}
