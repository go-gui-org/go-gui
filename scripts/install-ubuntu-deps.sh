#!/usr/bin/env bash
# Install the Ubuntu packages go-gui's CI actually links against.
#
# The list is short because the Linux build is cgo-free: X11 goes through
# jezek/xgb over a socket, GL through purego's dlopen, and go-glyph shapes
# text in pure Go. That leaves exactly two opt-in cgo paths, and only the
# first is built by any job:
#
#   -tags otoaudio   oto v3's ALSA driver, #cgo pkg-config: alsa
#   -tags hunspell   gui/backend/spellcheck on Linux, #cgo pkg-config: hunspell
#
# libhunspell-dev is therefore NOT installed. Add it here, and install
# pkg-config alongside, if a job ever starts building -tags hunspell.
#
# Everything else this script used to install -- FreeType, HarfBuzz,
# Pango, fontconfig, and the libx11/libxext/libxcursor/libxinerama/libxi/
# libxrandr/libxfixes/libxkbcommon headers -- had no caller. They were
# left from the era when go-glyph shaped through cgo and the GL backend
# used go-gl.
#
# libegl1 is the exception to all of the above: it is a runtime package,
# not a build one. gui/backend/gl dlopens libEGL.so.1 by soname, so no
# headers are wanted -- only the shared object, for any test that reaches
# the GL backend.
#
# Usage: ./scripts/install-ubuntu-deps.sh
set -euo pipefail

sudo apt-get update
sudo apt-get install -y \
  pkg-config \
  libasound2-dev \
  libegl1
