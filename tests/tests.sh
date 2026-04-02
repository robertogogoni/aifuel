#!/bin/bash
# tests.sh — Test suite for aifuel module
#
# Tests derived from real bugs in competing projects:
#   - ccusage: token undercounting, cost miscalculation, session dedup
#   - Monitor: timezone issues, conflicting data, team plan
#   - CodexBar: implausible cost/token combos, peak hours
#   - General: API failures, stale data, empty states, race conditions
#
# Usage: bash ~/aifuel/tests/tests.sh

set -o pipefail

PASS=0
FAIL=0
SKIP=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
DIM='\033[2m'
BOLD='\033[1m'
R='\033[0m'

_test() {
    local name="$1" result="$2" expected="$3"
    if [ "$result" = "$expected" ]; then
        echo -e "  ${GREEN}PASS${R}  $name"
        ((PASS++))
    else
        echo -e "  ${RED}FAIL${R}  $name"
        echo -e "        ${DIM}expected: ${expected}${R}"
        echo -e "        ${DIM}     got: ${result}${R}"
        ((FAIL++))
    fi
}

_test_not_empty() {
    local name="$1" result="$2"
    if [ -n "$result" ] && [ "$result" != "null" ] && [ "$result" != "0" ]; then
        echo -e "  ${GREEN}PASS${R}  $name ${DIM}(${result})${R}"
        ((PASS++))
    else
        echo -e "  ${RED}FAIL${R}  $name ${DIM}(empty/null/zero)${R}"
        ((FAIL++))
    fi
}

_test_numeric() {
    local name="$1" result="$2" min="$3" max="$4"
    local int_result="${result%.*}"
    if [ "$int_result" -ge "$min" ] 2>/dev/null && [ "$int_result" -le "$max" ] 2>/dev/null; then
        echo -e "  ${GREEN}PASS${R}  $name ${DIM}(${result}, range ${min}-${max})${R}"
        ((PASS++))
    else
        echo -e "  ${RED}FAIL${R}  $name ${DIM}(${result} not in ${min}-${max})${R}"
        ((FAIL++))
    fi
}

_skip() {
    local name="$1" reason="$2"
    echo -e "  ${YELLOW}SKIP${R}  $name ${DIM}(${reason})${R}"
    ((SKIP++))
}

OVERLAY="$HOME/.local/lib/aifuel/aifuel-claude-overlay.sh"
MODULE="$HOME/.local/lib/aifuel/aifuel.sh"
ANALYTICS="$HOME/.local/lib/aifuel/analytics.sh"
COOKIE_FETCHER="$HOME/.local/lib/aifuel/fetch-usage-via-cookies.sh"
LIVE_FEED="$HOME/.cache/aifuel/cache/claude-usage-live.json"
CACHE="$HOME/.cache/aifuel/cache/aifuel-cache-claude.json"

