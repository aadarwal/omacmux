#!/bin/bash
set -euo pipefail

# Legacy upgrade script — delegates to bin/omacmux upgrade.

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

exec "$REPO_DIR/bin/omacmux" upgrade
