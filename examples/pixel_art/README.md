# Pixel Art

> **Framework:** graphics **Description:** One small sprite drawn at 12x, smooth
> with the linear image filter and sharp with the nearest filter.

![Preview](screenshot.png)

<!-- explorer: tags=graphics category=graphics run=go -->

---

## Run

```sh
go run ./examples/pixel_art/
```

## What it demonstrates

`DrawContext.SetImageFilter` sets how the image calls that follow sample their
texels. The default, `ImageFilterLinear`, blends texels, so a small image drawn
large looks blurred. `ImageFilterNearest` takes one texel per pixel, so each
texel stays a sharp square. Use it for pixel art, retro UIs and small sprites.

- The sprite is an 8x8 in-memory image registered with `gui.UseImage`.
- The filter is saved and restored by `Save`/`Restore`, with the transform.
- Each `OnDraw` starts with the linear filter.

## How it works

```go
// drawSprites draws the sprite twice at the same size. The left copy
// uses the default linear filter and looks blurred. The right copy
// sets the nearest filter inside Save/Restore, so each texel stays a
// sharp square and the filter goes back to linear after it.
func drawSprites(dc *gui.DrawContext, src string) {
	dc.Image(0, 0, drawn, drawn, src, gui.Opt[float32]{}, gui.Color{})

	dc.Save()
	dc.SetImageFilter(gui.ImageFilterNearest)
	dc.Image(drawn+gap, 0, drawn, drawn, src, gui.Opt[float32]{}, gui.Color{})
	dc.Restore()
}
```

See `main.go` for the implementation.
