#!/usr/bin/env bash
# bundle-windows-dlls.sh — download SDL2 + SDL2_mixer Windows runtime DLLs
# for dynamic Windows builds.
#
# Static builds (make build-windows, CI release) do NOT need these —
# -tags static,audio links everything into a single self-contained .exe.
# This script is only for development/dynamic builds where the binary
# loads SDL2 and its codecs from DLLs at runtime.
#
# SDL2_mixer loads codec DLLs (libFLAC, libmpg123, libvorbis, libogg,
# libopus, etc.) on demand. Without them, audio playback fails with
# "DLL not found" errors even if SDL2_mixer.dll is present.

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
if ! curl -sSfLO "$MIXER_URL"; then
	echo "WARNING: SDL2_mixer ${MIXER_VERSION} not found, trying 2.8.1..."
	MIXER_VERSION="2.8.1"
	MIXER_URL="https://github.com/libsdl-org/SDL_mixer/releases/download/release-${MIXER_VERSION}/SDL2_mixer-${MIXER_VERSION}-win32-x64.zip"
	curl -sSfLO "$MIXER_URL"
fi
unzip -o "SDL2_mixer-${MIXER_VERSION}-win32-x64.zip" -d sdl2_mixer

# Copy all DLLs to a flat layout (includes codec DLLs from SDL2_mixer).
mkdir -p flat
cp sdl2/*.dll flat/ 2>/dev/null || true
cp sdl2_mixer/*.dll flat/ 2>/dev/null || true

echo "DLLs staged to $OUTDIR/flat/"
ls -la flat/

# Warn if expected codec DLLs are missing.
echo ""
echo "Checking for common codec DLLs..."
for dll in libFLAC libmpg123 libvorbis libogg libopus libvorbisfile libwavpack; do
	found=$(find flat/ -maxdepth 1 -iname "${dll}*.dll" -print -quit 2>/dev/null || true)
	if [ -n "$found" ]; then
		echo "  OK: $(basename "$found")"
	else
		echo "  WARN: ${dll}*.dll not found — audio may fail for ${dll#lib} content"
	fi
done
