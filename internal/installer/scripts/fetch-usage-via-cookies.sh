#!/bin/bash
# fetch-usage-via-cookies.sh — Fetch Claude usage via browser cookies
#
# Uses cookies extracted by hack-browser-data (decrypted from Chrome).
# Cookie jar is refreshed periodically. Falls back to cached data if stale.
#
# Data flow:
#   1. Read cookie jar from ~/.cache/aifuel/cookie-jar.json
#   2. Hit claude.ai/api/organizations/{org}/usage with those cookies
#   3. Cache the result
#   4. If cookie jar is stale (>12h), re-extract with hack-browser-data

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

CACHE_DIR="$AIFUEL_CACHE_DIR"
COOKIE_JAR="$HOME/.cache/aifuel/cookie-jar.json"
USAGE_CACHE="$CACHE_DIR/claude-usage-cookie.json"
LIVE_FEED="$CACHE_DIR/claude-usage-live.json"
LIVE_FEED_TTL=180  # 3 min (extension polls every 2 min)
USAGE_CACHE_TTL=120
COOKIE_JAR_MAX_AGE=43200  # 12 hours
HBD_RESULTS_DIR="$HOME/.cache/aifuel/hbd-results"
LOG="$AIFUEL_LOG_FILE"

_log() { printf '[%s] [%-5s] [cookie] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >> "$LOG" 2>/dev/null; }

_ensure_dirs
mkdir -p "$HBD_RESULTS_DIR" 2>/dev/null

# ── Check live feed from Chrome extension (freshest source) ───────────────────

if [ -f "$LIVE_FEED" ]; then
    feed_age=$(( $(date +%s) - $(stat -c %Y "$LIVE_FEED") ))
    if [ "$feed_age" -lt "$LIVE_FEED_TTL" ]; then
        _log INFO "using live feed from Chrome extension (${feed_age}s old)"
        cat "$LIVE_FEED"
        exit 0
    fi
fi

# ── Check usage cache ─────────────────────────────────────────────────────────

if [ -f "$USAGE_CACHE" ]; then
    age=$(( $(date +%s) - $(stat -c %Y "$USAGE_CACHE") ))
    if [ "$age" -lt "$USAGE_CACHE_TTL" ]; then
        cat "$USAGE_CACHE"
        exit 0
    fi
fi

# ── Extract cookies from cookie jar ───────────────────────────────────────────

_read_cookie_jar() {
    local jar="$1"
    [ -f "$jar" ] || return 1

    # Try JSON format (hack-browser-data export)
    local org_id session_key cf_clearance
    org_id=$(jq -r '.[] | select(.Host == ".claude.ai" and .KeyName == "lastActiveOrg") | .Value' "$jar" 2>/dev/null | strings | grep -oP '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1)
    session_key=$(jq -r '.[] | select(.Host == ".claude.ai" and .KeyName == "sessionKey") | .Value' "$jar" 2>/dev/null | strings | grep -oP 'sk-ant-sid[^\s]+' | head -1)
    cf_clearance=$(jq -r '.[] | select(.Host == ".claude.ai" and .KeyName == "cf_clearance") | .Value' "$jar" 2>/dev/null | strings | grep -oP '[a-zA-Z0-9_.-]{30,}' | tail -1)

    [ -z "$org_id" ] || [ -z "$session_key" ] && return 1

    echo "$org_id|$session_key|$cf_clearance"
    return 0
}

# ── Try to refresh cookie jar if stale ────────────────────────────────────────

_refresh_cookie_jar() {
    local hbd_bin
    hbd_bin=$(_find_hbd)
    [ -z "$hbd_bin" ] || [ ! -x "$hbd_bin" ] && return 1
    _log INFO "refreshing cookie jar via hack-browser-data..."

    # Detect Chrome profile
    local chrome_profile
    chrome_profile=$(_detect_chrome_profile)
    [ -z "$chrome_profile" ] && { _log WARN "no Chrome profile found"; return 1; }

    # Run HBD pointing at detected Chrome profile
    local old_dir
    old_dir=$(pwd)
    cd "$(dirname "$hbd_bin")" || return 1
    "$hbd_bin" -b chromium --profile-path "$chrome_profile" -f json --dir "$HBD_RESULTS_DIR" &>/dev/null
    cd "$old_dir" || true

    # Find the newly exported cookie file
    local fresh_export
    fresh_export=$(find "$HBD_RESULTS_DIR" -name "*cookie.json" -newer "$COOKIE_JAR" 2>/dev/null | head -1)
    if [ -n "$fresh_export" ]; then
        cp "$fresh_export" "$COOKIE_JAR"
        _log INFO "cookie jar refreshed from $fresh_export"
        return 0
    fi

    _log WARN "hack-browser-data ran but no fresh cookies found"
    return 1
}

# ── Determine which cookie jar to use ─────────────────────────────────────────

cookie_data=""

# Try existing cookie jar first
if [ -f "$COOKIE_JAR" ]; then
    jar_age=$(( $(date +%s) - $(stat -c %Y "$COOKIE_JAR") ))
    if [ "$jar_age" -gt "$COOKIE_JAR_MAX_AGE" ]; then
        _refresh_cookie_jar || true
    fi
    cookie_data=$(_read_cookie_jar "$COOKIE_JAR")
fi

# Fall back to finding any HBD export in the results dir
if [ -z "$cookie_data" ]; then
    local_export=$(find "$HBD_RESULTS_DIR" -name "*cookie.json" 2>/dev/null | head -1)
    if [ -n "$local_export" ]; then
        cookie_data=$(_read_cookie_jar "$local_export")
        if [ -n "$cookie_data" ]; then
            cp "$local_export" "$COOKIE_JAR"
            _log INFO "bootstrapped cookie jar from HBD export"
        fi
    fi
fi

if [ -z "$cookie_data" ]; then
    _log WARN "no usable cookies found"
    exit 1
fi

# ── Fetch usage from claude.ai internal API ───────────────────────────────────

IFS='|' read -r ORG_ID SESSION_KEY CF_CLEARANCE <<< "$cookie_data"

_log INFO "fetching usage from claude.ai (org: ${ORG_ID:0:8}...)"

COOKIE_HEADER="sessionKey=${SESSION_KEY}; lastActiveOrg=${ORG_ID}"
[ -n "$CF_CLEARANCE" ] && COOKIE_HEADER="${COOKIE_HEADER}; cf_clearance=${CF_CLEARANCE}"

response=$(curl -sf "https://claude.ai/api/organizations/${ORG_ID}/usage" \
    -H "Cookie: ${COOKIE_HEADER}" \
    -H "Referer: https://claude.ai/chats" \
    -H "Origin: https://claude.ai" \
    -H "Accept: application/json" \
    -H "User-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36" \
    --max-time 10 \
    2>/dev/null)

if [ $? -ne 0 ] || [ -z "$response" ]; then
    _log ERROR "claude.ai API request failed"
    # Return stale cache if available
    [ -f "$USAGE_CACHE" ] && cat "$USAGE_CACHE" && exit 0
    exit 1
fi

# Validate response has usage data
if ! echo "$response" | jq -e '.five_hour' &>/dev/null; then
    _log ERROR "invalid API response: $(echo "$response" | head -c 100)"
    exit 1
fi

_log INFO "got real usage data: 5h=$(echo "$response" | jq -r '.five_hour.utilization')% 7d=$(echo "$response" | jq -r '.seven_day.utilization')%"

# Check cookie expiry and warn if approaching
_check_cookie_expiry() {
    local jar="$1"
    [ -f "$jar" ] || return
    local expiry_str
    expiry_str=$(jq -r '.[] | select(.Host == ".claude.ai" and .KeyName == "sessionKey") | .ExpireDate' "$jar" 2>/dev/null)
    [ -z "$expiry_str" ] && return
    local expiry_epoch now days_left
    expiry_epoch=$(date -d "$expiry_str" +%s 2>/dev/null) || return
    now=$(date +%s)
    days_left=$(( (expiry_epoch - now) / 86400 ))
    if [ "$days_left" -lt 3 ]; then
        _log WARN "sessionKey expires in ${days_left} days! Re-run hack-browser-data to refresh."
        # Send desktop notification
        notify-send -u critical -a "aifuel" "Cookie Expiring" \
            "Claude sessionKey expires in ${days_left} days. Run hack-browser-data to refresh." 2>/dev/null
    elif [ "$days_left" -lt 7 ]; then
        _log INFO "sessionKey expires in ${days_left} days"
    fi
}
_check_cookie_expiry "$COOKIE_JAR"

# Cache and output
echo "$response" | jq -c '.' > "$USAGE_CACHE"
cat "$USAGE_CACHE"
