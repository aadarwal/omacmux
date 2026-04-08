#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"
echo "Building omacmux-remote..."
go build -o omacmux-remote .
echo "Starting on port ${PORT:-8384}..."
./omacmux-remote &
PID=$!
echo "$PID" > .pid
# Expose on tailnet
if command -v tailscale &>/dev/null; then
  tailscale serve --bg "${PORT:-8384}" 2>/dev/null || true
  HOSTNAME=$(tailscale status --self --json 2>/dev/null | jq -r '.Self.DNSName' 2>/dev/null | sed 's/\.$//' || echo "localhost")
  echo "Dashboard: https://$HOSTNAME"
else
  echo "Dashboard: http://localhost:${PORT:-8384}"
  echo "(install tailscale for remote access)"
fi
echo "PID: $PID"
