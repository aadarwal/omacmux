#!/bin/bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$HOME/.local/share/omacmux"
BREW_BASH="/opt/homebrew/bin/bash"

echo "==> omacmux upgrade"
echo ""

# 1. Verify omacmux is installed
if [[ ! -L "$INSTALL_DIR" ]]; then
  echo "ERROR: omacmux is not installed. Run ./install.sh first."
  exit 1
fi

# 2. Pull latest code
echo "==> Pulling latest changes..."
BEFORE=$(git -C "$REPO_DIR" rev-parse HEAD)
git -C "$REPO_DIR" pull --rebase
AFTER=$(git -C "$REPO_DIR" rev-parse HEAD)

# 3. Install any new Homebrew dependencies
echo "==> Updating Homebrew dependencies..."
brew bundle --file="$REPO_DIR/Brewfile"

# 4. Add Homebrew bash to /etc/shells (idempotent)
if [[ -f "$BREW_BASH" ]] && ! grep -q "$BREW_BASH" /etc/shells 2>/dev/null; then
  echo "==> Adding Homebrew bash to /etc/shells (requires sudo)..."
  echo "$BREW_BASH" | sudo tee -a /etc/shells > /dev/null
fi

# 5. Sync symlinks from manifest
source "$REPO_DIR/links.sh"

echo "==> Syncing config symlinks..."
for entry in "${OMACMUX_LINKS[@]}"; do
  src="$REPO_DIR/${entry%%:*}"
  dest="${entry#*:}"

  if [[ -L "$dest" ]]; then
    current_target=$(readlink "$dest")
    if [[ "$current_target" != "$src" ]]; then
      ln -sf "$src" "$dest"
      echo "    updated $dest"
    fi
  elif [[ -e "$dest" ]]; then
    mv "$dest" "${dest}.bak.$(date +%s)"
    mkdir -p "$(dirname "$dest")"
    ln -s "$src" "$dest"
    echo "    backed up + linked $dest (new)"
  else
    mkdir -p "$(dirname "$dest")"
    ln -s "$src" "$dest"
    echo "    linked $dest (new)"
  fi
done

# 6. Report what changed
echo ""
if [[ "$BEFORE" == "$AFTER" ]]; then
  echo "==> Already up to date."
else
  echo "==> Updated from ${BEFORE:0:7} to ${AFTER:0:7}"
  echo ""
  git -C "$REPO_DIR" log --oneline "${BEFORE}..${AFTER}"
fi

echo ""
echo "==> omacmux upgrade complete!"
