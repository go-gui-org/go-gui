# Canvas path fill

Status: implemented. Issue #906.

## Problem

The canvas had no path builder (`MoveTo` / `LineTo` / `QuadTo` / `CubicTo` /
`Close`) and no fill for concave or multi-contour shapes. `FilledPolygon` is a
fan from the first point (`gui/canvas_draw.go`), so it is correct only for
convex polygons.

Found while porting Qt's qcpainterbench (#723). Its Lines workload fills the
area under a cubic curve, and its Flower workload fills a 6-petal shape built
from 12 quadratics. The port flattened the curves itself and built the triangles
by hand: a strip down to the baseline, and a fan from the center. That works
only because both shapes have a simple structure.

`gui/svg/tessellate_scanline.go` already filled polylines with the nonzero and
even-odd rules, and `gui/svg/tessellate.go` had the quadratic and cubic
flattening. Both were SVG-only. `gui/svg` imports `gui`, so `DrawContext` could
not call them as they are.

## Design

One `CanvasPath` value holds the segments:

```go
var p gui.CanvasPath
p.MoveTo(x0, y0)
p.CubicTo(c1x, c1y, c2x, c2y, x1, y1)
p.LineTo(x1, base)
p.Close()
```

A repeated `MoveTo` opens a new contour. `Close` ends the contour at its
`MoveTo` point. A fill closes an unclosed contour by implication. A stroke
leaves it open with end caps.

Three methods draw the path: `FillPath` (solid color and fill rule),
`FillPathGradient` (gradient and fill rule) and `StrokePath` (color, width and
`StrokeStyle`). The zero fill rule is nonzero, the SVG default. `StrokePath` has
no unstyled twin: every style, including the zero one, runs through the shared
stroker.

The fill core lives in `gui/internal/pathfill`, shared with `gui/svg`. It holds
the scanline decomposition, the ear-clip fast path and the curve flattening.
`gui/svg` keeps its own `fillRule` type and converts at the boundary. The
package follows the `gui/internal/gradmesh` precedent: shared geometry with no
widget vocabulary. The stroker needed no package because it is one function. The
filler is 500 lines with caps and tests of its own.

`Reset` clears a path for rebuild without a new allocation: a path that changes
every frame stops allocating after the first frame. The tessellator workspace
rides on the `DrawContext` the same way (`pathFlatBuf`, `pathContourBuf`,
`pathScratch`), so a redrawn canvas fills its paths with zero allocations. The
test pins this with `testing.AllocsPerRun`.

Recorders see the path through the optional `DrawPathRecorder` extension.
Widening `DrawRecorder` would break every existing implementer, so the extension
follows the `DrawGradientRecorder` precedent: a recorder without it still gets
the path, as one flat polygon per tessellated triangle for a fill and one joined
polyline per contour for a stroke. A gradient path fill reaches a
gradient-capable recorder as triangles plus the gradient. Under a transform the
decorator bakes the coordinates and passes the width through the mean scale.
Bezier control points map exactly under an affine. Arcs map the way `Arc` maps:
radii scale per axis and angles stay unchanged.

## Decisions

### Repeated MoveTo opens a contour

No `BeginContour` method exists. SVG subpaths and Qt subpaths both start at a
move, and the shared core already splits contours on it. One spelling serves all
three.

### Canvas-style arcs

`ArcTo` takes a center, two radii, a start and a sweep, like `DrawContext.Arc`
and like Qt `arcTo`. It does not take the SVG endpoint form with `phi` and
`largeArc` flags. That form converts to cubics at the call site when SVG parity
needs it. The canvas form reuses the arc segment count, so a path arc draws like
an `Arc` primitive.

### No dash on StrokePath in v1

Dashes stay where they are (`DashedLine`, `DashedPolyline`, the SVG dash array).
The style struct carries caps and joins only, as in #905. A dash parameter would
force the array-against-pair choice now. It waits for a caller.

### One bad point drops the fill

`FillPath` screens the whole segment list and draws nothing when a coordinate is
not finite. This matches `FilledPolygon`: one bad point can poison a fan or a
join, so a partial shape never emits. The tessellator keeps its own guards
behind that, for contours that turn bad during flattening.

### The port keeps its flattened points

The Lines stroke still draws the flattened curve through `PolylineJoinedStyled`,
and the flower outline still draws through `PolylineJoined`. Only the two
hand-built triangulations are gone: the area is path cubics closed to the
baseline, and the flower is the cached outline as one closed contour. The
op-count tests did not move.

## Rejected Approaches

- **Sticky fill state on `DrawContext`.** Hidden state leaks across primitives
  and breaks the explicit immediate-mode calls. The rule travels with the call
  instead.
- **Widening `DrawRecorder` with path methods.** Every external implementer
  breaks in lockstep. The optional extension keeps them compiling and drawing,
  with documented lossy fallback.
- **A baked tessellation cache on the path.** A cache needs a key (tolerance,
  scale, transform) and an invalidation story. A stale entry draws the wrong
  shape without a sound. Per-frame tessellation of a 200-edge outline costs less
  than the bookkeeping around it, and the static case already skips `OnDraw`
  through `DrawCanvasCache`.
- **The core in `gui` root next to the stroker.** The stroker is one function.
  The filler is scanline, ear-clip, flattening and caps. The root gains four
  exports (`CanvasPath`, `FillRule`, the draw methods, the recorder) and no
  algorithm tests. The algorithm tests run once, in `pathfill`.
- **Endpoint-style arcs in v1.** Surface, validation and a second flattening
  path for a knob no caller requested. The canvas form covers the workloads.
