# Encoded image bytes

Issue #1228. Status: **implemented**.

## Problem

`UseImage` registers pixels that are already tightly packed NRGBA8. Applications
with a PNG or JPEG in memory—commonly from `go:embed`, a network response or a
database—had no public way to perform the same decode as an image loaded from a
file. They had to duplicate go-gui's decode and pixel conversion or write a
temporary file.

## Decision

`RegisterEncodedImage(key, data)` decodes PNG, JPEG or GIF data, converts it to
a zero-origin, tightly packed NRGBA8 buffer, and passes that buffer to
`UseImage`. It returns the resulting `mem:` source for `ImageCfg.Src` or
`DrawContext.Image`.

The key has the same content-addressed contract as `UseImage`: callers use a
stable key for immutable image content. The encoded input is not retained. After
registration, the decoded pixel buffer is owned by the image registry. A call
for a key that is already registered returns its source without decoding again.

## Pinned, not budgeted

`UseImage` entries live in a byte-budgeted LRU (32 MiB by default). That is safe
for generated imagery because its callers rebuild on a miss each frame: the
colour picker looks its key up during view generation and regenerates an evicted
plane. An encoded image is different. It is typically registered once, at
startup, and its source is kept in app state, so an eviction left that source
naming nothing and the image drew blank. Three 2048×2048 PNGs of about 120 KB
each were enough: the first was evicted before any frame was drawn.

`RegisterEncodedImage` entries are therefore pinned. They do not count toward
the budget and are never evicted by it, so they cannot push out generated
imagery either. Only `DropImage` removes them. The cost is that an app which
registers many distinct keys, for example images from the network, must drop the
ones it no longer shows. Since #1230 the drop also frees the backends' textures
([mem-image-texture-release.md](mem-image-texture-release.md)).

The same hazard applies to a `UseImage` caller that keeps its source instead of
registering each frame. `examples/qcpainterbench` did this and now registers
again when `HasImage` reports the key gone; the `UseImage` documentation says to
call it while building each frame.

## Name

The function is `RegisterEncodedImage`, not `UseEncodedImage` (the name first
proposed on the issue) or `UseEncodedImageBytes` (the first implementation). In
the registry, "Use" means a cache entry that the caller registers again every
frame and that the budget may evict. A pinned image has the opposite contract:
register it once, keep its source, and release it with `DropImage`. The verb
"Register" says that, so the two `mem:` functions no longer look alike while
behaving oppositely. `examples/showcase/docs/widget_image.md` sets every image
source side by side, with what each one asks of the app.

## Shared decoder

The file loader and the public byte API share `gui/internal/imgdecode`. Backend
call sites continue through `gui/backend/internal/imgload`, so moving the decode
implementation does not change disk-image behavior.

`imgdecode` registers the PNG, JPEG and GIF decoders itself. GIF used to decode
only because `gui/print_pdf_nearest.go` imported `image/gif`, a side effect a
refactor of PDF export could have removed. A GIF decodes to its first frame.
`image.Decode` returns that frame with its own bounds, which may be offset into
the canvas or smaller than it, while `DecodeConfig` reports the whole canvas.
`imgdecode` therefore draws the frame at its offset onto a transparent canvas of
the reported size, for files and bytes alike. Using the frame alone had made a
GIF file draw only that rectangle, stretched over the image. Decoding goes
through `image.Decode`, so an app that imports another decoder
(`golang.org/x/image/webp`, `golang.org/x/image/bmp`) gets that format in both
paths. TIFF happens to decode on desktop builds because `go-text/typesetting`
imports `golang.org/x/image/tiff`; it is not documented as supported, since the
web build does not link it and a dependency bump could drop it.

`examples/embedded_image` embeds one scene as PNG, JPEG and GIF and registers
each at startup.

## Decode limits

The byte path reads the image header before the full pixel decode. It rejects:

- encoded data larger than 16 MiB;
- an edge larger than the in-memory image registry's 4096-pixel limit; and
- a decoded area larger than 4096 × 4096 pixels.

These checks bound both encoded input and the decoded pixel allocation. They
also ensure every successful result is valid input to `UseImage`.

## Relationship to go-video

This API is for immutable, encoded still images. It deliberately ends in the
same packed NRGBA8 registry format as `UseImage`.

Video frames remain a different path. `PixelSurface` accepts already-decoded,
changing pixel planes, and `Shader.Surface` exposes those planes to shaders.
Video-specific decode, YUV formats and colour conversion stay in go-video. A
compressed video packet is therefore not an input to `RegisterEncodedImage`.

## Rejected approaches

- **Put encoded bytes on `ImageCfg`.** That mixes loading and error handling
  into view construction and risks decoding on repeated view builds.
- **Teach every renderer to decode image formats.** Decoding once before the
  renderer keeps backend behavior identical and avoids duplicate caches.
- **Use `PixelSurface` for encoded still images.** A mutable streaming surface
  has different ownership, invalidation and upload semantics from an immutable
  registry image.
- **Return a data URL.** It would make browser decoding easy but would not give
  native backends the same source or error behavior.
- **Keep encoded images in the budget and document re-registration.** Callers
  would gate a re-decode on `HasImage` in the view. Every eviction then costs a
  full decode in a frame (about 30 ms for a 1024×1024 PNG), and when the visible
  images exceed the budget they evict each other on every frame.
- **Keep the encoded bytes and decode again on a miss.** The memory bound would
  hold, but `LookupImage` runs in the backends' draw path under the registry
  lock, so the re-decode would stall drawing, and the lookup would gain an error
  path.
- **Pin `UseImage` entries too, or export a pin call.** Pinning by default would
  remove the bound that content-keyed generators rely on during a drag. An
  exported pin adds surface for a case that the per-frame `UseImage` idiom
  already covers. Making an encoded image `UseImage` plus a separate
  `PinImage(key)` call was rejected for the same reason, and because a two-step
  call is easy to half-do.
- **Keep the "Use" name and only document the difference.** The name would still
  suggest the per-frame contract, and nothing at the call site would point to
  `DropImage`.
