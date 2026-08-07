# svg_hittest

Click any shape in the rendered SVG; the right pane prints the matched `PathID`
and viewBox coordinates. Demonstrates `TessellatedPath.ContainsPoint` with the
typical "display→viewBox" coord conversion (divide by `cached.Scale`, add
viewBox origin).

## Run

```
go run ./examples/svg_hittest
```

Click in the empty space between shapes to see the empty-cell readout; click on
a circle/rect/triangle to see its PathID. Note that `ContainsPoint` ignores
stroke paths — fill triangulation is the hit target.
