# AIFuel Architecture

## Data Flow Overview

AIFuel uses a 5-phase data cascade to collect, process, cache, analyze, and display AI provider usage data in your waybar.

```
Phase 1: Collection        Phase 2: Aggregation       Phase 3: Caching
========================   ========================   ========================

  Chrome Extension ──┐
    (cookies auth)   │     ┌──────────────────┐       ┌──────────────────┐
                     ├────>│                  │       │                  │
  Systemd Poller ────┤     │   Overlay Script │──────>│   JSON Cache     │
    (background)     │     │   (per provider) │       │   (TTL-based)    │
                     ├────>│                  │       │                  │
  CLI Auth ──────────┘     └──────────────────┘       └────────┬─────────┘
    (oauth/token)                                              │
                                                               v
Phase 4: Analytics         Phase 5: Display
========================   ========================

  ┌──────────────────┐     ┌──────────────────┐
  │                  │     │                  │
  │  Analytics Engine│────>│  Waybar Module   │
  │  (depletion,     │     │  (JSON output)   │
  │   peak hours,    │     │                  │
  │   binding limits)│     ├──────────────────┤
  │                  │     │                  │
  └──────────────────┘     │  TUI Dashboard   │
                           │  (Bubble Tea)    │
                           │                  │
                           └──────────────────┘
```

## Component Map

### Data Collection Layer

| Component | Location | Purpose |
|-----------|----------|---------|
| Chrome Extension | `chrome-extension/` | Fetches usage via authenticated browser session cookies |
| Background Poller | `scripts/aifuel-feed.sh` | Systemd-managed periodic fetcher using CLI auth |
| Cookie Fetcher | `scripts/fetch-usage-via-cookies.sh` | Direct cookie-based API calls (fallback) |
| Native Host | configured via `com.aifuel.live_feed` | Bridges Chrome extension data to local filesystem |

### Processing Layer

| Component | Location | Purpose |
|-----------|----------|---------|
| Claude Overlay | `scripts/aifuel-claude.sh` | Merges live feed, CLI data, and ccusage into unified JSON |
| Provider Modules | `internal/providers/` | Go modules for each AI provider (Claude, Codex, Gemini, Antigravity) |
| Config Loader | `internal/config/` | Reads and validates `config.json` with defaults |

### Cache Layer

| Component | Location | Purpose |
|-----------|----------|---------|
| Provider Cache | `~/.cache/aifuel/cache/aifuel-cache-{provider}.json` | Per-provider cached API responses |
| Live Feed Cache | `~/.cache/aifuel/cache/claude-usage-live.json` | Real-time data from Chrome extension |
| History Store | `~/.cache/aifuel/history/` | JSONL daily logs for trend analysis |

The cache layer uses TTL-based expiration (default: 120 seconds). When the cache is fresh, the module serves cached data without making API calls. When the cache expires, the next request triggers a fresh fetch. Stale cache data is still served if the fresh fetch fails, preventing empty states.

### Analytics Engine

| Component | Location | Purpose |
|-----------|----------|---------|
| Depletion Calculator | `scripts/analytics.sh` / `internal/analytics/` | Predicts when rate limits will be exhausted |
| Peak Hours Detector | `scripts/analytics.sh` / `internal/analytics/` | Identifies high-traffic periods (Pacific Time business hours) |
| Binding Limit Resolver | `scripts/analytics.sh` / `internal/analytics/` | Determines which rate limit (5h, 7d, model-specific) is the constraint |
| Prompt Logger | `scripts/analytics.sh` / `internal/analytics/` | Tracks per-prompt cost and token counts from JSONL logs |

#### Depletion Calculation

The depletion calculator takes the current utilization percentage, the rate limit reset time, and the recent consumption rate to project when the user will hit 100%. It returns one of four statuses:

- **idle**: utilization is 0%, no consumption detected
- **safe**: projected depletion is beyond the reset window
- **warning**: projected depletion is within 60 minutes of the reset window
- **depleted**: utilization has reached 100%

#### Peak Hours Detection

Peak hours are computed in Pacific Time (America/Los_Angeles) since Anthropic's rate limits are most constrained during US business hours. The detector checks:

