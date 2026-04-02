# ⛽ AIFuel

[![Release](https://img.shields.io/github/v/release/robertogogoni/aifuel?style=flat-square&color=a6e3a1)](https://github.com/robertogogoni/aifuel/releases)
[![License](https://img.shields.io/github/license/robertogogoni/aifuel?style=flat-square&color=89b4fa)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)

Your AI usage, always visible. A real-time AI provider usage monitor for waybar.

## Features

- **Live usage tracking** for Claude, Codex, Gemini, and Antigravity in your waybar
- **Smart analytics** including depletion prediction, peak hours detection, and binding limit resolution
- **Multi-source data cascade** combining Chrome extension cookies, CLI auth, and background polling for maximum reliability
- **TUI dashboard** with full-screen terminal interface built on Bubble Tea
- **Desktop notifications** at configurable warning and critical thresholds

## Screenshots

> Screenshots and GIFs coming soon. The waybar module displays a compact fuel gauge icon with color-coded status (green/yellow/red) and a rich tooltip showing per-provider breakdowns, depletion estimates, and peak hour status.

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/robertogogoni/aifuel/master/install.sh | bash
```

This detects your OS and architecture, downloads the latest release binary, and runs the interactive installer.

## Manual Install

### Prerequisites

- Go 1.22+ (for building from source)
- `jq` (for shell scripts)
- `curl` (for API calls)
- waybar (for the status bar module)
- A Nerd Font (recommended: JetBrainsMono Nerd Font)

### Build from Source

```bash
git clone https://github.com/robertogogoni/aifuel.git
cd aifuel
go build -o ~/.local/bin/aifuel ./cmd/aifuel/
aifuel install
```

The `aifuel install` command sets up:

- Runtime scripts in `~/.local/lib/aifuel/`
- Default config in `~/.config/aifuel/config.json`
- Systemd user service for background polling
- Native messaging host manifest for the Chrome extension

## Waybar Integration

Add the custom module to your waybar config (`~/.config/waybar/config.jsonc`):

1. Add `"custom/aifuel"` to your desired module position (e.g., `"modules-right"`).

2. Add the module definition:

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

3. Add the styles to your `~/.config/waybar/style.css` (see `waybar/style.css` in this repo for the full stylesheet).

The module outputs JSON with three fields:

- `text`: the compact display string (e.g., "⛽ 42%")
- `tooltip`: a rich multi-line breakdown with analytics
- `class`: one of `ai-ok`, `ai-warn`, or `ai-crit` for color styling

## Provider Setup

AIFuel supports four providers out of the box. Enable them in `~/.config/aifuel/config.json`:

| Provider | Auth Method | Status |
|----------|-------------|--------|
| Claude | CLI auth, cookies, OAuth | Stable |
| Codex | JSON-RPC app-server, OAuth | Experimental |
| Gemini | OAuth via gemini CLI | Experimental |
| Antigravity | Local language server probe | Experimental |

For detailed setup instructions per provider, see [docs/PROVIDERS.md](docs/PROVIDERS.md).

### Quick Start (Claude)

```bash
# If you already have the Claude CLI authenticated:
claude auth

# That's it. AIFuel reads the Claude CLI session automatically.
```

For richer real-time data, install the Chrome extension from `chrome-extension/` (load unpacked in developer mode) while logged into claude.ai.

## Configuration Reference

The config file lives at `~/.config/aifuel/config.json`. All fields have sensible defaults.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `display_mode` | string | `"full"` | Display mode: `"full"`, `"compact"`, or `"minimal"` |
| `refresh_interval` | int | `30` | Seconds between waybar refreshes |
| `cache_ttl_seconds` | int | `120` | How long cached API responses are considered fresh |
| `notifications_enabled` | bool | `true` | Enable desktop notifications for usage thresholds |
| `notify_warn_threshold` | int | `80` | Usage percentage that triggers a warning notification |
| `notify_critical_threshold` | int | `95` | Usage percentage that triggers a critical notification |
| `notify_cooldown_minutes` | int | `15` | Minimum time between repeated notifications |
| `history_enabled` | bool | `true` | Log usage data to JSONL files for trend analysis |
| `history_retention_days` | int | `7` | How many days of history to keep |
| `theme` | string | `"auto"` | Color theme: `"auto"`, `"dark"`, or `"light"` |
| `providers` | object | (see below) | Per-provider enable/disable and settings |

### Provider Config

```json
{
  "providers": {
    "claude": { "enabled": true },
    "codex": { "enabled": false },
    "gemini": { "enabled": false },
    "antigravity": { "enabled": false }
  }
}
```

## CLI Commands

```bash
aifuel install          # Interactive TUI installer wizard
aifuel check            # Run diagnostics (dependencies, credentials, network)
aifuel status           # Quick one-line usage summary
aifuel dashboard        # Launch the rich TUI dashboard
aifuel uninstall        # Clean removal with config preservation option
aifuel version          # Print version and build info
```

## Architecture

AIFuel uses a 5-phase data cascade:

1. **Collection**: Chrome extension, systemd poller, and CLI auth gather raw data
2. **Aggregation**: Overlay scripts merge multiple data sources per provider
3. **Caching**: TTL-based JSON cache prevents redundant API calls
4. **Analytics**: Depletion prediction, peak hours detection, binding limit resolution
5. **Display**: Waybar JSON output, TUI dashboard, desktop notifications

For the full architecture with diagrams, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## How aifuel Compares

See the full [competitive comparison](docs/COMPARISON.md) against 28 tools including ccusage (12.3k stars), CodexBar (9.8k stars), and Claude Monitor (7.3k stars).

**What makes aifuel unique:**
- Only Go-native waybar module with a TUI dashboard
- 5-phase data cascade (Chrome extension, CLI auth, cookies, polling, local JSONL)
- Peak hour detection and depletion prediction in waybar
- Interactive Charm TUI install wizard

## Dependencies

### Runtime

- `jq` for JSON processing in shell scripts
- `curl` for HTTP requests
- `notify-send` (libnotify) for desktop notifications (optional)
- waybar for the status bar module

### Build

- Go 1.22+
- GoReleaser (for release builds)

### TUI Dashboard

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the terminal UI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) for terminal styling

## Testing

Run the test suite:

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

## Contributing

Contributions are welcome. Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Write tests for new functionality
4. Ensure all tests pass (`bash tests/tests.sh`)
5. Submit a pull request with a clear description

For bug reports and feature requests, open an issue on GitHub.

## License

[MIT](LICENSE) (c) 2026 Roberto Gogoni

## Credits

- [NihilDigit/waybar-ai-usage](https://github.com/NihilDigit/waybar-ai-usage) for the original claude-usage waybar concept
- [Charm](https://charm.sh/) for Bubble Tea, Lip Gloss, and the excellent Go TUI libraries
- [GoReleaser](https://goreleaser.com/) for cross-platform release automation
