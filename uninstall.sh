#!/bin/bash
set -euo pipefail

# Legacy uninstall script — delegates to bin/anu unlink.

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

exec "$REPO_DIR/bin/anu" unlink
