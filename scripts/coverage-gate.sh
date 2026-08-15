#!/usr/bin/env bash
# coverage-gate.sh — enforce total and per-package coverage floors on a
# coverage.out profile. Shared by CI (`.github/workflows/ci.yml` coverage
# job) and `make coverage-gate`, so the thresholds live in exactly one
# place. Emits ::error:: annotations, exits non-zero on any failure.

set -euo pipefail

PROFILE="${1:?usage: coverage-gate.sh <coverage.out>}"

# Total coverage threshold: the sum over every package in the profile
# (CI generates the profile with `go test ./gui/...`).
go tool cover -func="$PROFILE" | awk 'END {
  gsub(/%/, "", $3);
  if ($3 + 0 < 70) {
    printf "::error::Coverage %.1f%% below 70%% threshold\n", $3;
    exit 1
  } else {
    printf "Coverage %.1f%% >= 70%% threshold\n", $3
  }
}'

# Per-package floors: packages listed below must not drop under their
# floor. Floors are advisory for backend internals that CI runs but the
# host display rarely exercises.
failures=0
while IFS='|' read -r pkg floor; do
  pct=$(go tool cover -func="$PROFILE" | \
    awk -v pkg="$pkg" '
      $1 ~ "^"pkg"/" { stmts += $2; if ($3 > 0) cov += $2 }
      END { if (stmts > 0) printf "%.1f", 100 * cov / stmts;
            else print "0.0" }')
  if [ -n "$pct" ] && awk "BEGIN { exit ($pct < $floor) ? 0 : 1 }"; then
    echo "::error::$pkg coverage ${pct}% below ${floor}% floor"
    failures=$((failures + 1))
  fi
done << 'EOF'
github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv|0
github.com/go-gui-org/go-gui/gui/backend/internal/imgpath|0
github.com/go-gui-org/go-gui/gui/backend/internal/texcache|0
github.com/go-gui-org/go-gui/gui/backend/internal/imgload|0
github.com/go-gui-org/go-gui/gui/backend/internal/tempfont|0
github.com/go-gui-org/go-gui/gui/backend/internal/nativehost|0
github.com/go-gui-org/go-gui/gui/backend/filedialog|0
EOF
if [ "$failures" -gt 0 ]; then
  echo "::error::$failures package(s) below coverage floor"
  exit 1
fi
echo "All per-package coverage floors met."
