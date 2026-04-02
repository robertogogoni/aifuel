#!/bin/bash
# AIFuel Bar — Waybar module
# Reads config from ~/.config/aifuel/config.json
# Supports display modes: icon, compact, full

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"
# shellcheck source=aifuel-history.sh
source "$SCRIPT_DIR/aifuel-history.sh"
LIB_DIR=$(resolve_libexec_dir)

AIFUEL_PROVIDER="main"
CONFIG_FILE="$AIFUEL_CONFIG"

# Rotate log on each waybar refresh cycle
rotate_log

# ── Read config ───────────────────────────────────────────────────────────────

DISPLAY_MODE="icon"
HISTORY_ENABLED="true"
if [ -f "$CONFIG_FILE" ]; then
    DISPLAY_MODE=$(jq -r '.display_mode // "icon"' "$CONFIG_FILE")
    HISTORY_ENABLED=$(jq -r '.history_enabled // true' "$CONFIG_FILE")
fi
export AIFUEL_HISTORY_RETENTION
AIFUEL_HISTORY_RETENTION=$(jq -r '.history_retention_days // 7' "$CONFIG_FILE" 2>/dev/null || echo 7)

# Export cache TTL so provider subprocesses inherit it
export AIFUEL_CACHE_TTL
AIFUEL_CACHE_TTL=$(get_config_value "cache_ttl_seconds" "$CACHE_MAX_AGE_DEFAULT")

# ── Helpers ───────────────────────────────────────────────────────────────────

# countdown_from_iso removed — now using format_countdown from lib.sh

progress_bar_6() {
    local pct="$1"
    [ "$pct" -lt 0 ] 2>/dev/null && pct=0
    [ "$pct" -gt 100 ] 2>/dev/null && pct=100
    local filled=$(( (pct * 6 + 50) / 100 ))
    [ "$filled" -gt 6 ] && filled=6
    [ "$filled" -lt 0 ] && filled=0
    local empty=$(( 6 - filled )) bar="" i
    for (( i=0; i<filled; i++ )); do bar+="▰"; done
    for (( i=0; i<empty; i++ )); do bar+="▱"; done
    echo "$bar"
}

round_float() { printf "%.0f" "$1" 2>/dev/null || echo "0"; }

# ── Check which providers are enabled ─────────────────────────────────────────

claude_enabled=true
codex_enabled=true
gemini_enabled=true
antigravity_enabled=true
if [ -f "$CONFIG_FILE" ]; then
    claude_enabled=$(jq -r 'if .providers.claude.enabled == null then true else .providers.claude.enabled end' "$CONFIG_FILE")
    codex_enabled=$(jq -r 'if .providers.codex.enabled == null then true else .providers.codex.enabled end' "$CONFIG_FILE")
    gemini_enabled=$(jq -r 'if .providers.gemini.enabled == null then true else .providers.gemini.enabled end' "$CONFIG_FILE")
    antigravity_enabled=$(jq -r 'if .providers.antigravity.enabled == null then true else .providers.antigravity.enabled end' "$CONFIG_FILE")
fi

# ── Fetch provider data ──────────────────────────────────────────────────────

claude_json=""
codex_json=""
gemini_json=""
antigravity_json=""
claude_ok=false
codex_ok=false
gemini_ok=false
antigravity_ok=false

fetch_provider() {
    local name="$1" script="$2"
    local json
    json=$("$LIB_DIR/$script" 2>/dev/null)
    if [ -n "$json" ] && ! echo "$json" | jq -e '.error' &>/dev/null; then
        echo "$json"
        return 0
    fi
    return 1
}

if [ "$claude_enabled" = "true" ]; then
    claude_json=$(fetch_provider claude aifuel-claude.sh) && claude_ok=true
fi
if [ "$codex_enabled" = "true" ]; then
    codex_json=$(fetch_provider codex aifuel-codex.sh) && codex_ok=true
fi
if [ "$gemini_enabled" = "true" ]; then
    gemini_json=$(fetch_provider gemini aifuel-gemini.sh) && gemini_ok=true
fi
if [ "$antigravity_enabled" = "true" ]; then
    antigravity_json=$(fetch_provider antigravity aifuel-antigravity.sh) && antigravity_ok=true
fi

