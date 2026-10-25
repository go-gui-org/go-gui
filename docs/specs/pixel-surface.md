# Pixel surfaces and shader texture inputs

Issue #1168. Status: **phase 1 landed** (`PixelSurface` on the image path).
Phase 2 (planes and `Shader` texture inputs) pending.

Context: go-gui-org/go-video#1, where the video player moved into its own
repository. go-gui keeps only generic, video-agnostic hooks; everything YUV
(NV12 layout, colour matrices, conversion shaders) lives in go-video.

## Problem

An image could only change by registering a new one. A video player, camera
preview or emulator screen publishes a frame 30–60 times a second from a
producer goroutine. Through `UseImage` that means a new content key and a new
GPU texture per frame: texture churn, cache pressure and a layout rebuild for
every frame.

## Phasing

| Phase | Deliverable                                                                                                  | Status  |
| ----- | ------------------------------------------------------------------------------------------------------------ | ------- |
| 1     | `PixelSurface` (RGBA8) as `ImageCfg.Surface` on all six backends and PDF, `examples/pixel_surface`           | Landed  |
| 2     | Planes (`R8`, `RG8`, `RGBA8`) published together, `Shader.Surface` → `iChannel0..3`, `Window.RendersShaders` | Pending |

## Phase 1: `PixelSurface`

- **Owned, copy-on-update.** `Update(pix, stride)` copies straight-alpha RGBA8
  pixels into storage the surface owns and returns. The producer may reuse its
  buffer at once. Only the newest frame is kept: there is no queue, so a slow
  renderer drops frames instead of falling behind.
- **One lock, no leases.** Renderers read through `ReadPixels`, which visits the
  pixels under the surface lock, and upload before returning. An `Update` may
  wait for an upload in progress. This keeps ownership explicit and needs no
  second buffer.
- **Versions, per renderer.** Every publish bumps a version. Each renderer
  remembers the version it uploaded and uploads only when it changed, so two
  windows (or two backends) each see the newest frame and an unchanged frame
  costs nothing.
- **Wake without layout.** A window subscribes to the surfaces its image
  commands draw (`syncPixelSurfaces`, after each render pass) and `Update` wakes
  it with `InvalidateRender`. Subscriptions are dropped when the view stops
  drawing the surface and when the window is destroyed. A window that draws no
  surface skips the subscription pass entirely.
- **Renderer cache.** `gui/backend/internal/surfacecache` keeps one native
  resource per surface, on top of `texcache`. Each frame it releases the
  resources of surfaces that were not drawn or were closed. A fixed-size update
  reuses the texture (`glTexSubImage2D`, `replaceRegion`, `putImageData`).
  Android context loss forgets the handles and uploads again from the CPU copy.
  The backend binds its release function once at setup (`SetRelease`): a method
  value passed and stored on every frame would be allocated on every frame,
  surfaces or not.
- **Bounds.** 64 MiB of pixels per surface and 8192 pixels per edge.
- **Size.** The field lives in `shapeEffects`, not `Shape`, so `Shape` stays in
  its 320-byte size class (`TestShapeFitsSizeClass`). `RenderCmd` gains one
  pointer.

Verification: unit tests for ownership, versions, invalid input, concurrent
publication (race), window subscription lifecycle and zero-allocation `Update`;
cache tests for per-consumer versions, prune, context loss and zero-allocation
warm frames; a real-GL pixel test in `tools/glrendertest`; a soft-renderer pixel
test; a golden in both themes; a PDF export test.

## Phase 2: planes and shader texture inputs (pending)

Decided in #1168:

- A `PixelSurface` holds 1–4 planes (`R8`, `RG8`, `RGBA8`), each with its own
  size, published together under one version.
- `Shader.Surface` binds a surface: plane _i_ → `iChannel`_i_, with
  `iChannelResolution[i]` set to the plane size. Unused channels get a 1×1
  transparent texture. Sampling is linear, clamp to edge, top-left origin.
- `Window.RendersShaders()` lets a library choose between the shader path and an
  `RGBA8` image fallback (soft backend, PDF, WebGL2 missing) without asking the
  user.

## Rejected approaches

- **A versioned, mutable `UseImage` registration.** Fewer copies, but the caller
  must keep its storage alive until every renderer has uploaded, and reusing a
  key can show stale data. It would change the immutable contract every existing
  `UseImage` caller relies on.
- **A renderer frame-provider interface** (the renderer pulls frames and
  releases them). It could pass decoder frames through with fewer copies, but a
  render callback can block, and a missed or doubled release stalls the decoder
  or reuses live storage. Deferred until real native-frame interop needs it; see
  the zero-copy item in go-gui-org/go-video#1.
- **A new image key and texture per frame.** Texture churn and cache pressure;
  acceptable only for a throwaway diagnostic.
- **One surface per shader channel** (phase 2). NV12 publishes Y and then UV; a
  render between the two would draw the new Y with the old UV.
- **A video-specific surface type in `gui/`** (an `NV12Surface`, colour
  matrices, per-backend NV12 shaders). Keeps video concepts in go-gui; the
  planes and shader inputs above carry the same frames generically.
- **A raw native render callback** (GL context or Metal encoder handed to app
  code). Lets outside code break render state, clips and z-order, and every
  backend refactor would break it. A later zero-copy step imports external
  textures into the same `iChannel` binding instead.