- Weekday (Monday through Friday)
- Hours 5:00 AM to 11:00 AM PT

During peak hours, a multiplier is applied to depletion projections to account for increased competition for capacity.

#### Binding Limit Resolution

When multiple rate limits are active (5-hour window, 7-day window, model-specific caps), the binding limit resolver identifies which one is closest to exhaustion. This determines which limit is displayed prominently in the waybar module and which color class is applied.

### Display Layer

| Component | Location | Purpose |
|-----------|----------|---------|
| Waybar Module | `scripts/aifuel.sh` / Go CLI | Outputs JSON with `text`, `tooltip`, and `class` for waybar |
| TUI Dashboard | `internal/ui/` | Full-screen terminal dashboard built with Bubble Tea |
| Notification Bridge | Go CLI | Sends desktop notifications via `notify-send` at configurable thresholds |

## Chrome Extension Data Flow

```
┌─────────────────────────────────────────────────────────┐
│  Chrome Browser                                         │
│                                                         │
│  ┌─────────────────┐    ┌───────────────────────────┐   │
│  │  claude.ai       │    │  AIFuel Extension          │   │
│  │  (authenticated) │    │                           │   │
│  └────────┬────────┘    │  1. Read lastActiveOrg    │   │
│           │              │     cookie                │   │
│           │              │  2. Fetch /api/.../usage  │   │
│           │              │     with credentials      │   │
│           │              │  3. Send to native host   │   │
│           │              │     OR save to storage    │   │
│           │              └────────────┬──────────────┘   │
│           │                           │                  │
└───────────┼───────────────────────────┼──────────────────┘
            │                           │
            │                           v
            │              ┌───────────────────────────┐
            │              │  Native Messaging Host     │
            │              │  (com.aifuel.live_feed)    │
            │              │                           │
            │              │  Writes JSON to:          │
            │              │  ~/.cache/aifuel/cache/   │
            │              │  claude-usage-live.json   │
            │              └───────────────────────────┘
```

The Chrome extension polls every 2 minutes using `chrome.alarms`. It leverages the browser's existing authenticated session with claude.ai, avoiding the need to store or manage API keys. If the native messaging host is unavailable, data falls back to `chrome.storage.local` and is picked up on the next successful connection.

## Systemd Poller Architecture

```
┌──────────────────────────────────────────────┐
│  systemd user session                        │
│                                              │
│  aifuel-feed.service                         │
│  ├── Type: simple                            │
│  ├── Restart: on-failure (60s backoff)       │
│  ├── After: graphical-session.target         │
│  │                                           │
│  └── ExecStart: background-poller.sh         │
│       ├── Loop: sleep $refresh_interval      │
│       ├── For each enabled provider:         │
│       │   ├── Check cache TTL               │
│       │   ├── If expired: fetch fresh data  │
│       │   ├── Write to cache file           │
│       │   └── Append to history JSONL       │
│       └── On failure: log + continue        │
│                                              │
└──────────────────────────────────────────────┘
```

The systemd poller runs as a user service and handles all providers configured in `config.json`. It respects the cache TTL to avoid redundant API calls and writes to the same cache files that the waybar module reads. This ensures that even if the Chrome extension is not running, usage data stays fresh.

## Directory Structure

```
~/.local/lib/aifuel/          # Runtime scripts
    aifuel.sh                  # Main waybar module entry point
    aifuel-claude.sh           # Claude provider script
    aifuel-claude-overlay.sh   # Claude data overlay/merger
    analytics.sh               # Analytics engine (bash)
    background-poller.sh       # Systemd poller script
    dashboard.sh               # TUI launcher
    fetch-usage-via-cookies.sh # Cookie-based fetcher
    lib.sh                     # Shared utilities

~/.local/bin/aifuel            # Go CLI binary

~/.cache/aifuel/               # Runtime data
    cache/                     # Cached API responses (JSON)
    history/                   # Historical usage logs (JSONL)
    aifuel.log                 # Application log

~/.config/aifuel/              # User configuration
    config.json                # Main config file
```
