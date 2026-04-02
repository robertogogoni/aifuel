#!/bin/bash
# aifuel-claude.sh — Robust Claude usage fetcher
#
# Data source priority:
#   1. Cache (if fresh)
#   2. Cookie-based fetch from claude.ai (via HackBrowserData export)
#   3. JSONL session data + ccusage cost tracking (always available)
#
# The OAuth API (api.anthropic.com/api/oauth/usage) is intentionally NOT used
# as a primary source due to aggressive Cloudflare rate-limiting (known bug:
# anthropics/claude-code#31637). It's kept as a last-resort fallback only.

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

AIFUEL_PROVIDER="claude"
CACHE_FILE="$AIFUEL_CACHE_DIR/aifuel-cache-claude.json"
CREDENTIALS_FILE="$HOME/.claude/.credentials.json"
LOCK_FILE="$AIFUEL_CACHE_DIR/.claude-updating"
COOKIE_FETCHER="$SCRIPT_DIR/fetch-usage-via-cookies.sh"
LIVE_FEED_MAX_AGE=300  # 5 min

LIVE_FEED=$(resolve_live_feed)

# ── Phase 0: LIVE FEED fast path (bypasses ALL caches) ───────────────────────
# The Chrome extension writes fresh data every 2 min.
# On waybar hover (re-run), this returns instantly with live data.

_ensure_dirs

