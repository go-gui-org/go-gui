# Native Wayland backend

Status: in progress, experimental. Issue #919. Phases 2 (test harness) and 3
(protocol bindings) of 9 have landed.

## Problem

go-gui runs on Wayland only through XWayland. GNOME and KDE default to Wayland,
and the major distributions ship it. Without native support, go-gui is harder to
adopt on Linux, and devices with no X11 EGL platform cannot use their GPU
(#916).

## Decisions

### The backend is experimental and behind an env var

`GOGUI_WAYLAND=1` selects the Wayland backend. With no value, go-gui uses X11 as
before. If Wayland setup fails, go-gui falls back to X11. Both backends are
compiled in. purego opens the Wayland libraries only at run time, so the Wayland
code adds no link dependency.

### The renderer does not change

Mesa gives desktop OpenGL 4.5 core on the Wayland EGL platform. This was checked
under sway, weston and mutter in the harness (`eglinfo -p wayland`). NVIDIA does
the same. So the GL 3.3 renderer and `gui/backend/internal/glbind` are reused.
An OpenGL ES path is needed only on GLES-only devices, and is a separate, later
issue.

### libwayland-client through purego

EGL needs a real libwayland `wl_display*` and `wl_surface*`
(`eglGetPlatformDisplay(EGL_PLATFORM_WAYLAND_KHR)` and `wl_egl_window_create`).
So go-gui calls libwayland-client through purego. This keeps the build cgo-free.
The bindings live in `gui/backend/internal/wl`:

- Requests use `wl_proxy_marshal_array_flags`. It is not variadic and needs
  libwayland 1.20 or later (Debian 13 ships 1.23). `Load` fails by name on an
  older library.
- Events arrive through one purego callback, installed on each proxy with
  `wl_proxy_add_dispatcher`. libwayland passes it the opcode and the decoded
  argument array. One callback serves every proxy, so purego's fixed callback
  limit never applies.
- `internal/wlgen` (`go generate`) builds the `wl_interface` tables and the Go
  types, request methods and `SetHandlers` decoders from the XML in
  `protocols/`. The tables live in Go memory as package globals and are filled
  on the first `Load`.
- Calls use `purego.SyscallN` on raw addresses, not `purego.RegisterFunc`, which
  allocates several times per call. An idle `Dispatch` (called every frame)
  allocates 4 times, all inside purego; `TestDispatchIdleAllocs` pins it.
- A handler's panic is recovered before it can unwind through libwayland's C
  frames, and raised again from the Go call that ran the dispatch.
- The package builds for linux/amd64 and linux/arm64 only: a `wl_argument` is
  read as one 8-byte little-endian slot.

### Decorations

The backend uses xdg-decoration when the compositor offers it (KDE, wlroots).
Otherwise it uses libdecor when it is installed (GNOME, Cinnamon). If libdecor
is missing, the window has no frame, and a warning is printed once.

### Development happens mostly in a headless harness

`scripts/wayland/` runs clients under headless sway, weston and mutter in
Docker, with Mesa llvmpipe. sway supports screenshots (`grim`) and injected
input (`wtype`, `wlrctl`), weston supports screenshots, and mutter covers the
no-server-side-decorations path. Only the hardware pass (real GPU, HiDPI, IME,
clipboard with other apps) needs a Linux machine.

## Phases

| #   | Phase                                                                                   | State   |
| --- | --------------------------------------------------------------------------------------- | ------- |
| 1   | Issue #919                                                                              | done    |
| 2   | Headless test harness (`scripts/wayland/`, `make wayland-selftest`)                     | done    |
| 3   | Protocol code generation + purego libwayland-client core                                | done    |
| 4   | Window: xdg-shell, `wl_egl_window`, EGL Wayland display, frame callbacks, resize, close | pending |
| 5   | Input: pointer, keyboard through xkbcommon, scroll, touch                               | pending |
| 6   | Decorations, clipboard, fractional scale, cursor-shape, text-input-v3                   | pending |
| 7   | Hardware pass on Linux Mint (Intel/AMD): Cinnamon Wayland, nested sway/weston/KWin      | pending |
| 8   | Ship as experimental                                                                    | pending |
| 9   | OpenGL ES renderer path for GLES-only devices (separate issue)                          | pending |

## Rejected Approaches

- **Pure-Go Wayland wire protocol (go-wayland style).** EGL cannot render to its
  surfaces. A GPU path would then need GBM and dmabuf, which is more C, not
  less.
- **An exported option (`WindowCfg` or app option) as the flag.** It is API
  surface that must be removed when Wayland becomes the default.
- **A build tag as the flag.** Users would have to rebuild to test, and CI would
  have to build both variants.
- **go-gui draws its own window frame in the backend.** It does not look native,
  and it moves frame drawing into the backend. It can be reconsidered if
  libdecor turns out to be missing on too many systems.
- **Only fixing XWayland with an OpenGL ES path.** Devices with no X11 EGL
  platform (libhybris, #916) still fail.
