#!/bin/bash
# Native messaging host for aifuel Live Feed Chrome extension
# Receives usage data from the extension and writes it to a cache file.
#
# Chrome native messaging protocol: 4-byte length prefix (little-endian) + JSON payload

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

OUTPUT_FILE="$AIFUEL_CACHE_DIR/claude-usage-live.json"
LOG="$AIFUEL_LOG_FILE"

_ensure_dirs

_log() {
    printf '[%s] [%-5s] [native] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >> "$LOG" 2>/dev/null
}

# Read a native message (4-byte length prefix + JSON)
read_message() {
    local len_bytes
    len_bytes=$(head -c 4)
    [ -z "$len_bytes" ] && return 1

    # Convert 4 little-endian bytes to integer
    local len
    len=$(printf '%s' "$len_bytes" | od -An -td4 -N4 --endian=little 2>/dev/null | tr -d ' ')
    [ -z "$len" ] || [ "$len" -le 0 ] 2>/dev/null && return 1
    [ "$len" -gt 1048576 ] && return 1  # Sanity: max 1MB

    head -c "$len"
}

# Send a native response (4-byte length prefix + JSON)
send_message() {
    local msg="$1"
    local len=${#msg}
    printf "$(printf '\\x%02x\\x%02x\\x%02x\\x%02x' \
        $((len & 0xFF)) $(( (len >> 8) & 0xFF)) $(( (len >> 16) & 0xFF)) $(( (len >> 24) & 0xFF)))"
    printf '%s' "$msg"
}

# Main loop: read messages from Chrome extension
while true; do
    message=$(read_message) || break
    [ -z "$message" ] && break

    action=$(echo "$message" | jq -r '.action // ""' 2>/dev/null)

    case "$action" in
        write_usage)
            data=$(echo "$message" | jq -c '.data' 2>/dev/null)
            if [ -n "$data" ] && [ "$data" != "null" ]; then
                echo "$data" > "$OUTPUT_FILE"
                _log INFO "live feed: wrote usage data (5h=$(echo "$data" | jq -r '.five_hour.utilization // "?"')%)"
                send_message '{"status":"ok"}'
            else
                send_message '{"status":"error","message":"no data"}'
            fi
            ;;
        *)
            send_message '{"status":"error","message":"unknown action"}'
            ;;
    esac
done
