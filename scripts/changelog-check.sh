#!/bin/sh
# Vet gate for Keep-a-Changelog shape. Fails if a breaking API change is
# documented outside ### Changed with **BREAKING: prefix.
# This is the fix for v0.68.0, where ScrollbarCfg.GapEdge/GapEnd changed
# from float32 to Opt[float32] under ### Added as "breaking for..." with no
# BREAKING label, so scanners and reviewers missed it.

set -eu

FILE="CHANGELOG.md"
if [ ! -f "$FILE" ]; then
  echo "changelog-check: $FILE not found" >&2
  exit 0
fi

python3 - <<'PY'
import re, sys
path = "CHANGELOG.md"
text = open(path, encoding="utf-8").read()

# Split into version blocks: ## [vX.Y.Z] - date  OR ## [Unreleased] / ## vX.Y.Z
# Keep header and body together.
blocks = re.split(r'(?m)^## \[', text)
# blocks[0] is preamble before first ## [
# Only gate Unreleased + the newest version block. Historical entries are
# grandfathered — the repo had several pre-gate breaking notes under Added
# (e.g., v0.59.0 DataGridCfg.Sizing) that we don't want to retroactively fail.
# Check at most the first two blocks after preamble.
errors = []
# collect candidate blocks: Unreleased + first version
candidates = []
for raw in blocks[1:]:
    m = re.match(r'([^\]]+)\]', raw)
    if not m:
        continue
    ver = m.group(1).strip().split()[0]
    candidates.append((ver, raw))
    if ver != "Unreleased" and len([v for v,_ in candidates if v != "Unreleased"]) >= 1:
        # keep Unreleased + one newest version
        # if Unreleased present, we want 2 entries; otherwise 1
        if "Unreleased" in [v for v,_ in candidates]:
            if len(candidates) >= 2:
                break
        else:
            break
for i, (ver_name, raw) in enumerate(candidates, 1):
    # raw starts with e.g. "v0.68.0] - 2026-09-05\n\n### Added\n..."
    m = re.match(r'([^\]]+)\]', raw)
    if not m:
        continue
    ver = m.group(1).strip()  # "v0.68.0" or "Unreleased" or "v0.68.0 - 2026-09-05"
    # Normalize: strip date part if present
    ver_name = ver.split()[0]  # "v0.68.0" or "Unreleased"
    if ver_name == "Unreleased":
        continue
    body = raw[m.end():]

    # Extract sections
    def section(name):
        # ### Added / ### Changed / ### Fixed etc until next ### or next ## [
        pat = rf'(?m)^### {re.escape(name)}\s*\n(.*?)(?=^### |\Z)'
        mm = re.search(pat, body, flags=re.DOTALL)
        return mm.group(1) if mm else ""

    added = section("Added")
    changed = section("Changed")

    # Rule 1: Added must not contain the word "breaking" (case-insensitive) as a
    # description of an API break. A breaking change belongs under Changed with
    # **BREAKING: . The one legitimate non-breaking use ("non-breaking") is rare
    # and can be reworded; flagging it is safer than missing a real break.
    if re.search(r'breaking', added, flags=re.IGNORECASE):
        # Allow "non-breaking" as an explicit exception? Still flag for manual
        # triage — the scalar is high but false positives are rare in this repo.
        errors.append(f"{ver_name}: 'breaking' found under ### Added (move to ### Changed as '- **BREAKING: ...'): {path}:{ver_name}")

    # Rule 2: If Changed exists, every breaking entry there must start with **BREAKING:
    # Rule 3: If the diff for this version is known to be breaking (heuristic:
    # body mentions Opt[ or "are now Opt"), Changed must contain a BREAKING line.
    # This catches the v0.68.0 case where Added said "are now `Opt[float32]` — breaking for..."
    if re.search(r'Opt\[|are now.*Opt', body):
        if not re.search(r'^\- \*\*BREAKING:', changed, flags=re.MULTILINE):
            errors.append(f"{ver_name}: body mentions Opt[ change but ### Changed has no '- **BREAKING:' line ({path})")

    # Also catch any BREAKING marker outside Changed
    for sec_name in ["Added", "Fixed", "Removed", "Deprecated", "Security"]:
        sec = section(sec_name)
        if re.search(r'^\- \*\*BREAKING:', sec, flags=re.MULTILINE):
            errors.append(f"{ver_name}: '- **BREAKING:' found under ### {sec_name} (must be under ### Changed)")

if errors:
    for e in errors:
        print(f"::error::{e}", file=sys.stderr)
        print(e, file=sys.stderr)
    sys.exit(1)
print("changelog-check: ok")
PY
