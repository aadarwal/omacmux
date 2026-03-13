#!/bin/bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$HOME/.local/share/omacmux"

echo "==> omacmux setup"
echo ""

# ---------------------------------------------------------------------------
# 1. Detect OS and environment
# ---------------------------------------------------------------------------
OS="$(uname -s)"
IS_WSL=false
DISTRO=""
PKG_MANAGER=""

case "$OS" in
  Darwin)
    echo "    Detected: macOS"
    PKG_MANAGER="brew"
    ;;
  Linux)
    # Check for WSL
    if [[ -f /proc/version ]] && grep -qi microsoft /proc/version; then
      IS_WSL=true
      echo "    Detected: Linux (WSL)"
    else
      echo "    Detected: Linux"
    fi

    # Detect distro and package manager
    if command -v apt &> /dev/null; then
      PKG_MANAGER="apt"
      DISTRO="debian"
      echo "    Package manager: apt (Debian/Ubuntu)"
    elif command -v dnf &> /dev/null; then
      PKG_MANAGER="dnf"
      DISTRO="fedora"
      echo "    Package manager: dnf (Fedora/RHEL)"
    elif command -v pacman &> /dev/null; then
      PKG_MANAGER="pacman"
      DISTRO="arch"
      echo "    Package manager: pacman (Arch)"
    else
      echo "ERROR: No supported package manager found (need apt, dnf, or pacman)."
      exit 1
    fi
    ;;
  *)
    echo "ERROR: Unsupported OS: $OS"
    exit 1
    ;;
esac

echo ""

# ---------------------------------------------------------------------------
# 2. Install dependencies
# ---------------------------------------------------------------------------

install_with_brew() {
  # Install Homebrew if missing
  if ! command -v brew &> /dev/null; then
    echo "==> Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    eval "$(/opt/homebrew/bin/brew shellenv)"
  fi

  echo "==> Installing dependencies from Brewfile..."
  brew bundle --file="$REPO_DIR/Brewfile"
}

install_starship() {
  if ! command -v starship &> /dev/null; then
    echo "    Installing starship..."
    curl -sS https://starship.rs/install.sh | sh -s -- -y
  else
    echo "    starship already installed"
  fi
}

install_zoxide() {
  if ! command -v zoxide &> /dev/null; then
    echo "    Installing zoxide..."
    curl -sSfL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install/install.sh | sh
  else
    echo "    zoxide already installed"
  fi
}

install_mise() {
  if ! command -v mise &> /dev/null; then
    echo "    Installing mise..."
    curl https://mise.run | sh
  else
    echo "    mise already installed"
  fi
}

install_eza_from_source() {
  if ! command -v eza &> /dev/null; then
    if command -v cargo &> /dev/null; then
      echo "    Installing eza via cargo..."
      cargo install eza
    else
      echo "    WARNING: eza requires cargo (Rust). Install Rust first, then run: cargo install eza"
    fi
  else
    echo "    eza already installed"
  fi
}

install_gh_linux() {
  if ! command -v gh &> /dev/null; then
    echo "    Installing GitHub CLI..."
    case "$PKG_MANAGER" in
      apt)
        (type -p wget >/dev/null || sudo apt install -y wget) \
          && sudo mkdir -p -m 755 /etc/apt/keyrings \
          && out=$(mktemp) \
          && wget -nv -O"$out" https://cli.github.com/packages/githubcli-archive-keyring.gpg \
          && cat "$out" | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
          && sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
          && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
          && sudo apt update \
          && sudo apt install -y gh
        ;;
      dnf)
        sudo dnf install -y 'dnf-command(config-manager)' || true
        sudo dnf config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo
        sudo dnf install -y gh
        ;;
      pacman)
        sudo pacman -S --needed --noconfirm github-cli
        ;;
    esac
  else
    echo "    gh already installed"
  fi
}

install_nerd_font_linux() {
  local FONT_DIR="$HOME/.local/share/fonts"
  if ls "$FONT_DIR"/JetBrains*.ttf &> /dev/null; then
    echo "    JetBrainsMono Nerd Font already installed"
    return
  fi

  echo "    Installing JetBrainsMono Nerd Font..."
  mkdir -p "$FONT_DIR"
  local FONT_URL="https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.tar.xz"
  curl -fsSL "$FONT_URL" | tar -xJ -C "$FONT_DIR"
  fc-cache -f "$FONT_DIR"
  echo "    Font installed to $FONT_DIR"
}

