# Shader Channels

> **Framework:** graphics, performance **Description:** A fragment shader reads
> the planes of a PixelSurface as iChannel0 and iChannel1, here NV12 video
> frames converted to RGB on the GPU. Requires a GPU.

<!-- explorer: tags=graphics,performance category=graphics run=go-gpu -->

---

## Run

```sh
go run ./examples/shader_channels/
```

## What it demonstrates

A goroutine publishes 60 frames a second in NV12, the layout most video decoders
produce: a full-size luma plane and a half-size plane with the two chroma values
interleaved. A container's shader reads them as `iChannel0` and `iChannel1` and
converts them to RGB. The frames are color bars with a moving marker, converted
to YUV in Go, so no video code is involved.

The headless `soft` backend runs no shaders. A screenshot leaves the frame area
empty, and the window notes it: `Window.RendersShaders` reports which case
applies, so an app can draw an RGBA8 `Image` instead.

## How it works

- `NewPixelSurface(PixelSurfaceCfg{Planes: ...})` creates a surface with an `R8`
  plane and an `RG8` plane of half the size.
- `UpdatePlanes` copies both planes and publishes them as one frame, so the GPU
  never draws one plane's new pixels with the other's old ones. It allocates
  nothing at a fixed size.
- `Shader.Surface` binds the surface to the shader: plane _i_ is `iChannel`_i_.
  GLSL reads it with `texture(iChannel0, p)`, MSL with
  `iChannel0.sample(iChannelSampler, p)`. `p = uv * 0.5 + 0.5` puts (0,0) at the
  top-left pixel. `iChannelResolution[i]` holds plane _i_'s size.
- A new frame redraws the container without rebuilding the layout, and without
  `Shader.Animate`.

See `main.go` for the conversion and both shader bodies.