echo -e "${BOLD}AIFuel Test Suite${R}"
echo -e "${DIM}Testing against bugs from ccusage, Monitor, CodexBar, waybar-ai-usage${R}"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}1. Cost Calculation Accuracy${R} ${DIM}(ccusage #898, #899)${R}"
echo -e "${DIM}   Bug: inflated costs from wrong pricing tiers${R}"

# Our cost comes from ccusage itself, so test that our overlay passes it through correctly
rm -f "$CACHE" 2>/dev/null
overlay_data=$("$OVERLAY" 2>/dev/null)
daily_cost=$(echo "$overlay_data" | jq -r '.daily_cost // 0')
_test_numeric "daily cost is plausible (>0, <500)" "$daily_cost" 1 500

# Test per-model cost adds up to total
model_sum=$(echo "$overlay_data" | jq -r '[.models[].cost] | add // 0')
total_check=$(awk "BEGIN { diff = $daily_cost - $model_sum; print (diff < 1 && diff > -1) ? \"match\" : \"mismatch\" }")
_test "model costs sum to total (within \$1)" "$total_check" "match"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}2. Token Counting${R} ${DIM}(ccusage #901: streaming chunk dedup)${R}"
echo -e "${DIM}   Bug: output tokens undercounted from first-write-wins dedup${R}"

# We read the FINAL usage record per message (jq selects .message.usage)
# which already has the final count. Verify it's non-zero.
session_msgs=$(echo "$overlay_data" | jq -r '.session_messages // 0')
output_tokens=$(echo "$overlay_data" | jq -r '.total_output_tokens // 0')
_test_numeric "session messages > 0" "$session_msgs" 1 10000
_test_numeric "output tokens > 0" "$output_tokens" 1 10000000

# Test output tokens per message is reasonable (50-2000 avg for Opus)
if [ "$session_msgs" -gt 0 ]; then
    avg_out=$(( output_tokens / session_msgs ))
    _test_numeric "avg output/msg plausible (10-5000)" "$avg_out" 10 5000
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}3. Rate Limit Data Integrity${R} ${DIM}(Monitor #185: conflicting data)${R}"
echo -e "${DIM}   Bug: dashboard shows 74% but monitor shows 9-36%${R}"

five_hour=$(echo "$overlay_data" | jq -r '.five_hour // -1')
seven_day=$(echo "$overlay_data" | jq -r '.seven_day // -1')
_test_numeric "5h utilization in range 0-100" "$five_hour" 0 100
_test_numeric "7d utilization in range 0-100" "$seven_day" 0 100

# Data source should be consistent
source=$(echo "$overlay_data" | jq -r '.data_source // ""')
_test_not_empty "data source is set" "$source"

# If live feed exists, verify it matches overlay
if [ -f "$LIVE_FEED" ]; then
    live_5h=$(jq -r '.five_hour.utilization // -1' "$LIVE_FEED")
    _test "live feed matches overlay 5h" "$five_hour" "$live_5h"
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}4. Timezone Handling${R} ${DIM}(Monitor #188, #189: Windows TZ bug)${R}"
echo -e "${DIM}   Bug: timezone defaults to UTC, peak hours miscalculated${R}"

# Our analytics use TZ=America/Los_Angeles explicitly for peak detection
source "$ANALYTICS"
peak_json=$(compute_peak_status)
peak_label=$(echo "$peak_json" | jq -r '.label')
_test_not_empty "peak status computed" "$peak_label"

# Verify peak detection matches actual PT time
pt_hour=$(TZ=America/Los_Angeles date +%H)
pt_dow=$(TZ=America/Los_Angeles date +%u)
expected_peak="false"
[ "$pt_dow" -le 5 ] && [ "$pt_hour" -ge 5 ] && [ "$pt_hour" -lt 11 ] && expected_peak="true"
actual_peak=$(echo "$peak_json" | jq -r '.is_peak')
_test "peak detection correct for $(TZ=America/Los_Angeles date '+%H:%M %a PT')" "$actual_peak" "$expected_peak"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}5. Implausible Data Detection${R} ${DIM}(CodexBar #602: impossible cost/token)${R}"
echo -e "${DIM}   Bug: \$0.08 for 2.3B tokens${R}"

# Verify cost-per-token ratio is plausible
if [ "$output_tokens" -gt 0 ]; then
    # Opus: ~$15/M output tokens. Ratio should be $0.001-$0.1 per 1K tokens
    cost_per_k=$(awk "BEGIN { printf \"%.4f\", ($daily_cost / $output_tokens) * 1000 }")
    _test_numeric "cost per 1K output tokens plausible (range 0.01-100)" "${cost_per_k%.*}" 0 100
fi

# Check prompt log for implausible entries
prompts=$(echo "$overlay_data" | jq -r '.analytics.recent_prompts // []')
prompt_count=$(echo "$prompts" | jq 'length')
if [ "$prompt_count" -gt 0 ]; then
    # Each prompt's cost should be < $5 for a single turn
    max_prompt_cost=$(echo "$prompts" | jq '[.[].cost] | max')
    _test_numeric "max single prompt cost < \$5" "${max_prompt_cost%.*}" 0 5
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}6. Peak Hours Integration${R} ${DIM}(CodexBar #609, #611: peak hours feature)${R}"
echo -e "${DIM}   Feature request: show peak/off-peak status${R}"

analytics=$(echo "$overlay_data" | jq '.analytics // empty')
_test_not_empty "analytics.peak exists" "$(echo "$analytics" | jq -r '.peak.label // ""')"
_test_not_empty "analytics.peak.multiplier exists" "$(echo "$analytics" | jq -r '.peak.multiplier // ""')"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}7. Depletion Calculation${R} ${DIM}(derived from Monitor P90 predictions)${R}"

