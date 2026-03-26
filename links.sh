# Symlink manifest — sourced by install.sh, uninstall.sh, upgrade.sh
# REPO_DIR must be set before sourcing this file.
# Format: "source_relative_path:destination_absolute_path"

OMACMUX_LINKS=(
  "config/ghostty/config:$HOME/.config/ghostty/config"
  "config/tmux/tmux.conf:$HOME/.config/tmux/tmux.conf"
  "config/tmux/session-bar.sh:$HOME/.config/tmux/session-bar.sh"
  "config/nvim:$HOME/.config/nvim"
  "config/starship.toml:$HOME/.config/starship.toml"
  "config/git/config:$HOME/.config/git/config"
  "config/claude/settings.json:$HOME/.claude/settings.json"
  "shell/bashrc:$HOME/.bashrc"
  "shell/bash_profile:$HOME/.bash_profile"
  "config/bash/inputrc:$HOME/.inputrc"
  "shell/.hushlogin:$HOME/.hushlogin"
)
