# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.5.0] - 2026-04-03

### Added

- **Chrome extension popup UI**: Click the toolbar icon to see 5-hour and 7-day usage bars, cost stats, reset countdowns, and per-model concurrency limits in a Catppuccin-themed popup. Replaces the invisible background-only extension.
- **Toolbar badge**: Extension icon shows current 5-hour usage percentage with color-coded background (green/yellow/red at thresholds). Updates every poll cycle.
- **Browser notifications**: Desktop alerts when usage hits configurable warning (80%) and critical (95%) thresholds, with cooldown to prevent spam.
- **Extension options page**: Adjust poll interval (1/2/5/10 min), toggle badge visibility, enable/disable notifications, and set warning/critical thresholds from the extension settings.
- **Masked admin key input**: `aifuel admin setup` now uses a huh password-masked input field with real-time validation, a spinner during key verification, and a feature preview showing what the Admin API unlocks.
- **Interactive home menu**: Running `aifuel` when already installed shows a feature menu (status, dashboard, auth, config, admin, chrome, diagnostics, reinstall) instead of re-running the install wizard.
- **Native bubbletea dashboard** (`aifuel dashboard`): Real-time TUI with 4 tabbed views (Rate Limits, Cost & Usage, Analytics, Account), auto-refresh every 30 seconds, keyboard navigation (tab/h/l to switch, r to refresh, q to quit), alt-screen mode, and mouse support. Integrates live feed data, admin API cost reports, analytics, and account metadata.
- **Dashboard legacy mode**: `aifuel dashboard --legacy` runs the original shell-based dashboard for environments without bubbletea support.

### Changed

- Root command now detects existing installation and shows the interactive home menu instead of the install wizard.
- Dashboard command defaults to native bubbletea TUI (previously shelled out to dashboard.sh).

## [1.4.0] - 2026-04-03

### Added

- **`/api/organizations/{org}/rate_limits` endpoint**: Chrome extension now fetches per-model concurrency limits and thinking requests-per-minute caps every 30 minutes. Displayed in `aifuel status --full` under "Per-Model Limits" with a table showing concurrency slots and thinking RPM per model group.
- **`/v1/models` endpoint discovered**: OAuth API returns the full model catalog with token limits (max_input_tokens, max_tokens), creation dates, and capabilities (batch, citations, code_execution, thinking, etc.).
- **Anthropic Admin API integration** (`aifuel admin` command tree):
  - `aifuel admin setup`: Interactive key provisioning with verification against `/v1/organizations/me`. Stores key in config.json with 0600 permissions.
  - `aifuel admin cost`: Official USD cost report for the last 7 days from `/v1/organizations/cost_report`, grouped by description/model.
  - `aifuel admin usage`: Token usage report from `/v1/organizations/usage_report/messages` with per-model breakdown (input, output, cache read, cache create).
  - `aifuel admin analytics [date]`: Claude Code productivity metrics from `/v1/organizations/usage_report/claude_code` showing sessions, lines added/removed, commits, PRs, edit acceptance rate, and estimated cost.
- **Admin API client** (`internal/installer/admin.go`): Full Go client with typed structs for CostReport, UsageReport, ClaudeCodeReport, OrgInfo. Supports `ANTHROPIC_ADMIN_KEY` env var or config.json storage.
- **4 new claude.ai endpoints discovered**: `/api/organizations/{org}/rate_limits` (per-model concurrency), `/api/organizations/{org}/chat_conversations` (226 web conversations), `/api/organizations/{org}/projects` (14 projects), `/api/organizations/{org}/members` (org members).

### Changed

- Chrome extension `pollUsage()` now fetches org info and rate limits in parallel with `Promise.all()`.
- `aifuel status --full` now shows per-model rate limits section when data is available from the Chrome extension.

## [1.3.0] - 2026-04-03

### Added