depletion=$(echo "$analytics" | jq '.depletion_5h // empty')
depl_status=$(echo "$depletion" | jq -r '.status // ""')
_test_not_empty "depletion status computed" "$depl_status"

# If there's a depletion time, it should be > 0 and < 300 min (5h window)
depl_mins=$(echo "$depletion" | jq -r '.depletes_in_minutes // "null"')
if [ "$depl_mins" != "null" ]; then
    _test_numeric "depletion minutes plausible (1-300)" "$depl_mins" 1 300
fi

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}8. Empty State Handling${R} ${DIM}(common failure: no data = crash)${R}"

# Test with no live feed, no cache, no cookie jar
echo -e "${DIM}   Simulating: no live feed, no cache...${R}"
_backup_live=$(mktemp)
_backup_cache=$(mktemp)
cp "$LIVE_FEED" "$_backup_live" 2>/dev/null
cp "$CACHE" "$_backup_cache" 2>/dev/null
rm -f "$LIVE_FEED" "$CACHE" 2>/dev/null

empty_output=$("$OVERLAY" 2>/dev/null)
empty_exit=$?
empty_source=$(echo "$empty_output" | jq -r '.data_source // "MISSING"')
empty_has_json=$(echo "$empty_output" | jq -e '.' &>/dev/null && echo "valid" || echo "invalid")

_test "overlay produces valid JSON with no live feed" "$empty_has_json" "valid"
_test_not_empty "overlay returns data source on empty state" "$empty_source"
_test "overlay exits 0 on empty state" "$empty_exit" "0"

# Restore
cp "$_backup_live" "$LIVE_FEED" 2>/dev/null
cp "$_backup_cache" "$CACHE" 2>/dev/null
rm -f "$_backup_live" "$_backup_cache"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}9. Stale Data Handling${R} ${DIM}(general: old cache should still work)${R}"

# Touch cache to make it 5 minutes old
touch -d "5 minutes ago" "$CACHE" 2>/dev/null
stale_output=$("$OVERLAY" 2>/dev/null)
stale_json=$(echo "$stale_output" | jq -e '.' &>/dev/null && echo "valid" || echo "invalid")
_test "overlay produces valid JSON with stale cache" "$stale_json" "valid"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}10. Concurrent Access${R} ${DIM}(multi-monitor waybar instances)${R}"

# Simulate two concurrent overlay runs
rm -f "$CACHE" 2>/dev/null
"$OVERLAY" > /tmp/aifuel-test-1.json 2>/dev/null &
pid1=$!
"$OVERLAY" > /tmp/aifuel-test-2.json 2>/dev/null &
pid2=$!
wait $pid1 $pid2

if jq -e '.data_source' /tmp/aifuel-test-1.json >/dev/null 2>&1; then out1="valid"; else out1="invalid"; fi
if jq -e '.data_source' /tmp/aifuel-test-2.json >/dev/null 2>&1; then out2="valid"; else out2="invalid"; fi
_test "concurrent run 1 produces valid JSON" "$out1" "valid"
_test "concurrent run 2 produces valid JSON" "$out2" "valid"
rm -f /tmp/aifuel-test-1.json /tmp/aifuel-test-2.json

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}11. Waybar JSON Output${R} ${DIM}(must have text + tooltip + class)${R}"

