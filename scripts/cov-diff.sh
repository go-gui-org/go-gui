#!/usr/bin/env bash
# cov-diff.sh — compare two Go coverage profiles and report regressions.
# Usage: cov-diff.sh baseline.out current.out

set -euo pipefail

BASELINE="$1"
CURRENT="$2"

if [ ! -f "$BASELINE" ]; then
  echo "::warning::No baseline coverage file at $BASELINE"
  exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

pkg_cov() {
  awk -f "$SCRIPT_DIR/pkgcov.awk" "$1" | sort
}

while IFS=$'\t' read -r pkg base cur; do
  if [ "$base" = "-" ]; then
    printf "| %s | — | %s%% | new |\n" "$pkg" "$cur"
  elif [ "$cur" = "-" ]; then
    printf "| %s | %s%% | — | removed |\n" "$pkg" "$base"
  else
    delta=$(awk "BEGIN { printf \"%.1f\", $cur - $base }")
    printf "| %s | %s%% | %s%% | %s%% |\n" "$pkg" "$base" "$cur" "$delta"
  fi
done < <(join -t $'\t' -a 1 -a 2 -e "-" -o 0,1.2,2.2 \
  <(pkg_cov "$BASELINE") \
  <(pkg_cov "$CURRENT"))
