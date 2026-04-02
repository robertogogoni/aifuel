#!/bin/bash
# lib.sh — Shared functions for aifuel
# Sourced by all provider scripts, main module, and TUI.

# ── Paths ─────────────────────────────────────────────────────────────────────

AIFUEL_VERSION="0.1.0"
AIFUEL_CONFIG="$HOME/.config/aifuel/config.json"
AIFUEL_CACHE_DIR="$HOME/.cache/aifuel/cache"
AIFUEL_LOG_DIR="$HOME/.cache/aifuel"
AIFUEL_LOG_FILE="$AIFUEL_LOG_DIR/aifuel.log"
AIFUEL_LOG_MAX_LINES=1000

# ── Logging ───────────────────────────────────────────────────────────────────

_ensure_dirs() {
    [ -d "$AIFUEL_LOG_DIR" ] || mkdir -p "$AIFUEL_LOG_DIR" 2>/dev/null
    [ -d "$AIFUEL_CACHE_DIR" ] || mkdir -p "$AIFUEL_CACHE_DIR" 2>/dev/null
}

_log() {
    local level="$1"; shift
    _ensure_dirs
    local ts
    ts=$(date '+%Y-%m-%d %H:%M:%S')
    local caller="${AIFUEL_PROVIDER:-main}"
    printf '[%s] [%-5s] [%s] %s\n' "$ts" "$level" "$caller" "$*" >> "$AIFUEL_LOG_FILE" 2>/dev/null
}

log_info()  { _log INFO  "$@"; }
log_warn()  { _log WARN  "$@"; }
log_error() { _log ERROR "$@"; }

# Rotate log if it gets too big (keep last N lines)
rotate_log() {
    if [ -f "$AIFUEL_LOG_FILE" ]; then
        local lines
        lines=$(wc -l < "$AIFUEL_LOG_FILE" 2>/dev/null || echo 0)
        if [ "$lines" -gt "$AIFUEL_LOG_MAX_LINES" ]; then
            local tmp
            tmp=$(mktemp "${AIFUEL_LOG_FILE}.XXXXXX")
            tail -n "$AIFUEL_LOG_MAX_LINES" "$AIFUEL_LOG_FILE" > "$tmp" && mv "$tmp" "$AIFUEL_LOG_FILE"
        fi
    fi
}

# ── Error output ──────────────────────────────────────────────────────────────

# Print error JSON and exit. Usage: error_json "message" ["hint"]
# Automatically includes provider name from AIFUEL_PROVIDER.
error_json() {
    local msg="$1"
    local hint="${2:-}"
    local provider="${AIFUEL_PROVIDER:-unknown}"
    local full_msg="$msg"
    [ -n "$hint" ] && full_msg="$msg. Hint: $hint"
    log_error "$full_msg"
    jq -n -c --arg e "$full_msg" --arg p "$provider" '{"error":$e,"provider":$p}'
    exit 1
}

# ── Cache ─────────────────────────────────────────────────────────────────────

CACHE_MAX_AGE_DEFAULT=55  # seconds

# Read a value from the config file. Usage: get_config_value "key" "default"
get_config_value() {
    local key="$1" default="$2"
    if [ -f "$AIFUEL_CONFIG" ]; then
        local val
        val=$(jq -r ".$key // empty" "$AIFUEL_CONFIG" 2>/dev/null)
        [ -n "$val" ] && echo "$val" && return
    fi
    echo "$default"
}

# Check cache freshness. Usage: check_cache "/path/to/cache.json" [ttl_seconds]
# TTL resolution: explicit param > AIFUEL_CACHE_TTL env > config > 55s default
check_cache() {
    local cache_file="$1"
    local ttl="${2:-${AIFUEL_CACHE_TTL:-}}"
    [ -z "$ttl" ] && ttl=$(get_config_value "cache_ttl_seconds" "$CACHE_MAX_AGE_DEFAULT")
    _ensure_dirs
    if [ -f "$cache_file" ] && [ -s "$cache_file" ]; then
        local cache_age
        cache_age=$(( $(date +%s) - $(stat -c %Y "$cache_file") ))
        if [ "$cache_age" -lt "$ttl" ]; then
            cat "$cache_file"
            exit 0
        fi
    fi
}

