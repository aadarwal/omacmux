#!/usr/bin/env bash
# Generates the tmux session bar for status-format[0]
# Called by hooks on session events and on config reload

current=$(tmux display-message -p '#S')
format=""
i=0

while IFS= read -r session; do
  letter=$(printf "\\$(printf '%03o' $((65 + i)))")
  if [ "$session" = "$current" ]; then
    format+="#[fg=black,bg=blue,bold] ${letter}:${session} #[bg=default]"
  else
    format+="#[fg=brightblack] ${letter}:${session} "
  fi
  i=$((i + 1))
done < <(tmux list-sessions -F '#S' 2>/dev/null | grep -v '^_stash$')

tmux set -g status-format[0] "$format"
