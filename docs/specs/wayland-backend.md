# Native Wayland backend

Status: experimental. Issue #919. Phases 2 (test harness), 3 (protocol
bindings), 4 (window), 5 (input) and 6 (desktop parity) of 9 have landed, phase
7 (hardware pass) is done but for HiDPI on Muffin, and phase 8 ships it. With
`GOGUI_WAYLAND=1` an app opens a native Wayland window with a frame, renders at
fractional scales, takes pointer, keyboard, scroll, touch and input method
input, shares the clipboard and primary selection with other apps, sets cursors,
and moves and resizes.

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
- **Size and scale, fractional.** With `wp_fractional_scale_v1` and
  `wp_viewporter` (sway, mutter, KWin), the scale comes in 120ths: the buffer is
  the logical size times the scale, rounded, at buffer scale 1, and the viewport
  shows it at the logical size. Without them the integer buffer scale is used. A
  scale above 8 is clamped.
- **Scale on older compositors.** Muffin 6.6 (Cinnamon) and weston 13 offer
  neither fractional scale nor `wl_compositor` v6, so the compositor never says
  what scale it wants. There every `wl_output` is bound, including one plugged
  in later, and its integer scale is recorded (applied on `done` from v2).
  `wl_surface.enter` and `.leave` track the outputs the window is on. The window
  takes the highest of their scales, as GTK and SDL do. A window on no output
  keeps its scale, and an unplugged output leaves every window, because the
  compositor sends no leave for it. Without this, a 2× output showed a 1× buffer
  stretched inside a sharp libdecor frame. Rejected alternatives: the highest
  scale of all outputs, which oversizes the buffer on a 1× output of a mixed-DPI
  setup, and the desktop's scale setting (`GDK_SCALE`, Cinnamon's
  `scaling-factor`), which is specific to one desktop and does not follow the
  window between outputs.
- **Not yet.** Show/hide, opacity, the window icon and drag and drop are no-ops
  on Wayland.

### Input goes through one seat and the X11 key path

- **One seat for every window.** `wlDisplay` binds the first `wl_seat` and gets
  its pointer, keyboard and touch as the seat's capabilities come and go. Each
  device sends events to the surface its enter event named, so the seat keeps
  the focused window per device and finds it by surface in `wlDisplay.wins`. A
  destroyed window is removed from both.
- **Keys become X11 keysyms.** Wayland sends evdev key codes and a keymap in XKB
  text form. libxkbcommon (package `internal/xkb`, through purego) compiles the
  keymap, tracks the modifier state, and gives the keysym and an X11 modifier
  mask. From there it is the X11 path: `x11key` maps keys and modifiers, and the
  X11 compose machine handles dead keys and Multi_key. libxkbcommon's character
  table is the fallback, so a Cyrillic or Greek layout types text. libxkbcommon
  is loaded before the first window; without it the app uses X11.
- **The client repeats keys.** The compositor sends only press and release. The
  loop fires repeats at the rate and delay of `wl_keyboard.repeat_info` (25/s
  after 600 ms until it arrives) and ends its wait in time for the next one.
  Repeats are key downs with `KeyRepeat` set, plus the character.
- **Scroll is grouped by `wl_pointer.frame`.** A frame whose axes all clicked is
  a wheel: `x11ScrollLines` lines per click, from `axis_value120` (v8) or
  `axis_discrete` (v5–7). Any other frame is a touchpad: `ScrollPrecise`, in
  points. Wayland's positive axis scrolls down, so the sign flips.
- **Touch is grouped by `wl_touch.frame`.** One frame gives, in order, a began,
  a moved and an ended event, like the web backend. All fingers belong to the
  window the first finger touched; a finger on another window at the same time
  is ignored. gui turns a single finger into mouse events.
- **The keymap fd is checked.** The size the compositor gives must fit the file
  (fstat) before it is mapped, because reading a mapping past the end of a file
  raises SIGBUS.

### Decorations

The backend uses xdg-decoration when the compositor offers it (KDE, wlroots).
Otherwise it uses libdecor when it is installed (GNOME, Cinnamon, weston). If
libdecor is missing, the window has no frame, and a warning is printed once.

