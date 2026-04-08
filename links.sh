# Symlink manifest — sourced by install.sh, uninstall.sh, upgrade.sh, bin/omacmux
# REPO_DIR must be set before sourcing this file.
#
# Format: "strategy:source_relative_path:destination_absolute_path"
#
# Strategies:
#   merge    — shell rc files: append a 'source' line instead of replacing
#   replace  — full symlink: backup existing file, then symlink ours
#   additive — targets unlikely to exist already (bins, new configs)

OMACMUX_LINKS=(
  "replace:config/ghostty/config:$HOME/.config/ghostty/config"
  "replace:config/tmux/tmux.conf:$HOME/.config/tmux/tmux.conf"
  "additive:config/tmux/session-bar.sh:$HOME/.config/tmux/session-bar.sh"
  "replace:config/nvim:$HOME/.config/nvim"
  "replace:config/starship.toml:$HOME/.config/starship.toml"
  "replace:config/git/config:$HOME/.config/git/config"
  "replace:config/claude/settings.json:$HOME/.claude/settings.json"
  "merge:shell/bashrc:$HOME/.bashrc"
  "merge:shell/bash_profile:$HOME/.bash_profile"
  "additive:config/bash/inputrc:$HOME/.inputrc"
  "additive:shell/.hushlogin:$HOME/.hushlogin"
  "additive:config/bash/bin/swarm:$HOME/.local/bin/swarm"
)

# Helper: parse a link entry into its components
# Usage: omacmux_parse_link "$entry"; echo "$_strategy $_source $_dest"
omacmux_parse_link() {
  local entry="$1"
  _strategy="${entry%%:*}"
  local rest="${entry#*:}"
  _source="${rest%%:*}"
  _dest="${rest#*:}"
}
