#!/bin/bash
set -euo pipefail

# Legacy upgrade script — delegates to bin/anu upgrade.

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"

exec "$REPO_DIR/bin/anu" upgrade
