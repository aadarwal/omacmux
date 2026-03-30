#!/usr/bin/env bash

# Visual notification (always)
osascript -e "display notification \"${CLAUDE_NOTIFICATION}\" with title \"Claude Code\""

# Voice notification (if voice mode active)
_voice_state="$HOME/.local/share/omacmux/voice/state.json"
if [[ -f "$_voice_state" ]]; then
  _enabled=$(jq -r '.enabled // false' "$_voice_state" 2>/dev/null)
  if [[ "$_enabled" == "true" ]]; then
    say -r 200 "${CLAUDE_NOTIFICATION}" &>/dev/null &
  fi
fi
