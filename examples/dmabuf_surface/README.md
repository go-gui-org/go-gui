# DMA-BUF Surface

> **Framework:** graphics, performance **Description:** Frames that live in GPU
> buffers (Linux DMA-BUFs) drawn by a shader without a copy, the way a hardware
> video decoder hands them over. Requires a GPU; elsewhere the frames are
> copied.

<!-- explorer: tags=graphics,performance category=graphics run=go-gpu -->

---

## Run

```sh
go run ./examples/dmabuf_surface/
```

## What it demonstrates

A hardware video decoder leaves each picture in GPU memory. Copying it to main
memory and uploading it again costs more than the decoding. Here a goroutine
stands in for such a decoder: it paints NV12 frames into a small pool of GBM
buffers and lends each to a `PixelSurface` with `UpdateDMABuf`. The OpenGL
backend on Linux imports the buffers and the shader samples them directly.

The window's note says which path runs:

- **Frames in DMA-BUFs, drawn without a copy:** Linux, a GPU with a DRM render
  node, and an EGL display that imports DMA-BUFs (Mesa on Wayland or X11).
- **Frames are copied with UpdatePlanes:** any other window. The producer checks
  `Window.ImportsDMABuf` every frame and copies the same picture instead.
- **This renderer runs no shaders:** the software renderer and headless
  screenshots.

## How it works

- `UpdateDMABuf(planes, frame)` takes one `DMABufPlane` per configured plane,
  here an `R8` luma buffer and a `GR88` chroma buffer at half size. Each names
  the buffer's file descriptor, offset, pitch and DRM format modifier.
- `frame` is a `GPUFrame`: a pooled pointer, so publishing allocates nothing.
  The surface calls its `Release` once no renderer draws it any more, after a
  later frame or `Close` replaced it and the GPU finished reading it. Here
  `Release` returns the frame to the pool, and only a frame back in the pool is
  painted again. When all three are still lent, the producer skips a tick.
- `Release` runs on a render thread and must not block: the pool's channel has
  room for every frame.
- The shader is the one in `examples/shader_channels`. It reads an imported
  frame and a copied one the same way.

`internal/gbm` allocates the buffers. It is a stand-in for a decoder, and the GL
backend's import test uses it too.