# Record history snapshots
if [ "$HISTORY_ENABLED" = "true" ]; then
    _record() {
        local ok="$1" json="$2" name="$3"
        if $ok; then
            local fh sd
            fh=$(echo "$json" | jq -r '.five_hour // 0')
            sd=$(echo "$json" | jq -r '.seven_day // 0')
            record_snapshot "$name" "$fh" "$sd"
        fi
    }
    _record "$claude_ok" "$claude_json" "claude"
    _record "$codex_ok" "$codex_json" "codex"
    _record "$gemini_ok" "$gemini_json" "gemini"
    _record "$antigravity_ok" "$antigravity_json" "antigravity"
fi

# ── Compute usage + extract all data ──────────────────────────────────────────

max_pct=0

# Color helpers for Pango markup
_pango_pct_color() {
    local pct="$1"
    if [ "$pct" -ge 85 ] 2>/dev/null; then echo "#f38ba8"  # red
    elif [ "$pct" -ge 60 ] 2>/dev/null; then echo "#f9e2af"  # yellow
    elif [ "$pct" -ge 30 ] 2>/dev/null; then echo "#a6e3a1"  # green
    else echo "#94e2d5"  # teal
    fi
}

_pango_bar() {
    local pct="$1" width="${2:-10}"
    local filled=$(( (pct * width + 50) / 100 ))
    [ "$filled" -gt "$width" ] && filled=$width
    [ "$filled" -lt 0 ] && filled=0
    local empty=$(( width - filled )) bar="" i
    local color=$(_pango_pct_color "$pct")
    for (( i=0; i<filled; i++ )); do bar+="━"; done
    local dim_bar=""
    for (( i=0; i<empty; i++ )); do dim_bar+="─"; done
    echo "<span foreground='${color}'>${bar}</span><span foreground='#585b70'>${dim_bar}</span>"
}

if $claude_ok; then
    c_api_status=$(echo "$claude_json" | jq -r '.api_status // "ok"')
    c5=$(round_float "$(echo "$claude_json" | jq -r '.five_hour // 0')")
    c7=$(round_float "$(echo "$claude_json" | jq -r '.seven_day // 0')")
    c5r=$(echo "$claude_json" | jq -r '.five_hour_reset // ""')
    c7r=$(echo "$claude_json" | jq -r '.seven_day_reset // ""')
    c_bar=$(progress_bar_6 "$c5")
    c_sonnet=$(round_float "$(echo "$claude_json" | jq -r '.seven_day_sonnet // 0')")
    c_sonnet_r=$(echo "$claude_json" | jq -r '.seven_day_sonnet_reset // ""')
    c_extra_credits=$(echo "$claude_json" | jq -r '.extra_usage_credits // 0')
    c_extra_enabled=$(echo "$claude_json" | jq -r '.extra_usage_enabled // false')
    c_plan=$(echo "$claude_json" | jq -r '.plan // "unknown"')
    c_source=$(echo "$claude_json" | jq -r '.data_source // "unknown"')
    c_msgs=$(echo "$claude_json" | jq -r '.session_messages // 0')
    c_out_raw=$(echo "$claude_json" | jq -r '.total_output_tokens // 0')
    c_cread_raw=$(echo "$claude_json" | jq -r '.total_cache_read_tokens // 0')
    c_ccreate_raw=$(echo "$claude_json" | jq -r '.total_cache_creation_tokens // 0')
    c_burn=$(echo "$claude_json" | jq -r '.burn_rate_per_hour // 0')
    c_projected=$(echo "$claude_json" | jq -r '.projected_daily_cost // 0')
    c_hours=$(echo "$claude_json" | jq -r '.hours_active // 0')
    c_daily_cost=$(echo "$claude_json" | jq -r '.daily_cost // 0')
    c_daily_tokens=$(echo "$claude_json" | jq -r '.daily_tokens // 0')
    c_daily_output=$(echo "$claude_json" | jq -r '.daily_output // 0')
    # Analytics data (depletion, peak, binding, prompts)
    c_analytics=$(echo "$claude_json" | jq -r '.analytics // empty')
    if [ -n "$c_analytics" ] && [ "$c_analytics" != "null" ]; then
        c_peak_label=$(echo "$c_analytics" | jq -r '.peak.label // "off-peak"')
        c_peak_is=$(echo "$c_analytics" | jq -r '.peak.is_peak // false')
        c_peak_transition=$(echo "$c_analytics" | jq -r '.peak.transition_minutes // 0')
        c_binding=$(echo "$c_analytics" | jq -r '.binding.binding_limit // "five_hour"')
        c_binding_pct=$(echo "$c_analytics" | jq -r '.binding.binding_pct // 0')
        c_depl_5h_mins=$(echo "$c_analytics" | jq -r '.depletion_5h.depletes_in_minutes // null')
        c_depl_5h_status=$(echo "$c_analytics" | jq -r '.depletion_5h.status // "ok"')
        c_session_mins=$(echo "$c_analytics" | jq -r '.session.session_minutes // 0')
        c_prompt_count=$(echo "$c_analytics" | jq -r '.recent_prompts | length')
        c_last_prompt_cost=$(echo "$c_analytics" | jq -r '(.recent_prompts[-1].cost // 0) | . * 100 | round / 100')
        c_last_prompt_model=$(echo "$c_analytics" | jq -r '.recent_prompts[-1].model // ""')
    fi

    [ "$c7" -gt "$max_pct" ] 2>/dev/null && max_pct=$c7
    [ "$c5" -gt "$max_pct" ] 2>/dev/null && max_pct=$c5
