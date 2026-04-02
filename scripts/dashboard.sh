#!/bin/bash
# aifuel Analytics Dashboard — Rich TUI with gum + Unicode
# Launched from waybar click or directly: dashboard.sh
#
# Features:
#   - Live rate limit bars with pacing indicator
#   - 14-day cost sparkline graph
#   - Session breakdown table
#   - Per-model cost distribution
#   - Burn rate and projections
#   - Monthly totals

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

CCUSAGE_BIN=$(_find_ccusage)
OVERLAY="$SCRIPT_DIR/aifuel-claude.sh"
LIVE_FEED=$(resolve_live_feed)
COOKIE_CACHE="$AIFUEL_CACHE_DIR/claude-usage-cookie.json"

# ── Colors (Catppuccin Mocha) ─────────────────────────────────────────────────

R='\033[0m'
BOLD='\033[1m'
DIM='\033[2m'
RED='\033[38;5;211m'
GREEN='\033[38;5;115m'
YELLOW='\033[38;5;223m'
BLUE='\033[38;5;111m'
PURPLE='\033[38;5;183m'
CYAN='\033[38;5;117m'
TEAL='\033[38;5;116m'
GRAY='\033[38;5;243m'
WHITE='\033[38;5;255m'
BG_BAR='\033[48;5;236m'

# ── Unicode drawing ───────────────────────────────────────────────────────────

SPARK_CHARS=('▁' '▂' '▃' '▄' '▅' '▆' '▇' '█')
H_LINE="━"
H_LINE_DIM="─"
BOX_TL="╭" BOX_TR="╮" BOX_BL="╰" BOX_BR="╯" BOX_V="│" BOX_H="─"

_repeat() { local s="" i; for ((i=0; i<$1; i++)); do s+="$2"; done; echo -n "$s"; }

_bar() {
    local pct="$1" width="${2:-20}" filled empty
    filled=$(( (pct * width + 50) / 100 ))
    [ "$filled" -gt "$width" ] && filled=$width
    [ "$filled" -lt 0 ] && filled=0
    empty=$(( width - filled ))
    local color="$GREEN"
    [ "$pct" -ge 60 ] && color="$YELLOW"
    [ "$pct" -ge 85 ] && color="$RED"
    echo -ne "${color}$(_repeat "$filled" "$H_LINE")${GRAY}$(_repeat "$empty" "$H_LINE_DIM")${R}"
}

_sparkline() {
    local values=("$@")
    local max=0 v
    for v in "${values[@]}"; do
        [ "${v%.*}" -gt "$max" ] 2>/dev/null && max="${v%.*}"
    done
    [ "$max" -eq 0 ] && max=1
    for v in "${values[@]}"; do
        local idx=$(( (${v%.*} * 7) / max ))
        [ "$idx" -gt 7 ] && idx=7
        [ "$idx" -lt 0 ] && idx=0
        echo -ne "${TEAL}${SPARK_CHARS[$idx]}${R}"
    done
}

