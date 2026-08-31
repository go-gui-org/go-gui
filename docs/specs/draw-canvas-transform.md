# Canvas transform stack

Status: implemented. Issue #474.

## Problem

`gui.DrawContext` accepted only absolute pixel coordinates. Any responsive
layout or viewport zoom made the caller transform every coordinate by hand
before every draw call, and a drawing routine could not be reused at a second
position or size without being rewritten to take an offset and a factor.

## Design

Four methods, all relative:

```go
func (dc *DrawContext) Translate(dx, dy float32)
func (dc *DrawContext) ScaleBy(sx, sy float32)
func (dc *DrawContext) Save()
func (dc *DrawContext) Restore()
```

The transform is the axis-aligned affine `p' = (p.x*sx + tx, p.y*sy + ty)` —
scale per axis plus translation, no rotation and no shear. That group is closed
under composition, so an arbitrarily deep nest collapses to four floats.

Composition follows HTML canvas:

- `Translate(dx, dy)` → `tx += dx*sx; ty += dy*sy`. The offset is in current
  local units, so a `Translate` after a `ScaleBy` moves by scaled units.
- `ScaleBy(ax, ay)` → `sx *= ax; sy *= ay`. Scaling happens about the current
  origin, so the translation is untouched.

## Decisions

### Geometry carries the matrix, not the vertices

The active transform is stamped on `DrawCanvasTriBatch` at `takeBatch`, and
`emitDrawCanvasGeometry` copies it onto `RenderCmd.HasXform` and its four
fields. Those fields predate this feature — the SVG animation path already used
them — and every renderer (`gui/backend/{gl,metal,web,ios,android}` plus `soft`)
already applies `v*S + T` before the command origin and scale. No backend
changed.

Two reasons this beat baking coordinates at each drawing method:

1. **Delegation.** `Line` calls `Polyline`, `Circle` calls `Arc` calls
   `Polyline`, `FilledCircle` calls `FilledArc`. Baking at each public entry
   would apply the transform twice on every one of those paths. Stamping the
   batch makes a nested call append into an already-transformed batch, so the
   transform applies exactly once by construction.
2. **Cost.** Nothing is rewritten per vertex. Stroke widths, dash lengths, miter
   joins and per-vertex gradient colors are all computed in local space and
   mapped afterwards, so they scale for free.

The price is that curve flattening picks its segment count in local space: a
circle scaled up by 10 is as faceted as the unscaled one. Fixing it means
feeding `max(|sx|, |sy|)` into `arcPoints` and the bezier tolerance, which is a
follow-up, not a blocker.

A transform change joins the run-length merge key in `getBatch`, because a batch
carries one matrix for all its triangles.

### `ScaleBy`, not `Scale`

`DrawContext.Scale` is an existing public field holding the device pixel ratio.
Renaming it to free the name would break `go-map`, `go-term` and `go-charts` for
a cosmetic gain. `ScaleBy` also reads more correctly for an accumulating
operation, and pairs with `Translate`.

### Leaves bake

Text, images and the lowered radial gradient bake their coordinates at record
time. `RenderText`, `RenderImage` and `RenderGradient` have no xform fields to
ride on, and all three are leaves — no primitive delegates to them — so baking
there cannot double-apply.

A text style's px fields scale too: `Size`, `LineSpacing`, `StrokeWidth` and
`CellHeight` by `sy`, `LetterSpacing`, `CellWidth` and `EmojiBoxWidth` by `sx`.
Font size takes `sy` because a font size is a vertical em measure; under a
uniform scale, the case with an unambiguous answer, every field scales alike.
Glyphs are not sheared by a non-uniform scale: composing
`TextStyle.AffineTransform` would allocate per entry per frame on a path kept
allocation-free, and it collides with the existing `RotationRadians` precedence.

`TextWidth` and `FontHeight` stay in local units. They answer a question about
the same space `Text(x, y, ...)` takes its arguments in; scaling them would
break every centering calculation.

Image rects are normalized after transform, so a negative scale lands the rect
at the mirrored position with positive extents rather than being dropped by the
`W <= 0` guard at emit. Image content is never mirrored.

### A non-uniform scale declines the radial fast path

`emitRadialGradient` lowers a concentric radial fill to one shader quad whose
ramp is always centered with radius `max(W, H)/2`. Under a non-uniform scale the
fill is an ellipse, which that command cannot express, so `concentricRadial`
declines and the existing `fillConcentricRings` fallback takes over. Ring
geometry lands in a normal batch and is therefore transformed exactly. More
triangles, same picture.

### An identity transform is no transform

A context transformed and then fully restored reports itself untransformed
(`activeXform`). Stamping the identity on every later batch would break the
run-length merge against the batches drawn before the first `Save`, and put a
matrix on every command for no effect.

### Reset, not balance-checking

`resetFor` clears the transform and the stack. It runs immediately before every
`OnDraw`, so an unbalanced `Save` cannot survive into the next redraw or into
another canvas sharing the window's scratch context. The stack is capped at
`maxXformDepth` so a `Save` inside a draw loop cannot grow without bound.
`Restore` on an empty stack is a no-op: `OnDraw` runs inside the frame, and a
panic there would take the window down over a caller's bookkeeping slip.

### Recorders see baked coordinates

`DrawRecorder` is exported and implemented outside this repo. Handing over local
coordinates plus a matrix would need a wider interface and would silently
misplace every existing implementer's export until they adopted it. Baking makes
them all correct with no change on their side.

The baking is done by `xformRecorder`, a decorator returned from `dc.rec()`,
rather than at each of the two dozen call sites — which would both push
`canvas_draw.go` past its 800-line gate and give delegation a second chance to
apply the transform twice.

The decorator implements every optional extension interface, so the unwrap
helpers (`gradientRecorder`, `vertexColorRecorder`, `imageRecorder`) assert
against the **inner** recorder. Asserting against the decorator would always
succeed and would quietly kill the flat-triangle degradation that keeps an
export from dropping a gradient fill.

Scalars with no axis of their own — stroke widths, dash and gap lengths, corner
radii — take the geometric mean of the two scales, exact under a uniform one. A
circle under a non-uniform scale is handed over as a full-sweep `Arc`, which the
recorder API expresses exactly.

### Cache

No new invalidation key. The transform is a pure function of the `OnDraw` body
and the result is already in the cache entry. The existing contract stands: a
transform derived from application state must move `DrawCanvasCfg.Version`, like
any other `OnDraw` input.

## Fixed on the way

`pdfRenderSvg` ignored `RenderCmd.HasXform` entirely, so an animated SVG already
printed at the wrong position. It now applies the affine in the same order the
GPU and soft backends do. The mapping is split out as `svgCmdVertex` so that
order is assertable without a PDF writer in the way.

## Verification

- `gui/canvas_draw_transform_test.go` — composition in both orders, nesting,
  empty-stack `Restore`, reset across redraws, the delegation single-apply check
  for all three delegating pairs, batch splitting and re-merging,
  text/image/clip baking, the radial fallback, non-finite rejection, stack
  depth, and the recorder decorator including the unwrap degradation.
- `gui/render_draw_canvas_test.go` — the matrix reaches the command with the
  triangles still local, and survives batch pooling across redraws.
- `gui/print_pdf_test.go` — `svgCmdVertex` applies the affine before the origin
  and scale.
- Golden case `canvas_transform`. `serializeCmd` had to learn to print the
  matrix first: the transform rides on the command, so the triangle fingerprint
  alone is identical with and without it and the golden would have proved
  nothing. Every pre-existing golden is byte-unchanged, which is the proof that
  an untransformed canvas behaves exactly as before.
