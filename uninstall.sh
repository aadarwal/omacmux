#!/bin/bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

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

source "$REPO_DIR/links.sh"

for entry in "${OMACMUX_LINKS[@]}"; do
  unlink_file "${entry#*:}"
done

# Remove the install symlink
if [[ -L "$HOME/.local/share/omacmux" ]]; then
  rm "$HOME/.local/share/omacmux"
  echo "    removed ~/.local/share/omacmux symlink"
fi

echo ""
echo "==> omacmux removed."
echo "    Homebrew packages were NOT uninstalled."
echo "    Run 'brew bundle cleanup --file=~/omacmux/Brewfile --force' to remove them."
