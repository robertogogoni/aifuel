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

//go:embed all:scripts
var embeddedScripts embed.FS

// Config holds the aifuel configuration
type Config struct {
	Providers        []string `json:"providers"`
	DisplayMode      string   `json:"display_mode"`
	Notifications    bool     `json:"notifications"`
	CacheTTL         int      `json:"cache_ttl_seconds"`
	ChromeExtension  bool     `json:"chrome_extension"`
	ChromeVariant    string   `json:"chrome_variant,omitempty"`
	ChromeProfile    string   `json:"chrome_profile,omitempty"`
	Version          string   `json:"version"`
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

// InstallScripts copies scripts from embedded FS or local scripts/ dir to ~/.local/lib/aifuel/
func InstallScripts() error {
	_, _, libDir := GetInstallDirs()

	// Try embedded scripts first
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

	cfg := Config{
		Providers:       sel.Providers,
		DisplayMode:     sel.DisplayMode,
		Notifications:   sel.Notifications,
		CacheTTL:        sel.CacheTTL,
		ChromeExtension: sel.ChromeExtension,
		ChromeVariant:   sel.ChromeVariant,
		ChromeProfile:   sel.ChromeProfile,
		Version:         "1.0.0",
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

// InstallChromeExtension copies the Chrome extension files
func InstallChromeExtension() error {
	configDir, _, _ := GetInstallDirs()
	extDir := filepath.Join(configDir, "chrome-extension")

	if err := os.MkdirAll(extDir, 0755); err != nil {
		return fmt.Errorf("failed to create extension dir: %w", err)
	}

	manifest := `{
  "manifest_version": 3,
  "name": "aifuel - AI Usage Monitor",
  "version": "1.0.0",
  "description": "Monitors AI provider usage and feeds data to aifuel waybar module",
  "permissions": [
    "storage",
    "nativeMessaging",
    "cookies"
  ],
  "host_permissions": [
    "https://console.anthropic.com/*",
    "https://platform.openai.com/*",
    "https://aistudio.google.com/*"
  ],
  "background": {
    "service_worker": "background.js"
  },
  "icons": {
    "48": "icon48.png",
    "128": "icon128.png"
  }
}`

	backgroundJS := `// aifuel Chrome Extension - Background Service Worker
// Monitors AI provider usage pages and sends data via native messaging

const NATIVE_HOST = "com.aifuel.monitor";

let port = null;

function connectNativeHost() {
  try {
    port = chrome.runtime.connectNative(NATIVE_HOST);
    port.onMessage.addListener((msg) => {
      console.log("aifuel native:", msg);
    });
    port.onDisconnect.addListener(() => {
      console.log("aifuel native host disconnected");
      port = null;
      // Retry after 30 seconds
      setTimeout(connectNativeHost, 30000);
    });
  } catch (e) {
    console.error("aifuel: failed to connect native host:", e);
  }
}

function sendUsageData(data) {
  if (port) {
    port.postMessage(data);
  }
}

// Listen for tab updates to detect AI provider pages
chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.status !== "complete" || !tab.url) return;

  const providers = [
    { pattern: /console\.anthropic\.com/, name: "claude" },
    { pattern: /platform\.openai\.com/, name: "codex" },
    { pattern: /aistudio\.google\.com/, name: "gemini" },
  ];

  for (const provider of providers) {
    if (provider.pattern.test(tab.url)) {
      sendUsageData({
        provider: provider.name,
        url: tab.url,
        timestamp: Date.now(),
      });
      break;
    }
  }
});

// Connect on startup
connectNativeHost();
console.log("aifuel extension loaded");
`

	if err := os.WriteFile(filepath.Join(extDir, "manifest.json"), []byte(manifest), 0644); err != nil {
		return fmt.Errorf("failed to write manifest.json: %w", err)
	}

	if err := os.WriteFile(filepath.Join(extDir, "background.js"), []byte(backgroundJS), 0644); err != nil {
		return fmt.Errorf("failed to write background.js: %w", err)
	}

	return nil
}

// SetupNativeHost creates the native messaging host manifest for Chrome
func SetupNativeHost(chromeProfilePath string) error {
	if chromeProfilePath == "" {
		return fmt.Errorf("no Chrome profile path provided")
	}

	_, _, libDir := GetInstallDirs()
	nativeDir := GetNativeMessagingHostDir(chromeProfilePath)

	if err := os.MkdirAll(nativeDir, 0755); err != nil {
		return fmt.Errorf("failed to create NativeMessagingHosts dir: %w", err)
	}

	hostScript := filepath.Join(libDir, "aifuel-native-host.sh")

	// Create the native host script
	hostScriptContent := `#!/bin/bash
# aifuel native messaging host
# Receives messages from the Chrome extension and writes to cache

CACHE_DIR="${HOME}/.cache/aifuel"
mkdir -p "$CACHE_DIR"

# Read message length (4 bytes, little-endian)
read_message() {
    local len
    len=$(head -c 4 | od -An -td4 | tr -d ' ')
    if [ -z "$len" ] || [ "$len" -le 0 ] 2>/dev/null; then
        return 1
    fi
    head -c "$len"
}

# Write response (4-byte length prefix + JSON)
write_message() {
    local msg="$1"
    local len=${#msg}
    printf "$(printf '\\x%02x\\x%02x\\x%02x\\x%02x' \
        $((len & 0xFF)) \
        $(((len >> 8) & 0xFF)) \
        $(((len >> 16) & 0xFF)) \
        $(((len >> 24) & 0xFF)))"
    printf '%s' "$msg"
}

while true; do
    msg=$(read_message) || break
    # Write received data to cache
    echo "$msg" >> "$CACHE_DIR/chrome-feed.jsonl"
    write_message '{"status":"ok"}'
done
`

	if err := os.WriteFile(hostScript, []byte(hostScriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write native host script: %w", err)
	}

	// Create the native messaging host manifest
	hostManifest := fmt.Sprintf(`{
  "name": "com.aifuel.monitor",
  "description": "aifuel AI usage monitor native messaging host",
  "path": "%s",
  "type": "stdio",
  "allowed_origins": [
    "chrome-extension://aifuelmonitorextensionid/"
  ]
}`, hostScript)

	manifestPath := filepath.Join(nativeDir, "com.aifuel.monitor.json")
	if err := os.WriteFile(manifestPath, []byte(hostManifest), 0644); err != nil {
		return fmt.Errorf("failed to write native messaging host manifest: %w", err)
	}

	return nil
}

// CreateWaybarSnippet generates the waybar module configuration
func CreateWaybarSnippet(displayMode string) string {
	_, _, libDir := GetInstallDirs()
	scriptPath := filepath.Join(libDir, "aifuel-feed.sh")

	var format string
	switch displayMode {
	case "icon":
		format = "{icon}"
	case "compact":
		format = "{icon} {percentage}%"
	default:
		format = "{icon} {percentage}% {text}"
	}

	snippet := fmt.Sprintf(`"custom/aifuel": {
    "exec": "%s",
    "return-type": "json",
    "interval": 55,
    "format": "%s",
    "format-icons": {
        "critical": "\u26fd",
        "warning": "\u26fd",
        "normal": "\u26fd",
        "good": "\u26fd"
    },
    "tooltip": true
}`, scriptPath, format)

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
	manifestPath := filepath.Join(nativeDir, "com.aifuel.monitor.json")

	if _, err := os.Stat(manifestPath); err == nil {
		return os.Remove(manifestPath)
	}

	return nil
}
