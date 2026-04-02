# Competitive Landscape

How aifuel compares to other AI usage monitoring tools (as of March 2026).

## Quick Overview

| Tool | Stars | Language | Waybar | Providers | TUI | Cost Tracking | Depletion Prediction |
|------|-------|----------|--------|-----------|-----|---------------|---------------------|
| **aifuel** | new | Go + Shell | Native | 4 | Yes | Yes (via ccusage) | Yes |
| [ccusage](https://github.com/ryoppippi/ccusage) | 12.3k | TypeScript | No | 5 | No | Yes (core) | No |
| [CodexBar](https://github.com/steipete/CodexBar) | 9.8k | Swift | CLI only | 17 | No | Yes (scan) | No |
| [Claude Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor) | 7.3k | Python | No | 1 | Yes | Yes | Yes (ML) |
| [ccseva](https://github.com/Iamshankhadeep/ccseva) | 790 | Electron | No | 1 | No | Yes | No |
| [claude-code-otel](https://github.com/ColeMurray/claude-code-otel) | 329 | Docker | No | 1 | No | Yes | No |
| [antigravity-usage](https://github.com/skainguyen1412/antigravity-usage) | 217 | TypeScript | No | 1 | No | No | No |
| [claude-code-usage-bar](https://github.com/leeguooooo/claude-code-usage-bar) | 181 | Python | No | 1 | No | No | No |
| [Lumo](https://github.com/zhnd/lumo) | 133 | Tauri | No | 1 | No | Yes | No |
| [claude-usage-bar](https://github.com/mnapoli/claude-usage-bar) | 101 | Tauri | No | 1 | No | No | No |
| [claude-meter](https://github.com/abhishekray07/claude-meter) | 76 | Python | No | 1 | No | No | No |
| [claudelytics](https://github.com/nwiizo/claudelytics) | 75 | Rust | No | 1 | Yes | No | No |
| [par_cc_usage](https://github.com/paulrobello/par_cc_usage) | 83 | Python | No | 1 | Yes | Yes | No |
| [TokenGauge](https://github.com/oorestisime/TokenGauge) | 4 | Rust | Native | 7 | Yes | No | No |
| [waybar-ai-usage](https://github.com/NihilDigit/waybar-ai-usage) | 31 | Python | Native | 4 | No | No | No |

## Detailed Feature Comparison

### Core Features

| Feature | aifuel | ccusage | CodexBar | Claude Monitor | TokenGauge | waybar-ai-usage |
|---------|--------|---------|----------|----------------|------------|-----------------|
| Rate limit bars | Yes | No | Yes | Yes | Yes | Yes |
| Cost tracking | Yes | **Best** | Yes | Yes | No | No |
| Depletion prediction | Yes | No | No | **ML-based** | No | No |
| Peak hour detection | **Unique** | No | No | No | No | No |
| Burn rate projections | Yes | No | No | Yes | No | No |
| Per-model breakdown | Yes | **Best** | Yes | Yes | No | No |
| Desktop notifications | Yes | No | No | Yes | No | No |
| History / sparklines | Yes | Yes | No | Yes | No | No |
| Clipboard export | Yes | No | No | No | No | No |

### Data Sources

| Source | aifuel | ccusage | CodexBar | Claude Monitor | TokenGauge | waybar-ai-usage |
|--------|--------|---------|----------|----------------|------------|-----------------|
| Chrome extension (live) | **Unique** | No | No | No | No | No |
| CLI OAuth tokens | Yes | No | Yes | No | No | No |
| Browser cookies | Yes | No | Yes | No | No | Yes |
| Local JSONL parsing | Yes | **Core** | No | **Core** | No | No |
| Background polling | Yes | No | No | No | No | No |
| CodexBar CLI backend | No | No | N/A | No | Yes | No |

### Integration

| Platform | aifuel | ccusage | CodexBar | Claude Monitor | TokenGauge | waybar-ai-usage |
|----------|--------|---------|----------|----------------|------------|-----------------|
| Waybar (native JSON) | Yes | No | No | No | Yes | Yes |
| TUI dashboard | Yes | No | No | Yes | Yes | No |
| Claude Code statusLine | No | Yes | No | No | No | No |
| macOS menu bar | No | No | **Best** | No | No | No |
| systemd service | Yes | No | No | No | No | No |
| Settings UI | Yes (TUI) | No | Yes (native) | No | No | No |

### Providers Supported

| Provider | aifuel | ccusage | CodexBar | Claude Monitor | TokenGauge | waybar-ai-usage |
|----------|--------|---------|----------|----------------|------------|-----------------|
| Claude | Yes | Yes | Yes | Yes | Yes | Yes |
| Codex | Yes | Yes | Yes | No | Yes | Yes |
| Gemini | Yes | No | Yes | No | No | No |
| Antigravity | Yes | No | Yes | No | No | No |
| Copilot | No | No | Yes | No | Yes | Yes |
| z.ai | No | No | Yes | No | Yes | No |
| Kimi / Kimi K2 | No | No | Yes | No | Yes | No |
| Others | No | OpenCode, Pi, Amp | Droid, Kiro, Augment, etc. | No | MiniMax | OpenCode Zen |

### Installation

| Method | aifuel | ccusage | CodexBar | Claude Monitor | TokenGauge | waybar-ai-usage |
|--------|--------|---------|----------|----------------|------------|-----------------|
| curl one-liner | Yes | No | No | No | Yes | No |
| Go binary | Yes | No | No | No | No | No |
| TUI install wizard | **Unique** | No | No | No | No | No |
| npx (zero-install) | No | **Best** | No | No | No | No |
| pip / uv | No | No | No | Yes | No | Yes |
| Homebrew | No | No | Yes | No | No | No |
| AUR | Planned | No | No | No | No | Yes |
| DMG / AppImage | No | No | Yes | No | No | No |

## What Makes aifuel Different

1. **Only Go-native waybar module with a TUI dashboard.** No other tool combines compiled binary performance, native waybar JSON output, and a full terminal dashboard in one package.

2. **Multi-source data cascade.** aifuel is the only tool that combines a Chrome extension (live feed), CLI OAuth tokens, browser cookies, background polling, and local JSONL parsing in a 5-phase priority cascade. Every other tool relies on one or two data sources.

3. **Peak hour detection** is unique to aifuel. No competitor tracks Anthropic's peak/off-peak windows or adjusts depletion estimates accordingly.

4. **Interactive TUI install wizard** using Charm libraries. Every other tool uses pip install, npx, or manual config. aifuel detects your system, walks you through provider selection, and sets up waybar, systemd, and the Chrome extension automatically.

5. **4 providers with real-time rate limits in waybar.** Only CodexBar supports more providers (17), but it has no native waybar output. Only waybar-ai-usage matches the waybar integration, but with limited analytics.

## Acknowledgments

aifuel builds on ideas from across this ecosystem:

- [NihilDigit/waybar-ai-usage](https://github.com/NihilDigit/waybar-ai-usage): original waybar concept and cookie-based auth
- [ccusage](https://github.com/ryoppippi/ccusage): JSONL cost analysis and per-model breakdowns
- [CodexBar](https://github.com/steipete/CodexBar): multi-provider architecture and Linux CLI
- [Claude-Code-Usage-Monitor](https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor): ML-based predictions and burn rate analytics
- [Charm](https://charm.sh/): Bubble Tea, Lip Gloss, and Huh for the TUI
