#!/usr/bin/env bash
# Checks the Wayland test harness itself (#919): every compositor starts,
# a reference EGL client (weston-simple-egl, EGL + xdg-shell, the same stack
# the backend uses) maps and stays up, and screenshots come out where the
# compositor supports them. Run it after editing the Dockerfile or
# in-container.sh, before blaming the backend for a failure.
#
# Usage: scripts/wayland/selftest.sh   (screenshots land in build/wayland/)
set -uo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"
mkdir -p build/wayland
status=0
for comp in sway weston mutter; do
  shot=build/wayland/selftest-$comp.png
  rm -f "$shot"
  # mutter has no screenshot path without gnome-shell; it is checked for
  # the client staying up only.
  args=(-c "$comp" -t 3)
  [ "$comp" = mutter ] || args+=(-s "$shot")
  if ! out=$(SHOW_COMPOSITOR_LOG=1 scripts/wayland/run.sh "${args[@]}" -- weston-simple-egl 2>&1); then
    printf '%s\n' "$out"
    echo "FAIL $comp: client exited early or compositor did not start"
    status=1
    continue
  fi
  if [ "$comp" != mutter ] && [ ! -s "$shot" ]; then
    echo "FAIL $comp: no screenshot"
    status=1
    continue
  fi
  echo "ok   $comp"
done
exit "$status"
