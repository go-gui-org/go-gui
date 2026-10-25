# Pixel Surface

An animated 640x360 color-bar image with a moving white marker. A goroutine
publishes a new frame 60 times a second through `PixelSurface.Update`, and the
window redraws without rebuilding its layout. No decoder or video file is
involved: the frames are synthetic, so the example shows only the surface API.

Run from the repository root:

```sh
go run ./examples/pixel_surface/
```

To write a static frame to a PNG without opening a window:

```sh
go run ./examples/pixel_surface/ -screenshot build/pixel-surface.png
```

## How it works

Create a `PixelSurface` once, keep it in the window state, and pass it as
`ImageCfg.Surface`. A non-nil surface takes precedence over `ImageCfg.Src`.

- `Update` takes straight-alpha RGBA8 pixels and a byte stride. It copies the
  pixels before it returns, so the producer can reuse its buffer at once, and it
  replaces the previous frame instead of queueing frames. It is safe to call
  from any goroutine.
- Every window drawing the surface redraws on the next frame. Layout is not
  rebuilt, and each renderer uploads only when the surface has a new frame.
- `Resize` publishes transparent pixels at the new size.
- `Close` rejects further updates. Windows stop drawing the surface and each
  renderer releases its texture on its next frame. Stop the producer, then close
  the surface.

At a fixed size, updates reuse CPU and GPU storage and allocate nothing. A
surface holds at most 64 MiB of pixels, with an 8192-pixel limit per edge. Each
renderer keeps its own uploaded copy: these are copies, not native zero-copy
frames.

The surface draws on every backend: OpenGL, Metal, WebGL/Canvas2D, Android GLES,
iOS Metal, the software renderer and PDF export.
