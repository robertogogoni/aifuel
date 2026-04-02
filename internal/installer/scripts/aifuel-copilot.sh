#!/bin/bash
# GitHub Copilot usage fetcher for aifuel
# Method 1 (Primary): CodexBar CLI
# Method 2 (Fallback): gh CLI org metrics
#
# Output: JSON with provider, five_hour, five_hour_reset, seven_day,
#         seven_day_reset, plan, and data_source fields.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

AIFUEL_PROVIDER="copilot"
CACHE_FILE="$AIFUEL_CACHE_DIR/aifuel-cache-copilot.json"

check_cache "$CACHE_FILE"

# Helper: build output JSON
build_output() {
    local five_hour="${1:-0}"
    local five_hour_reset="${2:-}"
    local seven_day="${3:-0}"
    local seven_day_reset="${4:-}"
    local plan="${5:-unknown}"
    local data_source="$6"

    jq -n -c \
        --argjson five_hour "$five_hour" \
        --arg five_hour_reset "$five_hour_reset" \
        --argjson seven_day "$seven_day" \
        --arg seven_day_reset "$seven_day_reset" \
        --arg plan "$plan" \
        --arg data_source "$data_source" \
        '{
            provider: "copilot",
            five_hour: $five_hour,
            five_hour_reset: $five_hour_reset,
            seven_day: $seven_day,
            seven_day_reset: $seven_day_reset,
            plan: $plan,
            data_source: $data_source
        }'
}

# ── Method 1: CodexBar CLI ──────────────────────────────────────────────────

try_codexbar() {
    if ! command -v codexbar &>/dev/null; then
        log_warn "codexbar not found in PATH"
        return 1
    fi

    log_info "trying CodexBar CLI for copilot..."
    local raw_json
    raw_json=$(codexbar --provider copilot --format json 2>/dev/null)
    if [ $? -ne 0 ] || [ -z "$raw_json" ]; then
        log_warn "CodexBar CLI returned no data for copilot"
        return 1
    fi

    # Check for error in CodexBar output
    local cb_error
    cb_error=$(echo "$raw_json" | jq -r '.error // empty' 2>/dev/null)
    if [ -n "$cb_error" ]; then
        log_warn "CodexBar error: $cb_error"
        return 1
    fi

    # Map CodexBar fields to aifuel format
    local five_hour seven_day five_hour_reset seven_day_reset plan
    five_hour=$(echo "$raw_json" | jq -r '.usedPercent // .five_hour // .rate_limit.used_percent // 0' 2>/dev/null)
    seven_day=$(echo "$raw_json" | jq -r '.weeklyPercent // .seven_day // .rate_limit.weekly_percent // 0' 2>/dev/null)
    five_hour_reset=$(echo "$raw_json" | jq -r '.resetsAt // .five_hour_reset // .rate_limit.resets_at // ""' 2>/dev/null)
    seven_day_reset=$(echo "$raw_json" | jq -r '.weeklyResetsAt // .seven_day_reset // .rate_limit.weekly_resets_at // ""' 2>/dev/null)
    plan=$(echo "$raw_json" | jq -r '.plan // .planType // "copilot"' 2>/dev/null)

    local result
    result=$(build_output "$five_hour" "$five_hour_reset" "$seven_day" "$seven_day_reset" "$plan" "codexbar")
    if [ -n "$result" ]; then
        log_info "CodexBar CLI succeeded for copilot"
        echo "$result"
        return 0
    fi
    return 1
}

# ── Method 2: gh CLI org metrics ────────────────────────────────────────────

try_gh_api() {
    if ! command -v gh &>/dev/null; then
        log_warn "gh CLI not found"
        return 1
    fi

    # Check gh authentication
    if ! gh auth status &>/dev/null; then
        log_warn "gh CLI not authenticated"
        return 1
    fi

    # Determine org name from config or fallback to user login
    local org
    org=$(get_config_value "providers.copilot.org" "")
    if [ -z "$org" ]; then
        org=$(gh api user --jq '.login' 2>/dev/null)
    fi

    if [ -z "$org" ]; then
        log_warn "could not determine GitHub org/user"
        return 1
    fi

    log_info "trying gh API for copilot metrics (org=$org)..."
    local api_response
    api_response=$(gh api "orgs/${org}/copilot/metrics" \
        -H "Accept: application/vnd.github+json" \
        -H "X-GitHub-Api-Version: 2022-11-28" 2>/dev/null)

    if [ $? -ne 0 ] || [ -z "$api_response" ]; then
        log_warn "gh API request failed for copilot metrics"
        return 1
    fi

    # Check for error in response
    local api_error
    api_error=$(echo "$api_response" | jq -r '.message // empty' 2>/dev/null)
    if [ -n "$api_error" ]; then
        log_warn "gh API error: $api_error"
        return 1
    fi

    # Parse the response: gh copilot metrics may return seat/usage data
    # Map available data to our format; copilot metrics don't have exact
    # rate limit windows like Claude, so we extract what we can.
    local total_seats active_users suggestions_count
    total_seats=$(echo "$api_response" | jq -r '.total_seats // 0' 2>/dev/null)
    active_users=$(echo "$api_response" | jq -r '.total_active_users // 0' 2>/dev/null)
    suggestions_count=$(echo "$api_response" | jq -r '.total_suggestions_count // 0' 2>/dev/null)

    # Copilot doesn't have rate limit percentages like Claude/Codex,
    # so we report seat utilization as a proxy metric
    local seat_pct=0
    if [ "$total_seats" -gt 0 ] 2>/dev/null; then
        seat_pct=$(( (active_users * 100) / total_seats ))
    fi

    local plan_type
    plan_type=$(echo "$api_response" | jq -r '.copilot_plan // "business"' 2>/dev/null)

    local result
    result=$(build_output "$seat_pct" "" "0" "" "$plan_type" "gh-api")
    if [ -n "$result" ]; then
        log_info "gh API succeeded for copilot"
        echo "$result"
        return 0
    fi
    return 1
}

# ── Main ─────────────────────────────────────────────────────────────────────

# Try Method 1: CodexBar
if cb_output=$(try_codexbar) && [ -n "$cb_output" ]; then
    cache_output "$CACHE_FILE" "$cb_output"
    exit 0
fi

# Try Method 2: gh API
if gh_output=$(try_gh_api) && [ -n "$gh_output" ]; then
    cache_output "$CACHE_FILE" "$gh_output"
    exit 0
fi

# Both methods failed — try stale cache before giving up
fallback_stale_cache "$CACHE_FILE" "both CodexBar and gh API methods failed"

# No cache either
error_json "both CodexBar and gh API methods failed" "ensure gh CLI is authenticated or codexbar is installed; run 'aifuel check'"
