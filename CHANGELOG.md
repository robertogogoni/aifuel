# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.1] - 2026-03-29

### Added

- `aifuel setup-chrome` command: auto-detects Chrome variant, finds extension ID from Preferences (supports unpacked extensions), and configures native messaging host in one step
- `resolve_live_feed()` in lib.sh: checks both aifuel and legacy ai-usage cache paths so the Chrome extension live feed works across both systems
- Competitive comparison doc (`docs/COMPARISON.md`) covering 28 tools in the ecosystem

### Fixed

- Empty cache bug: `check_cache()` now requires non-empty files (`-s` flag) before treating them as valid hits. A 0-byte cache created as a side effect would previously cause scripts to return empty output
- Install URLs corrected from `/main/` to `/master/` branch
- Chrome extension installer: replaced broken inline tab-URL watcher with the correct cookie-based claude.ai usage fetcher
- Native messaging host: fixed host name from `com.aifuel.monitor` to `com.aifuel.live_feed`, points to the installed `native-host.sh` instead of generating a broken inline script
- Config generator: `WriteConfig()` now produces the correct nested provider format (`providers.claude.enabled`) that the shell scripts expect
- Waybar snippet: `exec` points to `aifuel.sh` (waybar module) instead of `aifuel-feed.sh` (poller), added `on-click` for TUI and `on-click-right` for dashboard
- Removed non-existent icon references from Chrome extension manifest

## [1.0.0] - 2026-03-29

### Added

- Multi-provider support: Claude, Codex, Gemini, Antigravity
- 5-phase data cascade: live feed, cache, cookies, OAuth, local JSONL
- Waybar module with color-coded status (ok/warn/crit) and rich Pango tooltip
- Analytics engine: depletion prediction, peak hour detection, binding limit resolution, per-prompt cost
- TUI dashboard with sparklines, burn rate, 14-day history, per-model breakdown
- Interactive settings panel via TUI (display mode, providers, notifications, theme)
- Chrome extension for real-time usage data via native messaging
- Background poller (systemd user service) for data freshness when Chrome is closed
- Desktop notifications at configurable warning/critical thresholds
- Cookie-based fetcher with auto-detection of Chrome variants (Canary, Stable, Chromium, Brave)
- Go Charm CLI installer wizard with Catppuccin Mocha theme
- CLI commands: install, check, status, dashboard, uninstall, version
- Self-contained binary with embedded scripts (go:embed)
- curl-pipe-bash bootstrap installer with GitHub Releases fallback to source build
- GoReleaser config for cross-platform releases (linux/amd64, linux/arm64)
- GitHub Actions release workflow
- Comprehensive test suite (14 test groups covering cost accuracy, token counting, timezone handling, edge cases)
- Usage history tracking with JSONL persistence and sparkline visualization
- Clipboard export (wl-copy, xclip, xsel)
- Light/dark theme auto-detection
- Log rotation and structured logging

[1.0.1]: https://github.com/robertogogoni/aifuel/releases/tag/v1.0.1
[1.0.0]: https://github.com/robertogogoni/aifuel/releases/tag/v1.0.0
