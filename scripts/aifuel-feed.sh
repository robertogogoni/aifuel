#!/bin/bash
# aifuel-feed.sh — Convenience wrapper for the background poller
# Can be run directly or via systemd: systemctl --user start aifuel-feed
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/background-poller.sh" "$@"
