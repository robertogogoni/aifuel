package installer

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Version is set by main.go from the build-time ldflags value.
// InstallScripts and InstallChromeExtension inject this into deployed files,
// replacing the "dev" placeholder so the repo never has a hardcoded version.
var Version = "dev"

//go:embed all:scripts
var embeddedScripts embed.FS

//go:embed all:chrome-extension
var embeddedChromeExt embed.FS

// ProviderConfig holds per-provider settings
type ProviderConfig struct {
	Enabled bool `json:"enabled"`
}

// Config holds the aifuel configuration
type Config struct {
	DisplayMode           string                    `json:"display_mode"`
	RefreshInterval       int                       `json:"refresh_interval"`
	CacheTTLSeconds       int                       `json:"cache_ttl_seconds"`
	NotificationsEnabled  bool                      `json:"notifications_enabled"`
	NotifyWarnThreshold   int                       `json:"notify_warn_threshold"`
	NotifyCritThreshold   int                       `json:"notify_critical_threshold"`
	NotifyCooldownMinutes int                       `json:"notify_cooldown_minutes"`
	HistoryEnabled        bool                      `json:"history_enabled"`
	HistoryRetentionDays  int                       `json:"history_retention_days"`
	Theme                 string                    `json:"theme"`
	Providers             map[string]ProviderConfig `json:"providers"`
}

// WizardSelections holds the user's wizard choices
type WizardSelections struct {
	Providers       []string
	DisplayMode     string
	Notifications   bool
	CacheTTL        int
	ChromeExtension bool
	ChromeVariant   string
	ChromeProfile   string
}