# Return stale cache if it exists (better than nothing on transient errors).
# Usage: fallback_stale_cache "/path/to/cache.json" "error_context"
fallback_stale_cache() {
    local cache_file="$1"
    local context="${2:-transient error}"
    if [ -f "$cache_file" ]; then
        log_warn "using stale cache after $context"
        cat "$cache_file"
        exit 0
    fi
}

# ── Atomic write ──────────────────────────────────────────────────────────────

# Write content to file atomically (tmp + mv). Usage: atomic_write "/path/to/file" "content"
atomic_write() {
    local target="$1"
    local content="$2"
    local dir
    dir=$(dirname "$target")
    [ -d "$dir" ] || mkdir -p "$dir" 2>/dev/null
    local tmp
    tmp=$(mktemp "${dir}/.tmp.XXXXXX") || { log_error "mktemp failed for $target"; return 1; }
    printf '%s\n' "$content" > "$tmp" || { rm -f "$tmp"; log_error "write failed for $target"; return 1; }
    mv "$tmp" "$target" || { rm -f "$tmp"; log_error "mv failed for $target"; return 1; }
}

# Write output to cache and print it. Usage: cache_output "/path/to/cache.json" "json_string"
cache_output() {
    local cache_file="$1"
    local content="$2"
    atomic_write "$cache_file" "$content"
    printf '%s\n' "$content"
}

# ── Live feed resolution ─────────────────────────────────────────────────────

# Find the Chrome extension live feed file. Checks aifuel's own cache first,
# then falls back to the legacy ai-usage path (for users migrating from ai-usage
# or running both systems). Returns the path to the freshest valid file, or empty.
resolve_live_feed() {
    local candidate
    for candidate in \
        "$AIFUEL_CACHE_DIR/claude-usage-live.json" \
        "$HOME/.cache/ai-usage/cache/claude-usage-live.json"; do
        if [ -f "$candidate" ] && [ -s "$candidate" ]; then
            echo "$candidate"
            return
        fi
    done
}

# ── Countdown formatting ─────────────────────────────────────────────────────

# Format an ISO 8601 timestamp as a human-friendly countdown.
# Returns: "< 1m", "42m", "2h 30m", "1d 5h", "expired", or "—"
format_countdown() {
    local iso="$1"
    [ -z "$iso" ] || [ "$iso" = "null" ] || [ "$iso" = "" ] && echo "—" && return
    local reset_epoch now_epoch diff_s
    reset_epoch=$(date -d "$iso" +%s 2>/dev/null) || { echo "—"; return; }
    now_epoch=$(date +%s)
    diff_s=$(( reset_epoch - now_epoch ))
    if [ "$diff_s" -le 0 ]; then echo "expired"; return; fi
    local d=$(( diff_s / 86400 )) h=$(( (diff_s % 86400) / 3600 )) m=$(( (diff_s % 3600) / 60 ))
    if [ "$d" -gt 0 ]; then printf '%dd %dh' "$d" "$h"
    elif [ "$h" -gt 0 ]; then printf '%dh %dm' "$h" "$m"
    elif [ "$m" -gt 0 ]; then printf '%dm' "$m"
    else echo "< 1m"; fi
}

# ── Token formatting ──────────────────────────────────────────────────────────

# Format a raw token count as human-readable: 0, 842, 24.9K, 6.1M
format_tokens() {
    local n="${1:-0}"
    if [ "$n" -ge 1000000 ] 2>/dev/null; then
        awk "BEGIN { printf \"%.1fM\", $n / 1000000 }"
    elif [ "$n" -ge 1000 ] 2>/dev/null; then
        awk "BEGIN { printf \"%.1fK\", $n / 1000 }"
    else
        echo "$n"
    fi
}

# Format a cost as $X.XX
format_cost() {
    awk "BEGIN { printf \"$%.2f\", ${1:-0} }"
}

# ── Retry curl ───────────────────────────────────────────────────────────────