install_with_apt() {
  echo "==> Installing dependencies via apt..."
  sudo apt update
  sudo apt install -y \
    tmux \
    neovim \
    bash \
    fzf \
    bat \
    ripgrep \
    fd-find \
    jq \
    tree

  # bat is installed as batcat on Debian/Ubuntu - create symlink
  if command -v batcat &> /dev/null && ! command -v bat &> /dev/null; then
    echo "    Creating symlink: bat -> batcat"
    mkdir -p "$HOME/.local/bin"
    ln -sf "$(command -v batcat)" "$HOME/.local/bin/bat"
  fi

  # fd is installed as fdfind on Debian/Ubuntu - create symlink
  if command -v fdfind &> /dev/null && ! command -v fd &> /dev/null; then
    echo "    Creating symlink: fd -> fdfind"
    mkdir -p "$HOME/.local/bin"
    ln -sf "$(command -v fdfind)" "$HOME/.local/bin/fd"
  fi

  echo ""
  echo "==> Installing tools not available in apt repos..."

  # eza has an official apt repo for Ubuntu 20.04+
  if ! command -v eza &> /dev/null; then
    echo "    Installing eza from official repo..."
    sudo mkdir -p /etc/apt/keyrings
    wget -qO- https://raw.githubusercontent.com/eza-community/eza/main/deb.asc | sudo gpg --dearmor -o /etc/apt/keyrings/gierens.gpg 2>/dev/null || true
    echo "deb [signed-by=/etc/apt/keyrings/gierens.gpg] http://deb.gierens.de stable main" | sudo tee /etc/apt/sources.list.d/gierens.list > /dev/null
    sudo chmod 644 /etc/apt/keyrings/gierens.gpg /etc/apt/sources.list.d/gierens.list
    sudo apt update
    sudo apt install -y eza || install_eza_from_source
  else
    echo "    eza already installed"
  fi

  install_starship
  install_zoxide
  install_mise
  install_gh_linux
  install_nerd_font_linux
}

install_with_dnf() {
  echo "==> Installing dependencies via dnf..."
  sudo dnf install -y \
    tmux \
    neovim \
    bash \
    fzf \
    bat \
    ripgrep \
    fd-find \
    jq \
    tree

  # eza may be available in Fedora repos
  if ! command -v eza &> /dev/null; then
    echo "    Installing eza..."
    sudo dnf install -y eza 2>/dev/null || install_eza_from_source
  else
    echo "    eza already installed"
  fi

  echo ""
  echo "==> Installing tools via official install scripts..."
  install_starship
  install_zoxide
  install_mise
  install_gh_linux
  install_nerd_font_linux
}

install_with_pacman() {
  echo "==> Installing dependencies via pacman..."
  sudo pacman -S --needed --noconfirm \
    tmux \
    neovim \
    bash \
    fzf \
    bat \
    ripgrep \
    fd \
    jq \
    tree \
    eza \
    zoxide \
    starship

  echo ""
  echo "==> Installing tools via official install scripts..."
  install_mise
  install_gh_linux
  install_nerd_font_linux
}

case "$PKG_MANAGER" in
  brew)   install_with_brew   ;;
  apt)    install_with_apt    ;;
  dnf)    install_with_dnf    ;;
  pacman) install_with_pacman ;;
esac

# ---------------------------------------------------------------------------
# 3. Add Homebrew bash to /etc/shells (macOS only)
# ---------------------------------------------------------------------------
BREW_BASH="/opt/homebrew/bin/bash"

if [[ "$OS" == "Darwin" && -f "$BREW_BASH" ]] && ! grep -q "$BREW_BASH" /etc/shells 2>/dev/null; then
  echo ""
  echo "==> Adding Homebrew bash to /etc/shells (requires sudo)..."
  echo "$BREW_BASH" | sudo tee -a /etc/shells > /dev/null
fi

# ---------------------------------------------------------------------------
# 4. Symlink repo to ~/.local/share/omacmux
# ---------------------------------------------------------------------------
echo ""
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

# ---------------------------------------------------------------------------
# 5. Link config files
# ---------------------------------------------------------------------------
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

echo "==> Linking config files..."
link_file "$REPO_DIR/config/ghostty/config" "$HOME/.config/ghostty/config"
link_file "$REPO_DIR/config/tmux/tmux.conf" "$HOME/.config/tmux/tmux.conf"
link_file "$REPO_DIR/config/nvim"           "$HOME/.config/nvim"
link_file "$REPO_DIR/config/starship.toml"  "$HOME/.config/starship.toml"
link_file "$REPO_DIR/config/git/config"     "$HOME/.config/git/config"
link_file "$REPO_DIR/shell/bashrc"          "$HOME/.bashrc"
link_file "$REPO_DIR/shell/bash_profile"    "$HOME/.bash_profile"

# ---------------------------------------------------------------------------
# 6. Set up git identity
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# 7. Shell setup
# ---------------------------------------------------------------------------
if [[ "$OS" == "Darwin" ]]; then
  # macOS: offer to set Homebrew bash as default shell
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
else
  # Linux: bash 5.x is typically the default; just verify the version
  BASH_MAJOR="${BASH_VERSINFO[0]}"
  BASH_MINOR="${BASH_VERSINFO[1]}"
  if [[ "$BASH_MAJOR" -lt 4 || ( "$BASH_MAJOR" -eq 4 && "$BASH_MINOR" -lt 3 ) ]]; then
    echo ""
    echo "    WARNING: bash $BASH_VERSION detected. omacmux requires bash 4.3+."
    echo "    Please update bash and re-run the installer."
  else
    echo ""
    echo "    bash $BASH_VERSION detected (OK)"
  fi
fi

# ---------------------------------------------------------------------------
# 8. Done
# ---------------------------------------------------------------------------
echo ""
echo "==> omacmux installed successfully!"
echo ""
echo "    Open a new terminal window, then:"
echo ""
echo "    t                  Start/attach tmux session"
echo "    tdl cx             Dev layout: nvim + claude + terminal"
echo "    tdl cx c           Dev layout: nvim + claude + opencode + terminal"
echo "    tdlm cx            One layout per subdirectory (monorepo)"
echo "    tsl 4 cx           4 tiled panes running claude (AI swarm)"
echo ""
echo "    nvim will auto-install plugins on first launch (~30-60s)."
echo ""
