#!/usr/bin/env bash
# Dev loop: rebuild and relaunch a Go app whenever the module changes.
#
# Usage: scripts/dev-loop.sh <app path> [args...]
#
# - The app path is a Go package, e.g. ./examples/get_started/.
# - Extra args are preserved and passed to every relaunched instance.
# - On save: the app is rebuilt (app package + its dependency tree, which
#   covers gui/), the old process is killed, and the new binary is launched.
# - A failed build prints an error and keeps watching; the last good binary
#   keeps running, and the next save retries the build.
#
# Bash script, but callable directly from Fish or any other shell:
#   ./scripts/dev-loop.sh ./examples/get_started/
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Poll interval in seconds. A save landing between polls is picked up on the
# next tick; rebuilding takes much longer than 0.5s, so edits coalesce.
POLL_SECONDS="${DEV_LOOP_POLL_SECONDS:-0.5}"

if [ "$#" -lt 1 ]; then
	echo "usage: dev-loop.sh <app path> [args...]" >&2
	exit 1
fi

APP="$1"
shift
APP_ARGS=("$@")

# build/ is gitignored, so the binary and stamp never dirty the tree.
WORKDIR="$ROOT/build/dev-loop"
BIN="$WORKDIR/$(basename "$APP")"
STAMP="$WORKDIR/.stamp"

APP_PID=""

cleanup() {
	# Kill the running app, if any, and remove our artifacts.
	if [ -n "$APP_PID" ]; then
		kill "$APP_PID" 2>/dev/null || true
		wait "$APP_PID" 2>/dev/null || true
	fi
	rm -rf "$WORKDIR"
}

# Exit on SIGINT/SIGTERM so the EXIT trap below always runs; a plain fatal
# signal would leave the app orphaned. Ctrl-C (SIGINT) is the normal way out.
trap 'exit 0' INT
trap 'exit 1' TERM
trap cleanup EXIT

# Rebuild into a fresh temp file (never clobber the running binary), then
# swap it in. Returns non-zero on build failure without touching anything.
build_app() {
	local tmpbin
	tmpbin="$(mktemp "$WORKDIR/bin.XXXXXX")"
	if ! go build -o "$tmpbin" "$APP"; then
		rm -f "$tmpbin"
		return 1
	fi
	mv "$tmpbin" "$BIN"
	return 0
}

relaunch() {
	# Kill the previous instance before starting the new one, so at most one
	# app is alive at a time.
	if [ -n "$APP_PID" ]; then
		kill "$APP_PID" 2>/dev/null || true
		wait "$APP_PID" 2>/dev/null || true
	fi
	"$BIN" "${APP_ARGS[@]}" &
	APP_PID=$!
	echo "relaunched $(basename "$BIN") (pid $APP_PID)"
}

# Files that trigger a rebuild: Go source plus the module files, which pin
# dependency versions. Everything else (docs, scripts, .go files under build/
# or .git) is ignored.
changed_since_stamp() {
	find . \
		-path ./build -prune -o \
		-path ./.git -prune -o \
		\( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) \
		-newer "$STAMP" -print -quit
}

mkdir -p "$WORKDIR"

# Initial build: fail loudly and exit, since there is no previous binary to
# keep running.
if ! build_app; then
	echo "initial build failed; nothing to run" >&2
	exit 1
fi
touch "$STAMP"
relaunch

while true; do
	sleep "$POLL_SECONDS"
	if ! changed_since_stamp | grep -q .; then
		continue
	fi

	echo "change detected; rebuilding..."
	if ! build_app; then
		# Keep watching: touching the stamp stops the failed build from
		# retriggering itself every poll. The running app is untouched, and
		# the next save attempts a fresh build.
		echo "build failed; keeping previous binary and watching for changes" >&2
		touch "$STAMP"
		continue
	fi
	touch "$STAMP"
	relaunch
done
