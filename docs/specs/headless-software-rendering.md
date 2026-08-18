# Headless software rendering

Issue #333 (phase 1), issue #360 (phase 2). Status: complete.

## Problem

go-gui could not produce a pixel image of a frame without a GPU and a window.
Every backend under `gui/backend/` is GPU-backed or native, so there was no way
to take a screenshot in CI, no way to write a pixel-level regression test, and
no starting point for a software backend.

## Approach

The render pipeline already terminates in a flat `[]gui.RenderCmd`, and
`gui/print_pdf.go` already proves that stream replays outside a backend. The
software rasterizer is a third consumer of the same input, not a pipeline
change: no existing backend is touched and none consults it.

Two findings shaped the design.

**go-glyph is pure Go.** It shapes and rasterizes through `go-text/typesetting`
with no `import "C"`, so text works on CPU.

**`glyph.DrawBackend` is a six-method interface.** Implementing it over a CPU
framebuffer (`gui/backend/soft/glyph_backend.go`) yields every text render kind
— `RenderText`, `RenderLayout`, `RenderLayoutTransformed`, `RenderRTF`,
`RenderTextPath`, text gradients — from the same `glyph.TextSystem` the GPU
backends drive, and supplies a real `gui.TextMeasurer` as a side effect. This
removes the issue's stated obstacle: headless frames are measured with true font
metrics, not the approximate extents a nil measurer falls back to.

## Placement

The package is `gui/backend/soft`, not package `gui` as the issue proposed.
`gui/svg` imports `gui`, so a rasterizer inside `gui` could never call
`SetSvgParser` and every SVG would render blank. A subpackage has everything it
needs: the `Render*` constants are exported (only the `renderKind` type is not),
`RenderCmd`'s fields are exported, and the one unexported payload — `textPath` —
is reachable through `gui.ComputeTextPathPlacements`. `gui/backend/gl` is the
existence proof.

## API

```go
func RenderToImage(w *gui.Window, scale float32) (*image.RGBA, error)
func RenderToPNG(w *gui.Window, scale float32, path string) error
func Release(w *gui.Window)
```

`scale` is the device pixel ratio; `scale <= 0` means 1. A window that has not
rendered yet has its `WindowCfg.OnInit` run first, as a backend would, so the
same window value passed to `backend.Run` can be passed here instead.

Preparation is idempotent and keeps the warm glyph atlas, so driving state
between captures — `TestClick`, `SetFocus`, render again — costs only the frame.
`Release` frees the text system for callers that render many windows in one
process.

## How a frame is rendered

1. Prepare: build a `glyph.TextSystem` over the software draw backend, register
   the bundled icon font and `gui.AppFontPaths` / `AppFontData`, install it as
   the window's `gui.TextMeasurer`, and install `svg.New()` as the SVG parser.
2. `w.BackingScale = scale` — read during layout by `text_optical.go` and
   `text_ink.go`, so it must be set before the frame, not only used by the
   blitter.
3. `w.SetHeadlessRender(true)`, then `w.TestRender(nil)` to settle one frame,
   then `w.Renderers()` for the command stream.
4. Replay the stream twice (below), then hand back the buffer as an
   `*image.RGBA`. The buffer is premultiplied, which is what `image.RGBA` is, so
   the PNG boundary is zero-copy.

### Why the stream is replayed twice

go-glyph rasterizes a glyph into a staging buffer when it is first drawn but
only hands the page to the backend at `Commit`. On screen the next frame
corrects the sampling; a one-shot capture has no next frame. The first pass
therefore draws the text kinds with a nil target — shaping and rasterization
run, the quads are discarded — and `Commit` uploads the atlas. Shapes, gradients
and images are skipped on the warm pass and drawn once, for real, in the second.

### Rasterization

Everything reduces to one operation: composite a source through a coverage mask,
restricted to the clip rect. The mask comes from `golang.org/x/image/vector`,
which antialiases every edge; the source is a solid color (`image.Uniform`), a
gradient sampler, or a scaled image sampler. Two consequences worth naming:

- A stroke rect is one path, not two draws: the outer rounded rect plus the
  inner one wound backwards, which the rasterizer's signed accumulation turns
  into a hole.
- Clipping is free. The mask is allocated at the intersection of the shape's
  bounding box and the clip rect, so geometry outside it is never rasterized.
  `RenderClip` replaces the clip rather than nesting it, matching the scissor
  semantics the GPU backends and `gui/print_pdf.go` implement; a degenerate rect
  restores the full buffer.

Gradients are **not** resampled to five stops. That cap is a shader uniform
budget; a CPU sampler has none, so the full stop list is honoured — the same
choice the web backend's canvas gradients make.

## Reproducibility

`(*Window).SetHeadlessRender` is window-scoped, not a process global: theme,
focus and state are all window-owned here and this belongs with them. It
suppresses visuals whose appearance depends on wall-clock time — today the
blinking caret, gated in `inputCursorOn`. The caret was already off headlessly
(no animation goroutine drives the blink atomic), so the flag turns an accident
of timing into a guarantee, and gives any later wall-clock visual one place to
consult. It is deliberately not a general "no animation" switch.

go-shirei's `ResetInputSession` has no counterpart here: go-gui's leak surface —
focus, hover, mouse position — is per-`Window`, so two captures share state only
when the caller reuses the window, which is the point of `RenderToImage` taking
one.

## Phase split