fi

# ── Notifications ────────────────────────────────────────────────────────────

NOTIFY_STATE_FILE="$AIFUEL_CACHE_DIR/notify-state.json"

NOTIFY_ENABLED="true"
NOTIFY_WARN_THRESH=80
NOTIFY_CRIT_THRESH=95
NOTIFY_COOLDOWN_MIN=15
if [ -f "$CONFIG_FILE" ]; then
    NOTIFY_ENABLED=$(jq -r '.notifications_enabled // true' "$CONFIG_FILE" 2>/dev/null)
    NOTIFY_WARN_THRESH=$(jq -r '.notify_warn_threshold // 80' "$CONFIG_FILE" 2>/dev/null)
    NOTIFY_CRIT_THRESH=$(jq -r '.notify_critical_threshold // 95' "$CONFIG_FILE" 2>/dev/null)
    NOTIFY_COOLDOWN_MIN=$(jq -r '.notify_cooldown_minutes // 15' "$CONFIG_FILE" 2>/dev/null)
fi

_send_notification() {
    local provider="$1" pct="$2" reset_info="$3"

    [ "$pct" -lt "$NOTIFY_WARN_THRESH" ] 2>/dev/null && return

    local now last_time cooldown_s
    now=$(date +%s)
    cooldown_s=$((NOTIFY_COOLDOWN_MIN * 60))
    if [ -f "$NOTIFY_STATE_FILE" ]; then
        last_time=$(jq -r ".${provider}_last // 0" "$NOTIFY_STATE_FILE" 2>/dev/null)
        if [ $((now - last_time)) -lt "$cooldown_s" ]; then
            return
        fi
    fi

    local urgency="normal"
    [ "$pct" -ge "$NOTIFY_CRIT_THRESH" ] 2>/dev/null && urgency="critical"

    notify-send -u "$urgency" -a "aifuel" \
        "AIFuel Alert" \
        "${provider^} usage at ${pct}% — resets in ${reset_info}" 2>/dev/null

    local state="{}"
    [ -f "$NOTIFY_STATE_FILE" ] && state=$(cat "$NOTIFY_STATE_FILE" 2>/dev/null)
    state=$(echo "$state" | jq --argjson t "$now" ".${provider}_last = \$t" 2>/dev/null)
    if [ -n "$state" ]; then
        atomic_write "$NOTIFY_STATE_FILE" "$state"
    else
        log_warn "failed to update notification state for $provider"
    fi
}

if [ "$NOTIFY_ENABLED" = "true" ] && command -v notify-send &>/dev/null; then
    $claude_ok && _send_notification "claude" "$c5" "$(format_countdown "$c5r")"
    $codex_ok && _send_notification "codex" "$x5" "$(format_countdown "$x5r")"
    $gemini_ok && _send_notification "gemini" "$g5" "$(format_countdown "$g5r")"
    $antigravity_ok && _send_notification "antigravity" "$a5" "$(format_countdown "$a5r")"
