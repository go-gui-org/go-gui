# Canvas stroke style

Status: implemented. Issue #905.

## Problem

Canvas strokes had no line caps, and joins were miter or bevel only. `Arc`,
`Line` and `Polyline` ended flush at the end point. The qcpainterbench port drew
two half-disc `FilledArc` fills per arc to fake round caps, and settled for
miter joins on its curves.

`gui/svg/stroke.go` already tessellated butt, round and square caps and miter,
round and bevel joins, but only for SVG. `gui/svg` imports `gui`, so
`DrawContext` could not call it.

## Design

One `StrokeStyle` value selects the caps and joins:

```go
gui.StrokeStyle{Cap: gui.StrokeRoundCap, Join: gui.StrokeRoundJoin}
```

Each stroked primitive gains a `*Styled` twin (`LineStyled`, `PolylineStyled`,
`PolylineJoinedStyled`, `ArcStyled`, `CircleStyled`, `RoundedRectStyled`,
`QuadBezierStyled`, `CubicBezierStyled`). The current calls are unchanged. A
Styled call with the zero style draws exactly what the unstyled call draws,
through the same code path. Only a non-zero style tessellates through the shared
stroker.

The stroker core lives in `gui` as `AppendStrokeTris`, shared with `gui/svg`,
which keeps its own `SvgStrokeCap`/`SvgStrokeJoin` domain types and converts at
the boundary. The miter limit stays fixed at 4x the half width: both stroke
paths this core replaces already used that limit. Dashes keep butt caps per
dash; the style applies to path ends only. `Rect` keeps its four-quad build and
has no Styled form in this version.

Recorders see the style through the optional `DrawStrokeRecorder` extension.
Widening `DrawRecorder` would break every existing implementer, so the extension
follows the `DrawGradientRecorder` precedent: a recorder without it still gets
the stroke, as the equivalent unstyled primitive. Caps and joins past the
default do not survive that fallback. Under a transform the decorator bakes
coordinates and passes the style through untouched. A rotated arc or circle
lowers to `PolylineJoinedStyled` rather than `PolylineJoined`, so the style
survives the lowering.

## Decisions

### Zero style delegates to the legacy path

`PolylineStyled` with the zero style calls `Polyline`; it does not run the
shared stroker with default settings. The legacy output stays bit for bit: no
golden re-record, no pixel shift in maps and charts, and the existing
triangle-count tests keep passing unmodified. The pin is a test per primitive
comparing legacy output with zero-style output.

### The stroker lives in `gui`, not in an internal package

There is no `internal/` tree, and the export policy accepts root exports shared
with leaf subpackages. The core is one function plus unexported helpers;
`gui/svg` is its only internal-class consumer.

### Two stroker corrections ride along

Moving the core exposed two defects in the SVG stroker, confirmed with coverage
probes before the fix:

- The round-join fan swept the long way around the vertex. It painted the inner
  disc the quads already cover and left the outer corner open. It now sweeps the
  short way across the outer wedge.
- The square cap extended sideways along the normal instead of along the path.
  The cap call now takes the outward travel direction, recovered from the
  segment normal.

Round caps, miter joins and bevel joins were already correct and are
byte-identical. No SVG testdata uses round joins or square caps, and the full
suite passes unmodified.

## Rejected Approaches

- **Sticky `SetLineCap`/`SetLineJoin` on `DrawContext`.** Hidden state leaks
  across primitives and breaks the explicit immediate-mode calls. The style
  travels with the call instead.
- **Widening `DrawRecorder` with style parameters.** Every external implementer
  breaks in lockstep. The optional extension keeps them compiling and drawing,
  with documented lossy fallback.
- **Per-primitive cap/join parameters.** One more parameter per method, times
  eight primitives, with no room to grow. The style struct carries future
  settings without touching signatures.
- **A shared `internal/stroke` package.** A new package for one function,
  against the flat-packages rule, when the export policy already accepts the
  root as the hub.
- **A configurable miter limit in v1.** Surface, validation and golden matrix
  for a knob no caller requested. The limit stays 4x until one does.
