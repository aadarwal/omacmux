#!/bin/bash
# Right-click pane swap: first click marks, second click swaps
TARGET="$1"
SOURCE=$(tmux show -gv @swap_pane 2>/dev/null)
echo "$(date +%T) target=$TARGET source=$SOURCE" >> /tmp/pane-swap.log

if [ -n "$SOURCE" ]; then
  ERR=$(tmux swap-pane -s "$SOURCE" -t "$TARGET" 2>&1)
  echo "$(date +%T) swap result=$? err=$ERR" >> /tmp/pane-swap.log
  tmux set -gu @swap_pane
  tmux select-pane -M 2>/dev/null
else
  tmux set -g @swap_pane "$TARGET"
  tmux select-pane -t "$TARGET" -m
fi
