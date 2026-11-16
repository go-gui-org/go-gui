# Live software renderer

Issue #1265. Status: **in progress** (step 4 of 5 landed).

## Problem

On Linux without a GPU (VMs, remote desktops, thin clients, CI under Xvfb), EGL
falls back to Mesa's llvmpipe (`gui/backend/gl/egl_linux.go`, #916). llvmpipe
emulates a GPU pipeline on the CPU. go-gui already has a CPU renderer,
`gui/backend/soft`, but before #1265 only screenshots and golden tests could use
it.

## Measurement (step 1)

Laptop with 4 cores and an Intel UHD 620, at 1×, 3 runs per cell, median. Full
tables are in the issue comments.

| Case                   | llvmpipe       | GPU (iris)    | soft, estimated (offline render + swizzle) |
| ---------------------- | -------------- | ------------- | ------------------------------------------ |
| showcase idle          | 0 % / 191 MB   | 0 % / 142 MB  | ≈ 0 % / ~100 MB                            |
| showcase spinner page  | 285 % / 203 MB | 27 % / 153 MB | ~98 %                                      |
| benchmark, 100 buttons | 276 % / 206 MB | 32 % / 144 MB | ~93 % at 110 fps                           |

llvmpipe costs 2–3× soft's CPU while animating and about 2× its memory, so the
work went ahead. The ratio is far below what CUI reported for its own renderer
(10× CPU, 14× memory).

Step 2 measured the real present path against llvmpipe on the same server
(XWayland on Cinnamon, `GOGUI_WAYLAND=0`), same laptop, 3 runs, median:

| Case                   | llvmpipe                | soft (`GOGUI_RENDERER=soft`) |
| ---------------------- | ----------------------- | ---------------------------- |
| showcase idle          | 0 % / 191 MB            | 0 % / 82 MB                  |
| showcase spinner page  | 262 % / 201 MB          | 105 % / 99 MB                |
| benchmark, 100 buttons | 250 % / 206 MB, 102 fps | 80 % / 78 MB, 120 fps        |