_box_line() {
    local width="$1" left="$2" right="$3" fill="${4:- }"
    local inner=$(( width - 2 ))
    local content_len=$(( ${#left} + ${#right} ))
    local pad=$(( inner - content_len ))
    [ "$pad" -lt 0 ] && pad=0
    echo -e "${GRAY}${BOX_V}${R} ${left}$(_repeat "$pad" "$fill")${right} ${GRAY}${BOX_V}${R}"
}

_section_header() {
    local title="$1" width="${2:-52}"
    local inner=$(( width - 4 ))
    local pad=$(( inner - ${#title} ))
    echo -e "${GRAY}${BOX_TL}$(_repeat "$inner" "$BOX_H")${BOX_TR}${R}"
    echo -e "${GRAY}${BOX_V}${R} ${BOLD}${WHITE}${title}${R}$(_repeat "$((pad - 1))" " ") ${GRAY}${BOX_V}${R}"
    echo -e "${GRAY}${BOX_V}$(_repeat "$inner" "$BOX_H")${BOX_V}${R}"
}

_section_footer() {
    local width="${1:-52}"
    local inner=$(( width - 4 ))
    echo -e "${GRAY}${BOX_BL}$(_repeat "$inner" "$BOX_H")${BOX_BR}${R}"
}

# ── Data collection ───────────────────────────────────────────────────────────

# Live usage data (from extension or cookie cache)
usage_json=""
if [ -f "$LIVE_FEED" ]; then
    feed_age=$(( $(date +%s) - $(stat -c %Y "$LIVE_FEED") ))
    if [ "$feed_age" -lt 300 ]; then
        usage_json=$(cat "$LIVE_FEED")
    fi
fi
[ -z "$usage_json" ] && [ -f "$COOKIE_CACHE" ] && usage_json=$(cat "$COOKIE_CACHE")

# Overlay data (session + cost)
overlay_json=$("$OVERLAY" 2>/dev/null)

# ccusage daily history
if [ -n "$CCUSAGE_BIN" ] && [ -x "$CCUSAGE_BIN" ]; then
    daily_json=$("$CCUSAGE_BIN" daily --json --offline --since "$(date -d '14 days ago' +%Y%m%d)" 2>/dev/null)
    session_json=$("$CCUSAGE_BIN" session --json --offline --since "$(date +%Y%m%d)" 2>/dev/null)
    monthly_json=$("$CCUSAGE_BIN" monthly --json --offline 2>/dev/null)
else
    daily_json=""
    session_json=""
    monthly_json=""
fi

# ── Header ────────────────────────────────────────────────────────────────────

clear
W=52

echo ""
echo -e "  ${BOLD}${PURPLE}󰧑  aifuel Analytics${R}  ${GRAY}$(date '+%H:%M %b %d')${R}"
echo -e "  ${GRAY}$(_repeat 48 '━')${R}"
echo ""

# ── Rate Limits ───────────────────────────────────────────────────────────────

_section_header "Rate Limits" $W

if [ -n "$usage_json" ]; then
    fh=$(echo "$usage_json" | jq -r '.five_hour.utilization // 0' | cut -d. -f1)
    sd=$(echo "$usage_json" | jq -r '.seven_day.utilization // 0' | cut -d. -f1)
    fhr=$(echo "$usage_json" | jq -r '.five_hour.resets_at // ""')
    sdr=$(echo "$usage_json" | jq -r '.seven_day.resets_at // ""')

    # Format reset countdowns
    _countdown() {
        local iso="$1"
        [ -z "$iso" ] || [ "$iso" = "null" ] && echo "---" && return
        local reset_epoch now diff_s
        reset_epoch=$(date -d "$iso" +%s 2>/dev/null) || { echo "---"; return; }
        now=$(date +%s)
        diff_s=$(( reset_epoch - now ))
        [ "$diff_s" -le 0 ] && echo "now" && return
        local h=$(( diff_s / 3600 )) m=$(( (diff_s % 3600) / 60 ))
        printf '%dh %02dm' "$h" "$m"
    }

    fh_reset=$(_countdown "$fhr")
    sd_reset=$(_countdown "$sdr")

    echo -e "${GRAY}${BOX_V}${R}  ${WHITE}5-Hour${R}  $(_bar "$fh" 20)  ${BOLD}$(printf '%3d' "$fh")%${R}  ${DIM}${fh_reset}${R}  ${GRAY}${BOX_V}${R}"
    echo -e "${GRAY}${BOX_V}${R}  ${WHITE}7-Day ${R}  $(_bar "$sd" 20)  ${BOLD}$(printf '%3d' "$sd")%${R}  ${DIM}${sd_reset}${R}  ${GRAY}${BOX_V}${R}"

    # Sonnet limit
    cs=$(echo "$usage_json" | jq -r '.seven_day_sonnet.utilization // 0' | cut -d. -f1)
    if [ "$cs" -gt 0 ] 2>/dev/null; then
        echo -e "${GRAY}${BOX_V}${R}  ${BLUE}Sonnet${R}  $(_bar "$cs" 20)  ${BOLD}$(printf '%3d' "$cs")%${R}           ${GRAY}${BOX_V}${R}"
    fi

    # Extra usage
    extra=$(echo "$usage_json" | jq -r '.extra_usage.used_credits // 0')
    if [ "$extra" != "0" ] && [ "$extra" != "null" ]; then
        extra_dollars=$(awk "BEGIN { printf \"%.2f\", $extra / 100 }")
        echo -e "${GRAY}${BOX_V}${R}  ${YELLOW}  Overuse: \$${extra_dollars} this month${R}$(_repeat 14 ' ')${GRAY}${BOX_V}${R}"
    fi

    # Depletion time + peak status (from analytics)
    a_json=$(echo "$overlay_json" | jq '.analytics // empty' 2>/dev/null)
    if [ -n "$a_json" ] && [ "$a_json" != "null" ]; then
        depl_mins=$(echo "$a_json" | jq -r '.depletion_5h.depletes_in_minutes // ""')
        depl_status=$(echo "$a_json" | jq -r '.depletion_5h.status // "ok"')
        peak_is=$(echo "$a_json" | jq -r '.peak.is_peak // false')
        peak_trans=$(echo "$a_json" | jq -r '.peak.transition_minutes // 0')
        binding=$(echo "$a_json" | jq -r '.binding.binding_limit // "five_hour"')
        binding_pct=$(echo "$a_json" | jq -r '.binding.binding_pct // 0')

        if [ -n "$depl_mins" ] && [ "$depl_mins" != "null" ]; then
            d_color="$GREEN"
            [ "$depl_status" = "warning" ] && d_color="$YELLOW"
            [ "$depl_status" = "critical" ] && d_color="$RED"
            d_h=$(( depl_mins / 60 )); d_m=$(( depl_mins % 60 ))
            [ "$d_h" -gt 0 ] && d_str="${d_h}h ${d_m}m" || d_str="${d_m}m"
            echo -e "${GRAY}${BOX_V}${R}  ${GRAY}Depletes${R} ${d_color}${BOLD}${d_str}${R} ${GRAY}at current rate${R}$(_repeat 12 ' ')${GRAY}${BOX_V}${R}"
        fi

        if [ "$peak_is" = "true" ]; then
            echo -e "${GRAY}${BOX_V}${R}  ${RED}${BOLD}  PEAK${R} ${GRAY}burns 2x, ends ${peak_trans}m${R}$(_repeat 16 ' ')${GRAY}${BOX_V}${R}"
        else
            echo -e "${GRAY}${BOX_V}${R}  ${GREEN}  off-peak${R} ${GRAY}normal rate${R}$(_repeat 22 ' ')${GRAY}${BOX_V}${R}"
        fi

        if [ "$binding" != "five_hour" ]; then
            echo -e "${GRAY}${BOX_V}${R}  ${YELLOW}  Binding: ${binding} (${binding_pct}%)${R}$(_repeat 18 ' ')${GRAY}${BOX_V}${R}"
        fi
    fi

    source_label=""
    feed_age_s=""
    if [ -f "$LIVE_FEED" ]; then
        feed_age_s="$(( $(date +%s) - $(stat -c %Y "$LIVE_FEED") ))s ago"
        source_label="live"
    else
        source_label="cached"
    fi
    echo -e "${GRAY}${BOX_V}${R}  ${GRAY}${source_label} ${feed_age_s}${R}$(_repeat $(( 38 - ${#source_label} - ${#feed_age_s} )) ' ')${GRAY}${BOX_V}${R}"
else
    echo -e "${GRAY}${BOX_V}${R}  ${RED}No rate limit data available${R}$(_repeat 18 ' ')${GRAY}${BOX_V}${R}"
fi

_section_footer $W
echo ""

# ── Today's Spend ─────────────────────────────────────────────────────────────

_section_header "Today's Spend" $W

if [ -n "$overlay_json" ]; then
    daily_cost=$(echo "$overlay_json" | jq -r '.daily_cost // 0')
    daily_tokens=$(echo "$overlay_json" | jq -r '.daily_tokens // 0')
    burn=$(echo "$overlay_json" | jq -r '.burn_rate_per_hour // 0')
    projected=$(echo "$overlay_json" | jq -r '.projected_daily_cost // 0')
    hours=$(echo "$overlay_json" | jq -r '.hours_active // 0')
    msgs=$(echo "$overlay_json" | jq -r '.session_messages // 0')
    out_tok=$(echo "$overlay_json" | jq -r '.total_output_tokens // 0')

    cost_fmt=$(awk "BEGIN { printf \"%.2f\", $daily_cost }")
    burn_fmt=$(awk "BEGIN { printf \"%.2f\", $burn }")
    proj_fmt=$(awk "BEGIN { printf \"%.2f\", $projected }")
    tok_fmt=$(awk "BEGIN { t=$daily_tokens; if(t>=1000000) printf \"%.1fM\", t/1000000; else if(t>=1000) printf \"%.1fK\", t/1000; else printf \"%d\", t }")

    echo -e "${GRAY}${BOX_V}${R}  ${YELLOW}${BOLD}\$${cost_fmt}${R}  ${GRAY}total${R}    ${WHITE}${tok_fmt}${R} ${GRAY}tokens${R}$(_repeat 10 ' ')${GRAY}${BOX_V}${R}"
    echo -e "${GRAY}${BOX_V}${R}  ${GREEN}\$${burn_fmt}/hr${R} ${GRAY}burn${R}    ${CYAN}\$${proj_fmt}/day${R} ${GRAY}proj${R}$(_repeat 6 ' ')${GRAY}${BOX_V}${R}"
    # Session duration
    sess_min=$(echo "$overlay_json" | jq -r '.analytics.session.session_minutes // 0')
    sess_str=""
    if [ "$sess_min" -gt 0 ] 2>/dev/null; then
        s_h=$(( sess_min / 60 )); s_m=$(( sess_min % 60 ))
        sess_str="  ${s_h}h ${s_m}m session"
    fi

    echo -e "${GRAY}${BOX_V}${R}  ${WHITE}${msgs}${R} ${GRAY}messages${R}  ${WHITE}$(awk "BEGIN { t=$out_tok; if(t>=1000000) printf \"%.1fM\", t/1000000; else if(t>=1000) printf \"%.1fK\", t/1000; else printf \"%d\", t }")${R} ${GRAY}out${R}${DIM}${sess_str}${R}  ${GRAY}${BOX_V}${R}"

    # Last prompt cost
    last_cost=$(echo "$overlay_json" | jq -r '(.analytics.recent_prompts[-1].cost // 0) | . * 100 | round / 100')
    last_model=$(echo "$overlay_json" | jq -r '.analytics.recent_prompts[-1].model // ""')
    if [ "$last_cost" != "0" ] && [ -n "$last_model" ]; then
        echo -e "${GRAY}${BOX_V}${R}  ${GRAY}Last turn${R} ${YELLOW}\$${last_cost}${R} ${GRAY}(${last_model})${R}$(_repeat 16 ' ')${GRAY}${BOX_V}${R}"
    fi

    # Per-model breakdown
    models=$(echo "$overlay_json" | jq -r '.models // []')
    if [ -n "$models" ] && [ "$models" != "[]" ] && [ "$models" != "null" ]; then
        echo -e "${GRAY}${BOX_V}$(_repeat 48 "$BOX_H")${BOX_V}${R}"
        while IFS='|' read -r model cost output; do
            [ -z "$model" ] && continue
            color="$PURPLE"
            echo "$model" | grep -qi "haiku" && color="$TEAL"
            echo "$model" | grep -qi "sonnet" && color="$BLUE"
            cost_f=$(awk "BEGIN { printf \"%.2f\", $cost }")
            out_f=$(awk "BEGIN { t=$output; if(t>=1000) printf \"%.1fK\", t/1000; else printf \"%d\", t }")
            # Cost proportion bar
            prop=$(awk "BEGIN { printf \"%.0f\", ($cost / $daily_cost) * 20 }")
            echo -e "${GRAY}${BOX_V}${R}  ${color}${model}${R}$(_repeat $((16 - ${#model})) ' ')${YELLOW}\$${cost_f}${R}  ${GRAY}out:${out_f}${R}$(_repeat 6 ' ')${GRAY}${BOX_V}${R}"
        done < <(echo "$models" | jq -r '.[]? | [.model, .cost, .output] | join("|")' 2>/dev/null)
    fi
fi

_section_footer $W
echo ""

# ── 14-Day Cost History ───────────────────────────────────────────────────────

_section_header "14-Day History" $W

if [ -n "$daily_json" ]; then
    # Build sparkline from daily costs
    costs=()
    labels=()
    while IFS='|' read -r date cost; do
        [ -z "$date" ] && continue
        costs+=("${cost%.*}")
        labels+=("$(date -d "$date" '+%d')")
    done < <(echo "$daily_json" | jq -r '.daily[]? | [.date, .totalCost] | join("|")' 2>/dev/null)

    if [ ${#costs[@]} -gt 0 ]; then
        # Find max for scale
        max_cost=0
        total_cost=0
        for c in "${costs[@]}"; do
            [ "${c:-0}" -gt "$max_cost" ] 2>/dev/null && max_cost="$c"
            total_cost=$(( total_cost + ${c:-0} ))
        done
        avg_cost=$(( total_cost / ${#costs[@]} ))

        # Draw bar chart (vertical bars)
        chart_height=6
        for (( row=chart_height; row>=1; row-- )); do
            threshold=$(( (row * max_cost) / chart_height ))
            line="  "
            for c in "${costs[@]}"; do
                if [ "${c:-0}" -ge "$threshold" ] 2>/dev/null; then
                    # Color based on cost level
                    if [ "${c:-0}" -ge 50 ]; then
                        line+="${RED}█${R} "
                    elif [ "${c:-0}" -ge 30 ]; then
                        line+="${YELLOW}█${R} "
                    else
                        line+="${GREEN}█${R} "
                    fi
                else
                    line+="${GRAY}  ${R}"
                fi
            done
            if [ "$row" -eq "$chart_height" ]; then
                echo -e "${GRAY}${BOX_V}${R}${line}${GRAY}\$${max_cost}${R}$(_repeat 4 ' ')${GRAY}${BOX_V}${R}"
            elif [ "$row" -eq 1 ]; then
                echo -e "${GRAY}${BOX_V}${R}${line}${GRAY}\$0${R}$(_repeat $(( 4 + ${#max_cost} - 1 )) ' ')${GRAY}${BOX_V}${R}"
            else
                echo -e "${GRAY}${BOX_V}${R}${line}$(_repeat $(( 6 + ${#max_cost} )) ' ')${GRAY}${BOX_V}${R}"
            fi
        done

        # X-axis labels
        label_line="  "
        for l in "${labels[@]}"; do
            label_line+="${GRAY}${l}${R}"
        done
        echo -e "${GRAY}${BOX_V}${R}${label_line}$(_repeat $(( 6 + ${#max_cost} )) ' ')${GRAY}${BOX_V}${R}"
        echo -e "${GRAY}${BOX_V}${R}  ${GRAY}avg: \$${avg_cost}/day  total: \$${total_cost}${R}$(_repeat 10 ' ')${GRAY}${BOX_V}${R}"
    fi
fi

_section_footer $W
echo ""

# ── Monthly Summary ───────────────────────────────────────────────────────────

if [ -n "$monthly_json" ]; then
    _section_header "Monthly" $W

    while IFS='|' read -r month cost; do
        [ -z "$month" ] && continue
        cost_f=$(awk "BEGIN { printf \"%.2f\", $cost }")
        month_short=$(date -d "${month}-01" '+%b %Y' 2>/dev/null || echo "$month")
        echo -e "${GRAY}${BOX_V}${R}  ${WHITE}${month_short}${R}$(_repeat $((12 - ${#month_short})) ' ')${YELLOW}${BOLD}\$${cost_f}${R}$(_repeat $((26 - ${#cost_f})) ' ')${GRAY}${BOX_V}${R}"
    done < <(echo "$monthly_json" | jq -r '.monthly[]? | [.month, .totalCost] | join("|")' 2>/dev/null)

    _section_footer $W
    echo ""
fi

# ── Sessions Today ────────────────────────────────────────────────────────────

if [ -n "$session_json" ]; then
    session_count=$(echo "$session_json" | jq '.sessions | length' 2>/dev/null)
    if [ "${session_count:-0}" -gt 0 ] 2>/dev/null; then
        _section_header "Sessions Today (${session_count})" $W

        echo "$session_json" | jq -r '.sessions[:8][] | [.sessionId[0:20], .totalCost, .outputTokens] | join("|")' 2>/dev/null | while IFS='|' read -r sid cost output; do
            [ -z "$sid" ] && continue
            cost_f=$(awk "BEGIN { printf \"%.2f\", $cost }")
            out_f=$(awk "BEGIN { t=$output; if(t>=1000) printf \"%.1fK\", t/1000; else printf \"%d\", t }")
            # Truncate session ID
            sid_short="${sid:0:18}"
            echo -e "${GRAY}${BOX_V}${R}  ${CYAN}${sid_short}${R}$(_repeat $((20 - ${#sid_short})) ' ')${YELLOW}\$${cost_f}${R}  ${GRAY}${out_f}${R}$(_repeat 6 ' ')${GRAY}${BOX_V}${R}"
        done

        _section_footer $W
        echo ""
    fi
fi

# ── Recent Prompts ────────────────────────────────────────────────────────────

prompt_data=$(echo "$overlay_json" | jq -r '.analytics.recent_prompts // []' 2>/dev/null)
prompt_count=$(echo "$prompt_data" | jq 'length' 2>/dev/null)

if [ "${prompt_count:-0}" -gt 0 ] 2>/dev/null; then
    _section_header "Recent Turns (${prompt_count})" $W

    echo "$prompt_data" | jq -r '.[] | [.ts, .model, .cost, .output, .total_tokens] | join("|")' 2>/dev/null | while IFS='|' read -r ts model cost output total; do
        [ -z "$ts" ] && continue
        # Format timestamp to local time HH:MM
        t_local=$(date -d "$ts" '+%H:%M' 2>/dev/null || echo "??:??")
        cost_f=$(awk "BEGIN { printf \"%.2f\", $cost }")
        out_f=$(awk "BEGIN { t=$output; if(t>=1000) printf \"%.1fK\", t/1000; else printf \"%d\", t }")
        color="$PURPLE"
        echo "$model" | grep -qi "haiku" && color="$TEAL"
        echo "$model" | grep -qi "sonnet" && color="$BLUE"
        echo -e "${GRAY}${BOX_V}${R}  ${DIM}${t_local}${R}  ${color}${model}${R}$(_repeat $((12 - ${#model})) ' ')${YELLOW}\$${cost_f}${R}  ${GRAY}out:${out_f}${R}$(_repeat 6 ' ')${GRAY}${BOX_V}${R}"
    done

    _section_footer $W
    echo ""
fi

# ── Footer ────────────────────────────────────────────────────────────────────

plan=$(echo "$overlay_json" | jq -r '.plan // "unknown"' | sed 's/default_claude_//' | sed 's/_/ /g')
echo -e "  ${GRAY}Plan: ${BLUE}${plan}${R}  ${GRAY}|  Press any key to close${R}"
echo ""

# Wait for keypress
read -rsn1
