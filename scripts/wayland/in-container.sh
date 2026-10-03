#!/usr/bin/env bash
# Runs inside the gogui-wayland image (see Dockerfile). Starts one headless
# compositor, runs a client under it, and optionally takes a screenshot.
# run.sh calls this; it is not meant to be run on the host.
#
# Environment (set by run.sh):
#   COMPOSITOR  sway | weston | mutter
#   RUN_SECS    seconds to let the client run before the screenshot and stop;
#               0 waits for the client to exit by itself
#   SHOT        screenshot path inside the container, or empty for none
#   AFTER       shell commands run once the client has had a second to map
#               its window (input injection: wtype, wlrctl)
# Arguments: the client command line.
#
# Exit status: the client's own status if it exits within RUN_SECS, 0 if it
# is still running at RUN_SECS (a GUI app runs until closed), and 125 when
# the compositor never came up.
set -uo pipefail

mkdir -p "$XDG_RUNTIME_DIR" && chmod 700 "$XDG_RUNTIME_DIR"
export WAYLAND_DISPLAY=wayland-gogui
log=/tmp/compositor.log

case $COMPOSITOR in
sway)
  # One fixed-size headless output; no input devices exist, wtype and
  # wlrctl create virtual ones through the wlroots protocols.
  cat >/tmp/sway.cfg <<'EOF'
output HEADLESS-1 resolution 1280x800
default_border none
xwayland disable
EOF
  WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1 WLR_RENDERER=pixman \
    sway -c /tmp/sway.cfg >"$log" 2>&1 &
  # sway takes the first free wayland-N name and ignores WAYLAND_DISPLAY,
  # so the socket is found below instead.
  ;;
weston)
  # --debug exposes the screenshooter protocol weston-screenshooter needs.
  weston --backend=headless --renderer=pixman --width=1280 --height=800 \
    --socket="$WAYLAND_DISPLAY" --debug >"$log" 2>&1 &
  ;;
mutter)
  # mutter wants a session bus; GNOME draws no server-side decorations.
  dbus-run-session -- mutter --headless --wayland --no-x11 \
    --virtual-monitor 1280x800 --wayland-display "$WAYLAND_DISPLAY" >"$log" 2>&1 &
  ;;
*)
  echo "unknown COMPOSITOR=$COMPOSITOR (want sway, weston or mutter)" >&2
  exit 2
  ;;
esac
comp=$!

# Wait up to 10 s for a Wayland socket to appear.
sock=
for _ in $(seq 100); do
  sock=$(find "$XDG_RUNTIME_DIR" -maxdepth 1 -type s -name 'wayland-*' 2>/dev/null | head -1)
  [ -n "$sock" ] && break
  sleep 0.1
done
if [ -z "$sock" ]; then
  echo "compositor $COMPOSITOR did not start; its log:" >&2
  cat "$log" >&2
  exit 125
fi
export WAYLAND_DISPLAY=${sock##*/}
# Make sure no X11 fallback can hide a Wayland failure.
unset DISPLAY

screenshot() {
  [ -n "$SHOT" ] || return 0
  case $COMPOSITOR in
  sway) grim "$SHOT" ;;
  weston)
    # weston-screenshooter writes wayland-screenshot-<time>.png in the cwd.
    (cd /tmp && weston-screenshooter) &&
      mv "$(ls -t /tmp/wayland-screenshot-*.png | head -1)" "$SHOT"
    ;;
  mutter) echo "screenshots are not supported under mutter" >&2 ;;
  esac
}

"$@" &
client=$!

if [ -n "${AFTER:-}" ]; then
  sleep 1
  bash -c "$AFTER"
fi

status=0
if [ "$RUN_SECS" -gt 0 ]; then
  # Poll so a client that dies early reports its status at once.
  for _ in $(seq $((RUN_SECS * 10))); do
    kill -0 "$client" 2>/dev/null || break
    sleep 0.1
  done
  if kill -0 "$client" 2>/dev/null; then
    screenshot
    kill "$client" 2>/dev/null
    wait "$client" 2>/dev/null
  else
    wait "$client"
    status=$?
    screenshot
  fi
else
  wait "$client"
  status=$?
  screenshot
fi

kill "$comp" 2>/dev/null
wait "$comp" 2>/dev/null
if [ "$status" -ne 0 ] && [ -n "${SHOW_COMPOSITOR_LOG:-}" ]; then
  cat "$log" >&2
fi
exit "$status"
