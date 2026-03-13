#!/bin/bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$HOME/.local/share/omacmux"
BREW_BASH="/opt/homebrew/bin/bash"

echo "==> omacmux setup"
echo ""

# 1. Check we're on macOS
if [[ "$(uname)" != "Darwin" ]]; then
  echo "ERROR: omacmux is for macOS only."
  exit 1
fi

# 2. Install Homebrew if missing
if ! command -v brew &> /dev/null; then
  echo "==> Installing Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  eval "$(/opt/homebrew/bin/brew shellenv)"
fi

# 3. Install dependencies
echo "==> Installing dependencies from Brewfile..."
brew bundle --file="$REPO_DIR/Brewfile"

# 4. Add Homebrew bash to allowed shells
if [[ -f "$BREW_BASH" ]] && ! grep -q "$BREW_BASH" /etc/shells 2>/dev/null; then
  echo "==> Adding Homebrew bash to /etc/shells (requires sudo)..."
  echo "$BREW_BASH" | sudo tee -a /etc/shells > /dev/null
fi

# 5. Symlink repo to ~/.local/share/omacmux
echo "==> Linking omacmux to $INSTALL_DIR..."
mkdir -p "$(dirname "$INSTALL_DIR")"
if [[ -L "$INSTALL_DIR" ]]; then
  ln -sf "$REPO_DIR" "$INSTALL_DIR"
elif [[ -e "$INSTALL_DIR" ]]; then
  mv "$INSTALL_DIR" "${INSTALL_DIR}.bak.$(date +%s)"
  ln -s "$REPO_DIR" "$INSTALL_DIR"
  echo "    backed up existing $INSTALL_DIR"
else
  ln -s "$REPO_DIR" "$INSTALL_DIR"
fi
echo "    linked $INSTALL_DIR -> $REPO_DIR"

# 6. Link config files
link_file() {
  local src="$1" dest="$2"
  mkdir -p "$(dirname "$dest")"
  if [[ -L "$dest" ]]; then
    ln -sf "$src" "$dest"
    echo "    updated $dest"
  elif [[ -e "$dest" ]]; then
    mv "$dest" "${dest}.bak.$(date +%s)"
    ln -s "$src" "$dest"
    echo "    backed up + linked $dest"
  else
    ln -s "$src" "$dest"
    echo "    linked $dest"
  fi
}

source "$REPO_DIR/links.sh"

echo "==> Linking config files..."
for entry in "${OMACMUX_LINKS[@]}"; do
  link_file "$REPO_DIR/${entry%%:*}" "${entry#*:}"
done

# 7. Set up git identity
echo ""
echo "==> Git identity setup"
GIT_CONFIG="$REPO_DIR/config/git/config"

if ! grep -q '^\[user\]' "$GIT_CONFIG" 2>/dev/null; then
  read -p "    Your name for git commits: " git_name
  read -p "    Your email for git commits: " git_email
  if [[ -n "$git_name" && -n "$git_email" ]]; then
    cat >> "$GIT_CONFIG" <<EOF
[user]
	email = $git_email
	name = $git_name
EOF
    echo "    Git identity saved."
  else
    echo "    Skipped (you can set this later with 'git config --global')"
  fi
else
  echo "    Git identity already configured."
fi

# 8. Optionally set Homebrew bash as default shell
if [[ -f "$BREW_BASH" && "$SHELL" != "$BREW_BASH" ]]; then
  echo ""
  read -p "==> Set Homebrew bash ($BREW_BASH) as default shell? [y/N] " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    chsh -s "$BREW_BASH"
    echo "    Default shell changed to $BREW_BASH"
  else
    echo "    Keeping current shell ($SHELL)"
    echo "    NOTE: tsl() requires bash 4.3+. Run 'chsh -s $BREW_BASH' later if needed."
  fi
fi

# 9. Done
echo ""
echo "==> omacmux installed successfully!"
echo ""
echo "    Open a new Ghostty window, then:"
echo ""
echo "    t                  Start/attach tmux session"
echo "    tdl cx             Dev layout: nvim + claude + terminal"
echo "    tdl cx c           Dev layout: nvim + claude + opencode + terminal"
echo "    tdlm cx            One layout per subdirectory (monorepo)"
echo "    tsl 4 cx           4 tiled panes running claude (AI swarm)"
echo ""
echo "    nvim will auto-install plugins on first launch (~30-60s)."
echo ""
