#!/bin/bash
# analytics.sh — aifuel analytics engine
#
# Computes derived metrics from raw data sources:
#   - Depletion time (when you'll hit the rate limit wall)
#   - Peak hour detection and burn multiplier
#   - Per-prompt cost log (last N interactions)
#   - Multi-layer binding limit (which limit constrains you)
#   - Session duration and conversation hours
#
# Called by the overlay to enrich output.
# Designed for speed: all pure computation, no network calls.

set -o pipefail

# ── Inputs (set by caller) ────────────────────────────────────────────────────
# FIVE_HOUR_PCT, FIVE_HOUR_RESET, SEVEN_DAY_PCT, SEVEN_DAY_RESET
# SONNET_PCT, SONNET_RESET, BURN_RATE, HOURS_ACTIVE, DAILY_COST
# JSONL_FILE (path to current session JSONL)

# ── Peak Hour Detection ───────────────────────────────────────────────────────
# Anthropic peak: weekdays 5am-11am PT (12:00-18:00 UTC)
# During peak, session burns ~2x faster

compute_peak_status() {
    local pt_hour pt_dow
    pt_hour=$(TZ=America/Los_Angeles date +%H)
    pt_dow=$(TZ=America/Los_Angeles date +%u)  # 1=Mon, 7=Sun

    local is_peak="false"
    local peak_label="off-peak"
    local peak_multiplier=1

    if [ "$pt_dow" -le 5 ] && [ "$pt_hour" -ge 5 ] && [ "$pt_hour" -lt 11 ]; then
        is_peak="true"
        peak_label="PEAK"
        peak_multiplier=2
    fi

    # Calculate time to next peak / off-peak transition
    local next_transition=""
    if [ "$is_peak" = "true" ]; then
        # Currently peak: when does off-peak start? 11am PT today
        next_transition=$(TZ=America/Los_Angeles date -d "today 11:00" +%s 2>/dev/null)
    elif [ "$pt_dow" -le 5 ]; then
        if [ "$pt_hour" -lt 5 ]; then
            # Before peak today
            next_transition=$(TZ=America/Los_Angeles date -d "today 05:00" +%s 2>/dev/null)
        else
            # After peak today, next peak is tomorrow (if weekday)
            if [ "$pt_dow" -lt 5 ]; then
                next_transition=$(TZ=America/Los_Angeles date -d "tomorrow 05:00" +%s 2>/dev/null)
            else
                # Friday after peak, next is Monday
                next_transition=$(TZ=America/Los_Angeles date -d "next Monday 05:00" +%s 2>/dev/null)
            fi
        fi
    fi

    local transition_mins=""
    if [ -n "$next_transition" ]; then
        local now diff
        now=$(date +%s)
        diff=$(( next_transition - now ))
        if [ "$diff" -gt 0 ]; then
            transition_mins=$(( diff / 60 ))
        fi
    fi

    jq -n -c \
        --argjson peak "$is_peak" \
        --arg label "$peak_label" \
        --argjson mult "$peak_multiplier" \
        --argjson trans "${transition_mins:-0}" \
        '{is_peak: $peak, label: $label, multiplier: $mult, transition_minutes: $trans}'
}

# ── Depletion Time ────────────────────────────────────────────────────────────
# Estimates when you'll hit 100% based on current burn rate and utilization.
# Uses the rate-limit reset time to bound the estimate.

compute_depletion() {
    local pct="$1" reset_iso="$2" burn="$3" peak_mult="${4:-1}"

    [ -z "$pct" ] || [ "$pct" = "0" ] && echo '{"depletes_in_minutes":null,"status":"idle"}' && return
    [ "$pct" -ge 100 ] 2>/dev/null && echo '{"depletes_in_minutes":0,"status":"depleted"}' && return

    # How long until reset?
    local reset_epoch now remaining_s
    if [ -n "$reset_iso" ] && [ "$reset_iso" != "null" ] && [ "$reset_iso" != "" ]; then
        reset_epoch=$(date -d "$reset_iso" +%s 2>/dev/null)
        now=$(date +%s)
        remaining_s=$(( reset_epoch - now ))
        [ "$remaining_s" -le 0 ] && remaining_s=0
    else
        remaining_s=18000  # default 5h
    fi

    # Time elapsed in this window = total_window - remaining
    local window_total_s=$((remaining_s + (remaining_s * pct / (100 - pct + 1)) ))
    local elapsed_s=$(( window_total_s - remaining_s ))
    [ "$elapsed_s" -le 0 ] && elapsed_s=1

    # Current velocity: pct% consumed in elapsed_s seconds
    # Adjusted for peak multiplier
    local effective_rate
    effective_rate=$(awk "BEGIN { printf \"%.4f\", ($pct / $elapsed_s) * $peak_mult }")

    # Time to reach 100%
    local remaining_pct=$(( 100 - pct ))
    local depletion_s
    depletion_s=$(awk "BEGIN { v=$effective_rate; if(v<=0) print 999999; else printf \"%.0f\", $remaining_pct / v }")

    # Cap at reset time (can't deplete past the window reset)
    if [ "$depletion_s" -gt "$remaining_s" ] 2>/dev/null; then
        # Won't deplete before reset
        jq -n -c --argjson mins "$(( remaining_s / 60 ))" '{"depletes_in_minutes":null,"status":"safe","resets_in_minutes":$mins}'
        return
    fi

    local depletion_mins=$(( depletion_s / 60 ))
    local status="ok"
    [ "$depletion_mins" -lt 30 ] && status="warning"
    [ "$depletion_mins" -lt 10 ] && status="critical"

    jq -n -c \
        --argjson mins "$depletion_mins" \
        --arg status "$status" \
        "{\"depletes_in_minutes\":\$mins,\"status\":\$status}"
}

