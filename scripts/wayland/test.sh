#!/usr/bin/env bash
# Runs a package's tests under each headless compositor (#919).
#
# Builds the test binary for the container (linux, no cgo, Docker host CPU),
# then runs it under sway, weston and mutter with GOGUI_REQUIRE_WAYLAND=1, so
# a test that cannot reach the compositor fails instead of skipping.
#
# Usage: scripts/wayland/test.sh <package> [go test flags...]
# Example: scripts/wayland/test.sh ./gui/backend/internal/wl -test.run Registry
# COMPOSITORS="sway weston" limits the compositors.
set -uo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
pkg=${1:?usage: test.sh <package> [test flags...]}
shift
cd "$root"

arch=$(docker info --format '{{.Architecture}}')
case $arch in
aarch64 | arm64) arch=arm64 ;;
x86_64 | amd64) arch=amd64 ;;
*) echo "unsupported Docker host CPU: $arch" >&2; exit 1 ;;
esac

bin=build/wayland/$(basename "$pkg").test
mkdir -p build/wayland
CGO_ENABLED=0 GOOS=linux GOARCH=$arch go test -c -o "$bin" "$pkg" || exit 1

status=0
for comp in ${COMPOSITORS:-sway weston mutter}; do
  echo "== $comp"
  if ! GOGUI_REQUIRE_WAYLAND=1 SHOW_COMPOSITOR_LOG=1 \
    scripts/wayland/run.sh -c "$comp" -- env GOGUI_REQUIRE_WAYLAND=1 "$bin" "$@"; then
    echo "FAIL under $comp"
    status=1
  fi
done
exit "$status"
