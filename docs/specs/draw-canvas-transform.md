# Canvas transform stack

Status: implemented. Issues #474 (translate/scale), #904 (rotation).

## Problem

`gui.DrawContext` accepted only absolute pixel coordinates. Any responsive
layout or viewport zoom made the caller transform every coordinate by hand
before every draw call, and a drawing routine could not be reused at a second
position or size without being rewritten to take an offset and a factor.

## Design

Five methods, all relative:

```go
func (dc *DrawContext) Translate(dx, dy float32)
func (dc *DrawContext) ScaleBy(sx, sy float32)
func (dc *DrawContext) Rotate(rad float32)
func (dc *DrawContext) Save()
func (dc *DrawContext) Restore()
```

The transform is the full 2x3 affine `p' = (xx*x + xy*y + tx, yx*x + yy*y + ty)`
— translate, per-axis scale and rotation, composed in call order. That group is
closed under composition, so an arbitrarily deep nest collapses to six floats.

Composition follows HTML canvas:

- `Translate(dx, dy)` moves by the offset in current local units, so a
  `Translate` after a `ScaleBy` moves by scaled units and after a `Rotate` by
  rotated ones.
- `ScaleBy(ax, ay)` scales each basis column in place. Scaling happens about the
  current origin, so the translation is untouched.
- `Rotate(rad)` post-multiplies by the rotation, in y-down radians, so positive
  turns clockwise on screen. The translation is untouched, so the rotation
  happens about the current origin: to spin about another point, bracket with
  `Translate`.

## Decisions

### Geometry carries the matrix, not the vertices

The active transform is stamped on `DrawCanvasTriBatch` at `takeBatch`, and
`emitDrawCanvasGeometry` copies it onto `RenderCmd.HasXform` and its six fields
(`ScaleX/Y`, `TransX/Y`, plus the off-diagonals `XformXY/XformYX`, zero for
translate+scale). Those base fields predate this feature — the SVG animation
path already used them — and every renderer
(`gui/backend/{gl,metal,web,ios,android}` plus `soft`) applies the full affine
before the command origin and scale. The rotation change extended that
per-vertex apply in each backend; the SVG path, which never sets the
off-diagonals, behaves exactly as before.

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
`CellHeight` by the vertical column length, `LetterSpacing`, `CellWidth` and
`EmojiBoxWidth` by the horizontal one. Font size takes the vertical measure
because a font size is a vertical em measure; under a uniform scale, the case
with an unambiguous answer, every field scales alike. Under rotation the anchor
maps through the full matrix and the style gains the angle: added onto
`RotationRadians`, so the emit path keeps its GPU bracket, or composed onto an
explicit `AffineTransform`, which keeps its precedence over rotation. The
composed affine carries the CTM's rotation only — the anchor already moved, and
the scale already rode in on `Size` and the advances — and heap-allocates, but
only on that path: unrotated text never touches the pointer. Glyphs are not
sheared by a non-uniform scale or a sheared CTM: a mirror keeps angle zero, so
it never turns text upside down. Under a mirror that also turns, the angle reads
whichever column keeps content upright — `R(θ)·diag(1,-1)` equals
`R(θ+π)·diag(-1,1)`, so the two readings differ by π — and a tiny turn under
`ScaleBy(-1, 1)` stays at ≈0 instead of jumping to ≈π.

`TextWidth` and `FontHeight` stay in local units. They answer a question about
the same space `Text(x, y, ...)` takes its arguments in; scaling them would
break every centering calculation.

Image rects are normalized after transform, so a negative scale lands the rect
at the mirrored position with positive extents rather than being dropped by the
`W <= 0` guard at emit. Image content is never mirrored. Under rotation the
entry records the mapped corner, the mapped size and the angle, and the emit
path wraps the blit in a rotation bracket about that corner — exact whenever the
CTM columns stand perpendicular (every chain except a non-uniform scale
sandwiched between two rotations), un-sheared otherwise. Under a mirroring
rotated CTM the recorded corner is the mapped `(x, y+h)` or `(x+w, y)`, matching
the column the angle was read from, so the unmirrored blit still covers the
mapped rect. A rotated clip, which a scissor cannot express, bakes to its
bounding box.

