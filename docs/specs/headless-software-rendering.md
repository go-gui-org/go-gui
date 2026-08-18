# Headless software rendering

Issue #333. Status: phase 1 landed.

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
run, the quads are discarded — and `Commit` uploads the atlas. Shapes,
gradients and images are skipped on the warm pass and drawn once, for real,
in the second.

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

Phase 1 (landed) covers `RenderClip`, `RenderRect`, `RenderStrokeRect`,
`RenderCircle`, `RenderLine`, `RenderGradient`, `RenderGradientBorder`,
`RenderImage`, and every text kind.

Deferred to phase 2 (issue #360): `RenderSvg` (barycentric triangle
rasterization over `Triangles`/`VertexColors`, honouring `HasXform`, `RotAngle`,
`VertexAlphaScale`), `RenderShadow` and `RenderBlur` (separable box-blur
passes), `RenderFilterBegin/End/Composite` (offscreen buffer plus
`ColorMatrix`), `RenderStencilBegin/End`, `RenderRotateBegin/End`, and
`RenderTermGrid`. `RenderCustomShader` stays unsupported by design — it is GLSL.

The deferred kinds are listed in one explicit `case` in
`gui/backend/soft/draw.go` rather than caught by a `default`, so a newly added
render kind is a compile-visible decision. This follows the precedent in
`gui/print_pdf.go`.

## Not in this phase

Pixel golden images (issue #361). Platform font variance and antialiasing
stability make them a separate decision from the renderer itself; the tests here
assert pixel values from constructed command streams instead, which is
platform-independent.

## Attribution

The design of go-shirei's headless renderer (zlib, © 2025 Judi Systems) was
cited as prior art for the `HeadlessRender` and `HeadlessScale` ideas. No code
was copied.
