# Headless Wayland harness

Builds and runs go-gui under a real Wayland compositor without a Linux desktop.
It is used to develop the experimental Wayland backend (#919) on macOS. It needs
Docker (on macOS, colima works).

## Files

| File              | What it does                                                           |
| ----------------- | ---------------------------------------------------------------------- |
| `Dockerfile`      | Debian image with sway, weston, mutter, Mesa llvmpipe and Wayland libs |
| `run.sh`          | Runs one command under one compositor, with optional screenshot/input  |
| `build.sh`        | Builds a Go package for the container (linux, no cgo, Docker host CPU) |
| `selftest.sh`     | Checks the harness with `weston-simple-egl`; `make wayland-selftest`   |
| `test.sh`         | Runs a package's tests under each compositor, compositor required      |
| `in-container.sh` | Starts the compositor inside the container; called by `run.sh`         |

`run.sh` builds the image the first time it runs and again whenever the
`Dockerfile` changes.

## Compositors

| Name     | Use                                                      | Screenshot             | Input injection   |
| -------- | -------------------------------------------------------- | ---------------------- | ----------------- |
| `sway`   | Default. wlroots, tiles the window to fill the output    | `grim`                 | `wtype`, `wlrctl` |
| `weston` | Reference compositor. Has no seat, so no input devices   | `weston-screenshooter` | none              |
| `mutter` | GNOME. Offers no server-side decorations (libdecor path) | none                   | none              |

The output is 1280×800 in every compositor.

## Examples

Build the showcase and run it for 5 seconds under sway, with a screenshot:

```sh
scripts/wayland/build.sh ./examples/showcase build/wayland/showcase
scripts/wayland/run.sh -t 5 -s build/wayland/showcase.png -- build/wayland/showcase
```

Run the Wayland window tests of the GL backend:

```sh
scripts/wayland/test.sh ./gui/backend/gl -test.run Wayland -test.v
```

Close the window, or float and resize it, through sway (`SWAYSOCK` is set for
`-a` commands):

```sh
scripts/wayland/run.sh -t 10 -a 'swaymsg kill' -- build/wayland/showcase
scripts/wayland/run.sh -t 4 -a 'swaymsg "floating enable, resize set 500 360"' -s build/wayland/resized.png -- build/wayland/showcase
scripts/wayland/run.sh -t 4 -a 'swaymsg "output * scale 2"' -s build/wayland/hidpi.png -- build/wayland/showcase
```

Type text and move the pointer one second after the client starts (sway only):

```sh
scripts/wayland/run.sh -t 5 -a 'wtype hello; wlrctl pointer move 100 100' -- build/wayland/showcase
```

Run a package's tests under sway, weston and mutter. `GOGUI_REQUIRE_WAYLAND=1`
is set, so a test that cannot reach the compositor fails instead of skipping:

```sh
scripts/wayland/test.sh ./gui/backend/internal/wl
COMPOSITORS=sway scripts/wayland/test.sh ./gui/backend/internal/wl -test.run Registry -test.v
```

Check which GL the Wayland EGL platform offers:

```sh
scripts/wayland/run.sh -- eglinfo -B -p wayland
```

## Exit status

- The client's own status if it exits before `-t` seconds pass.
- 0 if it is still running at `-t` (a GUI app runs until it is closed).
- 125 if the compositor did not start.

Set `SHOW_COMPOSITOR_LOG=1` to print the compositor log when the client fails.
`run.sh` passes `GOGUI_WAYLAND` (default `1`) and `GOGUI_DEBUG` into the
container.
