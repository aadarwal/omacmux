#!/bin/bash
set -euo pipefail

echo "==> Removing omacmux..."

unlink_file() {
  local path="$1"
  if [[ -L "$path" ]]; then
    rm "$path"
    # Restore most recent backup if one exists
    local backup
    backup=$(ls -t "${path}.bak."* 2>/dev/null | head -1) || true
    if [[ -n "$backup" ]]; then
      mv "$backup" "$path"
      echo "    restored $path from backup"
    else
      echo "    removed $path"
    fi
  else
    echo "    skipped $path (not a symlink)"
  fi
}

unlink_file "$HOME/.config/ghostty/config"
unlink_file "$HOME/.config/tmux/tmux.conf"
unlink_file "$HOME/.config/nvim"
unlink_file "$HOME/.config/starship.toml"
unlink_file "$HOME/.config/git/config"
unlink_file "$HOME/.bashrc"
unlink_file "$HOME/.bash_profile"

# Remove the install symlink
if [[ -L "$HOME/.local/share/omacmux" ]]; then
  rm "$HOME/.local/share/omacmux"
  echo "    removed ~/.local/share/omacmux symlink"
fi

echo ""
echo "==> omacmux removed."
echo "    Installed packages were NOT uninstalled."
if [[ "$(uname)" == "Darwin" ]]; then
  echo "    Run 'brew bundle cleanup --file=~/omacmux/Brewfile --force' to remove them."
else
  echo "    Remove tmux, neovim, and other packages via your system package manager."
fi
