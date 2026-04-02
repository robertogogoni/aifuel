#!/bin/bash
# aifuel-codexbar.sh — Universal CodexBar bridge for aifuel
#
# Queries CodexBar CLI for ANY provider and maps the output to
# aifuel's standard JSON format. Called with a provider name:
#   aifuel-codexbar.sh <provider_name>
#
# Output: JSON with provider, five_hour, five_hour_reset, seven_day,
#         seven_day_reset, plan, and data_source fields.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

PROVIDER="${1:-}"
if [ -z "$PROVIDER" ]; then
    echo '{"error":"no provider specified","provider":"codexbar"}' >&2
    exit 1
fi

AIFUEL_PROVIDER="codexbar-${PROVIDER}"
CACHE_FILE="$AIFUEL_CACHE_DIR/aifuel-cache-codexbar-${PROVIDER}.json"

check_cache "$CACHE_FILE"

# ── Check CodexBar binary ───────────────────────────────────────────────────

if ! command -v codexbar &>/dev/null; then
    error_json "codexbar binary not found" "install codexbar for universal provider support"
fi

# ── Query CodexBar ──────────────────────────────────────────────────────────

log_info "querying CodexBar for provider: $PROVIDER"

raw_json=$(codexbar --provider "$PROVIDER" --format json 2>/dev/null)
codexbar_exit=$?

if [ "$codexbar_exit" -ne 0 ] || [ -z "$raw_json" ]; then
    fallback_stale_cache "$CACHE_FILE" "CodexBar returned no data for $PROVIDER"
    error_json "CodexBar failed for $PROVIDER (exit=$codexbar_exit)" "check that codexbar supports this provider"
fi

# Check for error in CodexBar output
cb_error=$(echo "$raw_json" | jq -r '.error // empty' 2>/dev/null)
if [ -n "$cb_error" ]; then
    fallback_stale_cache "$CACHE_FILE" "CodexBar error: $cb_error"
    error_json "CodexBar error: $cb_error" "check codexbar configuration for $PROVIDER"
fi

# ── Map CodexBar fields to aifuel format ────────────────────────────────────

# Rate limit fields (try multiple possible CodexBar field names)
five_hour=$(echo "$raw_json" | jq -r '
    .usedPercent //
    .five_hour //
    .rate_limit.used_percent //
    .rate_limit.primary.usedPercent //
    0
' 2>/dev/null)

seven_day=$(echo "$raw_json" | jq -r '
    .weeklyPercent //
    .seven_day //
    .rate_limit.weekly_percent //
    .rate_limit.secondary.usedPercent //
    0
' 2>/dev/null)

five_hour_reset=$(echo "$raw_json" | jq -r '
    .resetsAt //
    .five_hour_reset //
    .rate_limit.resets_at //
    .rate_limit.primary.resetsAt //
    ""
' 2>/dev/null)

seven_day_reset=$(echo "$raw_json" | jq -r '
    .weeklyResetsAt //
    .seven_day_reset //
    .rate_limit.weekly_resets_at //
    .rate_limit.secondary.resetsAt //
    ""
' 2>/dev/null)

plan=$(echo "$raw_json" | jq -r '
    .plan //
    .planType //
    .subscription //
    "unknown"
' 2>/dev/null)

# ── Check for cost data (CodexBar may return cost instead of rate limits) ───

daily_cost=$(echo "$raw_json" | jq -r '.dailyCost // .daily_cost // .cost.daily // null' 2>/dev/null)
total_tokens=$(echo "$raw_json" | jq -r '.totalTokens // .total_tokens // .tokens.total // null' 2>/dev/null)

# Build output JSON
output=$(jq -n -c \
    --arg provider "$PROVIDER" \
    --argjson five_hour "${five_hour:-0}" \
    --arg five_hour_reset "${five_hour_reset:-}" \
    --argjson seven_day "${seven_day:-0}" \
    --arg seven_day_reset "${seven_day_reset:-}" \
    --arg plan "${plan:-unknown}" \
    --arg data_source "codexbar" \
    '{
        provider: $provider,
        five_hour: $five_hour,
        five_hour_reset: $five_hour_reset,
        seven_day: $seven_day,
        seven_day_reset: $seven_day_reset,
        plan: $plan,
        data_source: $data_source
    }')

# Append cost fields if CodexBar returned cost data
if [ -n "$daily_cost" ] && [ "$daily_cost" != "null" ]; then
    output=$(echo "$output" | jq -c --argjson dc "$daily_cost" '. + {daily_cost: $dc}')
fi
if [ -n "$total_tokens" ] && [ "$total_tokens" != "null" ]; then
    output=$(echo "$output" | jq -c --argjson tt "$total_tokens" '. + {total_tokens: $tt}')
fi

log_info "CodexBar bridge succeeded for $PROVIDER"
cache_output "$CACHE_FILE" "$output"
