<p align="center">
  <br>
  <img src="https://img.shields.io/badge/%E2%9B%BD-aifuel-fab387?style=for-the-badge&labelColor=1e1e2e" alt="aifuel" />
  <br><br>
  <strong>Real-time AI provider usage monitor for waybar</strong>
  <br>
  <sub>Know exactly how much fuel is left in your Claude, Codex, Gemini, and Copilot tanks.</sub>
  <br><br>
  <a href="https://github.com/robertogogoni/aifuel/releases"><img src="https://img.shields.io/github/v/release/robertogogoni/aifuel?style=flat-square&color=a6e3a1&label=release" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/robertogogoni/aifuel?style=flat-square&color=89b4fa" alt="License"></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://github.com/robertogogoni/aifuel/stargazers"><img src="https://img.shields.io/github/stars/robertogogoni/aifuel?style=flat-square&color=f9e2af" alt="Stars"></a>
  <a href="https://aur.archlinux.org/packages/aifuel-bin"><img src="https://img.shields.io/badge/AUR-aifuel--bin-1793d1?style=flat-square&logo=archlinux&logoColor=white" alt="AUR"></a>
</p>

---

aifuel is a Go CLI that monitors your AI provider usage and renders it as a color-coded fuel gauge in [waybar](https://github.com/Alexays/Waybar). It tracks Claude, Codex, Gemini, Copilot, Antigravity, and any [CodexBar](https://github.com/nicepkg/codexbar)-supported provider through a 5-phase data cascade that never loses signal. The TUI, CLI, and notifications all use the [Catppuccin Mocha](https://catppuccin.com/) palette.

**v1.3.0** introduces a pixel-art fuel pump logo, Catppuccin-themed `--help`, a rich `status --full` dashboard with progress bars, an interactive `config` editor, shell completions, and expanded API coverage including Opus rate limits, OAuth app quotas, and extra usage credits.

## Features

**Live Fuel Gauge** :fuelpump: Track 5-hour and 7-day rate limits, Sonnet and Opus quotas, OAuth app limits, and extra usage credits directly in your status bar.

**5-Phase Data Cascade** :zap: Chrome extension, TTL cache, cookie fetch, OAuth fallback, and local JSONL, in that order. If one source drops, the next takes over transparently.

**Multi-Provider** :globe_with_meridians: Claude (stable), Codex (stable), Copilot (stable), Gemini (experimental), Antigravity (experimental), plus any provider via the CodexBar bridge.

**Rich TUI Dashboard** :bar_chart: Full-screen terminal interface built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) with sparklines, burn rate, 14-day history, and per-model breakdown.

**Smart Analytics** :brain: Depletion prediction, peak hour detection, binding limit resolution, per-prompt cost tracking, and session aggregation across concurrent sessions.

**Desktop Notifications** :bell: Configurable warnings at 80% and critical alerts at 95% with cooldown to prevent notification spam.

**Catppuccin Everywhere** :art: Mocha palette throughout: waybar CSS classes, TUI chrome, CLI output, pixel-art logo, and even `--help` text.

**Claude Code Integration** :link: Compact statusLine output shows `5h:16% 7d:3% $21.59 359msg` right in your terminal.

## What's New

| Version | Feature | Description |
|---------|---------|-------------|
| **v1.7.0** | **Wizard auth page** | Install wizard authenticates providers inline (no separate `aifuel auth` step). One command from zero to working. |
| **v1.7.0** | **Legacy path migration** | Extension auto-deploys to both aifuel and legacy ai-usage paths. Old native host manifests cleaned up. |
| **v1.6.0** | **Live conversation cost** | Content script on claude.ai tracks streaming tokens and shows floating cost widget + per-message badges |
| **v1.6.0** | **Tabbed popup** | Three tabs: Usage (limits, sparkline), Chats (searchable conversations), Estimate (token calculator) |
| **v1.6.0** | **Context menu** | Right-click selected text, "Ask Claude about this" |
| **v1.5.0** | **Bubbletea dashboard** | Native Go TUI with 4 tabs, auto-refresh, keyboard nav |
| **v1.5.0** | **Chrome popup + badge** | Toolbar icon with live usage %, Catppuccin popup, browser notifications |
| **v1.4.0** | **Admin API** | `aifuel admin cost/usage/analytics` with official Anthropic billing data |
| **v1.4.0** | **Per-model limits** | Concurrency slots and thinking RPM per model group |
| **v1.3.0** | **Rich TUI** | Pixel-art logo, themed help, `status --full`, `config` editor, shell completions |

## Quick Install

