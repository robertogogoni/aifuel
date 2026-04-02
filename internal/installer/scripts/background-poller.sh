#!/bin/bash
# background-poller.sh — Keeps aifuel data fresh in the background
#
# Runs as a systemd user service. Polls usage data every 2 minutes.
# Priority: Chrome extension live feed > cookie-based fetch > local data
#
# The Chrome extension writes to claude-usage-live.json when Chrome is running.
# This poller ensures data stays fresh even when Chrome is closed by
# falling back to cookie-based fetch.

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

CACHE_DIR="$AIFUEL_CACHE_DIR"
LIVE_FEED="$CACHE_DIR/claude-usage-live.json"
OVERLAY="$SCRIPT_DIR/aifuel-claude.sh"
LOG="$AIFUEL_LOG_FILE"
POLL_INTERVAL=120  # 2 minutes

_log() {
    printf '[%s] [%-5s] [poller] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >> "$LOG" 2>/dev/null
}

_ensure_dirs
_log INFO "background poller started (interval: ${POLL_INTERVAL}s)"

while true; do
    # Check if Chrome extension is keeping the feed fresh
    if [ -f "$LIVE_FEED" ]; then
        feed_age=$(( $(date +%s) - $(stat -c %Y "$LIVE_FEED") ))
        if [ "$feed_age" -lt 300 ]; then
            # Extension is active, no need to poll
            sleep "$POLL_INTERVAL"
            continue
        fi
    fi

    # Chrome extension not active. Run the overlay to refresh data.
    _log INFO "extension inactive, refreshing via overlay..."
    "$OVERLAY" > /dev/null 2>/dev/null

    sleep "$POLL_INTERVAL"
done
