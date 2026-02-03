#!/bin/bash
# Convenience script to clear Go build/module caches.
# Safe to run on Linux/macOS and Windows Git Bash.

set -e

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools Go Cache Cleaner"
echo "════════════════════════════════════════════════════════════════"
echo ""

if ! command -v go >/dev/null 2>&1; then
    echo "⚠️  Go is not installed or not in PATH; skipping go clean."
else
    echo "🧹 Running: go clean -cache -modcache -testcache"
    go clean -cache -modcache -testcache || true
    echo "✓ Go clean complete"
fi

OS="$(uname -s)"
case "$OS" in
    CYGWIN*|MINGW*|MSYS*)
        # Windows paths under Git Bash
        CACHE_DIR="${LOCALAPPDATA:-$APPDATA}/go-build"
        ;;
    *)
        CACHE_DIR="${GOCACHE:-$HOME/.cache/go-build}"
        ;;
esac

if [ -n "$CACHE_DIR" ] && [ -d "$CACHE_DIR" ]; then
    echo "🗑️  Removing build cache dir: $CACHE_DIR"
    rm -rf "$CACHE_DIR" || sudo rm -rf "$CACHE_DIR" || true
else
    echo "ℹ️  No cache directory found at $CACHE_DIR (nothing to remove)."
fi

echo ""
echo "✅ Done. Re-run ./scripts/linux/build.sh to rebuild VideoTools."