### One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/robertogogoni/aifuel/master/install.sh | bash
```

Detects your OS and architecture, downloads the latest release binary, and launches the interactive TUI installer.

### AUR (Arch Linux)

```bash
yay -S aifuel-bin
aifuel install
```

### Go install

```bash
go install github.com/robertogogoni/aifuel/cmd/aifuel@latest
aifuel install
```

### Build from source

```bash
git clone https://github.com/robertogogoni/aifuel.git
cd aifuel
go build -o ~/.local/bin/aifuel ./cmd/aifuel/
aifuel install
```

## Getting Started

**1. Install aifuel** using any method above. The `aifuel install` wizard handles everything:

- Detects your system (waybar, jq, curl, Chrome, ccusage)
- Lets you choose providers and configure display settings
- Installs scripts, systemd service, and Chrome extension
- Authenticates each provider inline (Claude, Codex, Gemini, Copilot)
- Prints the waybar config snippet to copy

**2. Add the waybar module** to `~/.config/waybar/config.jsonc`:

```jsonc
// Add "custom/aifuel" to your "modules-right" (or wherever you prefer)
"custom/aifuel": {
    "exec": "~/.local/lib/aifuel/aifuel.sh",
    "return-type": "json",
    "interval": 30,
    "tooltip": true,
    "on-click": "~/.local/lib/aifuel/aifuel-tui.sh",
    "on-click-right": "~/.local/lib/aifuel/dashboard.sh",
    "format": "{}",
    "escape": true
}
```

**3. Reload waybar.** Your fuel gauge should appear within 30 seconds.

**4. (Optional) Set up the Chrome extension** for real-time data and the popup UI:

```bash
aifuel setup-chrome
```

**5. (Optional) Enable Admin API** for official cost and usage reports:

```bash
aifuel admin setup
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `aifuel` | Feature menu (if installed) or install wizard (if not) |
| `aifuel install` | Full TUI wizard: detect, configure, install, authenticate |
| `aifuel config` | Interactive config editor with live form fields |
| `aifuel status` | One-line styled usage summary |
| `aifuel status --full` | Rich dashboard with progress bars, model breakdown, account info |
| `aifuel status --json` | Machine-readable JSON output (pipe to jq, scripts) |
| `aifuel statusline` | Compact output for Claude Code statusLine |
| `aifuel auth` | Show auth status for all providers |
| `aifuel auth <provider>` | Authenticate with a specific provider |
| `aifuel check` | Run diagnostics (dependencies, credentials, network) |
| `aifuel dashboard` | Real-time bubbletea TUI with tabbed views and auto-refresh |
| `aifuel dashboard --legacy` | Shell-based dashboard (original) |
| `aifuel admin setup` | Store Anthropic Admin API key with masked input |
| `aifuel admin cost` | Official USD cost report (last 7 days) |
| `aifuel admin usage` | Token usage by model from Admin API |
| `aifuel admin analytics` | Claude Code productivity metrics (sessions, LOC, commits) |
| `aifuel setup-chrome` | Configure Chrome extension, native host, clean legacy paths |
| `aifuel completion bash\|zsh\|fish` | Generate shell completion scripts |
| `aifuel uninstall` | Clean removal with config preservation option |
| `aifuel version` | Display pixel-art logo with version info |

## Provider Setup

| Provider | Auth Method | CLI Tool | Status |
|----------|-------------|----------|--------|
| Claude | CLI auth, cookies, OAuth, Chrome extension | `claude` | Stable |
| Codex | JSON-RPC, OAuth, `OPENAI_API_KEY` | `codex` | Stable |
| Copilot | `gh` CLI + CodexBar | `gh` | Stable |
| Gemini | OAuth via gemini CLI | `gemini` | Experimental |
| Antigravity | Local language server probe | n/a | Experimental |
| Any (CodexBar) | CodexBar CLI bridge | `codexbar` | Experimental |

### Quick start per provider

```bash
# Claude (most common)
aifuel auth claude

# Codex
aifuel auth codex

# Copilot (requires gh CLI)
aifuel auth copilot

# Gemini
aifuel auth gemini

# Any CodexBar-supported provider
aifuel auth codexbar
```

For detailed per-provider configuration, see [docs/PROVIDERS.md](docs/PROVIDERS.md).

## Waybar Integration

### Module config

Add to `~/.config/waybar/config.jsonc`:

```jsonc
"custom/aifuel": {
    "exec": "~/.local/lib/aifuel/aifuel.sh",
    "return-type": "json",
    "interval": 30,
    "tooltip": true,
    "on-click": "~/.local/lib/aifuel/aifuel-tui.sh",
    "on-click-right": "~/.local/lib/aifuel/dashboard.sh",
    "format": "{}",
    "escape": true
}
```

### CSS classes

The module outputs a `class` field for conditional styling:

| Class | Condition | Suggested Color |
|-------|-----------|-----------------|
| `ai-ok` | Usage below 60% | `#a6e3a1` (Catppuccin Green) |
| `ai-warn` | Usage 60%-84% | `#f9e2af` (Catppuccin Yellow) |
| `ai-crit` | Usage 85%+ | `#f38ba8` (Catppuccin Red) |

See [`waybar/style.css`](waybar/style.css) for the complete stylesheet.

### Output format

The module emits waybar-compatible JSON with three fields:

- **`text`**: compact display string (e.g., `⛽ 42%`)
- **`tooltip`**: rich multi-line breakdown with per-provider analytics, depletion estimates, and reset countdowns
- **`class`**: one of `ai-ok`, `ai-warn`, or `ai-crit`

## Claude Code Integration

Add aifuel as your Claude Code status line provider in your Claude Code settings:

```json
{
  "statusLine": {
    "command": "aifuel statusline"
  }
}
```

Output format:

```
5h:16% 7d:3% $21.59 359msg
```

This gives you a persistent view of your rate limits, daily cost, and message count right in the terminal while coding.

## Configuration

Config file: `~/.config/aifuel/config.json`

Edit interactively with `aifuel config` or modify the JSON directly.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `display_mode` | string | `"full"` | Display mode: `"full"`, `"compact"`, or `"icon"` |
| `refresh_interval` | int | `30` | Seconds between waybar refreshes |
| `cache_ttl_seconds` | int | `120` | How long cached API responses stay fresh |
| `notifications_enabled` | bool | `true` | Enable desktop notifications for usage thresholds |
| `notify_warn_threshold` | int | `80` | Percentage that triggers a warning notification |
| `notify_critical_threshold` | int | `95` | Percentage that triggers a critical notification |
| `notify_cooldown_minutes` | int | `15` | Minimum time between repeated notifications |
| `history_enabled` | bool | `true` | Log usage data to JSONL for trend analysis |
| `history_retention_days` | int | `7` | How many days of history to retain |
| `theme` | string | `"auto"` | Color theme: `"auto"`, `"dark"`, or `"light"` |
| `providers` | object | see below | Per-provider enable/disable toggles |

### Provider config

```json
{
  "providers": {
    "claude": { "enabled": true },
    "codex": { "enabled": false },
    "gemini": { "enabled": false },
    "copilot": { "enabled": false },
    "antigravity": { "enabled": false }
  }
}
```

## Architecture

aifuel uses a **5-phase data cascade** that prioritizes speed and never fails silently:

```
Phase 1: Live Feed        Chrome extension polls claude.ai every 2 min
             |             Fastest source, real-time data
             v
Phase 2: Cache            TTL-based JSON cache (55-120s)
             |             Prevents redundant API calls
             v
Phase 3: Cookie Fetch     HackBrowserData extracts cookies, hits claude.ai API
             |             Works without Chrome extension
             v
Phase 4: OAuth Fallback   api.anthropic.com with OAuth tokens
             |             Last resort, rate-limited
             v
Phase 5: Local Data       JSONL session logs + ccusage
                           Always available, never fails
```

Each phase is attempted in order. The first one that returns valid data wins. If all live sources fail, local session data provides a baseline that never goes stale.

### File locations

| Path | Purpose |
|------|---------|
| `~/.config/aifuel/config.json` | User configuration |
| `~/.config/aifuel/chrome-extension/` | Chrome extension source files |
| `~/.local/lib/aifuel/` | Runtime shell scripts |
| `~/.local/bin/aifuel` | Go binary |
| `~/.cache/aifuel/` | Cached API responses |
| `~/.cache/aifuel/aifuel.log` | Structured log file |
| `~/.config/systemd/user/aifuel-feed.service` | Background poller service |
| `~/.config/systemd/user/aifuel-feed.timer` | Poller timer (systemd) |

## API Coverage

### Usage endpoint: `/api/organizations/{org}/usage`

| Field | Type | Description | Since |
|-------|------|-------------|-------|
| `five_hour` | float | 5-hour utilization percentage | v1.0.0 |
| `five_hour_resets_at` | string | ISO 8601 reset timestamp | v1.0.0 |
| `seven_day` | float | 7-day utilization percentage | v1.0.0 |
| `seven_day_resets_at` | string | ISO 8601 reset timestamp | v1.0.0 |
| `seven_day_sonnet` | float | 7-day Sonnet-specific utilization | v1.0.0 |
| `seven_day_sonnet_resets_at` | string | ISO 8601 reset timestamp | v1.0.0 |
| `seven_day_opus` | float | 7-day Opus utilization (activates when limits go live) | v1.3.0 |
| `seven_day_opus_resets_at` | string | ISO 8601 reset timestamp | v1.3.0 |
| `seven_day_oauth_apps` | float | 7-day OAuth app utilization | v1.3.0 |
| `seven_day_cowork` | float | 7-day cowork utilization | v1.3.0 |
| `extra_usage.is_enabled` | bool | Whether extra usage credits are active | v1.0.0 |
| `extra_usage.used_credits` | float | Credits consumed this period | v1.0.0 |
| `extra_usage.monthly_limit` | float | Monthly credit cap | v1.3.0 |
| `extra_usage.utilization` | float | Credit utilization percentage | v1.3.0 |