rm -f "$CACHE" 2>/dev/null
waybar_output=$("$MODULE" 2>/dev/null)
has_text=$(echo "$waybar_output" | jq -r '.text // "MISSING"')
has_tooltip=$(echo "$waybar_output" | jq -r '.tooltip // "MISSING"')
has_class=$(echo "$waybar_output" | jq -r '.class // "MISSING"')

_test_not_empty "waybar output has .text" "$has_text"
_test_not_empty "waybar output has .tooltip" "$has_tooltip"
_test_not_empty "waybar output has .class" "$has_class"

# Verify no unescaped Pango markup breaks JSON
json_valid=$(echo "$waybar_output" | jq -e '.' &>/dev/null && echo "valid" || echo "invalid")
_test "waybar output is valid JSON (Pango doesn't break it)" "$json_valid" "valid"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}12. Analytics Engine Edge Cases${R}"

source "$ANALYTICS"

# Test depletion at 0%
d0=$(compute_depletion 0 "" 0 1)
_test "depletion at 0% returns idle" "$(echo "$d0" | jq -r '.status')" "idle"

# Test depletion at 100%
d100=$(compute_depletion 100 "" 0 1)
_test "depletion at 100% returns depleted" "$(echo "$d100" | jq -r '.status')" "depleted"

# Test depletion at 50% with 2h remaining
d50=$(compute_depletion 50 "$(date -d '+2 hours' -Iseconds)" 1 1)
d50_status=$(echo "$d50" | jq -r '.status')
_test_not_empty "depletion at 50% returns status" "$d50_status"

# Test binding limit with various combos
b1=$(compute_binding_limit 80 20 10)
_test "binding picks highest: 5h at 80%" "$(echo "$b1" | jq -r '.binding_limit')" "five_hour"
b2=$(compute_binding_limit 20 80 10)
_test "binding picks highest: 7d at 80%" "$(echo "$b2" | jq -r '.binding_limit')" "seven_day"
b3=$(compute_binding_limit 20 10 90)
_test "binding picks highest: sonnet at 90%" "$(echo "$b3" | jq -r '.binding_limit')" "sonnet"

# Test prompt log with empty JSONL
empty_prompts=$(compute_prompt_log "/nonexistent/file.jsonl" 5)
_test "prompt log with missing file returns []" "$empty_prompts" "[]"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}13. Package Update Survival${R}"

# Verify the forwarder is in place
forwarder_ok=$(grep -c "overlay" ~/.local/lib/aifuel/aifuel-claude.sh 2>/dev/null)
_test_numeric "claude forwarder references overlay" "$forwarder_ok" 1 10

# Verify patches exist
_test_not_empty "aifuel.sh.patch exists" "$(ls ~/.local/lib/aifuel/patches/aifuel.sh.patch 2>/dev/null)"
_test_not_empty "lib.sh.patch exists" "$(ls ~/.local/lib/aifuel/patches/lib.sh.patch 2>/dev/null)"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo -e "${BOLD}14. Error Log Cleanliness${R}"

error_count=$(grep -c '\[ERROR\]' ~/.cache/aifuel/aifuel.log 2>/dev/null)
warn_recent=$(tail -20 ~/.cache/aifuel/aifuel.log 2>/dev/null | grep -c '\[WARN \]')
_test "no errors in log" "$error_count" "0"
_test "no recent warnings (last 20 lines)" "$warn_recent" "0"

echo ""

# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo -e "${BOLD}Results: ${GREEN}${PASS} passed${R}, ${RED}${FAIL} failed${R}, ${YELLOW}${SKIP} skipped${R}"
echo ""

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
