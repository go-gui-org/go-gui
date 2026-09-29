#!/usr/bin/env bash
# Breaking-change gate for the theme surface (issue #846).
#
# gui/testdata/theme_surface.golden lists every exported field of Theme,
# ThemeCfg and TextStyle, one "Type.Field type" line each. TestThemeSurface
# keeps it in step with the code. This script reads the golden's diff against
# the fork point: a removed line is a field that was removed, renamed or
# retyped, and that breaks apps and custom themes outside this repo. Such a
# branch must add a "**BREAKING:" line under ## [Unreleased] in CHANGELOG.md.
#
# Added lines pass silently: adding a role is free (docs/theme-tokens.md).
#
# The diff is against the working tree, not HEAD, so the gate also fires on a
# local run before the rename is committed. In CI the tree is clean and the
# two are the same.
#
# No "changelog: skip" escape hatch, on purpose: a removed theme field always
# reaches users. Re-recording the golden with -update does not help either;
# the removed line is still in the diff this script reads.
#
# THEME_SURFACE_BASE overrides the base, for replaying the gate over past
# commits. Not used by CI.
# Usage: ./scripts/theme-surface-check.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

GOLDEN="gui/testdata/theme_surface.golden"
FILE="CHANGELOG.md"

# --- Resolve the base to diff against -------------------------------------
# Same resolution as changelog-entry-check.sh: the fork point from the
# default branch.
BASE_REF="${THEME_SURFACE_BASE:-}"
[ -n "$BASE_REF" ] || \
for cand in "origin/${GITHUB_BASE_REF:-}" origin/main main; do
  case "$cand" in origin/) continue;; esac
  if git rev-parse --verify --quiet "$cand" >/dev/null; then BASE_REF="$cand"; break; fi
done

if [ -z "$BASE_REF" ]; then
  echo "theme-surface-check: no base ref found, skipping"
  exit 0
fi

BASE="$(git merge-base "$BASE_REF" HEAD)"

# The golden did not exist at the base (the branch that adds this gate):
# nothing can have been removed.
if ! git cat-file -e "$BASE:$GOLDEN" 2>/dev/null; then
  echo "theme-surface-check: $GOLDEN not at base, ok"
  exit 0
fi

# Removed lines only. The "---" file header is dropped by requiring a
# character after "-" that is not "-".
REMOVED="$(git diff "$BASE" -- "$GOLDEN" | grep -E '^-[^-]' | sed 's/^-//' || true)"
if [ -z "$REMOVED" ]; then
  echo "theme-surface-check: no theme field removed, ok"
  exit 0
fi

# --- Require a BREAKING entry this branch added under Unreleased ------------
# Presence alone is not enough: Unreleased can already hold an earlier PR's
# BREAKING entry. So take the BREAKING lines this branch added to the
# changelog and require at least one of them to sit inside Unreleased.
UNRELEASED="$(awk '/^## \[Unreleased\]/{f=1;next} /^## \[/{f=0} f' "$FILE")"
ADDED_BREAKING="$(git diff "$BASE" -- "$FILE" \
  | grep -E '^\+[^+]' | sed 's/^+//' | grep -F '**BREAKING:' || true)"

FOUND=0
if [ -n "$ADDED_BREAKING" ]; then
  while IFS= read -r line; do
    # No -q, same reason as changelog-entry-check.sh: an early close of the
    # pipe kills printf with SIGPIPE on a large Unreleased block.
    if printf '%s\n' "$UNRELEASED" | grep -xF -- "$line" >/dev/null; then
      FOUND=1
    fi
  done <<<"$ADDED_BREAKING"
fi

if [ "$FOUND" -eq 1 ]; then
  echo "theme-surface-check: theme field(s) removed, BREAKING entry present, ok"
  exit 0
fi

echo "::error::theme-surface-check: theme field(s) removed without a BREAKING entry" >&2
cat >&2 <<EOF
theme-surface-check: this branch removes, renames or retypes exported fields
of Theme, ThemeCfg or TextStyle:

$(printf '%s\n' "$REMOVED" | sed 's/^/  /')

Apps and custom themes outside this repo use these names. Either:

1. Keep the old field for one release, mark it "// Deprecated:", and fill it
   next to the new one (docs/theme-tokens.md, "Text roles"). Or
2. Add "- **BREAKING: <what> (#N)**" under ### Changed in ## [Unreleased] of
   $FILE, with the migration spelled out.
EOF
exit 1
