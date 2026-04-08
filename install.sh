#!/bin/bash
set -euo pipefail

# Legacy install script — delegates to bin/omacmux init.
# For new installs, prefer:  brew install aadarwal/omacmux/omacmux && omacmux init

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

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

# 3. Run omacmux init (handles deps, linking, git identity, shell prompt)
exec "$REPO_DIR/bin/omacmux" init --replace-all
