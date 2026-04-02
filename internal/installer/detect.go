package installer

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectionResult holds the result of a system detection scan
type DetectionResult struct {
	WaybarFound   bool
	WaybarPath    string
	JqFound       bool
	JqPath        string
	CurlFound     bool
	CurlPath      string
	GumFound      bool
	GumPath       string
	CcusageFound  bool
	CcusagePath   string
	NotifyFound   bool
	NotifyPath    string
	ChromeFound   bool
	ChromeVariant string
	ChromeProfile string
}

// DetectAll runs all system detections and returns the result
func DetectAll() DetectionResult {
	result := DetectionResult{}

	result.WaybarPath, result.WaybarFound = DetectDependency("waybar")
	result.JqPath, result.JqFound = DetectDependency("jq")
	result.CurlPath, result.CurlFound = DetectDependency("curl")
	result.GumPath, result.GumFound = DetectDependency("gum")
	result.CcusagePath, result.CcusageFound = DetectCcusage()
	result.NotifyPath, result.NotifyFound = DetectDependency("notify-send")
	result.ChromeProfile, result.ChromeVariant = DetectChrome()
	result.ChromeFound = result.ChromeVariant != ""

	return result
}

// DetectWaybar checks if waybar is available
func DetectWaybar() bool {
	_, err := exec.LookPath("waybar")
	return err == nil
}

// DetectDependency checks if a given binary is available on PATH
func DetectDependency(name string) (path string, found bool) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return p, true
}

// DetectChrome detects installed Chrome variants, preferring Canary
func DetectChrome() (profilePath string, variant string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}

	// Ordered by preference: canary first, then stable, chromium, brave
	variants := []struct {
		name       string
		binary     string
		configDir  string
		nativeDir  string
	}{
		{
			name:      "Chrome Canary",
			binary:    "google-chrome-canary",
			configDir: filepath.Join(home, ".config", "google-chrome-canary"),
			nativeDir: filepath.Join(home, ".config", "google-chrome-canary", "NativeMessagingHosts"),
		},
		{
			name:      "Google Chrome",
			binary:    "google-chrome-stable",
			configDir: filepath.Join(home, ".config", "google-chrome"),
			nativeDir: filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts"),
		},
		{
			name:      "Chromium",
			binary:    "chromium",
			configDir: filepath.Join(home, ".config", "chromium"),
			nativeDir: filepath.Join(home, ".config", "chromium", "NativeMessagingHosts"),
		},
		{
			name:      "Brave",
			binary:    "brave",
			configDir: filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser"),
			nativeDir: filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "NativeMessagingHosts"),
		},
	}

	for _, v := range variants {
		// Check if the binary exists
		if _, err := exec.LookPath(v.binary); err == nil {
			return v.configDir, v.name
		}
		// Also check if the config directory exists (browser may have a non-standard binary name)
		if info, err := os.Stat(v.configDir); err == nil && info.IsDir() {
			return v.configDir, v.name
		}
	}

	return "", ""
}

// DetectCcusage checks for ccusage binary
func DetectCcusage() (path string, found bool) {
	// Check PATH first
	if p, err := exec.LookPath("ccusage"); err == nil {
		return p, true
	}

	// Check common install locations
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}

	locations := []string{
		filepath.Join(home, ".local", "bin", "ccusage"),
		filepath.Join(home, "go", "bin", "ccusage"),
		"/usr/local/bin/ccusage",
		"/usr/bin/ccusage",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc, true
		}
	}

	return "", false
}