// CreateDirectories creates all needed aifuel directories
func CreateDirectories() error {
	configDir, cacheDir, libDir := GetInstallDirs()

	dirs := []string{
		configDir,
		cacheDir,
		libDir,
		filepath.Join(configDir, "chrome-extension"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// InstallScripts copies scripts from embedded FS to ~/.local/lib/aifuel/
// and stamps the runtime version into lib.sh (replacing the "dev" placeholder).
func InstallScripts() error {
	_, _, libDir := GetInstallDirs()

	entries, err := fs.ReadDir(embeddedScripts, "scripts")
	if err != nil {
		return fmt.Errorf("failed to read embedded scripts: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".gitkeep" {
			continue
		}

		data, err := fs.ReadFile(embeddedScripts, filepath.Join("scripts", entry.Name()))
		if err != nil {
			return fmt.Errorf("failed to read embedded script %s: %w", entry.Name(), err)
		}

		// Inject the build-time version into lib.sh
		if entry.Name() == "lib.sh" && Version != "dev" {
			data = []byte(strings.Replace(string(data),
				`AIFUEL_VERSION="dev"`,
				fmt.Sprintf(`AIFUEL_VERSION="%s"`, Version), 1))
		}

		destPath := filepath.Join(libDir, entry.Name())
		if err := os.WriteFile(destPath, data, 0755); err != nil {
			return fmt.Errorf("failed to write script %s: %w", destPath, err)
		}
	}

	return nil
}

// WriteConfig generates config.json from wizard selections
func WriteConfig(sel WizardSelections) error {
	configDir, _, _ := GetInstallDirs()

	// Build providers map: all known providers, enabled based on selection
	allProviders := []string{"claude", "codex", "gemini", "antigravity", "copilot"}
	providers := make(map[string]ProviderConfig, len(allProviders))
	for _, p := range allProviders {
		enabled := false
		for _, sp := range sel.Providers {
			if sp == p {
				enabled = true
				break
			}
		}
		providers[p] = ProviderConfig{Enabled: enabled}
	}

	cfg := Config{
		DisplayMode:           sel.DisplayMode,
		RefreshInterval:       30,
		CacheTTLSeconds:       sel.CacheTTL,
		NotificationsEnabled:  sel.Notifications,
		NotifyWarnThreshold:   80,
		NotifyCritThreshold:   95,
		NotifyCooldownMinutes: 15,
		HistoryEnabled:        true,
		HistoryRetentionDays:  7,
		Theme:                 "auto",
		Providers:             providers,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// SetupSystemd creates and enables the aifuel-feed systemd user service
func SetupSystemd() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	_, _, libDir := GetInstallDirs()
	serviceDir := filepath.Join(home, ".config", "systemd", "user")

	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user dir: %w", err)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=aifuel feed service - AI usage data for waybar
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=%s/aifuel-feed.sh
Environment=HOME=%s

[Install]
WantedBy=default.target
`, libDir, home)

	timerContent := `[Unit]
Description=aifuel feed timer - periodic AI usage refresh

[Timer]
OnBootSec=30s
OnUnitActiveSec=55s
AccuracySec=5s

[Install]
WantedBy=timers.target
`

	servicePath := filepath.Join(serviceDir, "aifuel-feed.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	timerPath := filepath.Join(serviceDir, "aifuel-feed.timer")
	if err := os.WriteFile(timerPath, []byte(timerContent), 0644); err != nil {
		return fmt.Errorf("failed to write timer file: %w", err)
	}

	// Reload systemd
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable and start timer
	if err := exec.Command("systemctl", "--user", "enable", "--now", "aifuel-feed.timer").Run(); err != nil {
		return fmt.Errorf("failed to enable aifuel-feed.timer: %w", err)
	}

	return nil
}

// InstallChromeExtension copies the Chrome extension files (including icons/ subdirectory)
// from the embedded chrome-extension/ directory
func InstallChromeExtension() error {
	configDir, _, _ := GetInstallDirs()
	extDir := filepath.Join(configDir, "chrome-extension")

	if err := os.MkdirAll(extDir, 0755); err != nil {
		return fmt.Errorf("failed to create extension dir: %w", err)
	}

	// Walk all files recursively (handles icons/ subdirectory)
	err := fs.WalkDir(embeddedChromeExt, "chrome-extension", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute relative path from "chrome-extension/" root
		relPath := strings.TrimPrefix(path, "chrome-extension/")
		if relPath == "" || path == "chrome-extension" {
			return nil // skip root
		}

		destPath := filepath.Join(extDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		data, err := fs.ReadFile(embeddedChromeExt, path)
		if err != nil {
			return fmt.Errorf("failed to read embedded %s: %w", relPath, err)
		}

		// Inject the build-time version into manifest.json
		if relPath == "manifest.json" && Version != "dev" {
			data = []byte(strings.Replace(string(data),
				`"version": "dev"`,
				fmt.Sprintf(`"version": "%s"`, Version), 1))
		}

		return os.WriteFile(destPath, data, 0644)
	})

	if err != nil {
		return fmt.Errorf("failed to install chrome extension: %w", err)
	}

	// Also deploy to legacy ai-usage path if Chrome loaded the extension from there.
	// This ensures reload picks up the new files regardless of which path Chrome uses.
	home, _ := os.UserHomeDir()
	legacyDir := filepath.Join(home, ".config", "ai-usage", "chrome-extension")
	if info, err := os.Stat(legacyDir); err == nil && info.IsDir() {
		// Copy all files from aifuel dir to legacy dir
		_ = filepath.Walk(extDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || path == extDir {
				return nil
			}
			relPath, _ := filepath.Rel(extDir, path)
			destPath := filepath.Join(legacyDir, relPath)
			if info.IsDir() {
				return os.MkdirAll(destPath, 0755)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			return os.WriteFile(destPath, data, 0644)
		})
	}

	return nil
}

// CleanLegacyNativeHost removes the old ai-usage native messaging manifest
func CleanLegacyNativeHost() {
	profilePath, variant := DetectChrome()
	if variant == "" || profilePath == "" {
		return
	}
	nativeDir := GetNativeMessagingHostDir(profilePath)
	oldManifest := filepath.Join(nativeDir, "com.ai_usage.live_feed.json")
	if _, err := os.Stat(oldManifest); err == nil {
		_ = os.Remove(oldManifest)
	}
}

// SetupNativeHost creates the native messaging host manifest for Chrome
func SetupNativeHost(chromeProfilePath string, chromeExtensionId string) error {
	if chromeProfilePath == "" {
		return fmt.Errorf("no Chrome profile path provided")
	}

	_, _, libDir := GetInstallDirs()
	nativeDir := GetNativeMessagingHostDir(chromeProfilePath)

	if err := os.MkdirAll(nativeDir, 0755); err != nil {
		return fmt.Errorf("failed to create NativeMessagingHosts dir: %w", err)
	}

	hostScriptPath := filepath.Join(libDir, "native-host.sh")

	// Determine allowed_origins
	origin := "chrome-extension://YOUR_EXTENSION_ID_HERE/"
	if chromeExtensionId != "" {
		origin = fmt.Sprintf("chrome-extension://%s/", chromeExtensionId)
	}

	hostManifest := fmt.Sprintf(`{
  "name": "com.aifuel.live_feed",
  "description": "aifuel AI usage live feed native messaging host",
  "path": "%s",
  "type": "stdio",
  "allowed_origins": [
    "%s"
  ]
}`, hostScriptPath, origin)

	manifestPath := filepath.Join(nativeDir, "com.aifuel.live_feed.json")
	if err := os.WriteFile(manifestPath, []byte(hostManifest), 0644); err != nil {
		return fmt.Errorf("failed to write native messaging host manifest: %w", err)
	}

	return nil
}

// CreateWaybarSnippet generates the waybar module configuration
func CreateWaybarSnippet(displayMode string) string {
	_, _, libDir := GetInstallDirs()
	execPath := filepath.Join(libDir, "aifuel.sh")
	onClickPath := filepath.Join(libDir, "aifuel-tui.sh")
	onClickRightPath := filepath.Join(libDir, "dashboard.sh")

	snippet := fmt.Sprintf(`"custom/aifuel": {
    "exec": "%s",
    "return-type": "json",
    "interval": 55,
    "format": "{}",
    "tooltip": true,
    "on-click": "%s",
    "on-click-right": "%s"
}`, execPath, onClickPath, onClickRightPath)

	return snippet
}

// RemoveDirectories removes aifuel directories, optionally preserving config
func RemoveDirectories(preserveConfig bool) error {
	configDir, cacheDir, libDir := GetInstallDirs()

	var errs []string

	// Remove cache
	if err := os.RemoveAll(cacheDir); err != nil {
		errs = append(errs, fmt.Sprintf("cache dir: %v", err))
	}

	// Remove lib
	if err := os.RemoveAll(libDir); err != nil {
		errs = append(errs, fmt.Sprintf("lib dir: %v", err))
	}

	// Remove or preserve config
	if !preserveConfig {
		if err := os.RemoveAll(configDir); err != nil {
			errs = append(errs, fmt.Sprintf("config dir: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("removal errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// DisableSystemdService stops and disables the aifuel systemd service
func DisableSystemdService() error {
	// Stop and disable timer
	_ = exec.Command("systemctl", "--user", "stop", "aifuel-feed.timer").Run()
	_ = exec.Command("systemctl", "--user", "disable", "aifuel-feed.timer").Run()

	// Stop service if running
	_ = exec.Command("systemctl", "--user", "stop", "aifuel-feed.service").Run()

	// Remove service files
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	serviceDir := filepath.Join(home, ".config", "systemd", "user")
	os.Remove(filepath.Join(serviceDir, "aifuel-feed.service"))
	os.Remove(filepath.Join(serviceDir, "aifuel-feed.timer"))

	// Reload
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

// RemoveNativeHost removes the native messaging host manifest
func RemoveNativeHost() error {
	profilePath, variant := DetectChrome()
	if variant == "" {
		return nil // No Chrome found, nothing to remove
	}

	nativeDir := GetNativeMessagingHostDir(profilePath)
	manifestPath := filepath.Join(nativeDir, "com.aifuel.live_feed.json")

	if _, err := os.Stat(manifestPath); err == nil {
		return os.Remove(manifestPath)
	}

	return nil
}
