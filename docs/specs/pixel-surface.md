# Pixel surfaces and shader texture inputs

Issue #1168. Status: **phases 1 and 2 landed** (`PixelSurface` on the image
path; planes and `Shader` texture inputs). Zero-copy import is a later, separate
issue.

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

| Phase | Deliverable                                                                                                  | Status |
| ----- | ------------------------------------------------------------------------------------------------------------ | ------ |
| 1     | `PixelSurface` (RGBA8) as `ImageCfg.Surface` on all six backends and PDF, `examples/pixel_surface`           | Landed |
| 2     | Planes (`R8`, `RG8`, `RGBA8`) published together, `Shader.Surface` → `iChannel0..3`, `Window.RendersShaders` | Landed |

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
- **Metal ordering.** OpenGL drivers order `glTexSubImage2D` against draws
  already submitted; Metal's `replaceRegion` does not: it copies at once, even
  while a committed frame still samples the texture. Surface textures are
  therefore _streamed_ (`metalStreamTexture`): binding one records the frame's
  command buffer, and the next upload (`metalUpdateStreamTexture`) first waits
  for that buffer if it was already submitted. Only the render thread waits, for
  at most the frame still running; `Update` never waits on the GPU, and a
  producer that outpaces the GPU only replaces the frame that will be uploaded
  next. Every streamed upload goes through one helper per Metal backend
  (`waitTextureIdle`), which phase 2's planes reuse. The video spike's NV12 path
  needed the same wait. The glyph atlas keeps plain `metalUpdateTexture`: its
  uploads leave the pixels of glyphs already in use unchanged, so an in-flight
  frame cannot see a difference.
- **Bounds.** 64 MiB of pixels per surface and 8192 pixels per edge.
- **Size.** The field lives in `shapeEffects`, not `Shape`, so `Shape` stays in
  its 320-byte size class (`TestShapeFitsSizeClass`). `RenderCmd` gains one
  pointer.

Verification: unit tests for ownership, versions, invalid input, concurrent
publication (race), window subscription lifecycle and zero-allocation `Update`;
cache tests for per-consumer versions, prune, context loss and zero-allocation
warm frames; a real-GL pixel test in `tools/glrendertest`; a soft-renderer pixel
test; a golden in both themes; a PDF export test.

## Phase 2: planes and shader texture inputs

- **Planes.** `PixelSurfaceCfg.Planes` gives a surface 1–4 planes (`R8`, `RG8`,
  `RGBA8`), each with its own size; without it a surface is one `RGBA8` plane,
  as in phase 1. `UpdatePlanes` checks every source before copying any, copies
  all planes and bumps one version, so a renderer never pairs one plane's new
  pixels with another's old ones. It allocates nothing at a fixed size. `Update`
  and `Resize` stay single-plane. `ReadPlanes` lends the planes to a renderer
  under the surface lock; `ReadPixels`, the image path, serves only a single
  `RGBA8` plane, so an `Image` of any other surface draws nothing.
- **Shader inputs.** `Shader.Surface` binds plane _i_ as `iChannel`_i_. Only a
  shader with a `Surface` gets the channel declarations
  (`BuildGLSLFragmentChannels`, `BuildGLSLESFragmentChannels`,
  `msl.BuildCustomChannels`); every other shader compiles byte-identical source
  to before, checked against `main` when this landed. `ShaderHash` and its
  per-window memo tell the two wrapper variants apart. `iChannelResolution` is
  filled in the wrapper from the bound textures (`textureSize`,
  `get_width`/`get_height`), so it needs no uniform: the built-in block has only
  six free slots. GLSL binds plane _i_ to texture unit _i_+1 (unit 0 is the
  wrapper's `tex`); MSL takes the planes as `fs_main` parameters at
  `texture(1)`..`texture(4)` with a `constexpr` linear, clamp-to-edge
  `iChannelSampler`. A channel with no plane, or every channel of a closed
  surface, reads a 1×1 transparent texture.
- **Redraw and caches.** A shader command's surface is its `Shader.Surface`, an
  image command's its `RenderCmd.Surface`: one source per kind, read through
  `RenderCmd.DrawnSurface`. Window subscriptions, cache pruning and the
  backends' channel binding all read it, so they cannot disagree, and a new
  frame redraws a shader without layout or `Animate`. Each backend keeps a
  second `surfacecache` for plane textures, pruned by shader commands
  (`PruneShaders`) while the image cache is pruned by image commands, and
  resolved with `ResolvePlanes`.
- **Backends.** GL, WebGL2 (in the shader renderer's own context), Android GLES
  (C bridge), Metal and iOS create `R8`/`RG8`/`RGBA8` textures and upload with
  an unpack alignment of 1, since single-channel rows are not 4-byte aligned.
  Metal plane textures are streamed like phase 1's, through the same
  `waitTextureIdle`, so video's planes take the one Metal synchronization path.
- **`Window.RendersShaders`.** Each GPU backend reports it once its shader path
  is ready (`SetWindowRendersShaders`); the web backend only with WebGL2. Soft,
  headless test windows and PDF export report false.

Verification: plane tests (atomic publication, rejected input leaves the frame
unchanged, no torn planes under `-race`, zero-allocation `UpdatePlanes`);
wrapper tests that strip the channel parts and get the plain source back; a byte
comparison of the plain wrappers against `main`; hash and memo tests; cache
tests for plane resolve, pruning by kind and zero-allocation warm frames;
real-GL readback of both planes, `iChannelResolution` and the closed-surface
fallback in `tools/glrendertest`; C checks of the Android channel functions in
the GLES harness; a Metal test that compiles the channel wrapper (shared with
iOS); a golden; `examples/shader_channels` on a GPU and in Chrome (WebGL2).
Metal and iOS binding and drawing have no automated check, since no runner draws
through Metal: they are checked by hand on a Mac with the example.

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
