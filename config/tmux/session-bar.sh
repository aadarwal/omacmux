#!/usr/bin/env bash
# Generates per-session tmux status bars so each client sees its own session highlighted.
# Called by hooks on session events and on config reload.

# Collect base session names (exclude _stash and grouped ~sessions)
base_sessions=()
while IFS= read -r s; do
  base_sessions+=("$s")
done < <(tmux list-sessions -F '#S' 2>/dev/null | grep -v '^_stash$' | grep -v '~' | sort)

(( ${#base_sessions[@]} == 0 )) && exit 0

# Build a default format (no session highlighted) and set it globally
# so new sessions always have a visible session bar.
default_format=""
i=0
for session in "${base_sessions[@]}"; do
  letter=$(printf "\\$(printf '%03o' $((65 + i)))")
  default_format+="#[fg=#8a8a8d] ${letter}:${session} "
  ((i++))
done
tmux set -g status-format[0] "$default_format" 2>/dev/null

# For each live session (base + grouped), set a format highlighting its base name
while IFS=$'\t' read -r sid s; do
  [[ $s == _stash ]] && continue
  base="${s%%~*}"

  format=""
  i=0
  for session in "${base_sessions[@]}"; do
    letter=$(printf "\\$(printf '%03o' $((65 + i)))")
    if [[ $session == "$base" ]]; then
      format+="#[fg=#121212,bg=#e68e0d,bold] ${letter}:${session} #[bg=default]"
    else
      format+="#[fg=#8a8a8d] ${letter}:${session} "
    fi
    ((i++))
  done

  tmux set -t "$sid" status-format[0] "$format" 2>/dev/null
done < <(tmux list-sessions -F '#{session_id}'$'\t''#{session_name}' 2>/dev/null | grep -v $'\t''_stash$')
