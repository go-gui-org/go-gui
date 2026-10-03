# Native Wayland backend

Status: in progress, experimental. Issue #919. Phases 2 (test harness), 3
(protocol bindings) and 4 (window) of 9 have landed. With `GOGUI_WAYLAND=1` an
app opens a native Wayland window and renders, but takes no input yet.

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
- `SetHandlers` builds a decoder closure on each call. A proxy made every frame
  (`wl_callback`) instead takes a `<Type>Dispatcher` built once with
  `Handlers.Dispatcher()`, so a frame allocates no closure for it.
- A handler's panic is recovered before it can unwind through libwayland's C
  frames, and raised again from the Go call that ran the dispatch.
- The package builds for linux/amd64 and linux/arm64 only: a `wl_argument` is
  read as one 8-byte little-endian slot.

### A Wayland window is a Backend with one extra pointer

`platformState` (the Linux platform state) has a `wl *wlWindow` field. When it
is set, the window lives on Wayland and the X11 fields stay zero. The renderer
reads only the EGL handles, the drawable size and the scale, so it has no
Wayland branch. The X11 entry points the gui layer can reach (ShowWindow,
opacity, cursor, IME) already return early on a nil X connection. The branches
are few: `New` tries Wayland first, `Run` and `RunApp` pick the Wayland loop,
and `destroy` frees Wayland objects.

- **One connection for all windows.** `wlDisplay` holds the connection, the
  `wl_compositor` and `xdg_wm_base` globals and the EGL display. Windows count
  references to it; the last one disconnects. One loop drives every window.
- **Fallback.** Connecting, binding the globals, loading libwayland-egl and
  initializing EGL all happen before the first window. If any of them fails, a
  warning is printed once and the app uses X11.
- **Frame pacing.** `eglSwapInterval` is 0, so `eglSwapBuffers` never blocks in
  Mesa. Instead the loop asks for a `wl_surface.frame` callback before each swap
  and gives the window no new frame until it arrives. A compositor sends no
  callback to a hidden window; after 1 s the window renders anyway, at most once
  a second. `WindowCfg.VSyncOff` turns pacing off.
- **Waking.** `wl.Conn` polls an eventfd beside the compositor socket. `Wake` is
  safe on any goroutine; gui redraw requests, `App.OpenWindow` and `SetTitle`
  use it. `SetTitle` stores the title and the loop sends it, because the
  bindings are single-threaded.
- **Size and scale.** The size comes from `xdg_toplevel.configure`; zero keeps
  the current size. The integer buffer scale comes from
  `wl_surface.preferred_buffer_scale` (wl_surface v6). The whole surface is
  marked opaque unless `WindowCfg.Transparent` is set, so the alpha channel of
  the EGL config does not show.
- **Not yet.** Input is phase 5. Decorations, the system clipboard (copy and
  paste work inside the process only), fractional scale and cursors are phase 6.
  Show/hide and opacity are no-ops on Wayland.

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
| 4   | Window: xdg-shell, `wl_egl_window`, EGL Wayland display, frame callbacks, resize, close | done    |
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
- **A platform interface with an X11 and a Wayland implementation.** Cleaner on
  paper, but over 130 direct `b.plat` field uses in the X11 files would move
  behind methods, and X11 would be rewritten to add Wayland. The pointer field
  keeps X11 untouched.
- **Blocking vsync in `eglSwapBuffers` (swap interval 1).** Mesa then waits for
  the frame callback inside the swap, and a hidden window blocks the loop, and
  with it every other window, forever.
- **Only fixing XWayland with an OpenGL ES path.** Devices with no X11 EGL
  platform (libhybris, #916) still fail.