On a native X11 session (Cinnamon, same laptop, 3 runs, median; the llvmpipe and
GPU rows are step 1's X11 runs):

| Case                   | llvmpipe                | soft                  | GPU (iris)             |
| ---------------------- | ----------------------- | --------------------- | ---------------------- |
| showcase idle          | 0 % / 191 MB            | 0 % / 83 MB           | 0 % / 142 MB           |
| showcase spinner page  | 285 % / 203 MB          | 105 % / 100 MB        | 27 % / 153 MB          |
| benchmark, 100 buttons | 276 % / 206 MB, 113 fps | 76 % / 77 MB, 120 fps | 32 % / 144 MB, 120 fps |

Soft paced to the panel's RandR mode there: 16.650 ms, the 60.06 Hz of the eDP-1
mode, read from the window's CRTC.

Under Xvfb soft measured 99 % on the spinner page and 72 % on the benchmark at
117 fps. Soft ran 9–10 threads to llvmpipe's 18, and maps no `libEGL`. The step
1 estimate held: llvmpipe costs 2.5–3× the CPU and over 2× the memory, and soft
also draws more frames.

A finding that shapes step 3: on a Cinnamon Wayland session the compositor gives
clients no GPU device, so go-gui lands on llvmpipe on hardware whose GPU works
under X11. Auto-pick has to decide from the `GL_RENDERER` string, not from
whether a GPU exists.

## Phases

| Step | What                                                               | Status  |
| ---- | ------------------------------------------------------------------ | ------- |
| 1    | Measure llvmpipe against soft                                      | done    |
| 2    | X11 present path, `GOGUI_RENDERER=gl\|soft`                        | done    |
| 3    | Auto-pick: `GL_RENDERER` names a software rasterizer, or EGL fails | done    |
| 4    | Wayland present path with `wl_shm`                                 | done    |
| 5    | Zero allocations per frame on the present path                     | planned |

Scope decision: **1× first.** The renderer draws the full frame on every redraw
on one thread. At 2× the showcase spinner page takes about 52 ms per frame
(about 19 fps). HiDPI needs damage tracking or threading, which is a later
issue.

## Design (step 2)

**The env var.** `GOGUI_RENDERER=soft` selects soft. `gl`, an empty value or
anything else keeps GL; matching is exact and case-sensitive, like
`GOGUI_WAYLAND`. Step 3 changes only the unset case. On a Wayland session soft
declines the native Wayland backend (`waylandRequested`) and runs on XWayland,
with one log line, until step 4. Step 4 removed the decline: soft now runs on
native Wayland.

**A mode on the existing backend.** `platformState.softMode` is set before
anything can fail, so `Destroy` never reaches GL teardown on a soft window.
`newX11` skips the EGL display, the EGL config visual pick, the surface and
context, and the GL function table. The window's visual comes from
`pickSoftVisual`: a TrueColor visual with masks `0xff0000`, `0xff00` and `0xff`
(the root visual when it fits; depth 32 for a transparent window), on a server
whose ZPixmap for that depth is 32 bpp and little-endian. Anything else fails
with an error naming the problem instead of drawing swapped channels. All the
X11 event, IME, clipboard, DPI, cursor and window-state code is shared with GL.
The soft-specific sites are thin `*X11` wrappers in `softpresent_x11.go`:
`presentX11`, `resizeX11`, `applyScaleX11`, `exposeX11` and `monitorChangedX11`.

**`soft.Renderer`.** A backend keeps one per window. It draws the render
commands the window already holds, in one pass, into one reused buffer, and
keeps the decoded-image cache and the layer pool across frames. Because that
cache now lives as long as the window, it is the GL texture cache's bounded LRU:
by entry count and by `WindowCfg.MaxTextureBytes` (#1239), with `mem:` entries
keyed on the registry serial and swept when a registration leaves (#1230). It
does not set the headless flag or run `TestRender`, so animations stay live.
`RenderToImage` shares its draw core (`newFrameRenderer`, `bind`, `drawPass`)
and keeps its exact behaviour.

**One pass is enough because of go-glyph's mid-frame upload.** go-glyph uploads
newly rasterized glyphs before it emits the quads that sample them, but only to
a backend that implements `glyph.RectTextureUpdater`. The soft glyph backend now
does (`UpdateTextureRect`). Without it, a single-pass frame draws new glyphs
blank: `TestRendererColdAtlasSinglePass` fails with about 12,000 differing
pixels when the method is removed.

**Scale changes keep the text system.** `Renderer.SetScale` moves the window's
text system to the new density with `glyph.TextSystem.SetDPIScale`, as the GL
backend's `applyDPIScale` does; that purges go-glyph's layout cache, glyph cache
and atlas. Building a new system and freeing the old one would leave the
window's persisted text blocks holding layouts from the freed system.
`RenderToImage`'s `prepare` had that bug and takes the same path now; only a
scale go-glyph refuses (above 10) still rebuilds there.

**Upload.** The frame is swizzled from premultiplied RGBA to BGRA into one
reused band buffer and sent with core `PutImage` in row bands. xgb's `PutImage`
writes a 16-bit request length, so a request cannot exceed the server's
`MaximumRequestLength` (usually 256 KB). BIG-REQUESTS is out of reach without a
request writer of our own.

**Pacing.** GL is held to the display by `eglSwapInterval`. Soft has no vsync,
so after each frame it makes a `GetInputFocus` round trip, which gives the same
backpressure as a swap, and then sleeps out the rest of the monitor's frame
interval. The interval comes from the RandR mode of the window's CRTC (dot clock
/ (htotal × vtotal)), with 60 Hz as the fallback. It is re-read when the DPI
tracking sees the window move to another CRTC. The CRTC is resolved from the
window's position even when `Xft.dpi` sets the scale or the monitor's EDID size
is implausible: either would otherwise report no CRTC and pace every monitor at
the first one's rate. `WindowCfg.VSyncOff` skips the sleep.

**Expose.** The GL path ignores Expose. A soft window on a server without
backing store (Xvfb, an X session without a compositor) would stay damaged, so
it puts its last frame back when the last Expose of a series arrives.

**No-GPU advice.** On Linux the no-GPU-context error now ends with "or set
`GOGUI_RENDERER=soft`".

## Design (step 3)

**One probe per process, before the first X11 window.** With `GOGUI_RENDERER`
unset, `pickRenderer` (`gui/backend/gl/renderer_pick_linux.go`) runs
`probeGLRenderer` once and caches the choice. The probe opens the EGL display
the window would use, makes a GL 3.3 core context current with no surface
(`EGL_KHR_surfaceless_context`, which Mesa offers on every driver), reads
`GL_RENDERER` with `glGetString`, and destroys the context. `newX11` calls it
after its X connect and after the thread lock: the context is current on the
calling thread, and under Xvfb an EGL connect with no X client open can race the
server reset that the previous client's exit started.

**The decision.**

| Probe result                                                       | Choice | Why                                                            |
| ------------------------------------------------------------------ | ------ | -------------------------------------------------------------- |
| `GL_RENDERER` contains llvmpipe, softpipe, lavapipe or swiftshader | soft   | GL would draw on the CPU anyway, at 2.5–3× soft's CPU          |
| EGL display fails                                                  | soft   | The window would fail at the same call; soft needs no EGL      |
| The probe's own step fails (context create, make-current, string)  | GL     | The window has a surface and its own config; the default stays |
| Anything else                                                      | GL     | Hardware GL                                                    |

Matching is a case-insensitive substring test, because the strings carry
versions (`llvmpipe (LLVM 15.0.7, 256 bits)`, `zink (lavapipe)`). A soft choice
logs one line that names the reason and `GOGUI_RENDERER=gl`, and that custom
shaders are not drawn.

**The variable.** `soft` and `gl` skip the probe. `gl` keeps the step 2
behaviour: an EGL failure is `ErrNoGPUContext`. Any other non-empty value keeps
GL with one warning, so a typo (`Soft`) neither costs a probe nor picks a
renderer the user did not name.

**Cost on a GPU start.** One context create and destroy. A hardware result
leaves the display initialized; `newX11` initializes the same display again,
which EGL treats as a no-op. A soft result or an EGL failure terminates the
display. That frees the display's objects but not the driver: Mesa keeps it
mapped, about 70 MB with llvmpipe (measured at step 4; see Known limits).

**Wayland keeps GL** (superseded by step 4). Auto-pick runs on the X11 path
only. A Wayland session on llvmpipe (the Cinnamon case above) stays on native
Wayland GL until step 4. Moving users off native Wayland is a separate decision
from picking a renderer. A Wayland session whose Wayland setup fails reaches X11
and probes there.

**An X server with no soft visual.** `newX11` checks the soft visual before it
touches EGL, and `softVisualRejected` decides what a rejection means. After an
auto-pick from `GL_RENDERER`, GL draws, with one log line: GL worked on that
server (16-bit depth, big-endian) before auto-pick existed. After an auto-pick
from an EGL failure, neither renderer can run; the error is the GPU error first,
so it still matches `ErrNoGPUContext`, with the soft cause after it. A named
`GOGUI_RENDERER=soft` returns the soft cause.

**A soft window that fails to start.** `newX11` wraps a failure of the CPU
renderer's own setup in `softInitError`. After an auto-pick from `GL_RENDERER`,
it reopens the window on GL with one log line. After an auto-pick from an EGL
failure, the error is the GPU error first (it matches `ErrNoGPUContext`, without
the advice to set `GOGUI_RENDERER=soft`) with the soft cause after it. The probe
creates its context on the first EGL config, but the window uses the config that
`pickVisual` chooses (ARGB when `Transparent`), so a context failure in the
probe keeps GL instead of choosing soft for the whole process.

## Design (step 4)

**Mode branches, as on X11.** A soft Wayland window has a non-nil
`wlWindow.soft` (`wlSoftPresent`, `gui/backend/gl/softpresent_wayland_linux.go`)
and no EGL window, surface or context. The Wayland code branches at five sites:
`openWaylandDisplay` (no EGL on a named soft, the probe on an auto one),
`newWaylandBackend` (`initSoftWayland` or `initGLWayland`), `wlWindow.resize`
(`resizeWayland`), the loop's frame call (`presentWayland`) and
`destroyWayland`. With the X11 wrappers that is about 13 call sites, past the
"about a dozen" at which this spec said to reconsider a `presenter` interface.
It was reconsidered and kept as branches; see Rejected approaches.

**What Wayland does not need.** Pacing comes from the frame callbacks the loop
already waits for, as for GL. The compositor keeps the last buffer, so there is
no Expose repaint. There is no visual to pick: `wl_shm` always offers ARGB8888
and XRGB8888, whose little-endian memory order is the B, G, R, A that
`swizzleBGRA` writes (the `wl` package builds only for amd64 and arm64).

**Buffers.** `wlSlotRing` keeps up to three `wl_shm` buffers, made on demand,
each in its own memfd and mapping; the pool is destroyed at once and the fd
closed. A slot is busy from its commit until `wl_buffer.release`. A frame takes
a free slot of the frame's size; with all three held it is skipped, the window
stays dirty, and the loop blocks until the next event, which the release is.
Free slots of another size are freed on the next acquire, so a resize does not
keep the old frames' memory. The frame is swizzled straight into the mapped
memory: one copy, no band buffer and no allocation once the slots exist. An
opaque window takes XRGB8888, so the compositor ignores alpha, which soft does
not force to 1 as GL's `finishFrameAlpha` does. A transparent window takes
ARGB8888, premultiplied as Wayland expects. Damage is `DamageBuffer` over the
whole buffer on `wl_surface` v4 and later, `Damage` in logical pixels before.
Scale handling (viewport or integer buffer scale) is the GL path's code;
`resizeWayland` moves the renderer to the new scale.

**The choice on Wayland.**

| `GOGUI_RENDERER` | Wayland display                                                                                                                                                |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `soft`           | No `libwayland-egl`, no EGL. Native Wayland, no longer XWayland. No `wl_shm` fails the Wayland setup, and X11 soft takes over                                  |
| `gl`, other      | GL, no probe                                                                                                                                                   |
| unset            | `probeContext` on the Wayland EGL display. A software rasterizer picks soft and terminates the display; no `wl_shm` keeps GL. An EGL failure falls back to X11 |

The probe is per display, not shared with the X11 probe's process cache: on
Cinnamon Wayland the Wayland EGL display reports llvmpipe while the X11 one
reports the GPU, and a Wayland result must not decide for an X11 window that the
Wayland backend fell back to. `readRendererEnv` is the one reader of the
variable for both paths. `waylandRequested` no longer looks at `GOGUI_RENDERER`:
the windowing backend and the renderer are chosen separately.

**Measured** on the step 1 laptop, Cinnamon (muffin) Wayland session, native
Wayland, 1×, 3 runs, median. The compositor hands clients llvmpipe there.

| Case                                  | llvmpipe       | soft, auto-picked | soft, `GOGUI_RENDERER=soft` |
| ------------------------------------- | -------------- | ----------------- | --------------------------- |
| showcase idle                         | 0 % / 222 MB   | 0 % / 181 MB      | 0 % / 112 MB                |
| benchmark, 100 buttons, forced frames | 201 % / 237 MB | 64 % / 187 MB     | 64 % / 117 MB               |

Soft runs 14–15 threads to llvmpipe's 23–24. The auto-picked rows keep about 70
MB more than the named ones: the probe loads the GL driver, and Mesa keeps it
mapped after `eglTerminate` (`eglReleaseThread` frees none of it). The X11
auto-pick has the same cost (showcase 147 MB against about 85 MB named).

## Tests

- `gui/backend/soft`: `TestRendererMatchesPixelGoldens` (byte-identical to
  `RenderToImage` for every pixel-golden case),
  `TestRendererColdAtlasSinglePass`, `TestRendererReusesBuffer`,
  `TestRendererResize`, `TestRendererSetScale`,
  `TestRenderToImageRescaleKeepsMeasurer`, `TestRendererRejectsBadInput`,
  `TestRendererAfterRelease`, `TestRendererImageCacheBounded`,
  `TestRendererMemImageReregistered`, `TestUpdateTextureRect*`.
- `gui/backend/gl`, pure: `TestPickSoftVisual`, `TestSoftImageFormatOK`,
  `TestBandRows`, `TestSwizzleBGRA`, `TestModeRefresh`,
  `TestGPUContextErrorNamesSoftOnLinux`, and for step 3 `TestPickRendererEnv`,
  `TestAutoPick`, `TestIsSoftwareRasterizer`, `TestSoftVisualRejected`.
- `gui/backend/gl`, against an X server (`tools/glrendertest` runs them in CI
  under Xvfb, one process each): `TestSoftX11NeverTouchesEGL` drives frames, a
  resize, a scale change, Expose and destroy with no EGL state created;
  `TestSoftX11PixelsMatchRenderToImage` reads the window back with `GetImage`
  and compares it with `RenderToImage`, which covers the band split and the
  swizzle. `TestAutoPickSoftOnSoftwareRasterizer` runs the real probe on the
  runner's llvmpipe and requires a soft window with no EGL state;
  `TestAutoPickSoftOnEGLFailure` requires a soft window when EGL fails;
  `TestAutoPickSoftInitFailureFallsBackToGL` and
  `TestAutoPickSoftInitFailureAfterEGLFailure` fail the soft setup on purpose.
  glrendertest sets `GOGUI_RENDERER=gl` for every other test, so the GL tests
  stay on GL.
- Step 4, pure: `TestSlotRing*` (reuse after release, growth to the cap and the
  skipped frame, resize, alloc failure, destroy), `TestWlShmSize`,
  `TestWlShmFormat`, `TestCopyBGRA`, `TestWaylandAutoSoft`,
  `TestReadRendererEnv`, `TestSoftKeepsWayland`.
- Step 4, against a compositor (`scripts/wayland/test.sh`, or a Wayland
  desktop): `TestSoftWaylandNeverTouchesEGL` drives frames, a resize and two
  scale changes with no EGL state and requires buffers to come back through
  their releases; `TestSoftWaylandPixelsMatchRenderToImage` compares the
  committed shared memory with `RenderToImage`; `TestSoftWaylandAutoPick`
  requires soft with no EGL display where the compositor's EGL is a software
  rasterizer. `requireWayland` pins `GOGUI_RENDERER=gl` when unset, so the GL
  Wayland tests stay on GL on an llvmpipe compositor.

## Rejected approaches

- **Auto-pick per window, after the real context exists** (step 3). The window
  is already mapped with a visual EGL chose; switching it to soft needs a second
  teardown path and a recreated window, and the check repeats per window.
- **Auto-pick from the hardware** (`/dev/dri` render nodes,
  `LIBGL_ALWAYS_SOFTWARE`) with no GL probe (step 3). Wrong on the Cinnamon
  Wayland case: the GPU exists and EGL still hands out llvmpipe.
- **Auto-pick moving a Wayland session to XWayland soft** (step 3). It would cut
  CPU and memory on llvmpipe, but it changes the windowing backend, not only the
  renderer. Wayland waits for its own soft path (step 4).

- **A `presenter` interface that GL and soft implement.** It would move the GL
  frame, resize and teardown code behind the interface on Windows too, for one
  second implementation. Revisited at step 4, at about 13 call sites: kept as
  branches. Each site is a one-line delegation, the two soft paths share little
  (pacing, repaint and upload all differ), and the interface would move every
  Linux GPU user's frame path for no change in behaviour. Revisit if a third
  present path (Windows `SetDIBitsToDevice`) arrives.
- **A shared soft core with an upload interface** (step 4): the renderer, the
  error logging and the last frame shared, X11 `PutImage` and `wl_shm` behind
  one interface. The shared part is about 30 lines; the indirection costs more
  to read than it saves.
- **One `wl_shm` buffer, drawn after its release** (step 4). It stalls, or
  tears, on a compositor that holds a buffer until the next commit.
- **One growing pool with buffers carved from it** (step 4).
  `wl_shm_pool.resize` can only grow, so a window that shrinks needs a new pool
  anyway.
- **Soft on native Wayland when the Wayland EGL display fails** (step 4).
  Decided against: the Wayland setup fails and falls back to X11 as before,
  where the X11 probe decides.
- **A separate `x11soft` backend package.** It duplicates or exports the X11
  platform layer (about 3k lines), and two event layers drift apart.
- **Moving the soft renderer core to an internal package** so `gl` can use it
  with nothing exported. It moves about 3k lines and their tests for the sake of
  one exported type in a package that already exports a renderer.
- **A registration hook** (`soft` sets a function variable in a shared internal
  package from `init`). `gl` would silently lose soft whenever nothing imports
  the `soft` package.
- **Redraw when glyphs were uploaded** (a second pass on frames that add
  glyphs). Superseded by the mid-frame upload above, which needs no extra pass.
- **MIT-SHM now.** It needs SysV `shmget`/`shmat` syscalls and a fallback for
  remote X. It belongs in step 5, where the zero-allocation goal pays for it.
- **Soft drawing straight into BGRA** to skip the swizzle. It touches every draw
  routine to save about 1.3 ms per frame at 1×. Revisit with numbers.
- **`WindowCfg.Renderer`** (from the issue). The renderer is a property of the
  machine, not of the app; a Cfg field invites apps to hard-code it.
- **Damage tracking and multi-threaded tiles in the first round.** Damage
  tracking needs every paint to be deterministic for the same state. Measure the
  full-frame path first; HiDPI is a later issue.

## Known limits

- `RenderCustomShader` is GLSL; soft draws a shader container's children and no
  shader fill, as in soft screenshots. `GOGUI_RENDERER=gl` forces GL for an app
  that needs it.
- xgb's `PutImage` allocates a request buffer per band, about one frame's size
  per frame. Step 5 removes it.
- Windows has no soft path (out of the first round, per the issue).
- An auto-picked soft window keeps the GL driver the probe loaded, about 70 MB
  with llvmpipe, on X11 and Wayland. `GOGUI_RENDERER=soft` loads none.
