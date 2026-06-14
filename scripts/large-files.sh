#!/usr/bin/env bash
# Report Go source files in gui/ over 800 lines.
# Usage: ./scripts/large-files.sh
set -euo pipefail

THRESHOLD=800
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$ROOT"

echo "| Lines | File |"
echo "|------:|------|"
find gui -name '*.go' -not -name '*_test.go' \
  -exec wc -l {} \; \
  | awk -v thresh="$THRESHOLD" '$1 > thresh { printf "| %d | %s |\n", $1, $2 }' \
  | sort -rn

echo
echo "Threshold: >${THRESHOLD} lines (non-test Go source in gui/)"
