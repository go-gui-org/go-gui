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
  libhunspell-dev