### A non-uniform scale declines the radial fast path

`emitRadialGradient` lowers a concentric radial fill to one shader quad whose
ramp is always centered with radius `max(W, H)/2`. Under a non-uniform scale the
fill is an ellipse, which that command cannot express, so `concentricRadial`
declines and the existing `fillConcentricRings` fallback takes over. Ring
geometry lands in a normal batch and is therefore transformed exactly. More
triangles, same picture. Rotation alone keeps the quad: a rotated circle is
still a circle, so the bake maps the center through the full matrix and scales
the radius by the column length.

Whether the transform maps circles to circles is read off a structural
similarity flag, not re-derived from the floats: on arm64 a
Translate;Rotate;ScaleBy chain leaves the column dot product at -4.8e-8 instead
of exactly zero, so an exact re-check would drop every rotated glow to the mesh
path. `Translate` and `Rotate` preserve the flag; only a non-uniform `ScaleBy`
clears it. With no rotation in force `ScaleBy` re-derives it as `xx == yy`, the
old translate+scale check, so a non-uniform pair that multiplies back to uniform
keeps the quad. Under rotation a false negative would cost triangles, never
pixels, and the flag errs in that direction.

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
radii — take the root of the absolute determinant, exact under a uniform scale
and a pure rotation. A circle under a non-uniform scale is handed over as a
full-sweep `Arc`, which the recorder API expresses exactly; under rotation a
uniform transform still maps circles to circles, so only a rotated non-uniform
transform flattens circles to polygons. Rotated rects record as their mapped
outline polygons, rotated arcs and rounded rects flatten at the arc tolerance,
and a rotated image records its bounding box.

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
  depth, and the recorder decorator including the unwrap degradation; plus the
  rotation block — order dependence, affine stamping with unbaked vertices,
  `Transform()` declining rotated batches, merge splitting, non-finite angles,
  text angle/affine composition, mirror-keeps-zero, image frames, the rotated
  quad vs. mesh, and recorder polygon/arc flattening.
- `gui/render_draw_canvas_test.go` — the matrix reaches the command with the
  triangles still local, and survives batch pooling across redraws; plus the
  rotated affine emission and the rotated-image bracket.
- `gui/print_pdf_test.go` — `svgCmdVertex` applies the affine before the origin
  and scale.
- Golden case `canvas_transform`. `serializeCmd` had to learn to print the
  matrix first: the transform rides on the command, so the triangle fingerprint
  alone is identical with and without it and the golden would have proved
  nothing. Every pre-existing golden is byte-unchanged, which is the proof that
  an untransformed canvas behaves exactly as before. The rotation block pins the
  six-term `xform=[...]` print, the local (unbaked) triangles, and the text's
  `RotateBegin`/`RotateEnd` bracket.

## Rejected Approaches

- **CPU-bake rotation at record time.** Keeps `RenderCmd` at four floats and
  touches no backend, but rewrites every vertex per redraw and kills the
  zero-rewrite geometry path the whole design exists for. The motivating
  workload (qcpainterbench Flower, 193 points per pass per frame) pays it every
  frame; the matrix ride pays nothing.
- **Rotate-bracket only (no matrix on the command).** Reuses the MVP stack for
  everything with one new method, but the canvas emits images first, geometry
  second and text last regardless of `OnDraw` order, so a rotation between two
  geometries cannot bracket correctly without re-sorting emission into `OnDraw`
  order — a larger change with a worse failure mode (silently unrotated
  text/images) than two extra command fields.
- **Re-deriving uniformity from the floats.** An exact
  columns-equal-and-perpendicular check fails after ordinary chains on arm64
  (-4.8e-8 dot instead of zero); an epsilon check risks false positives, which
  unlike false negatives render wrong pixels. The structural flag errs toward
  the mesh, which is always correct.
