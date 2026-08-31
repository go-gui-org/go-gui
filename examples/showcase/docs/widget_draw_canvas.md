Procedural 2D drawing canvas with cached tessellation. Draw shapes, lines, text,
images, and arcs via the `OnDraw` callback. Output is tessellated into triangles
and cached by `Version`. The canvas re-draws only when the version changes.
Optional `Focusable` + `OnKeyDown` make the canvas keyboard-focusable.

## Usage

```go
gui.DrawCanvas(gui.DrawCanvasCfg{
    ID:      "my-canvas",
    Version: 1,
    Width:   400,
    Height:  300,
    Color:   gui.RGBA(30, 30, 40, 255),
    Radius:  8,
    Padding: gui.Some(gui.Padding{Top: 20, Right: 20,
        Bottom: 20, Left: 20}),
    OnDraw: func(dc *gui.DrawContext) {
        dc.FilledRoundedRect(10, 10, 100, 60, 8, gui.White)
        dc.DashedLine(0, 80, 400, 80, gui.Gray, 1, 6, 4)
        dc.PolylineJoined(pts, gui.White, 2)
        dc.Text(10, 90, "label", style)
    },
})
```

## Drawing API

### Shapes

| Method            | Signature                                                          | Description                            |
| ----------------- | ------------------------------------------------------------------ | -------------------------------------- |
| FilledRect        | (x, y, w, h float32, color Color)                                  | Filled rectangle                       |
| Rect              | (x, y, w, h float32, color Color, width float32)                   | Stroked rectangle                      |
| FilledRoundedRect | (x, y, w, h, radius float32, color Color)                          | Filled rectangle with rounded corners  |
| RoundedRect       | (x, y, w, h, radius float32, color Color, width float32)           | Stroked rectangle with rounded corners |
| FilledPolygon     | (points []float32, color Color)                                    | Filled convex polygon                  |
| FilledCircle      | (cx, cy, radius float32, color Color)                              | Filled circle                          |
| Circle            | (cx, cy, radius float32, color Color, width float32)               | Stroked circle                         |
| FilledArc         | (cx, cy, rx, ry, start, sweep float32, color Color)                | Filled elliptical arc                  |
| Arc               | (cx, cy, rx, ry, start, sweep float32, color Color, width float32) | Stroked elliptical arc                 |

### Lines

| Method         | Signature                                                             | Description                           |
| -------------- | --------------------------------------------------------------------- | ------------------------------------- |
| Line           | (x0, y0, x1, y1 float32, color Color, width float32)                  | Single solid line segment             |
| Polyline       | (points []float32, color Color, width float32)                        | Stroked open polyline (no joins)      |
| PolylineJoined | (points []float32, color Color, width float32)                        | Polyline with miter joins at vertices |
| DashedLine     | (x0, y0, x1, y1 float32, color Color, width, dashLen, gapLen float32) | Dashed line segment                   |
| DashedPolyline | (points []float32, color Color, width, dashLen, gapLen float32)       | Polyline with continuous dash pattern |

### Text

| Method     | Signature                                    | Description                             |
| ---------- | -------------------------------------------- | --------------------------------------- |
| Text       | (x, y float32, text string, style TextStyle) | Draw text at position (top-left origin) |
| TextWidth  | (text string, style TextStyle) float32       | Measure text width in given style       |
| FontHeight | (style TextStyle) float32                    | Line height for given style             |

### Images

| Method | Signature                                                               | Description                                                |
| ------ | ----------------------------------------------------------------------- | ---------------------------------------------------------- |
| Image  | (x, y, w, h float32, src string, bgOpacity Opt[float32], bgColor Color) | Draw image inside the canvas. `src` matches `ImageCfg.Src` |

`src` accepts the same forms as `ImageCfg.Src`:

- Local filesystem path
- `http://` / `https://` URL (cached on disk)
- `data:` URL (base64 payload)

`bgOpacity` is an `Opt[float32]` in [0, 1]. Zero value = 1.0. It modulates the
background-color alpha only. It does not fade the image texture itself.
`bgColor` paints behind the image (useful for PNGs with transparency). Zero
value = transparent.

Example:

```go
dc.Image(0, 0, 64, 64,
    "assets/tile.png",
    gui.SomeF(0.85), gui.Black)
```

### Transform

