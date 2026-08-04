#!/usr/bin/env bash
# Install Ubuntu system dependencies required to build go-gui with CGO.
# libegl1 is a runtime dependency, not a build one: gui/backend/gl dlopens
# libEGL.so.1 by soname. No GL headers are needed since the backend dropped
# github.com/go-gl/gl for the purego binding in gui/backend/internal/glbind.
# Usage: ./scripts/install-ubuntu-deps.sh
set -euo pipefail

sudo apt-get update
sudo apt-get install -y \
  pkg-config \
  libfreetype6-dev \
  libharfbuzz-dev \
  libpango1.0-dev \
  libfontconfig1-dev \
  libhunspell-dev \
  libegl1 \
  libx11-dev \
  libxext-dev \
  libxcursor-dev \
  libxinerama-dev \
  libxi-dev \
  libxrandr-dev \
  libxfixes-dev \
  libxkbcommon-dev \
  libasound2-dev