if [ -n "$LIVE_FEED" ] && [ -f "$LIVE_FEED" ]; then
    feed_age=$(( $(date +%s) - $(stat -c %Y "$LIVE_FEED") ))
    if [ "$feed_age" -lt "$LIVE_FEED_MAX_AGE" ]; then
        # Fast path: build output directly from live feed + local data
        # Skip all caches, locks, and network calls.
        _quick_session() {
            # Aggregate ALL today's sessions (not just the most recent)
            local files
            files=$(find "$HOME/.claude/projects" -name "*.jsonl" -type f -newermt "$(date +%Y-%m-%d)" 2>/dev/null)
            [ -z "$files" ] && return
            local total_out=0 total_cread=0 total_ccreate=0 total_msgs=0 total_sessions=0
            while IFS= read -r f; do
                [ -f "$f" ] || continue
                local stats
                stats=$(jq -n '[inputs | select(.message.usage != null) | .message.usage] | {
                    o: (map(.output_tokens // 0) | add // 0),
                    cr: (map(.cache_read_input_tokens // 0) | add // 0),
                    cc: (map(.cache_creation_input_tokens // 0) | add // 0),
                    m: length
                }' "$f" 2>/dev/null) || continue
                total_out=$(( total_out + $(echo "$stats" | jq '.o') ))
                total_cread=$(( total_cread + $(echo "$stats" | jq '.cr') ))
                total_ccreate=$(( total_ccreate + $(echo "$stats" | jq '.cc') ))
                total_msgs=$(( total_msgs + $(echo "$stats" | jq '.m') ))
                total_sessions=$(( total_sessions + 1 ))
            done <<< "$files"
            jq -n -c --argjson o "$total_out" --argjson cr "$total_cread" \
                --argjson cc "$total_ccreate" --argjson m "$total_msgs" --argjson s "$total_sessions" \
                '{total_output_tokens:$o, total_cache_read_tokens:$cr,
                  total_cache_creation_tokens:$cc, session_messages:$m, total_sessions:$s}'
        }
        _quick_cost() {
            # Use a short-lived cost cache (30s) to avoid 1.7s ccusage call on hover
            local cost_cache="$AIFUEL_CACHE_DIR/ccusage-quick.json"
            if [ -f "$cost_cache" ]; then
                local age=$(( $(date +%s) - $(stat -c %Y "$cost_cache") ))
                if [ "$age" -lt 30 ]; then
                    cat "$cost_cache"
                    return 0
                fi
            fi
            local bin
            bin=$(_find_ccusage)
            [ -z "$bin" ] || [ ! -x "$bin" ] && return
            local raw
            raw=$("$bin" --json --offline --since "$(date +%Y%m%d)" 2>/dev/null) || return
            local cost tok out hrs burn proj
            cost=$(echo "$raw" | jq -r '.totals.totalCost // 0')
            tok=$(echo "$raw" | jq -r '.totals.totalTokens // 0')
            out=$(echo "$raw" | jq -r '.totals.outputTokens // 0')
            hrs=$(date +%H); hrs=$(( hrs > 0 ? hrs : 1 ))
            burn=$(awk "BEGIN { printf \"%.2f\", $cost / $hrs }")
            proj=$(awk "BEGIN { printf \"%.2f\", $burn * 16 }")
            local models
            models=$(echo "$raw" | jq -c '[.daily[0].modelBreakdowns[]? | {
                model: (.modelName | sub("^claude-";"") | sub("-[0-9]{8}$";"")),
                cost: .cost, input: .inputTokens, output: .outputTokens,
                cache_read: .cacheReadTokens, cache_create: .cacheCreationTokens
            }]' 2>/dev/null)
            [ -z "$models" ] && models="[]"
            local result
            result=$(jq -n -c --argjson c "$cost" --argjson t "$tok" --argjson o "$out" \
                --argjson b "$burn" --argjson p "$proj" --argjson h "$hrs" \
                --argjson m "$models" \
                '{daily_cost:$c, daily_tokens:$t, daily_output:$o,
                  burn_rate_per_hour:$b, projected_daily_cost:$p, hours_active:$h, models:$m}')
            echo "$result" > "$cost_cache"
            echo "$result"
        }

        tier=$(jq -r '.claudeAiOauth.rateLimitTier // "unknown"' "$CREDENTIALS_FILE" 2>/dev/null)
        output=$(cat "$LIVE_FEED" | jq -c --arg plan "$tier" '{
            provider: "claude",
            five_hour: (.five_hour.utilization // 0),
            five_hour_reset: (.five_hour.resets_at // ""),
            seven_day: (.seven_day.utilization // 0),
            seven_day_reset: (.seven_day.resets_at // ""),
            seven_day_sonnet: (.seven_day_sonnet.utilization // 0),
            seven_day_sonnet_reset: (.seven_day_sonnet.resets_at // ""),
            extra_usage_credits: (.extra_usage.used_credits // 0),
            extra_usage_enabled: (.extra_usage.is_enabled // false),
            plan: $plan, data_source: "live"
        }' 2>/dev/null)

        td=$(_quick_session)
        [ -n "$td" ] && output=$(echo "$output" | jq -c --argjson td "$td" '. + $td')
        cd=$(_quick_cost)
        [ -n "$cd" ] && output=$(echo "$output" | jq -c --argjson cd "$cd" '. + $cd')

        # Analytics: depletion time, peak hours, binding limit, prompt log
        analytics_script="$SCRIPT_DIR/analytics.sh"
        if [ -x "$analytics_script" ]; then
            export FIVE_HOUR_PCT=$(echo "$output" | jq -r '.five_hour // 0')
            export FIVE_HOUR_RESET=$(echo "$output" | jq -r '.five_hour_reset // ""')
            export SEVEN_DAY_PCT=$(echo "$output" | jq -r '.seven_day // 0')
            export SEVEN_DAY_RESET=$(echo "$output" | jq -r '.seven_day_reset // ""')
            export SONNET_PCT=$(echo "$output" | jq -r '.seven_day_sonnet // 0')
            export BURN_RATE=$(echo "$output" | jq -r '.burn_rate_per_hour // 0')
            export JSONL_FILE=$(find "$HOME/.claude/projects" -name "*.jsonl" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-)
            analytics_json=$(bash -c "source '$analytics_script' && main" 2>/dev/null)
            [ -n "$analytics_json" ] && output=$(echo "$output" | jq -c --argjson a "$analytics_json" '. + {analytics: $a}')
        fi

        # Cache it too (for non-hover background reads)
        echo "$output" > "$CACHE_FILE"
        printf '%s\n' "$output"
        exit 0
    fi
fi

# ── Phase 0b: Cache check (for timer-based runs when live feed is stale) ─────

check_cache "$CACHE_FILE"

# ── Phase 1: Multi-instance lock ─────────────────────────────────────────────

if [ -f "$LOCK_FILE" ]; then
    lock_age=$(( $(date +%s) - $(stat -c %Y "$LOCK_FILE") ))
    if [ "$lock_age" -lt 10 ]; then
        sleep 1
        [ -f "$CACHE_FILE" ] && cat "$CACHE_FILE" && exit 0
    fi
    rm -f "$LOCK_FILE"
fi
touch "$LOCK_FILE" 2>/dev/null
trap 'rm -f "$LOCK_FILE" 2>/dev/null' EXIT

# ── Phase 2: Gather local data (always available) ────────────────────────────

_get_session_data() {
    # Aggregate ALL today's sessions
    local files
    files=$(find "$HOME/.claude/projects" -name "*.jsonl" -type f -newermt "$(date +%Y-%m-%d)" 2>/dev/null)
    [ -z "$files" ] && return
    local total_out=0 total_cread=0 total_ccreate=0 total_msgs=0 total_sessions=0
    while IFS= read -r f; do
        [ -f "$f" ] || continue
        local stats
        stats=$(jq -n '[inputs | select(.message.usage != null) | .message.usage] | {
            o: (map(.output_tokens // 0) | add // 0),
            cr: (map(.cache_read_input_tokens // 0) | add // 0),
            cc: (map(.cache_creation_input_tokens // 0) | add // 0),
            m: length
        }' "$f" 2>/dev/null) || continue
        total_out=$(( total_out + $(echo "$stats" | jq '.o') ))
        total_cread=$(( total_cread + $(echo "$stats" | jq '.cr') ))
        total_ccreate=$(( total_ccreate + $(echo "$stats" | jq '.cc') ))
        total_msgs=$(( total_msgs + $(echo "$stats" | jq '.m') ))
        total_sessions=$(( total_sessions + 1 ))
    done <<< "$files"
    jq -n -c --argjson o "$total_out" --argjson cr "$total_cread" \
        --argjson cc "$total_ccreate" --argjson m "$total_msgs" --argjson s "$total_sessions" \
        '{total_output_tokens:$o, total_cache_read_tokens:$cr,
          total_cache_creation_tokens:$cc, session_messages:$m, total_sessions:$s}'
}

_get_daily_cost() {
    local ccusage_bin
    ccusage_bin=$(_find_ccusage)
    [ -z "$ccusage_bin" ] || [ ! -x "$ccusage_bin" ] && return
    local raw
    raw=$("$ccusage_bin" --json --offline --since "$(date +%Y%m%d)" 2>/dev/null) || return
    local cost tokens output hours_active burn_rate projected
    cost=$(echo "$raw" | jq -r '.totals.totalCost // 0')
    tokens=$(echo "$raw" | jq -r '.totals.totalTokens // 0')
    output=$(echo "$raw" | jq -r '.totals.outputTokens // 0')
    hours_active=$(date +%H)
    hours_active=$(( hours_active > 0 ? hours_active : 1 ))
    burn_rate=$(awk "BEGIN { printf \"%.2f\", $cost / $hours_active }")
    projected=$(awk "BEGIN { printf \"%.2f\", $burn_rate * 16 }")
    # Per-model breakdown
    local models_json
    models_json=$(echo "$raw" | jq -c '[.daily[0].modelBreakdowns[]? | {
        model: (.modelName | sub("^claude-";"") | sub("-[0-9]{8}$";"")),
        cost: .cost, input: .inputTokens, output: .outputTokens,
        cache_read: .cacheReadTokens, cache_create: .cacheCreationTokens
    }]' 2>/dev/null)
    [ -z "$models_json" ] && models_json="[]"

    jq -n -c \
        --argjson cost "$cost" --argjson tokens "$tokens" --argjson output "$output" \
        --argjson burn "$burn_rate" --argjson proj "$projected" --argjson hrs "$hours_active" \
        --argjson models "$models_json" \
        '{daily_cost:$cost, daily_tokens:$tokens, daily_output:$output,
          burn_rate_per_hour:$burn, projected_daily_cost:$proj, hours_active:$hrs,
          models:$models}'
}

token_data=$(_get_session_data)
cost_data=$(_get_daily_cost)

# ── Helper: assemble final output ─────────────────────────────────────────────

_build_output() {
    local api_json="$1" source="$2"
    local tier
    tier=$(jq -r '.claudeAiOauth.rateLimitTier // "unknown"' "$CREDENTIALS_FILE" 2>/dev/null)

    local output
    if [ -n "$api_json" ]; then
        output=$(echo "$api_json" | jq -c --arg plan "$tier" --arg src "$source" '{
            provider: "claude",
            five_hour: (.five_hour.utilization // .five_hour // 0),
            five_hour_reset: (.five_hour.resets_at // .five_hour_reset // ""),
            seven_day: (.seven_day.utilization // .seven_day // 0),
            seven_day_reset: (.seven_day.resets_at // .seven_day_reset // ""),
            seven_day_sonnet: (.seven_day_sonnet.utilization // 0),
            seven_day_sonnet_reset: (.seven_day_sonnet.resets_at // ""),
            extra_usage_credits: (.extra_usage.used_credits // 0),
            extra_usage_enabled: (.extra_usage.is_enabled // false),
            plan: $plan, data_source: $src
        }' 2>/dev/null)
    else
        output=$(jq -n -c --arg plan "$tier" \
            '{provider:"claude", five_hour:0, five_hour_reset:"", seven_day:0,
              seven_day_reset:"", seven_day_sonnet:0, seven_day_sonnet_reset:"",
              extra_usage_credits:0, extra_usage_enabled:false,
              plan:$plan, data_source:"local", api_status:"unavailable"}')
    fi
    [ -n "$token_data" ] && output=$(echo "$output" | jq -c --argjson td "$token_data" '. + $td')
    [ -n "$cost_data" ] && output=$(echo "$output" | jq -c --argjson cd "$cost_data" '. + $cd')
    printf '%s\n' "$output"
}

# ── Phase 3: Cookie-based fetch (PRIMARY — hits claude.ai directly) ───────────

if [ -x "$COOKIE_FETCHER" ]; then
    cookie_result=$("$COOKIE_FETCHER" 2>/dev/null)
    if [ $? -eq 0 ] && echo "$cookie_result" | jq -e '.five_hour' &>/dev/null; then
        log_info "cookie fetch succeeded"
        output=$(_build_output "$cookie_result" "cookie")
        cache_output "$CACHE_FILE" "$output"
        exit 0
    fi
    log_info "cookie fetch failed, falling back"
fi

# ── Phase 4: OAuth API (FALLBACK — often rate-limited) ────────────────────────
# Only attempt if we haven't recently failed (simple cooldown via marker file)

OAUTH_FAIL_MARKER="$AIFUEL_CACHE_DIR/.oauth-cooldown"
OAUTH_COOLDOWN=1800  # 30 min — long cooldown since this path rarely works

_oauth_on_cooldown() {
    [ -f "$OAUTH_FAIL_MARKER" ] || return 1
    local age=$(( $(date +%s) - $(stat -c %Y "$OAUTH_FAIL_MARKER") ))
    [ "$age" -lt "$OAUTH_COOLDOWN" ]
}

if ! _oauth_on_cooldown; then
    access_token=$(jq -r '.claudeAiOauth.accessToken // empty' "$CREDENTIALS_FILE" 2>/dev/null)
    if [ -n "$access_token" ]; then
        log_info "trying OAuth API fallback..."
        resp=$(curl -sf -w '\n%{http_code}' "https://api.anthropic.com/api/oauth/usage" \
            -H "Authorization: Bearer $access_token" \
            -H "anthropic-beta: oauth-2025-04-20" \
            --max-time 8 2>/dev/null)
        http_code=$(echo "$resp" | tail -1)
        body=$(echo "$resp" | sed '$d')

        if [ "$http_code" = "200" ] && [ -n "$body" ] && ! echo "$body" | jq -e '.error' &>/dev/null; then
            log_info "OAuth API succeeded!"
            rm -f "$OAUTH_FAIL_MARKER" 2>/dev/null
            output=$(_build_output "$body" "api")
            cache_output "$CACHE_FILE" "$output"
            exit 0
        fi
        # Failed — set cooldown marker
        touch "$OAUTH_FAIL_MARKER" 2>/dev/null
        log_info "OAuth API failed (HTTP $http_code), cooldown ${OAUTH_COOLDOWN}s"
    fi
fi

# ── Phase 5: Local-only fallback ──────────────────────────────────────────────

if [ -f "$CACHE_FILE" ]; then
    log_info "returning stale cache + fresh local data"
    output=$(_build_output "$(cat "$CACHE_FILE")" "cache")
else
    log_info "returning local data only (JSONL + ccusage)"
    output=$(_build_output "" "local")
fi

cache_output "$CACHE_FILE" "$output"
