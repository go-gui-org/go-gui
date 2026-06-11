Procedural 2D drawing canvas with cached tessellation.
Draw shapes, lines, text, images, and arcs via the
`OnDraw` callback. Output is tessellated into triangles
and cached by `Version` — only re-drawn when the version
changes. Optional `IDFocus` + `OnKeyDown` make the
canvas keyboard-focusable.

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

| Method           | Signature                                                          | Description                        |
|------------------|--------------------------------------------------------------------|------------------------------------|
| FilledRect       | (x, y, w, h float32, color Color)                                 | Filled rectangle                   |
| Rect             | (x, y, w, h float32, color Color, width float32)                  | Stroked rectangle                  |
| FilledRoundedRect| (x, y, w, h, radius float32, color Color)                         | Filled rectangle with rounded corners |
| RoundedRect      | (x, y, w, h, radius float32, color Color, width float32)          | Stroked rectangle with rounded corners |
| FilledPolygon    | (points []float32, color Color)                                    | Filled convex polygon              |
| FilledCircle     | (cx, cy, radius float32, color Color)                              | Filled circle                      |
| Circle           | (cx, cy, radius float32, color Color, width float32)               | Stroked circle                     |
| FilledArc        | (cx, cy, rx, ry, start, sweep float32, color Color)                | Filled elliptical arc              |
| Arc              | (cx, cy, rx, ry, start, sweep float32, color Color, width float32) | Stroked elliptical arc             |

### Lines

| Method          | Signature                                                                     | Description                             |
|-----------------|-------------------------------------------------------------------------------|-----------------------------------------|
| Line            | (x0, y0, x1, y1 float32, color Color, width float32)                         | Single solid line segment               |
| Polyline        | (points []float32, color Color, width float32)                                | Stroked open polyline (no joins)        |
| PolylineJoined  | (points []float32, color Color, width float32)                                | Polyline with miter joins at vertices   |
| DashedLine      | (x0, y0, x1, y1 float32, color Color, width, dashLen, gapLen float32)        | Dashed line segment                     |
| DashedPolyline  | (points []float32, color Color, width, dashLen, gapLen float32)               | Polyline with continuous dash pattern   |

### Text

| Method     | Signature                                       | Description                                 |
|------------|--------------------------------------------------|---------------------------------------------|
| Text       | (x, y float32, text string, style TextStyle)     | Draw text at position (top-left origin)     |
| TextWidth  | (text string, style TextStyle) float32           | Measure text width in given style           |
| FontHeight | (style TextStyle) float32                        | Line height for given style                 |

### Images

| Method | Signature                                                                           | Description                                         |
|--------|-------------------------------------------------------------------------------------|-----------------------------------------------------|
| Image  | (x, y, w, h float32, src string, bgOpacity Opt[float32], bgColor Color)             | Draw image inside the canvas; `src` matches `ImageCfg.Src` |

`src` accepts the same forms as `ImageCfg.Src`:

- Local filesystem path
- `http://` / `https://` URL (cached on disk)
- `data:` URL (base64 payload)

`bgOpacity` is an `Opt[float32]` in [0, 1]; zero value = 1.0.
It modulates the background-color alpha only; it does not fade the
image texture itself. `bgColor` paints behind the image
(useful for PNGs with transparency); zero value = transparent.

Example:

```go
dc.Image(0, 0, 64, 64,
    "assets/tile.png",
    gui.SomeF(0.85), gui.Black)
```

## Keyboard Focus

Setting `IDFocus > 0` opts the canvas into tab order. The
paired `OnKeyDown` callback fires when the canvas is focused
and a key is pressed. Set `e.IsHandled = true` to stop
propagation. Bump `Version` to redraw after state changes.

```go
gui.DrawCanvas(gui.DrawCanvasCfg{
    ID:      "my-canvas",
    IDFocus: focusMyCanvas,
    Version: app.MyCanvasVersion,
    Width:   480, Height: 280,
    OnDraw: drawScene,
    OnKeyDown: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
        a := gui.State[App](w)
        switch e.KeyCode {
        case gui.KeyLeft:
            a.MarkerX -= 10
        case gui.KeyRight:
            a.MarkerX += 10
        default:
            return
        }
        a.MyCanvasVersion++
        e.IsHandled = true
    },
})
```

## Key Properties

| Property  | Type                           | Description                            |
|-----------|--------------------------------|----------------------------------------|
| ID        | string                         | Cache key (required)                   |
| Version   | uint64                         | Bump to invalidate cache               |
| Width     | float32                        | Canvas width                           |
| Height    | float32                        | Canvas height                          |
| Color     | Color                          | Background fill                        |
| Radius    | float32                        | Corner radius                          |
| Padding   | Opt[Padding]                   | Inner padding (shrinks draw area)      |
| Clip      | bool                           | Clip drawing to bounds                 |
| IDFocus   | uint32                         | Focus / tab order (0 = not focusable)  |
| OnDraw    | func(*DrawContext)             | Drawing callback                       |

## Events

| Callback      | Signature                          | Fired when                                 |
|---------------|------------------------------------|--------------------------------------------|
| OnClick       | func(*Layout, *Event, *Window)     | Canvas clicked                             |
| OnHover       | func(*Layout, *Event, *Window)     | Mouse enters canvas                        |
| OnMouseScroll | func(*Layout, *Event, *Window)     | Scroll wheel on canvas                     |
| OnKeyDown     | func(*Layout, *Event, *Window)     | Key pressed while canvas is focused        |

## Caching

Tessellation is cached per `ID`. Bump `Version` when data
changes to trigger a re-draw. Same version = same triangles,
zero cost per frame. Images are cached alongside triangles, so
bump `Version` to pick up a new `src` within the same widget ID.

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |

Setting `IDFocus > 0` advertises the canvas as an interactive
element (button role) to assistive tech; non-focusable canvases
advertise as images. Provide a meaningful `A11YLabel` on
interactive canvases.
