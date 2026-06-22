#!/usr/bin/env bash
# cov-diff.sh — compare two Go coverage profiles and report regressions.
# Usage: cov-diff.sh [--github-summary] baseline.out current.out
#
# With --github-summary, writes a markdown table to $GITHUB_STEP_SUMMARY
# and emits ::warning:: annotations for regressions > 2%.

set -euo pipefail

GITHUB_SUMMARY=false
if [ "${1:-}" = "--github-summary" ]; then
  GITHUB_SUMMARY=true
  shift
fi

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

output() {
  if [ "$GITHUB_SUMMARY" = true ] && [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    echo "$*" >> "$GITHUB_STEP_SUMMARY"
  fi
  # Always print to stdout so it appears in the step log.
  echo "$*"
}

output "## Coverage diff"
output ""
output "| Package | Baseline | PR | Δ |"
output "|---------|----------|----|---|"

regressions=0

while IFS=$'\t' read -r pkg base cur; do
  if [ "$base" = "-" ]; then
    output "| $pkg | — | ${cur}% | new |"
  elif [ "$cur" = "-" ]; then
    output "| $pkg | ${base}% | — | removed |"
  else
    delta=$(awk "BEGIN { printf \"%.1f\", $cur - $base }")
    if (( $(awk "BEGIN { print ($delta < -2) ? 1 : 0 }") )); then
      echo "::warning::$pkg coverage dropped ${delta}% (${base}% → ${cur}%)"
      regressions=$((regressions + 1))
    fi
    output "| $pkg | ${base}% | ${cur}% | ${delta}% |"
  fi
done < <(join -t $'\t' -a 1 -a 2 -e "-" -o 0,1.2,2.2 \
  <(pkg_cov "$BASELINE") \
  <(pkg_cov "$CURRENT"))

output ""
output "${regressions} package(s) with coverage drop > 2%."