fi

# ── CSS class ─────────────────────────────────────────────────────────────────

if [ "$max_pct" -ge 85 ]; then class="ai-crit"
elif [ "$max_pct" -ge 60 ]; then class="ai-warn"
else class="ai-ok"; fi

# Append light theme class if system is in light mode
_is_light_theme() {
    local theme_pref
    theme_pref=$(jq -r '.theme // "auto"' "$CONFIG_FILE" 2>/dev/null)
    if [ "$theme_pref" = "dark" ]; then return 1; fi
    if [ "$theme_pref" = "light" ]; then return 0; fi
    local gtk_theme
    gtk_theme=$(gsettings get org.gnome.desktop.interface color-scheme 2>/dev/null | tr -d "'")
    case "$gtk_theme" in *dark*) return 1 ;; *light*) return 0 ;; esac
    gtk_theme=$(gsettings get org.gnome.desktop.interface gtk-theme 2>/dev/null | tr -d "'")
    case "$gtk_theme" in *[Ll]ight*) return 0 ;; esac
    return 1
}
_is_light_theme && class="$class aifuel-light"

# ── Build rich Pango tooltip ───────────────────────────────────────────────────

if ! $claude_ok && ! $codex_ok && ! $gemini_ok && ! $antigravity_ok; then
    jq -n -c '{"text":"󰧑 ?","tooltip":"AIFuel: No data available\nCheck credentials","class":"ai-warn"}'
    exit 0
fi

_api_available=true
[ "$c_api_status" = "unavailable" ] && _api_available=false

# ── Header
H="<span foreground='#cdd6f4' size='large'><b>  AIFuel Dashboard</b></span>"
SEP="<span foreground='#45475a'>━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━</span>"
SEP_THIN="<span foreground='#313244'>──────────────────────────────────────</span>"
DIM='#6c7086'
LABEL='#bac2de'
VAL='#cdd6f4'

tooltip="${H}"$'\n'"${SEP}"