# ── Per-Prompt Cost Log ───────────────────────────────────────────────────────
# Extracts last N assistant turns with estimated cost per turn.
# Cost model: opus output=$15/M, input=$15/M, cache_read=$1.875/M, cache_create=$18.75/M
#             haiku output=$5/M, input=$1/M, cache_read=$0.1/M, cache_create=$1.25/M

compute_prompt_log() {
    local jsonl_file="$1" count="${2:-8}"
    [ -z "$jsonl_file" ] || [ ! -f "$jsonl_file" ] && echo "[]" && return

    # Use tail to avoid parsing the entire JSONL (can be >10MB for long sessions)
    tail -300 "$jsonl_file" | jq -c "[
        [inputs
         | select(.message.usage != null and .message.role == \"assistant\")
         | {
             ts: .timestamp,
             model: (.message.model // \"unknown\" | sub(\"^claude-\"; \"\") | sub(\"-[0-9]{8}$\"; \"\")),
             output: (.message.usage.output_tokens // 0),
             input: (.message.usage.input_tokens // 0),
             cache_read: (.message.usage.cache_read_input_tokens // 0),
             cache_create: (.message.usage.cache_creation_input_tokens // 0)
           }
        ]
        | .[-${count}:][]
        | . + {
            cost: (
              if (.model | test(\"opus\")) then
                (.output * 15 + .input * 15 + .cache_read * 1.875 + .cache_create * 18.75) / 1000000
              elif (.model | test(\"haiku\")) then
                (.output * 5 + .input * 1 + .cache_read * 0.1 + .cache_create * 1.25) / 1000000
              else
                (.output * 3 + .input * 3 + .cache_read * 0.3 + .cache_create * 3.75) / 1000000
              end
            ),
            total_tokens: (.output + .input + .cache_read + .cache_create)
          }
    ]" 2>/dev/null || echo "[]"
}

# ── Multi-Layer Binding Limit ─────────────────────────────────────────────────
# Determines which of the 3 rate limit layers is the tightest constraint.

compute_binding_limit() {
    local fh_pct="$1" sd_pct="$2" sonnet_pct="$3"

    local binding="five_hour"
    local max_pct="${fh_pct:-0}"

    if [ "${sd_pct:-0}" -gt "$max_pct" ] 2>/dev/null; then
        binding="seven_day"
        max_pct="$sd_pct"
    fi
    if [ "${sonnet_pct:-0}" -gt "$max_pct" ] 2>/dev/null; then
        binding="sonnet"
        max_pct="$sonnet_pct"
    fi

    jq -n -c \
        --arg bind "$binding" \
        --argjson pct "$max_pct" \
        '{binding_limit: $bind, binding_pct: $pct}'
}

# ── Session Duration ──────────────────────────────────────────────────────────

compute_session_duration() {
    local jsonl_file="$1"
    [ -z "$jsonl_file" ] || [ ! -f "$jsonl_file" ] && echo '{"session_minutes":0}' && return

    local first_ts last_ts
    first_ts=$(head -20 "$jsonl_file" | jq -r 'select(.timestamp != null) | .timestamp' 2>/dev/null | head -1)
    last_ts=$(tail -20 "$jsonl_file" | jq -r 'select(.timestamp != null) | .timestamp' 2>/dev/null | tail -1)

    if [ -n "$first_ts" ] && [ -n "$last_ts" ]; then
        local first_epoch last_epoch
        first_epoch=$(date -d "$first_ts" +%s 2>/dev/null) || { echo '{"session_minutes":0}'; return; }
        last_epoch=$(date -d "$last_ts" +%s 2>/dev/null) || { echo '{"session_minutes":0}'; return; }
        local mins=$(( (last_epoch - first_epoch) / 60 ))
        jq -n -c --argjson m "$mins" '{session_minutes: $m}'
    else
        echo '{"session_minutes":0}'
    fi
}

# ── Main: compute all analytics and output as JSON ────────────────────────────

main() {
    local fh_pct="${FIVE_HOUR_PCT:-0}"
    local fh_reset="${FIVE_HOUR_RESET:-}"
    local sd_pct="${SEVEN_DAY_PCT:-0}"
    local sd_reset="${SEVEN_DAY_RESET:-}"
    local sn_pct="${SONNET_PCT:-0}"
    local burn="${BURN_RATE:-0}"
    local jsonl="${JSONL_FILE:-}"

    # Compute all metrics
    local peak_json binding_json depletion_5h_json depletion_7d_json prompt_log_json session_json

    peak_json=$(compute_peak_status)
    local peak_mult
    peak_mult=$(echo "$peak_json" | jq -r '.multiplier')

    binding_json=$(compute_binding_limit "$fh_pct" "$sd_pct" "$sn_pct")
    depletion_5h_json=$(compute_depletion "$fh_pct" "$fh_reset" "$burn" "$peak_mult")
    depletion_7d_json=$(compute_depletion "$sd_pct" "$sd_reset" "$burn" "1")
    prompt_log_json=$(compute_prompt_log "$jsonl" 8)
    session_json=$(compute_session_duration "$jsonl")

    # Merge all into single JSON
    jq -n -c \
        --argjson peak "$peak_json" \
        --argjson binding "$binding_json" \
        --argjson depl_5h "$depletion_5h_json" \
        --argjson depl_7d "$depletion_7d_json" \
        --argjson prompts "$prompt_log_json" \
        --argjson session "$session_json" \
        '{
            peak: $peak,
            binding: $binding,
            depletion_5h: $depl_5h,
            depletion_7d: $depl_7d,
            recent_prompts: $prompts,
            session: $session
        }'
}

# Only run main if executed directly (not sourced)
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    main "$@"
fi