// GetScriptsDir finds the scripts directory. In dev mode it checks ./scripts/,
// otherwise returns the installed location.
func GetScriptsDir() string {
	// Check local dev directory first
	if info, err := os.Stat("scripts"); err == nil && info.IsDir() {
		abs, err := filepath.Abs("scripts")
		if err == nil {
			return abs
		}
	}

	// Check relative to the binary
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		scriptsDir := filepath.Join(dir, "scripts")
		if info, err := os.Stat(scriptsDir); err == nil && info.IsDir() {
			return scriptsDir
		}
		// Check one level up (bin/../share)
		shareDir := filepath.Join(dir, "..", "share", "aifuel", "scripts")
		if info, err := os.Stat(shareDir); err == nil && info.IsDir() {
			return shareDir
		}
	}

	// System install location
	if info, err := os.Stat("/usr/share/aifuel/scripts"); err == nil && info.IsDir() {
		return "/usr/share/aifuel/scripts"
	}

	return ""
}

// GetNativeMessagingHostDir returns the NativeMessagingHosts dir for the detected Chrome variant
func GetNativeMessagingHostDir(chromeProfilePath string) string {
	if chromeProfilePath == "" {
		return ""
	}
	return filepath.Join(chromeProfilePath, "NativeMessagingHosts")
}

// GetInstallDirs returns the standard aifuel directories
func GetInstallDirs() (configDir, cacheDir, libDir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	configDir = filepath.Join(home, ".config", "aifuel")
	cacheDir = filepath.Join(home, ".cache", "aifuel")
	libDir = filepath.Join(home, ".local", "lib", "aifuel")
	return
}

// IsServiceActive checks if a systemd user service is active
func IsServiceActive(service string) bool {
	cmd := exec.Command("systemctl", "--user", "is-active", "--quiet", service)
	return cmd.Run() == nil
}

// IsServiceEnabled checks if a systemd user service is enabled
func IsServiceEnabled(service string) bool {
	cmd := exec.Command("systemctl", "--user", "is-enabled", "--quiet", service)
	return cmd.Run() == nil
}

// DetectExtensionID scans Chrome's Preferences file for an extension matching
// the given name and returns its ID. For unpacked extensions, Chrome doesn't
// store the manifest name in Preferences, so we also match by path or by
// reading the manifest.json from the extension's directory on disk.
func DetectExtensionID(chromeProfilePath string, extensionName string) string {
	profiles := []string{"Default", "Profile 1", "Profile 2", "Profile 3"}

	configDir, _, _ := GetInstallDirs()
	aifuelExtDir := filepath.Join(configDir, "chrome-extension")

	for _, profile := range profiles {
		prefsPath := filepath.Join(chromeProfilePath, profile, "Preferences")
		data, err := os.ReadFile(prefsPath)
		if err != nil {
			continue
		}

		var prefs map[string]interface{}
		if err := json.Unmarshal(data, &prefs); err != nil {
			continue
		}

		extensions, ok := prefs["extensions"].(map[string]interface{})
		if !ok {
			continue
		}
		settings, ok := extensions["settings"].(map[string]interface{})
		if !ok {
			continue
		}

		for extID, extData := range settings {
			ext, ok := extData.(map[string]interface{})
			if !ok {
				continue
			}

			// Method 1: Match by manifest name (works for webstore extensions)
			if manifest, ok := ext["manifest"].(map[string]interface{}); ok {
				if name, ok := manifest["name"].(string); ok {
					if strings.EqualFold(name, extensionName) {
						return extID
					}
				}
			}

			// Method 2: Match by path (works for unpacked extensions)
			if extPath, ok := ext["path"].(string); ok {
				// Direct match: extension loaded from aifuel's directory
				if extPath == aifuelExtDir || strings.HasSuffix(extPath, "/aifuel/chrome-extension") {
					return extID
				}
				// Read the on-disk manifest to check the name
				diskManifest := filepath.Join(extPath, "manifest.json")
				if mData, err := os.ReadFile(diskManifest); err == nil {
					var m map[string]interface{}
					if json.Unmarshal(mData, &m) == nil {
						if name, ok := m["name"].(string); ok {
							if strings.EqualFold(name, extensionName) {
								return extID
							}
						}
					}
				}
			}
		}
	}

	return ""
}

// GetVersion returns version info from git or embedded
func GetVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	out, err := cmd.Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(out))
}