# ── Rate Limits Section
if $claude_ok; then
    if $_api_available; then
        c5_color=$(_pango_pct_color "$c5")
        c7_color=$(_pango_pct_color "$c7")
        c5_bar=$(_pango_bar "$c5" 12)
        c7_bar=$(_pango_bar "$c7" 12)

        tooltip+=$'\n'"<span foreground='${LABEL}'><b>Rate Limits</b></span>"
        tooltip+=$'\n'"  <span foreground='${DIM}'>5-Hour</span>   ${c5_bar}  <span foreground='${c5_color}'><b>${c5}%</b></span>  <span foreground='${DIM}'>resets</span> <span foreground='${VAL}'>$(format_countdown "$c5r")</span>"
        tooltip+=$'\n'"  <span foreground='${DIM}'>7-Day</span>    ${c7_bar}  <span foreground='${c7_color}'><b>${c7}%</b></span>  <span foreground='${DIM}'>resets</span> <span foreground='${VAL}'>$(format_countdown "$c7r")</span>"

        # Sonnet-specific limit (if active)
        if [ "$c_sonnet" -gt 0 ] 2>/dev/null; then
            cs_color=$(_pango_pct_color "$c_sonnet")
            cs_bar=$(_pango_bar "$c_sonnet" 12)
            tooltip+=$'\n'"  <span foreground='${DIM}'>Sonnet</span>   ${cs_bar}  <span foreground='${cs_color}'><b>${c_sonnet}%</b></span>  <span foreground='${DIM}'>resets</span> <span foreground='${VAL}'>$(format_countdown "$c_sonnet_r")</span>"
        fi

        # Extra usage / overuse credits
        if [ "$c_extra_enabled" = "true" ] && [ "$c_extra_credits" != "0" ] && [ "$c_extra_credits" != "null" ]; then
            extra_dollars=$(awk "BEGIN { printf \"%.2f\", ${c_extra_credits} / 100 }")
            tooltip+=$'\n'"  <span foreground='#f9e2af'>  Overuse credits: \$${extra_dollars} used this month</span>"
        fi
    else
        tooltip+=$'\n'"<span foreground='${LABEL}'><b>Rate Limits</b></span>  <span foreground='#f38ba8'>[API offline]</span>"
    fi

    # Depletion time + peak status
    if [ -n "$c_depl_5h_mins" ] && [ "$c_depl_5h_mins" != "null" ]; then
        depl_color="#a6e3a1"
        [ "$c_depl_5h_status" = "warning" ] && depl_color="#f9e2af"
        [ "$c_depl_5h_status" = "critical" ] && depl_color="#f38ba8"
        depl_h=$(( c_depl_5h_mins / 60 ))
        depl_m=$(( c_depl_5h_mins % 60 ))
        if [ "$depl_h" -gt 0 ]; then
            depl_str="${depl_h}h ${depl_m}m"
        else
            depl_str="${depl_m}m"
        fi
        tooltip+=$'\n'"  <span foreground='${DIM}'>Depletes</span> <span foreground='${depl_color}'><b>${depl_str}</b></span>  <span foreground='${DIM}'>at current rate</span>"
    elif [ "$c_depl_5h_status" = "safe" ]; then
        tooltip+=$'\n'"  <span foreground='#a6e3a1'>  Won't deplete before reset</span>"
    fi

    # Peak hour indicator
    if [ "$c_peak_is" = "true" ]; then
        tooltip+=$'\n'"  <span foreground='#f38ba8'><b>  PEAK HOURS</b></span> <span foreground='${DIM}'>(burns 2x faster, ends in ${c_peak_transition}m)</span>"
    else
        tooltip+=$'\n'"  <span foreground='#a6e3a1'>  off-peak</span> <span foreground='${DIM}'>(normal rate)</span>"
    fi

    # Binding limit highlight
    if [ -n "$c_binding" ] && [ "$c_binding" != "five_hour" ]; then
        tooltip+=$'\n'"  <span foreground='#f9e2af'>  Binding: ${c_binding} (${c_binding_pct}%)</span>"
    fi

    # Plan badge
    plan_display=$(echo "$c_plan" | sed 's/default_claude_//' | sed 's/_/ /g')
    tooltip+=$'\n'"  <span foreground='${DIM}'>Plan</span>     <span foreground='#89b4fa'>${plan_display}</span>"

    tooltip+=$'\n'"${SEP_THIN}"

    # ── Session Section
    if [ "$c_msgs" -gt 0 ] 2>/dev/null; then
        session_time=""
        if [ -n "$c_session_mins" ] && [ "$c_session_mins" != "0" ] && [ "$c_session_mins" != "null" ]; then
            s_h=$(( c_session_mins / 60 ))
            s_m=$(( c_session_mins % 60 ))
            session_time="  ${s_h}h ${s_m}m"
        fi
        tooltip+=$'\n'"<span foreground='${LABEL}'><b>Session</b></span>  <span foreground='${DIM}'>${session_time}</span>"
        tooltip+=$'\n'"  <span foreground='${DIM}'>Messages</span>    <span foreground='${VAL}'><b>${c_msgs}</b></span>"
        tooltip+=$'\n'"  <span foreground='${DIM}'>Output</span>      <span foreground='#a6e3a1'>$(format_tokens "$c_out_raw")</span> <span foreground='${DIM}'>tokens</span>"
        tooltip+=$'\n'"  <span foreground='${DIM}'>Cache Read</span>  <span foreground='#89dceb'>$(format_tokens "$c_cread_raw")</span> <span foreground='${DIM}'>tokens</span>"
        tooltip+=$'\n'"  <span foreground='${DIM}'>Cache Write</span> <span foreground='#74c7ec'>$(format_tokens "$c_ccreate_raw")</span> <span foreground='${DIM}'>tokens</span>"
        # Last prompt cost
        if [ -n "$c_last_prompt_cost" ] && [ "$c_last_prompt_cost" != "0" ] && [ "$c_last_prompt_cost" != "null" ]; then
            tooltip+=$'\n'"  <span foreground='${DIM}'>Last turn</span>  <span foreground='#f9e2af'>\$${c_last_prompt_cost}</span> <span foreground='${DIM}'>(${c_last_prompt_model})</span>"
        fi
        tooltip+=$'\n'"${SEP_THIN}"
    fi

    # ── Cost Section
    if [ "$c_daily_cost" != "0" ] && [ "$c_daily_cost" != "null" ]; then
        tooltip+=$'\n'"<span foreground='${LABEL}'><b>Today's Spend</b></span>"
        tooltip+=$'\n'"  <span foreground='${DIM}'>Total</span>       <span foreground='#f9e2af'><b>$(format_cost "$c_daily_cost")</b></span>  <span foreground='${DIM}'>$(format_tokens "$c_daily_tokens") tok</span>"

        # Per-model breakdown from overlay data
        models_json=$(echo "$claude_json" | jq -r '.models // []' 2>/dev/null)
        if [ -n "$models_json" ] && [ "$models_json" != "[]" ] && [ "$models_json" != "null" ]; then
            while IFS='|' read -r model cost output cache_read cache_create; do
                [ -z "$model" ] && continue
                local_color="#cba6f7"  # purple for opus
                echo "$model" | grep -qi "haiku" && local_color="#94e2d5"
                echo "$model" | grep -qi "sonnet" && local_color="#89b4fa"
                tooltip+=$'\n'"  <span foreground='${local_color}'>  ${model}</span>  <span foreground='#f9e2af'>$(format_cost "$cost")</span>  <span foreground='${DIM}'>out:$(format_tokens "$output")  cache:$(format_tokens "$cache_read")</span>"
            done < <(echo "$models_json" | jq -r '.[]? | [.model, .cost, .output, .cache_read, .cache_create] | join("|")' 2>/dev/null)
        fi

        tooltip+=$'\n'"${SEP_THIN}"

        # ── Burn Rate
        if [ "$c_burn" != "0" ] && [ "$c_burn" != "null" ]; then
            burn_color="#a6e3a1"
            projected_f=$(printf "%.0f" "$c_projected" 2>/dev/null)
            [ "${projected_f:-0}" -ge 50 ] && burn_color="#f9e2af"
            [ "${projected_f:-0}" -ge 100 ] && burn_color="#f38ba8"
            tooltip+=$'\n'"<span foreground='${LABEL}'><b>Burn Rate</b></span>"
            tooltip+=$'\n'"  <span foreground='${DIM}'>Rate</span>        <span foreground='${burn_color}'><b>$(format_cost "$c_burn")</b>/hr</span>"
            tooltip+=$'\n'"  <span foreground='${DIM}'>Projected</span>   <span foreground='${burn_color}'>$(format_cost "$c_projected")/day</span>  <span foreground='${DIM}'>(${c_hours}h active)</span>"
        fi
    fi

    # ── Footer
    tooltip+=$'\n'"${SEP}"
    src_icon="●"
    src_color="#a6e3a1"
    [ "$c_source" = "cache" ] && src_color="#f9e2af"
    [ "$c_source" = "local" ] && src_color="#f38ba8"
    tooltip+=$'\n'"<span foreground='${src_color}'>${src_icon}</span> <span foreground='${DIM}'>$(date '+%H:%M')  via ${c_source}  |  click for TUI</span>"
