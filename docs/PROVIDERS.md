# Provider Setup Guide

AIFuel supports multiple AI providers. Each provider has its own authentication method and data source. Enable providers in your `config.json` by setting `"enabled": true` under the provider's key.

## Claude

**Status:** Fully supported (default provider)

### Authentication Methods

#### 1. Claude CLI Auth (Recommended)

If you have the Claude CLI (`claude`) installed and authenticated, AIFuel will use its session automatically.

```bash
claude auth
```

AIFuel reads the session token from the Claude CLI's config directory. No additional setup is needed.

#### 2. Cookie-Based Fetch (Chrome Extension)

The AIFuel Chrome extension reads your authenticated session cookies from claude.ai and fetches usage data directly from the API.

Setup:

1. Install the Chrome extension from `chrome-extension/` (load unpacked in developer mode)
2. Configure the native messaging host manifest for `com.aifuel.live_feed`
3. Log in to claude.ai in Chrome
4. The extension automatically polls every 2 minutes

The extension reads the `lastActiveOrg` cookie to determine your organization, then calls:

```
GET https://claude.ai/api/organizations/{orgId}/usage
```

Data is sent to the native messaging host which writes it to `~/.cache/aifuel/cache/claude-usage-live.json`.

#### 3. OAuth Fallback

If neither CLI auth nor cookies are available, AIFuel falls back to OAuth token refresh. This requires an initial OAuth flow:

```bash
aifuel auth claude
```

This opens a browser window for the OAuth consent flow. The refresh token is stored securely in your system keyring (via `keyctl` on Linux).

### API Endpoints

**Usage data** (polled every 2 minutes via Chrome extension, or on-demand via cookies/OAuth):
```
GET https://claude.ai/api/organizations/{orgId}/usage
```

**Account metadata** (polled every 30 minutes via Chrome extension, cached 1 hour):
```
GET https://claude.ai/api/organizations/{orgId}
```

### Data Fields (v1.3.0)

From the usage endpoint:

| Field | Type | Description |
|-------|------|-------------|
| `five_hour.utilization` | float | 5-hour rate limit consumed (0 to 100) |
| `five_hour.resets_at` | ISO 8601 | When the 5-hour window resets |
| `seven_day.utilization` | float | 7-day rate limit consumed (0 to 100) |
| `seven_day.resets_at` | ISO 8601 | When the 7-day window resets |
| `seven_day_sonnet.utilization` | float | Sonnet-specific 7-day limit |
| `seven_day_opus.utilization` | float/null | Opus-specific 7-day limit (activates when Anthropic enables it) |
| `seven_day_oauth_apps.utilization` | float/null | OAuth app-specific limit |
| `seven_day_cowork.utilization` | float/null | Team/collaboration limit |
| `extra_usage.is_enabled` | bool | Whether extra usage credits are active |
| `extra_usage.used_credits` | float | Credits consumed this billing cycle |
| `extra_usage.monthly_limit` | float/null | Credit cap (null = unlimited) |
| `extra_usage.utilization` | float/null | Credit usage as percentage |

From the account endpoint:

| Field | Type | Description |
|-------|------|-------------|
| `rate_limit_tier` | string | Real rate tier (e.g., `default_claude_max_20x`) |
| `billing_type` | string | Subscription type (`stripe_subscription`, `prepaid`) |
| `capabilities` | array | Account capabilities (e.g., `["chat", "claude_max"]`) |
| `claude_ai_bootstrap_models_config` | array | Available models with active/inactive/overflow flags |

## Codex

**Status:** Experimental

### Authentication

Codex uses a local JSON-RPC interface via its app-server process. AIFuel connects to the Codex app-server socket to query usage data.

```bash
# Ensure Codex app-server is running
codex --app-server
```

If the app-server is not available, AIFuel falls back to OAuth authentication:

```bash
aifuel auth codex
```

### Data Fields

Codex provides:

- `daily_tokens`: total tokens consumed today
- `daily_cost`: estimated cost in USD
- `rate_limit_remaining`: requests remaining in the current window
- `rate_limit_reset`: timestamp when the limit resets

### Configuration

```json
{
  "providers": {
    "codex": {
      "enabled": true,
      "app_server_socket": "/tmp/codex-app-server.sock"
    }
  }
}
```

## Gemini

**Status:** Experimental

### Authentication

Gemini uses OAuth authentication via the `gemini` CLI tool. AIFuel reads the OAuth tokens managed by the Gemini CLI.

```bash
# Authenticate with Gemini CLI first
gemini auth login
```

If the Gemini CLI is not installed, you can authenticate directly through AIFuel:

```bash
aifuel auth gemini
```

### Token Refresh

Gemini OAuth tokens expire after 1 hour. AIFuel automatically refreshes tokens using the stored refresh token. If the refresh fails (e.g., token revoked), you will see a notification prompting re-authentication.

### Data Fields

Gemini provides:

- `daily_requests`: number of API requests today
- `daily_tokens`: total tokens consumed today
- `quota_remaining`: percentage of daily quota remaining
- `quota_reset`: timestamp for quota reset (midnight Pacific)

### Configuration

```json
{
  "providers": {
    "gemini": {
      "enabled": true,
      "project_id": "your-gcp-project-id"
    }
  }
}
```

## Antigravity

**Status:** Experimental

### Authentication

Antigravity runs as a local language server. No external authentication is needed. AIFuel probes the local Antigravity process to read usage metrics.

### How It Works

AIFuel connects to the Antigravity language server's diagnostic endpoint to read session metrics:

```
GET http://localhost:{port}/api/v1/usage
```

The port is auto-detected from the running Antigravity process. If multiple instances are running, AIFuel aggregates usage from all of them.

### Data Fields

Antigravity provides:

- `session_tokens`: tokens consumed in the current session
- `session_cost`: estimated cost for the current session
- `model`: active model name
- `uptime_minutes`: how long the server has been running

### Configuration

```json
{
  "providers": {
    "antigravity": {
      "enabled": true,
      "port": 0
    }
  }
}
```

Set `port` to `0` for auto-detection, or specify a fixed port if you run Antigravity on a known port.

## Adding a Custom Provider

AIFuel's provider system is extensible. To add a new provider:

1. Create a Go module in `internal/providers/yourprovider/`
2. Implement the `Provider` interface:

```go
type Provider interface {
    Name() string
    Enabled() bool
    Fetch(ctx context.Context) (*UsageData, error)
    Auth(ctx context.Context) error
}
```

3. Register the provider in `internal/providers/registry.go`
4. Add a default config entry in `config/config.json`

For shell-only providers (no Go required), create a fetch script at `scripts/aifuel-{provider}.sh` that outputs JSON to stdout with at least:

```json
{
  "utilization": 42,
  "provider": "yourprovider",
  "timestamp": "2026-01-01T00:00:00Z"
}
```
