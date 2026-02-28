#!/bin/bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPS_DIR="$BASE_DIR/deps"

if [ -d "$DEPS_DIR/bin" ]; then
  export PATH="$DEPS_DIR/bin:$PATH"
fi

if [ -d "$DEPS_DIR/tessdata" ]; then
  export TESSDATA_PREFIX="$DEPS_DIR/tessdata"
fi

exec "$BASE_DIR/VideoTools" "$@"
