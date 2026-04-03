package installer

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/robertogogoni/aifuel/internal/ui"
)

// RunUninstall performs interactive uninstallation of aifuel
func RunUninstall() error {
	fmt.Print(ui.RenderRichLogo())
	fmt.Println(ui.Warning.Render("  This will remove aifuel from your system."))
	fmt.Println()

	configDir, cacheDir, libDir := GetInstallDirs()

	// Show what will be removed
	fmt.Println(ui.Subtitle.Render("The following will be removed:"))
	fmt.Println()
	fmt.Println("  " + ui.Dim.Render("\u2022 ") + libDir + ui.Dim.Render(" (scripts)"))
	fmt.Println("  " + ui.Dim.Render("\u2022 ") + cacheDir + ui.Dim.Render(" (cache)"))
	fmt.Println("  " + ui.Dim.Render("\u2022 ") + configDir + ui.Dim.Render(" (configuration)"))
	fmt.Println("  " + ui.Dim.Render("\u2022 ") + "aifuel-feed.timer" + ui.Dim.Render(" (systemd timer)"))
	fmt.Println("  " + ui.Dim.Render("\u2022 ") + "aifuel-feed.service" + ui.Dim.Render(" (systemd service)"))

	_, variant := DetectChrome()
	if variant != "" {
		fmt.Println("  " + ui.Dim.Render("\u2022 ") + "Native messaging host" + ui.Dim.Render(fmt.Sprintf(" (%s)", variant)))
	}

	fmt.Println()

	var (
		confirmUninstall bool
		preserveConfig   bool
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Preserve configuration?").
				Description("Keep ~/.config/aifuel/ for future reinstallation").
				Affirmative("Yes, keep config").
				Negative("No, remove everything").
				Value(&preserveConfig),
			huh.NewConfirm().
				Title("Proceed with uninstall?").
				Description("This action cannot be undone").
				Affirmative("Yes, uninstall").
				Negative("Cancel").
				Value(&confirmUninstall),
		),
	).WithTheme(catppuccinTheme())

	if err := form.Run(); err != nil {
		return fmt.Errorf("uninstall form error: %w", err)
	}

	if !confirmUninstall {
		fmt.Println(ui.Dim.Render("Uninstall cancelled."))
		return nil
	}

	fmt.Println()
	fmt.Println(ui.Subtitle.Render("Removing aifuel..."))
	fmt.Println()

	// Step 1: Disable systemd
	if err := DisableSystemdService(); err != nil {
		fmt.Println(ui.RenderStepResult("Disable systemd service", false, err.Error()))
	} else {
		fmt.Println(ui.RenderStepResult("Disable systemd service", true, ""))
	}

	// Step 2: Remove native messaging host
	if err := RemoveNativeHost(); err != nil {
		fmt.Println(ui.RenderStepResult("Remove native messaging host", false, err.Error()))
	} else {
		fmt.Println(ui.RenderStepResult("Remove native messaging host", true, ""))
	}

	// Step 3: Remove directories
	if err := RemoveDirectories(preserveConfig); err != nil {
		fmt.Println(ui.RenderStepResult("Remove directories", false, err.Error()))
	} else {
		if preserveConfig {
			fmt.Println(ui.RenderStepResult("Remove directories", true, "config preserved"))
		} else {
			fmt.Println(ui.RenderStepResult("Remove directories", true, ""))
		}
	}

	fmt.Println()

	successMsg := ui.Success.Render("\u2714  aifuel has been uninstalled.")
	if preserveConfig {
		successMsg += "\n" + ui.Dim.Render("  Configuration preserved at "+configDir)
	}
	fmt.Println(successMsg)
	fmt.Println()

	return nil
}