- **Pixel-art fuel pump logo**: Rich ASCII art with Catppuccin warm-to-shadow gradient (Flamingo cap, Peach body, Green gauge, Mauve display, Maroon base). Renders on `aifuel`, `aifuel install`, `aifuel version`, and `aifuel uninstall`.
- **Themed Cobra help**: All `--help` output now uses Catppuccin colors via custom template functions (Mauve section headers, Peach command names, Subtext0 hints). Every subcommand inherits the theme automatically.
- **`aifuel status --full`**: Rich dashboard view with 20-cell progress bars per rate limit, color-coded percentages (green/yellow/red at 0/60/85% thresholds), reset countdowns, daily cost with burn rate and projections, per-model token breakdown, and account metadata.
- **`aifuel config`**: Interactive configuration editor using huh forms. Pre-fills from existing config.json. Edit providers, display mode, notifications, and cache TTL without touching JSON.
- **`aifuel completion [bash|zsh|fish]`**: Shell completion generation via Cobra's built-in engine. Tab-completes commands, flags, and valid args (provider names).
- **5 new API fields captured from `/api/organizations/{org}/usage`**: `seven_day_opus` (Opus-specific 7d limit), `seven_day_oauth_apps` (OAuth app limit), `seven_day_cowork` (team collaboration limit), `extra_usage.monthly_limit` (credit cap), `extra_usage.utilization` (credit usage percentage). All currently null but will activate automatically when Anthropic enables them.
- **Account endpoint discovery**: New fetch from `/api/organizations/{org}` provides the real `rate_limit_tier` (previously stale from credentials file), `billing_type`, `capabilities`, `active_flags`, and available models (active, non-overflow only). Cached for 1 hour.
- **Chrome extension org fetch**: `background.js` now fetches org metadata every 30 minutes alongside the 2-minute usage polls. Embedded as `_org` in the live feed JSON, providing the most reliable path for account data (browser handles Cloudflare challenges automatically).
- **`RenderProgressBar(pct, width)`** and **`ColorForPct(pct)`** UI helpers in theme.go for reusable progress bar rendering across status and future dashboard.
- **`getNullableFloat()`** helper to distinguish "0%" (active limit at zero) from "null" (limit not yet active) in the Go status display.
- Copilot added to default provider config template in the install wizard.

### Changed

- `RenderLogo()` retained for compact contexts; `RenderRichLogo()` is now the primary logo for wizard, uninstall, and version commands.
- `lib.sh` version string updated from "0.1.0" to "1.3.0".
- Chrome extension manifest version bumped to 1.3.0.
- Plan tier now sourced from org endpoint (accurate) instead of credentials file (stale). Falls back to credentials if org fetch fails.

### Fixed

- Rate limit tier showing stale value (`default_claude_max_5x`) when the actual org tier was `default_claude_max_20x`. The credentials file is only refreshed on re-auth; the org endpoint always returns the current value.

## [1.2.1] - 2026-03-29

### Fixed

- Session stats now aggregate ALL today's JSONL sessions instead of only the most recent one. Previously `session_messages` and `total_output_tokens` only reflected one session; now they cover all concurrent and sequential sessions on the machine.
- Added `total_sessions` field to the output JSON showing how many sessions are being tracked.
- Per-prompt analytics (recent turns, session duration) still use the most recent active session for relevance.

## [1.2.0] - 2026-03-29

### Added

- `aifuel auth` command: shows authentication status for all providers (Claude, Codex, Gemini, Copilot, CodexBar) with a styled table
- `aifuel auth <provider>`: triggers the official CLI auth flow interactively (claude auth login, codex auth login, gemini auth login, gh auth login, codexbar auth)
- `aifuel-copilot.sh`: native GitHub Copilot provider using CodexBar CLI (primary) with gh API fallback for org-level metrics
- `aifuel-codexbar.sh`: universal provider bridge that wraps the CodexBar CLI, enabling instant support for any provider CodexBar supports (Copilot, z.ai, Kimi, Kiro, Augment, Amp, and more)
- ccusage deep integration: `ccusage_sessions()`, `ccusage_daily()`, `ccusage_monthly()` helper functions in lib.sh for richer analytics data
- Copilot wired into main waybar module, TUI dashboard, settings panel, history tracking, and diagnostics
- CodexBar fallback: if a native provider fetch fails and CodexBar is installed, automatically retry via CodexBar bridge
- Copilot entry added to default config.json

## [1.1.0] - 2026-03-29

### Added

- `aifuel statusline` command for Claude Code statusLine integration (compact: `5h:16% 7d:3% $21.59 359msg`)
- `aifuel status --json` flag for machine-readable JSON output (pipe to jq, scripts, other tools)
- `aifuel status` rewritten with real provider data (5h/7d utilization, cost, burn rate, messages, data source)
- AUR PKGBUILD (`aifuel-bin`) for Arch Linux with auto-generated shell completions
- Shell completion scripts for bash, zsh, and fish (via cobra)
- README sections for statusLine integration, shell completions, and AUR install

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

[1.5.0]: https://github.com/robertogogoni/aifuel/releases/tag/v1.5.0
[1.4.0]: https://github.com/robertogogoni/aifuel/releases/tag/v1.4.0
[1.3.0]: https://github.com/robertogogoni/aifuel/releases/tag/v1.3.0
[1.2.1]: https://github.com/robertogogoni/aifuel/releases/tag/v1.2.1
[1.2.0]: https://github.com/robertogogoni/aifuel/releases/tag/v1.2.0
[1.1.0]: https://github.com/robertogogoni/aifuel/releases/tag/v1.1.0
[1.0.1]: https://github.com/robertogogoni/aifuel/releases/tag/v1.0.1
[1.0.0]: https://github.com/robertogogoni/aifuel/releases/tag/v1.0.0
