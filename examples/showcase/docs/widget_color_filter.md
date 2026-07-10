Post-processing color matrix transforms on container content.
Set `ColorFilter` on `ContainerCfg` to apply a 4×4 color
matrix to the container's rendered content. The container is captured
into an FBO, the matrix is applied via a GPU shader pass, and the
result is composited back. Works independently of or combined with
`BlurRadius`.

## Grayscale

```go
gui.Column(gui.ContainerCfg{
    Width:       120,
    Sizing:      gui.FixedFit,
    Color:       gui.RGBA(200, 130, 100, 255),
    ColorFilter: gui.ColorFilterGrayscale(),
})
```

## Sepia

```go
gui.Column(gui.ContainerCfg{
    Width:       120,
    Sizing:      gui.FixedFit,
    ColorFilter: gui.ColorFilterSepia(),
    Content:     []gui.View{ /* ... */ },
})
```

## Saturate

0 = grayscale, 1 = identity, >1 = oversaturated.

```go
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterSaturate(2.0),
    Content:     []gui.View{ /* ... */ },
})
```

## Brightness

1 = identity, <1 = dim, >1 = bright.

```go
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterBrightness(1.5),
    Content:     []gui.View{ /* ... */ },
})
```

## Contrast

1 = identity, 0 = all gray, >1 = higher contrast. Bias is injected
via the alpha column (correct for premultiplied-alpha FBO content).

```go
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterContrast(1.5),
    Content:     []gui.View{ /* ... */ },
})
```

## Hue Rotate

Rodrigues rotation around the (1,1,1) axis in RGB space.

```go
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterHueRotate(90),
    Content:     []gui.View{ /* ... */ },
})
```

## Invert

Negates RGB channels. Uses the alpha column to inject bias
(output.rgb = alpha − input.rgb).

```go
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterInvert(),
    Content:     []gui.View{ /* ... */ },
})
```

## Combined with Blur

When both `BlurRadius` and `ColorFilter` are set, the FBO
pipeline runs gaussian blur first, then the color matrix pass,
then composites. The SDF blur (`RenderBlur`) is suppressed.

```go
gui.Column(gui.ContainerCfg{
    BlurRadius:  15,
    ColorFilter: gui.ColorFilterBrightness(1.3),
    Color:       gui.RGBA(0, 255, 128, 255),
    Radius:      gui.SomeF(50),
})
```

## Constructors

| Constructor                  | Description                              |
|------------------------------|------------------------------------------|
| ColorFilterIdentity()        | No-op (identity matrix)                  |
| ColorFilterGrayscale()       | Luminance-weighted grayscale             |
| ColorFilterSepia()           | Warm sepia tone                          |
| ColorFilterSaturate(amount)  | 0=gray, 1=identity, >1=oversaturated     |
| ColorFilterBrightness(amount)| Scale RGB channels                       |
| ColorFilterContrast(amount)  | Scale around 0.5 midpoint                |
| ColorFilterHueRotate(deg)    | Rotate hue in degrees                    |
| ColorFilterInvert()          | Negate RGB, keep alpha                   |

## Custom Matrix

For effects not covered by the built-in constructors, set
`Matrix` directly. Column-major `[16]float32`, applied as
`clamp(M * pixel, 0, 1)` in the fragment shader.

```go
gui.Column(gui.ContainerCfg{
    ColorFilter: &gui.ColorFilter{
        Matrix: [16]float32{
            1, 0, 0, 0,  // column 0
            0, 1, 0, 0,  // column 1
            0, 0, 1, 0,  // column 2
            0, 0, 0, 1,  // column 3
        },
    },
})
```

## Notes

- Containers only — not available on individual widgets.
- No nesting. A single FBO pair is used; nested filter brackets
  are guarded against at the render level.
- Operates on premultiplied-alpha pixels from the FBO.
- Contrast and invert inject bias via the alpha column of the
  matrix, which is correct for premultiplied content.
- Rendered via Metal and OpenGL backends.
  ignores the color matrix.
