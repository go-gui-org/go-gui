#!/usr/bin/env bash
# Builds a Go package for the gogui-wayland container (#919): linux, no cgo,
# and the CPU of the Docker host (arm64 under colima on Apple silicon).
#
# Usage: scripts/wayland/build.sh <package> <output>   (output relative to the
# repo root, so run.sh can reach it under /src)
# Example: scripts/wayland/build.sh ./examples/showcase build/wayland/showcase
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
pkg=${1:?usage: build.sh <package> <output>}
out=${2:?usage: build.sh <package> <output>}

arch=$(docker info --format '{{.Architecture}}')
case $arch in
aarch64 | arm64) arch=arm64 ;;
x86_64 | amd64) arch=amd64 ;;
*) echo "unsupported Docker host CPU: $arch" >&2; exit 1 ;;
esac

cd "$root"
mkdir -p "$(dirname "$out")"
CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -o "$out" "$pkg"
