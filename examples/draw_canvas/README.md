# Draw Canvas

> **Framework:** graphics **Description:** Line chart, gradient fills and the
> translate/scale transform stack, using DrawCanvas and DrawContext.

![Preview](screenshot.png)

<!-- explorer: tags=graphics category=graphics run=go -->

---

## Run

```sh
go run ./examples/draw_canvas/
```

## What it demonstrates

Line chart, gradient fills, per-vertex colors and the transform stack, using
DrawCanvas and DrawContext.

The Transform panel draws one `badge` function four times. The function draws in
its own fixed 0..60 space and knows nothing about placement or size; `Translate`
and `ScaleBy` do the rest, including scaling its stroke widths and font size.
The last copy uses a negative scale, which mirrors the geometry but leaves the
label readable.

See `main.go` for the implementation.
