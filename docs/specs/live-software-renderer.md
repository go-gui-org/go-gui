# Live software renderer

Issue #1265. Status: **in progress** (step 2 of 5 landed).

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
| 3    | Auto-pick: `GL_RENDERER` names a software rasterizer, or EGL fails | planned |
| 4    | Wayland present path with `wl_shm`                                 | planned |
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
with one log line, until step 4.

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

## Tests

- `gui/backend/soft`: `TestRendererMatchesPixelGoldens` (byte-identical to
  `RenderToImage` for every pixel-golden case),
  `TestRendererColdAtlasSinglePass`, `TestRendererReusesBuffer`,
  `TestRendererResize`, `TestRendererSetScale`,
  `TestRenderToImageRescaleKeepsMeasurer`, `TestRendererRejectsBadInput`,
  `TestRendererAfterRelease`, `TestRendererImageCacheBounded`,
  `TestRendererMemImageReregistered`, `TestUpdateTextureRect*`.
- `gui/backend/gl`, pure: `TestSoftRequested`, `TestSoftDeclinesWayland`,
  `TestPickSoftVisual`, `TestSoftImageFormatOK`, `TestBandRows`,
  `TestSwizzleBGRA`, `TestModeRefresh`, `TestGPUContextErrorNamesSoftOnLinux`.
- `gui/backend/gl`, against an X server (`tools/glrendertest` runs them in CI
  under Xvfb, one process each): `TestSoftX11NeverTouchesEGL` drives frames, a
  resize, a scale change, Expose and destroy with no EGL state created;
  `TestSoftX11PixelsMatchRenderToImage` reads the window back with `GetImage`
  and compares it with `RenderToImage`, which covers the band split and the
  swizzle.

## Rejected approaches

- **A `presenter` interface that GL and soft implement.** It would move the GL
  frame, resize and teardown code behind the interface on Windows too, for one
  second implementation. Revisit at step 4, if the Wayland soft path spreads the
  mode checks past about a dozen sites.
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