libdecor without a plugin is the same case in practice: `libdecor_new` succeeds,
and a built-in fallback draws nothing. A desktop can have libdecor without a
plugin. Linux Mint 22 ships `libdecor-0-0` but not `libdecor-0-plugin-1-gtk`.
libdecor has no API to ask whether it found a plugin, so `decor.HasPlugin` looks
where libdecor looks (`$LIBDECOR_PLUGIN_DIR`, else `libdecor/plugins-1` beside
the loaded library) and the backend warns once, naming the package. The window
still goes through libdecor, so a wrong guess costs only the warning.

- libdecor is bound through purego in `internal/decor`, like libxkbcommon. It
  makes the xdg_surface and xdg_toplevel itself, so a libdecor window takes its
  configure, close, title, limits, move and resize through the frame, not the
  toplevel. Its callbacks are made once and route to the window by an id.
- `DecorationNone` asks xdg-decoration for client-side decorations, which here
  means none, and never loads libdecor. `DecorationHiddenTitlebar` gets the
  normal frame, as on X11.
- The loop calls `libdecor_dispatch(0)` each pass, so the GTK plugin's own work
  runs.

### Desktop integration

- **Clipboard and primary selection.** `wl_data_device` and
  `primary-selection-v1`. Copy offers UTF-8 text under the usual MIME types and
  serves pastes from a goroutine with a 5 s write deadline. Paste reads the best
  text type through a pipe, bounded by the X11 limits (1 s, 16 MiB). Text the
  app copied is read back from memory. A copy or paste from another goroutine is
  handed to the loop. A copy names the serial of the last key or button press;
  the compositor refuses an older serial than the current selection's. Without a
  data device (no seat), text stays in the process.
- **Cursors.** `cursor-shape-v1` names the shape and the compositor draws it.
  Without it, the image comes from the Xcursor theme (the X11 code in
  `xcursor.go`), copied into a `wl_shm` buffer; the theme is `XCURSOR_THEME` /
  `XCURSOR_SIZE`, then the GTK settings files. The cursor is set again on every
  pointer enter.
- **Move and resize.** `StartWindowDrag` and `StartWindowResize` send
  `xdg_toplevel.move` and `.resize` (or the libdecor calls) with the serial of
  the button press that is still held.
- **Input methods.** `text-input-v3`. While gui has an editable text focused
  (`IMEStart`) and the text input is on the window, it is enabled with the caret
  rectangle. Preedit and commit, applied at `done`, become the events the IBus
  path emits: `EventIMEComposition` and one `EventChar` with the whole commit.
  The IBus D-Bus client stays off on Wayland.

### Development happens mostly in a headless harness

`scripts/wayland/` runs clients under headless sway, weston and mutter in
Docker, with Mesa llvmpipe. sway supports screenshots (`grim`), injected input
(`wtype`, `wlrctl`) and clipboard tools (`wl-copy`, `wl-paste`), weston supports
screenshots, and mutter covers the no-server-side-decorations path. Tests drive
a pointer and play an input method on their own connection through
`wlr-virtual-pointer` and `input-method-v2`, which sway offers; the bindings for
those two protocols are used only by tests. Only the hardware pass (real GPU,
HiDPI, IME, clipboard with other apps) needs a Linux machine.

### Hardware pass (phase 7)

Linux Mint 22.3, Intel UHD 620 (Mesa 25.2 iris), libwayland 1.22. sway 1.9,
weston 13 and KWin 5.27 run nested in the Cinnamon X11 session. The GL context
on the Wayland EGL platform is the Intel GPU, not llvmpipe.

| Check                                               | sway                | weston                   | KWin                |
| --------------------------------------------------- | ------------------- | ------------------------ | ------------------- |
| `gui/backend/gl` tests (`GOGUI_REQUIRE_WAYLAND=1`)  | 160 pass            | 154 pass                 | 154 pass            |
| Showcase renders                                    | yes                 | yes                      | yes                 |
| Frame                                               | compositor (border) | none: no libdecor plugin | compositor (Breeze) |
| Typing, Ctrl+V from `wl-copy`, Ctrl+C to `wl-paste` | yes                 | –                        | –                   |
| Output scale 2 and 1.5, floating resize, close      | yes                 | –                        | –                   |
| Idle CPU over 5 s (X11: 0 ms)                       | 0 ms                | 0 ms                     | –                   |

Found and fixed:

- `TestWaylandFrameAllocs` measured over 1,300 allocs per frame on its first
  rounds. The allocations were go-glyph's background fallback-coverage warm,
  which parses every system font (647 here, few in the harness image).
  `AllocsPerRun` counts every goroutine. The test now takes the best of up to
  ten rounds. A frame costs 19 allocs, the same as in the harness.
- libdecor with no plugin left windows with no frame and no go-gui warning (see
  Decorations).

Not go-gui:

- sway ignores `resize set` sent right after `output * scale` (no delay). `foot`
  behaves the same way.
- `examples/benchmark` reports a higher FPS under Wayland (431) than under X11
  (112). Its FPS figure is an average of view-to-view rates. Under Wayland the
  loop runs `FrameFn` and presents 61 times a second, paced by frame callbacks.
  Per-frame view, layout and render times match X11. Under load the process uses
  45% of a core on Wayland and 34% on X11.

Cinnamon 6.6 Wayland session (Muffin 6.6.3), checked by hand on the same
machine:

- Without `libdecor-0-plugin-1-gtk`, the window has no frame and the warning
  prints. With the plugin, the window has a GTK frame and moves and resizes.
- IBus input through text-input-v3 works, and so do copy and paste with other
  apps.
- `wl` and `gl` tests: 19 and 154 pass, 0 fail.
- Muffin 6.6 offers neither `wp_fractional_scale_v1` nor `wl_compositor` v6
  (`preferred_buffer_scale`). Its display settings do not change scaling in the
  Wayland session, so HiDPI on Muffin is untested. A nested weston at scale 2
  (also `wl_compositor` v5) showed that go-gui then renders at 1× under a 2×
  libdecor frame. Fixed (see "Scale on older compositors"): weston at scale 2
  now gets `set_buffer_scale(2)` and a 1900×1400 buffer for a 950×700 window.

## Phases

| #   | Phase                                                                                   | State   |
| --- | --------------------------------------------------------------------------------------- | ------- |
| 1   | Issue #919                                                                              | done    |
| 2   | Headless test harness (`scripts/wayland/`, `make wayland-selftest`)                     | done    |
| 3   | Protocol code generation + purego libwayland-client core                                | done    |
| 4   | Window: xdg-shell, `wl_egl_window`, EGL Wayland display, frame callbacks, resize, close | done    |
| 5   | Input: pointer, keyboard through xkbcommon, scroll, touch                               | done    |
| 6   | Decorations, clipboard, fractional scale, cursor-shape, text-input-v3                   | done    |
| 7   | Hardware pass on Linux Mint (Intel/AMD): Cinnamon Wayland, nested sway/weston/KWin      | partial |
| 8   | Ship as experimental                                                                    | started |
| 9   | OpenGL ES renderer path for GLES-only devices (separate issue)                          | pending |

Phase 8 notes:

- CPU under load is no worse than XWayland. The 45% against 34% in the hardware
  pass compared a client inside a nested sway with one on the X11 desktop, so it
  measured the set-up. On one compositor (Cinnamon 6.6 Wayland session),
  `examples/benchmark` used 3.7 s of CPU per 10 s natively and 3.9–4.0 s through
  XWayland. The two 10 s `pprof` profiles match: GL calls through purego take
  about a third of the samples in both, then go-glyph layout and drawing.

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
- **libxkbcommon's compose tables instead of the X11 compose machine.** They
  would also read `~/.XCompose`, but the two backends would then compose
  differently. One machine keeps X11 and Wayland the same; libxkbcommon compose
  can replace both later.
- **Mapping evdev codes to keys without libxkbcommon.** It works only for a US
  layout. The keymap the compositor sends is the user's layout, and only
  libxkbcommon reads it.
- **libwayland-cursor for the fallback cursor.** A third library to load, while
  `xcursor.go` already reads Xcursor themes for X11. Only the `wl_shm` upload is
  new.
- **The IBus D-Bus client on Wayland.** It works on GNOME, but the compositor
  does not know about it: the candidate window cannot be placed, and fcitx5 and
  KDE's input method are left out. text-input-v3 serves every input method the
  compositor runs.
- **Reading our own selection through a pipe.** The source is served on the
  loop's thread, which would be blocked in the read until the timeout. Text the
  app copied is returned from memory.
- **Only fixing XWayland with an OpenGL ES path.** Devices with no X11 EGL
  platform (libhybris, #916) still fail.