### Account endpoint: `/api/organizations/{org}` (new in v1.3.0)

| Field | Type | Description |
|-------|------|-------------|
| `rate_limit_tier` | string | Actual rate limit tier (not stale credential data) |
| `billing_type` | string | Account billing type |
| `capabilities` | array | Enabled capabilities for the org |
| `available_models` | array | Active, non-overflow models |

## Shell Completions

### Bash

```bash
aifuel completion bash > ~/.local/share/bash-completion/completions/aifuel
```

### Zsh

```bash
aifuel completion zsh > "${fpath[1]}/_aifuel"
```

### Fish

```bash
aifuel completion fish > ~/.config/fish/completions/aifuel.fish
```

## Chrome Extension

The bundled Chrome extension provides the fastest data path (Phase 1 in the cascade) and browser-exclusive features that the CLI cannot offer.

### What it does

| Feature | Description |
|---------|-------------|
| **Usage polling** | Fetches rate limits every 2 min, org metadata and per-model limits every 30 min |
| **Toolbar badge** | Live 5-hour usage % with color-coded background (green/yellow/red) |
| **Popup (Usage tab)** | Rate limit bars, 24h sparkline chart, current conversation cost |
| **Popup (Chats tab)** | Searchable list of all claude.ai conversations, click to open |
| **Popup (Estimate tab)** | Paste text to estimate token count and cost for Opus/Sonnet |
| **Conversation cost** | Content script on claude.ai shows floating cost widget and per-message badges |
| **Context menu** | Right-click selected text on any page, "Ask Claude about this" |
| **Notifications** | Desktop alerts at configurable warning (80%) and critical (95%) thresholds |
| **Options page** | Poll interval, badge toggle, notification settings |
| **Native messaging** | Sends enriched data to waybar via `com.aifuel.live_feed` host |

### Setup

```bash
# 1. Load the extension in Chrome
#    Open chrome://extensions, enable "Developer mode",
#    click "Load unpacked" and select ~/.config/aifuel/chrome-extension/

# 2. Configure native messaging
aifuel setup-chrome

# 3. Log into claude.ai
#    Data flows to waybar within 2 minutes.
```

Works with Chrome, Chrome Canary, Chromium, and Brave. The install wizard handles the Chrome extension setup if you select it during installation.

## Testing

```bash
bash tests/tests.sh
```

The test suite validates:

- Cost calculation accuracy
- Token counting correctness
- Rate limit data integrity
- Timezone handling for peak hours
- Implausible data detection
- Empty and stale state handling
- Concurrent access safety
- Waybar JSON output format
- Analytics engine edge cases

## How aifuel Compares

See the full [competitive comparison](docs/COMPARISON.md) against 28 tools in the ecosystem.

What makes aifuel unique:

- **Only Go-native waybar module** with embedded scripts and a Charm TUI
- **5-phase data cascade** that degrades gracefully across Chrome extension, cookies, OAuth, and local data
- **Peak hour detection and depletion prediction** built into the waybar tooltip
- **Multi-provider** from a single status bar widget (not one module per provider)
- **Interactive TUI installer** that detects your system and configures everything

## Dependencies

### Runtime

- `jq` for JSON processing in shell scripts
- `curl` for HTTP requests
- `waybar` for the status bar module
- A Nerd Font (recommended: JetBrainsMono Nerd Font)

### Optional

- `notify-send` (libnotify) for desktop notifications
- `ccusage` for enhanced session analytics
- `hack-browser-data` for cookie-based fetching
- `gum` for enhanced terminal prompts

### Build

- Go 1.24+
- [GoReleaser](https://goreleaser.com/) (for release builds)

## Contributing

Contributions are welcome.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Write tests for new functionality
4. Ensure all tests pass (`bash tests/tests.sh`)
5. Submit a pull request with a clear description

For bug reports and feature requests, [open an issue](https://github.com/robertogogoni/aifuel/issues).

## License

[MIT](LICENSE) (c) 2026 Roberto Gogoni

## Credits

- [NihilDigit/waybar-ai-usage](https://github.com/NihilDigit/waybar-ai-usage) for the original claude-usage waybar concept
- [Charm](https://charm.sh/) (Bubble Tea, Lip Gloss, huh) for the Go TUI ecosystem
- [GoReleaser](https://goreleaser.com/) for cross-platform release automation
- [Catppuccin](https://catppuccin.com/) for the color palette that ties everything together