Phase 1 (issue #333) covered `RenderClip`, `RenderRect`, `RenderStrokeRect`,
`RenderCircle`, `RenderLine`, `RenderGradient`, `RenderGradientBorder`,
`RenderImage`, and every text kind.

Phase 2 (issue #360) added `RenderSvg`, `RenderShadow`, `RenderBlur`, the filter
bracket, the stencil bracket, the rotation bracket, and `RenderTermGrid`.
`RenderCustomShader` stays unsupported by design — it is GLSL, and there is no
CPU equivalent to compile.

The unsupported kinds are still listed in one explicit `case` in
`gui/backend/soft/draw.go` rather than caught by a `default`, so a newly added
render kind is a compile-visible decision. This follows the precedent in
`gui/print_pdf.go`.

## Layers: one mechanism for three brackets

Filters, stencil clipping and rotation all scope later drawing, and all three
use the same construction: `RenderFilterBegin` / `RenderStencilBegin` /
`RenderRotateBegin` re-point the render target at an offscreen layer, the
bracketed commands draw into it unaware, and the matching end composites the
layer back with the scoped effect applied — a coverage mask for the stencil, an
inverse-mapped resample for the rotation, blur plus colour matrix plus repeated
composite for the filter.

The alternative — carrying a transform and a clip mask through every draw path —
was rejected. It would have touched every phase 1 file and still not covered
text, which goes through go-glyph's draw backend rather than this package's path
builders. With a layer, a rotated caption and a stencil-clipped image are
correct for free.

Three consequences worth naming:

- **Nesting needs no depth counter.** An inner bracket composites into the outer
  bracket's layer, which is already masked. `RenderCmd.StencilDepth` is what the
  GL backend uses to unwind a shared stencil buffer; a layer stack has nothing
  to unwind, so it is read only as documentation of intent.
- **Layers are full window size and pooled.** Full size keeps every coordinate
  in device space, so no draw path does offset bookkeeping. The cost is bounded
  by pooling (`renderer.layerPool`) plus a per-layer dirty box: clearing and
  compositing walk only the pixels a bracket actually wrote.
- **The clip carries across the bracket, and is not restored on pop.** That is
  what the GPU backends do — a scissor survives an FBO bind — and the render
  stream re-emits the clip it wants after `RenderStencilEnd`.

An unbalanced stream is not allowed to swallow content: `finishBrackets` closes
anything still open at the end of the pass, and an end command of the wrong kind
is ignored rather than popping a bracket it did not open.

## SVG triangles

`RenderSvg` arrives as a flat triangle list with an optional colour per vertex.
Either way the coverage for a whole command comes from a **single path**, never
one triangle at a time: the rasterizer accumulates signed coverage across a
path, so the interior edges triangles share cancel exactly. Rasterized
separately, a shared edge is antialiased twice — two ~50% coverages that
composite to 75%, not 100% — and the background shows through as a lattice of
pale lines over every gradient.

Flat geometry is therefore filled as one path in one colour. Vertex-coloured
geometry is a mesh, and takes two passes: the same single-path coverage mask,
plus a pooled layer holding the shading, which is composited through it. The
shading is **written, not blended**, so a pixel two triangles both claim along a
shared edge is painted once — their colours agree there, and blending twice
would darken every seam of a translucent gradient. Each triangle paints every
pixel centre inside it plus a half-pixel skirt; the skirt keeps the mesh's outer
edge from thinning, where a pixel the mask includes at 40% can have its centre
just outside the triangle. Interior pixels are unaffected, since the triangles
tile and every centre inside the mesh is already claimed. Within a triangle the
colour is the barycentric interpolation of its three vertices, the CPU
equivalent of the GPU's varying interpolation.

The vertex transform order mirrors `gui/backend/gl`'s `drawSvg`:
`animateTransform` scale and translate, then rotation about (`RotCX`, `RotCY`),
then the command's own `Scale` and origin. `IsClipMask` geometry is skipped, as
it is in every other backend — no render pipeline consumes it yet.

## Shadow and blur

The GPU path is one SDF quad per command. The CPU path rasterizes the same
rounded rect into a coverage mask, blurs the mask, and composites the colour
through it; a shadow additionally multiplies in the complement of its caster's
coverage, which is the shader's second SDF. The blur is three box passes — the
standard Gaussian approximation, at a cost independent of the radius, which is
what makes a large shadow affordable on a CPU. `sigmaPerBlur` documents how a
command's blur radius maps onto a standard deviation, since the shaders ramp
linearly across a band rather than convolving.

## Bounds on hostile input

A CPU rasterizer turns an unvalidated number into wall-clock time, so the phase
2 kinds clamp what they accept rather than trusting the stream. Blur radii reach
`soft` from widget config and from an SVG filter's `stdDev`; `clampBlur` rejects
NaN and caps the radius at `maxBlur` device pixels, because an infinite radius
sizes the box-blur sliding window and the failure is a hang rather than a wrong
pixel. `blurPlanes` caps sigma again at the plane's own size, past which a box
already averages everything. `RenderFilterBegin.Layers` is the count of
`feMergeNode` elements in an untrusted document and each layer is a full
composite pass, so `maxFilterLayers` caps the repeat. Corner radii and extents
go through negated `>` comparisons, which reject NaN where `<= 0` would pass it
on to the rasterizer as NaN control points. `maxLayerDepth` caps bracket
nesting: past it the bracketed commands draw into the parent, which keeps the
begin/end pairing intact and loses only the scoped effect.

Compositing saturates rather than wrapping. A filter's colour matrix clamps each
channel independently, so it can leave a pixel whose colour exceeds its own
alpha; the unclamped premultiplied sum would pass 255 and wrap a bright pixel to
near-black. `overPremul` is the single place that blend is spelled.

## Not in either phase

Pixel golden images (issue #361). Platform font variance and antialiasing
stability make them a separate decision from the renderer itself; the tests here
assert pixel values from constructed command streams instead, which is
platform-independent.

## Attribution

The design of go-shirei's headless renderer (zlib, © 2025 Judi Systems) was
cited as prior art for the `HeadlessRender` and `HeadlessScale` ideas. No code
was copied.
