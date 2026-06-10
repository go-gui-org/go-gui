#!/usr/bin/env bash
# bundle-windows-dlls.sh — download SDL2 + SDL2_mixer Windows runtime DLLs
# for inclusion in the Windows release zip.
#
# The static binary does NOT need these, but they are bundled as
# defense-in-depth so the zip works for dynamic builds too.

set -euo pipefail

SDL2_VERSION="${SDL2_VERSION:-2.32.10}"
MIXER_VERSION="${MIXER_VERSION:-2.8.2}"
OUTDIR="${OUTDIR:-build/dlls}"

SDL2_URL="https://github.com/libsdl-org/SDL/releases/download/release-${SDL2_VERSION}/SDL2-${SDL2_VERSION}-win32-x64.zip"
MIXER_URL="https://github.com/libsdl-org/SDL_mixer/releases/download/release-${MIXER_VERSION}/SDL2_mixer-${MIXER_VERSION}-win32-x64.zip"

mkdir -p "$OUTDIR"
cd "$OUTDIR"

echo "Downloading SDL2 ${SDL2_VERSION}..."
curl -sSfLO "$SDL2_URL"
unzip -o "SDL2-${SDL2_VERSION}-win32-x64.zip" -d sdl2

echo "Downloading SDL2_mixer ${MIXER_VERSION}..."
curl -sSfLO "$SDL2_mixer-${MIXER_VERSION}-win32-x64.zip" 2>/dev/null || {
	echo "WARNING: SDL2_mixer ${MIXER_VERSION} not found, trying 2.8.1..."
	MIXER_VERSION="2.8.1"
	MIXER_URL="https://github.com/libsdl-org/SDL_mixer/releases/download/release-${MIXER_VERSION}/SDL2_mixer-${MIXER_VERSION}-win32-x64.zip"
	curl -sSfLO "$MIXER_URL"
}
unzip -o "SDL2_mixer-${MIXER_VERSION}-win32-x64.zip" -d sdl2_mixer

# Copy just the DLLs to a flat layout.
mkdir -p flat
cp sdl2/*.dll flat/ 2>/dev/null || true
cp sdl2_mixer/*.dll flat/ 2>/dev/null || true

echo "DLLs staged to $OUTDIR/flat/"
ls -la flat/
