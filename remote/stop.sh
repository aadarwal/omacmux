#!/usr/bin/env bash
cd "$(dirname "$0")"
[ -f .pid ] && kill "$(cat .pid)" 2>/dev/null && rm .pid
tailscale serve off 2>/dev/null || true
echo "stopped"
