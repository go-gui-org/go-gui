# Pixel surfaces and shader texture inputs

Issue #1168. Status: **phases 1 and 2 landed** (`PixelSurface` on the image
path; planes and `Shader` texture inputs). **Phase 3 in review** (#1326): frames
lent as Linux DMA-BUFs and imported without a copy.

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
| 3     | `UpdateDMABuf` and `GPUFrame`: GPU frames imported on the Linux GL backend, `Window.ImportsDMABuf` (#1326)   | Review |

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

## Phase 3: GPU frames without a copy (#1326)

A hardware decoder leaves each picture in GPU memory. Through `UpdatePlanes` it
goes GPU → CPU → GPU: measured with go-video's VA-API path on a Radeon Vega, the
download, two copies and the upload were about two thirds of playback's CPU
time. Phase 3 lets the surface carry a frame that stays on the GPU.

- **API.** `UpdateDMABuf(planes []DMABufPlane, frame GPUFrame)` publishes one
  DMA-BUF per configured plane (file descriptor, offset, pitch, DRM format
  modifier). The configured formats and sizes still describe what the shader
  samples: `R8` is `DRM_FORMAT_R8`, `RG8` is `DRM_FORMAT_GR88`, `RGBA8` is
  `DRM_FORMAT_ABGR8888`. `GPUFrame` is an interface with one method, `Release`,
  rather than a `func()`, so a producer passes a pooled pointer and publishing
  allocates nothing. The surface tells frames apart by it, so it must be
  comparable. On error the surface did not take the frame.
- **Compressed layouts.** `DMABufPlane.Metadata` lists up to three further
  memory planes of the same image plane, in the modifier's order: the metadata
  of AMD's DCC or Intel's CCS. Hardware decoders export such layouts (Intel's
  media compression on Tiger Lake and later), and an import without the metadata
  planes is refused. The surface copies them into fixed storage, so publishing
  still allocates nothing. EGL takes four memory planes per image, hence three.
- **Ownership.** The surface keeps a lent frame until a later `Update`,
  `UpdatePlanes`, `UpdateDMABuf`, `Resize` or `Close` replaces it and every
  renderer that imported it is done. Then it calls `Release`, once, never under
  its lock. A renderer holds a frame from `ReadDMABuf` (which takes the hold
  before its visit) to `DropGPUFrame`. At most 16 frames are lent at once; past
  that `UpdateDMABuf` fails rather than grow. The CPU planes stay allocated, so
  a producer can switch back to `UpdatePlanes` at any frame.
- **Who draws it.** `ReadPlanes` and `ReadPixels` report nothing to draw while a
  GPU frame is current. So every renderer that does not import, and every
  `Image`, draws the surface transparent with no change of its own.
  `Window.ImportsDMABuf` tells the producer which case applies: it checks the
  flag and keeps `UpdatePlanes` where it is false.
- **The GL backend on Linux** imports when its EGL display lists
  `EGL_EXT_image_dma_buf_import` and `EGL_EXT_image_dma_buf_import_modifiers`
  (hardware decoders export tiled buffers), and the context lists
  `GL_OES_EGL_image`. Each plane becomes an EGLImage on the render thread
  (`surfacecache.ResolveFrame`), and the plane's texture takes its storage with
  `glEGLImageTargetTexture2DOES`. Texture names are reused from frame to frame.
  A plane the driver refuses, at `eglCreateImageKHR` or with a GL error from the
  binding, leaves the surface transparent until its next frame, logs once, and
  the frame goes back unread. Layouts the display imports only for an external
  texture, which a `sampler2D` cannot read, are refused up front. The window
  then reports `ImportsDMABuf` false for good and rebuilds its layout, so a
  producer that checks the flag sends the next frames with `UpdatePlanes`: a
  layout the driver cannot import costs one transparent frame, not a blank
  video.
- **Synchronisation.** GL keeps a buffer alive for commands in flight, so a
  replaced frame's EGLImages go at once. The hazard is the producer: a decoder
  writes its next picture into a released buffer. So the frame's release waits
  on a `glFenceSync` inserted when the frame is retired. The fences are polled
  in order with `glClientWaitSync` (timeout 0, flushing) at the start of each
  frame. A window with a fence still pending draws again, so that the poll runs;
  window teardown waits for every fence. A fence that says nothing about the GPU
  never releases a frame by itself: when `glFenceSync` fails, a wait fails
  (`GL_WAIT_FAILED`), or teardown's wait runs out, `glFinish` confirms the GPU
  is done first. Implicit DMA-BUF sync, which varies by driver, is not relied
  on.
- **Elsewhere.** Every other backend reports `ImportsDMABuf` false. macOS
  (IOSurface into Metal) and Windows (D3D11 through `WGL_NV_DX_interop2`) can
  take the same shape later, as sibling methods with the same release contract.
  16-bit planes (`R16`, `RG16`, for HEVC Main 10) are left to that work: they
  need `UpdatePlanes` support on every backend first.

Verification:

- **Surface bookkeeping:** one release per frame (replaced, closed, never read,
  held by several windows), rejected input, the 16-frame bound, wakes, zero
  allocations per published frame, and a producer and renderer under `-race`.
- **Fence ordering:** the retire queue, tested with the fences injected,
  including a fence that cannot be made, a failed wait and a wait that runs out
  at teardown, each released only after `glFinish`.
- **On llvmpipe in `tools/glrendertest`:** the window flag matches the importer;
  a window without import draws a GPU frame transparent and still releases it; a
  refused import and a refused binding each release the frame once, turn
  `ImportsDMABuf` off and rebuild the layout; real GL fences release a retired
  frame.
- **On hardware:** a GPU test imports GBM buffers with known pixels in four
  layouts (GBM's implicit one, linear, tiled, and compressed with a metadata
  plane) and reads them back. It checks that a replaced frame goes back only
  after its fence (`GOGUI_REQUIRE_DMABUF=1`). CI's llvmpipe lists no DMA-BUF
  import and the runners have no render node, so this test runs by hand.
- **Example:** `examples/dmabuf_surface` shows a GBM-backed producer.

## Rejected approaches

- **A versioned, mutable `UseImage` registration.** Fewer copies, but the caller
  must keep its storage alive until every renderer has uploaded, and reusing a
  key can show stale data. It would change the immutable contract every existing
  `UseImage` caller relies on.
- **A renderer frame-provider interface** (the renderer pulls frames and
  releases them). It could pass decoder frames through with fewer copies, but a
  render callback can block, and a missed or doubled release stalls the decoder
  or reuses live storage. Phase 3 passes decoder frames through by push instead:
  the surface, not app code, counts the holds and releases each frame once.
- **A neutral `UpdateExternal(ExternalFrame{...})` entry point** (phase 3). One
  struct that grows a field per platform, most of them meaningless where the
  code runs, caught only at run time. A sibling method per platform keeps each
  one typed.
- **A separate `GPUSurface` type** (phase 3). It would duplicate the surface's
  versioning, wakes, window list, `Close` and cache, and add a second `Shader`
  field that must not be set with the first.
- **go-gui importing VA-API or another decoder API** (phase 3). It would tie the
  GUI library to video stacks; DMA-BUF is the neutral handoff.
- **Mapping a DMA-BUF to main memory for windows without import** (phase 3).
  Decoder buffers are tiled and uncached; that is the slow download phase 3
  removes. Those windows keep `UpdatePlanes`.
- **A new image key and texture per frame.** Texture churn and cache pressure;
  acceptable only for a throwaway diagnostic.
- **One surface per shader channel** (phase 2). NV12 publishes Y and then UV; a
  render between the two would draw the new Y with the old UV.
- **A video-specific surface type in `gui/`** (an `NV12Surface`, colour
  matrices, per-backend NV12 shaders). Keeps video concepts in go-gui; the
  planes and shader inputs above carry the same frames generically.
- **A raw native render callback** (GL context or Metal encoder handed to app
  code). Lets outside code break render state, clips and z-order, and every
  backend refactor would break it. Phase 3 imports GPU frames into the same
  `iChannel` binding instead.