# Curl wrapper with retry and exponential backoff for transient failures.
# Usage: retry_curl [--retries N] [curl_args...]
# Retries up to 3 times (default) on HTTP 429/5xx or connection errors.
# Does NOT retry on 400/401/403/404.
retry_curl() {
    local max_retries=3
    if [ "$1" = "--retries" ]; then
        max_retries="$2"
        shift 2
    fi
    local attempt=0
    local backoff=1
    local tmp_out tmp_hdr http_code curl_exit

    tmp_out=$(mktemp) || return 1
    tmp_hdr=$(mktemp) || { rm -f "$tmp_out"; return 1; }

    while [ "$attempt" -lt "$max_retries" ]; do
        http_code=$(curl -w "%{http_code}" -o "$tmp_out" -D "$tmp_hdr" "$@" 2>/dev/null)
        curl_exit=$?

        # Success: curl OK and HTTP 2xx/3xx
        if [ "$curl_exit" -eq 0 ] && [[ "$http_code" =~ ^[23] ]]; then
            cat "$tmp_out"
            rm -f "$tmp_out" "$tmp_hdr"
            return 0
        fi

        # Don't retry on auth/client errors or rate limits (retrying 429 worsens it)
        case "$http_code" in
            400|401|403|404|429) break ;;
        esac

        # Retryable: connection errors (curl exit 6,7,28,35,52,56) or HTTP 5xx
        local retryable=false
        case "$curl_exit" in
            6|7|28|35|52|56) retryable=true ;;
        esac
        case "$http_code" in
            500|502|503|504) retryable=true ;;
        esac

        if ! $retryable; then
            break
        fi

        attempt=$((attempt + 1))
        if [ "$attempt" -lt "$max_retries" ]; then
            log_warn "retry $attempt/$max_retries (HTTP $http_code, curl exit $curl_exit) — waiting ${backoff}s"
            sleep "$backoff"
            backoff=$((backoff * 2))
        fi
    done

    # Return last output even on failure (callers check exit code)
    cat "$tmp_out"
    rm -f "$tmp_out" "$tmp_hdr"
    return 1
}

# ── Tool finders ─────────────────────────────────────────────────────────────

# Find ccusage binary. Checks PATH first, then common install locations.
_find_ccusage() {
    command -v ccusage 2>/dev/null || \
    for p in "$HOME/.cache/.bun/bin/ccusage" "$HOME/.local/bin/ccusage" "$HOME/.bun/bin/ccusage"; do
        [ -x "$p" ] && echo "$p" && return
    done
}

# ── ccusage integration ──────────────────────────────────────────────────────

# Get session breakdown from ccusage. Returns JSON array of sessions.
ccusage_sessions() {
    local bin
    bin=$(_find_ccusage)
    [ -z "$bin" ] && return
    "$bin" session --json --offline --since "$(date +%Y%m%d)" 2>/dev/null
}

# Get daily breakdown for the last N days. Returns JSON with daily array.
ccusage_daily() {
    local days="${1:-14}"
    local bin
    bin=$(_find_ccusage)
    [ -z "$bin" ] && return
    "$bin" daily --json --offline --since "$(date -d "$days days ago" +%Y%m%d)" 2>/dev/null
}

# Get monthly totals. Returns JSON with monthly array.
ccusage_monthly() {
    local bin
    bin=$(_find_ccusage)
    [ -z "$bin" ] && return
    "$bin" monthly --json --offline 2>/dev/null
}

# Find hack-browser-data binary. Checks PATH first, then common locations.
_find_hbd() {
    command -v hack-browser-data 2>/dev/null || \
    for p in "$HOME/.local/bin/hack-browser-data" "$HOME/Downloads/hack-browser-data"*/hack-browser-data; do
        [ -x "$p" ] && echo "$p" && return
    done
}

# Detect Chrome profile directory. Checks common browsers in preference order.
_detect_chrome_profile() {
    for dir in \
        "$HOME/.config/google-chrome-canary" \
        "$HOME/.config/google-chrome" \
        "$HOME/.config/chromium" \
        "$HOME/.config/BraveSoftware/Brave-Browser"; do
        [ -d "$dir" ] && echo "$dir" && return
    done
}

# ── Resolve script directory ──────────────────────────────────────────────────

# Returns the directory where aifuel scripts live.
# Order: 1. Local lib, 2. System path, 3. Development fallback.
resolve_libexec_dir() {
    local xdg_dir="$HOME/.local/lib/aifuel"
    local sys_dir="/usr/share/aifuel/scripts"

    if [ -d "$xdg_dir" ]; then
        echo "$xdg_dir"
    elif [ -d "$sys_dir" ]; then
        echo "$sys_dir"
    else
        # Development fallback: use the directory of the calling script
        # BASH_SOURCE[1] refers to the script that sourced lib.sh
        cd "$(dirname "${BASH_SOURCE[1]}")" && pwd
    fi
}