fi

# ── CSS class ─────────────────────────────────────────────────────────────────

if [ "$max_pct" -ge 85 ]; then class="ai-crit"
elif [ "$max_pct" -ge 60 ]; then class="ai-warn"
else class="ai-ok"; fi

_is_light_theme && class="$class aifuel-light"

# ── Bar text ──────────────────────────────────────────────────────────────────

cost_short=$(awk "BEGIN { printf \"%.0f\", ${c_daily_cost:-0} }")
if $claude_ok && $_api_available; then
    # Show: utilization + depletion hint + cost
    depl_hint=""
    if [ -n "$c_depl_5h_mins" ] && [ "$c_depl_5h_mins" != "null" ]; then
        if [ "$c_depl_5h_mins" -lt 60 ] 2>/dev/null; then
            depl_hint=" ${c_depl_5h_mins}m"
        fi
    fi
    peak_dot=""
    [ "$c_peak_is" = "true" ] && peak_dot=" "
    text="󰧑 ${c5}%${depl_hint}${peak_dot} \$${cost_short}"
else
    text="󰧑 \$${cost_short} ${c_msgs:-0}msg"
fi

jq -n -c --arg text "$text" --arg tooltip "$tooltip" --arg class "$class" \
    '{"text": $text, "tooltip": $tooltip, "class": $class}'
