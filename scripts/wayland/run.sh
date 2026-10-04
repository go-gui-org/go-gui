#!/usr/bin/env bash
# Runs a command under a headless Wayland compositor in Docker (#919).
#
# The experimental Wayland backend is developed mostly on macOS, where there
# is no Wayland. This script builds the gogui-wayland image (Dockerfile next
# to it) on first use, mounts the repo at /src, and runs the command there
# under sway, weston or mutter, with Mesa llvmpipe for GL.
#
# Usage: scripts/wayland/run.sh [options] -- <command> [args...]
#   -c sway|weston|mutter  compositor (default sway)
#   -t <secs>              stop the client after this many seconds and count
#                          it as a pass if it is still running (default 0:
#                          wait for it to exit)
#   -s <file.png>          screenshot path, relative to the repo root
#   -a '<commands>'        shell commands run a second after the client
#                          starts, for input injection (wtype, wlrctl; sway)
#
# The command runs inside the container, so build Go binaries for linux
# first. scripts/wayland/build.sh does that for the container's CPU.
#
# Examples:
#   scripts/wayland/run.sh -- eglinfo -p wayland
#   scripts/wayland/run.sh -c weston -t 3 -s out.png -- ./showcase-linux
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
here="$root/scripts/wayland"

comp=sway secs=0 shot= after=
while getopts c:t:s:a: opt; do
  case $opt in
  c) comp=$OPTARG ;;
  t) secs=$OPTARG ;;
  s) shot=/src/$OPTARG ;;
  a) after=$OPTARG ;;
  *) sed -n '2,25p' "$0" >&2; exit 2 ;;
  esac
done
shift $((OPTIND - 1))
[ "${1:-}" = -- ] && shift
[ $# -gt 0 ] || { sed -n '2,25p' "$0" >&2; exit 2; }

# Tag the image with a hash of the Dockerfile, so editing it rebuilds once
# and an unchanged Dockerfile never rebuilds.
tag=gogui-wayland:$(shasum -a 256 "$here/Dockerfile" | cut -c1-12)
if ! docker image inspect "$tag" >/dev/null 2>&1; then
  docker build -q -t "$tag" "$here" >/dev/null
fi

tty=
[ -t 0 ] && [ -t 1 ] && tty=-it
exec docker run --rm $tty \
  -v "$root:/src" -w /src \
  -e COMPOSITOR="$comp" -e RUN_SECS="$secs" -e SHOT="$shot" -e AFTER="$after" \
  -e SHOW_COMPOSITOR_LOG="${SHOW_COMPOSITOR_LOG:-}" \
  -e GOGUI_WAYLAND="${GOGUI_WAYLAND:-1}" -e GOGUI_DEBUG="${GOGUI_DEBUG:-}" \
  "$tag" bash /src/scripts/wayland/in-container.sh "$@"