Every drawing method takes local coordinates. `Translate` and `ScaleBy` move and
size that local space, so drawing code written once can be placed and resized
without touching a single coordinate.

| Method    | Signature        | Description                              |
| --------- | ---------------- | ---------------------------------------- |
| Translate | (dx, dy float32) | Shift the origin, in current local units |
| ScaleBy   | (sx, sy float32) | Multiply the scale on each axis          |
| Save      | ()               | Push the current transform               |
| Restore   | ()               | Pop back to the transform `Save` pushed  |

Both are relative: they compose with what is already in force rather than
replacing it. `Save` and `Restore` bracket a change so it cannot leak into the
drawing that follows. Restoring with an empty stack does nothing.

`ScaleBy` is not called `Scale` because `DrawContext.Scale` is the device pixel
ratio field. The two are unrelated.

```go
for i, scale := range []float32{1, 1.5, 2} {
    dc.Save()
    dc.Translate(float32(i)*80, 0)
    dc.ScaleBy(scale, scale)
    badge(dc) // draws in its own 0..60 space, unaware of any of this
    dc.Restore()
}
```

The transform applies to everything: positions, stroke widths, font sizes, image
rects and gradients. A scale of 2 doubles the width of a 1px line and the size
of a 10px font, because both are stated in local units.

Four limits are worth knowing:

- Curves are flattened in local space, so scaling a circle up leaves it as
  faceted as the unscaled one. Draw it at its final size for a smooth result.
- A non-uniform scale positions and sizes a text run but does not shear its
  glyphs.
- A negative scale moves an image's rect but never mirrors its content.
- `TextWidth` and `FontHeight` answer in local units, which is the space `Text`
  takes its arguments in. Do not scale their results.

A transform that comes from application state — a zoom level, say — is an
`OnDraw` input like any other, so bump `Version` when it changes or the cached
tessellation is reused.

## Keyboard Focus

Setting `Focusable: true` opts the canvas into tab order (with a non-empty
`ID`). The paired `OnKeyDown` callback fires when the canvas is focused and a
key is pressed. Set `ctx.Consume()` to stop propagation. Bump `Version` to
redraw after state changes.

```go
gui.DrawCanvas(gui.DrawCanvasCfg{
    ID:      "my-canvas",
    Focusable: true,
    Version: app.MyCanvasVersion,
    Width:   480, Height: 280,
    OnDraw: drawScene,
    OnKeyDown: func(ctx gui.EventCtx) {
        a := gui.State[App](ctx.Window)
        switch ctx.Event.KeyCode {
        case gui.KeyLeft:
            a.MarkerX -= 10
        case gui.KeyRight:
            a.MarkerX += 10
        default:
            return
        }
        a.MyCanvasVersion++
        ctx.Consume()
    },
})
```

## Key Properties

| Property  | Type               | Description                                         |
| --------- | ------------------ | --------------------------------------------------- |
| ID        | string             | Cache key (required)                                |
| Version   | uint64             | Bump to invalidate cache                            |
| Width     | float32            | Canvas width                                        |
| Height    | float32            | Canvas height                                       |
| Color     | Color              | Background fill                                     |
| Radius    | float32            | Corner radius                                       |
| Padding   | Opt[Padding]       | Inner padding (shrinks draw area)                   |
| Clip      | bool               | Clip drawing to bounds                              |
| Focusable | bool               | Keyboard focus and tab order (needs a non-empty ID) |
| OnDraw    | func(*DrawContext) | Drawing callback                                    |

## Events

| Callback      | Signature      | Fired when                          |
| ------------- | -------------- | ----------------------------------- |
| OnClick       | func(EventCtx) | Canvas clicked                      |
| OnHover       | func(EventCtx) | Mouse enters canvas                 |
| OnMouseScroll | func(EventCtx) | Scroll wheel on canvas              |
| OnKeyDown     | func(EventCtx) | Key pressed while canvas is focused |

## Caching

Tessellation is cached per `ID`. Bump `Version` when data changes to trigger a
re-draw. Same version = same triangles, zero cost per frame. Images are cached
alongside triangles, so bump `Version` to pick up a new `src` within the same
widget ID.

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

A focusable canvas advertises as an interactive element (button role) to
assistive tech. Non-focusable canvases advertise as images. Provide a meaningful
`A11YLabel` on interactive canvases.

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
