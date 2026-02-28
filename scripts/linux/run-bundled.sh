#!/bin/bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPS_DIR="$BASE_DIR/deps"

if [ -d "$DEPS_DIR/bin" ]; then
  export PATH="$DEPS_DIR/bin:$PATH"
fi

if [ -d "$DEPS_DIR/lib" ]; then
  export LD_LIBRARY_PATH="$DEPS_DIR/lib:${LD_LIBRARY_PATH:-}"
fi

if [ -d "$DEPS_DIR/lib/gstreamer-1.0" ]; then
  export GST_PLUGIN_PATH="$DEPS_DIR/lib/gstreamer-1.0"
  export GST_PLUGIN_SYSTEM_PATH_1_0="$DEPS_DIR/lib/gstreamer-1.0"
fi

if [ -x "$DEPS_DIR/bin/gst-plugin-scanner" ]; then
  export GST_PLUGIN_SCANNER="$DEPS_DIR/bin/gst-plugin-scanner"
fi

if [ -d "$DEPS_DIR/lib/gio/modules" ]; then
  export GIO_EXTRA_MODULES="$DEPS_DIR/lib/gio/modules"
fi

if [ -d "$DEPS_DIR/tessdata" ]; then
  export TESSDATA_PREFIX="$DEPS_DIR/tessdata"
fi

exec "$BASE_DIR/VideoTools" "$@"
