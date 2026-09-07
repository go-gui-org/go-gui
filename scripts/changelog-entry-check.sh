#!/usr/bin/env bash
# Presence gate for CHANGELOG.md's Unreleased block.
#
# Sibling of changelog-check.sh, which gates the *shape* of released blocks
# (BREAKING placement) and deliberately skips Unreleased. This one gates
# *presence*: a branch that changes the exported surface or the release
# packaging must land its Unreleased entry in the same pass, not as cleanup
# before a tag. PR #538 shipped both-arch packaging with no entry; the text
# was reconstructed a day later in the release PR (#539) from the diff.
#
# Trigger is a heuristic by design (design C, chosen over an exported-surface
# diff that needs a base build, and over a path allowlist that fires on every
# internal refactor). It keys on the two things that empirically went
# undocumented here: exported declarations, and the packaging path set.
#
# Escape hatch: put "changelog: skip" in any commit message on the branch.
# Usage: ./scripts/changelog-entry-check.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

FILE="CHANGELOG.md"
[ -f "$FILE" ] || { echo "changelog-entry-check: $FILE not found" >&2; exit 0; }

# Paths whose change reaches users without touching Go: release packaging and
# the dependency install script CI and downstream builders run.
PACKAGING_PATHS='^(Makefile|\.github/workflows/release\.yml|scripts/install-.*\.sh)$'

# --- Resolve the base to diff against -------------------------------------
# A branch is compared to where it forked from the default branch. On the
# default branch itself there is no delta, so the gate is a no-op rather than
# an error — otherwise `make check` on a clean main would fail.
# CHANGELOG_ENTRY_BASE overrides the base, for replaying the gate over past
# commits when tuning the heuristic. Not used by CI.
BASE_REF="${CHANGELOG_ENTRY_BASE:-}"
[ -n "$BASE_REF" ] || \
for cand in "origin/${GITHUB_BASE_REF:-}" origin/main main; do
  case "$cand" in origin/) continue;; esac
  if git rev-parse --verify --quiet "$cand" >/dev/null; then BASE_REF="$cand"; break; fi
done

if [ -z "$BASE_REF" ]; then
  echo "changelog-entry-check: no base ref found, skipping"
  exit 0
fi

BASE="$(git merge-base "$BASE_REF" HEAD)"
if [ "$BASE" = "$(git rev-parse HEAD)" ]; then
  echo "changelog-entry-check: no commits since $BASE_REF, skipping"
  exit 0
fi

# --- Escape hatch ----------------------------------------------------------
if git log --format=%B "$BASE..HEAD" | grep -qiE '^[[:space:]]*changelog:[[:space:]]*skip'; then
  echo "changelog-entry-check: 'changelog: skip' in a commit message, skipping"
  exit 0
fi

CHANGED="$(git diff --name-only "$BASE..HEAD")"
[ -n "$CHANGED" ] || { echo "changelog-entry-check: no changed files, skipping"; exit 0; }

# --- Trigger 1: release packaging ------------------------------------------
REASON=""
PACK="$(printf '%s\n' "$CHANGED" | grep -E "$PACKAGING_PATHS" || true)"
if [ -n "$PACK" ]; then
  REASON="release packaging changed:$(printf '%s' "$PACK" | tr '\n' ' ')"
fi

# --- Trigger 2: exported declarations --------------------------------------
# Scan added/removed lines in non-test Go files for an exported declaration.
# Comment lines are excluded so a doc-comment rewrite (PR #533) does not fire.
GO_FILES="$(printf '%s\n' "$CHANGED" | grep -E '\.go$' | grep -v '_test\.go$' || true)"
if [ -n "$GO_FILES" ]; then
  # shellcheck disable=SC2086 # deliberate word splitting: pathspec list
  EXPORTED="$(git diff "$BASE..HEAD" -- $GO_FILES \
    | grep -E '^[+-][^+-]' \
    | grep -vE '^[+-][[:space:]]*//' \
    | grep -cE '^[+-][[:space:]]*(func [A-Z]|func \([^)]*\) [A-Z]|type [A-Z]|const [A-Z]|var [A-Z]|[A-Z][A-Za-z0-9_]*[[:space:]]+[^=])' \
    || true)"
  if [ "${EXPORTED:-0}" -gt 0 ]; then
    if [ -n "$REASON" ]; then REASON="$REASON; "; fi
    REASON="${REASON}${EXPORTED} exported-declaration line(s) changed"
  fi
fi

if [ -z "$REASON" ]; then
  echo "changelog-entry-check: no user-facing trigger, ok"
  exit 0
fi

# --- Require an entry this branch actually added ---------------------------
# Two conditions, both necessary. Unreleased must be non-empty (at least one
# list bullet; a bare "### Added" heading with nothing under it is not an
# entry), AND this branch must have touched CHANGELOG.md. Presence alone is
# not enough: Unreleased usually already holds an earlier PR's entry, so a
# branch that adds nothing still finds the block non-empty. That is exactly
# how #538 shipped undocumented while Unreleased was full of #536 and #537.
UNRELEASED="$(awk '/^## \[Unreleased\]/{f=1;next} /^## \[/{f=0} f' "$FILE")"
HAS_BULLET=0
printf '%s\n' "$UNRELEASED" | grep -qE '^[[:space:]]*-[[:space:]]+\S' && HAS_BULLET=1
TOUCHED=0
printf '%s\n' "$CHANGED" | grep -qxF "$FILE" && TOUCHED=1

if [ "$HAS_BULLET" -eq 1 ] && [ "$TOUCHED" -eq 1 ]; then
  echo "changelog-entry-check: ok ($REASON)"
  exit 0
fi

if [ "$TOUCHED" -eq 0 ]; then
  MSG="this branch is user-facing ($REASON) but does not touch $FILE"
else
  MSG="$FILE has no entry under ## [Unreleased], but this branch is user-facing ($REASON)"
fi
echo "::error::changelog-entry-check: $MSG" >&2
cat >&2 <<EOF
changelog-entry-check: $MSG

Add the entry in this branch, not in the release PR. Use the existing format:
a Keep a Changelog section (### Added / ### Changed / ### Fixed), a bolded
one-line summary, the issue or PR number, and prose saying what now happens
and why it matters. A breaking change goes under ### Changed as
"- **BREAKING: <what> (#N)**" with the migration spelled out.

If the change genuinely has no user-visible effect, put "changelog: skip" in
a commit message on this branch.
EOF
exit 1
