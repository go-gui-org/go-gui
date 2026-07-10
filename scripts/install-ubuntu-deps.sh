#!/usr/bin/env bash
# Install Ubuntu system dependencies required to build go-gui with CGO.
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
  libgl1-mesa-dev \
  libx11-dev \
  libxext-dev \
  libxcursor-dev \
  libxinerama-dev \
  libxi-dev \
  libxrandr-dev \
  libxfixes-dev \
  libxkbcommon-dev \
  libasound2-dev
