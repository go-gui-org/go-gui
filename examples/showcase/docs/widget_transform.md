Visual transforms on containers. `ContainerCfg.Transform` moves, turns, scales
or skews a container and everything in it. Layout ignores it: the container
keeps its place, and so do its neighbours. Pointer input and the IME caret
follow the drawn view.

## Usage

```go
gui.Column(gui.ContainerCfg{
    Width:     72,
    Height:    44,
    Sizing:    gui.FixedFixed,
    Color:     theme.ColorSelect,
    Transform: gui.Rotate(-12),
    Content: []gui.View{
        gui.Text(gui.TextCfg{Text: "NEW"}),
    },
})
```

## Parts

```go
gui.Translate(24, 12)   // move by (x, y) logical pixels
gui.Rotate(15)          // turn 15 degrees clockwise on screen
gui.RotateX(35)         // turn around the horizontal axis
gui.RotateY(-35)        // turn around the vertical axis
gui.Skew(12, 6)         // slant along X and Y, as CSS skew()
gui.Scale(1.3)          // scale both axes
gui.ScaleXY(-1, 1)      // mirror horizontally
gui.Perspective(500)    // viewing distance for RotateX/RotateY
gui.Origin(0, 0)        // origin at the top-left corner
```

The parts apply in one fixed order, outermost first, whatever order the methods
are chained in:

translate → perspective → rotateX → rotateY → rotate → skew → scale

about the origin, the center by default. A method sets its component and
replaces an earlier value: `Rotate(10).Rotate(20)` is `Rotate(20)`. The zero
value is the identity. `BackfaceHidden` hides a view while its back faces the
viewer, as CSS `backface-visibility: hidden`; use it for the two faces of a card
flip.

## Animate

```go
gui.Column(gui.ContainerCfg{
    ID:          "card",
    TransformFn: func() gui.Transform { return current },
})
```

`TransformFn` is read on every draw and wins over `Transform`. Return a spring's
or tween's current value and the view moves in render-only frames with no
rebuild. It runs under the frame lock: read animation state and return, nothing
else. `gui.HoverLift` owns that tween for a hover lift.

## Notes

- `Lerp` interpolates each component on its own, so a tween or spring can
  animate any transform into any other.
- `IsIdentity` reports whether a transform moves no point.
- `Window.LocalToWindow` and `Window.WindowToLocal` map a point between a
  widget's box and the window, through every transform above it.
- `RotatedBox` (quarter turns) is the layout-changing alternative; see
  `docs/widget_rotated_box.md`.
