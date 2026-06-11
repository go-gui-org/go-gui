Gaussian blur applied to a shape's fill. Set via the `BlurRadius`
field on `ContainerCfg`, `ButtonCfg`, or `RectangleCfg`.
Higher values produce softer, more diffused edges. The fill color must
have `A > 0` for the blur to be visible.

## Usage

```go
gui.Column(gui.ContainerCfg{
    Width:      150,
    Height:     150,
    Sizing:     gui.FixedFixed,
    Radius:     gui.SomeF(75),
    Color:      gui.RGBA(0, 255, 0, 150),
    BlurRadius: 20,
})
```

## Soft Rectangle

```go
gui.Column(gui.ContainerCfg{
    Width:      150,
    Height:     150,
    Sizing:     gui.FixedFixed,
    Radius:     gui.SomeF(20),
    Color:      gui.RGBA(255, 100, 100, 200),
    BlurRadius: 10,
})
```

## Heavy Glow

Large blur values create diffused glow effects.

```go
gui.Column(gui.ContainerCfg{
    Width:      200,
    Height:     100,
    Sizing:     gui.FixedFixed,
    Radius:     gui.SomeF(10),
    Color:      gui.RGBA(60, 120, 255, 255),
    BlurRadius: 50,
})
```

## Widgets That Support BlurRadius

| Widget       | Field      | Type    |
|--------------|------------|---------|
| ContainerCfg | BlurRadius | float32 |
| ButtonCfg    | BlurRadius | float32 |
| RectangleCfg | BlurRadius | float32 |

## Notes

- `BlurRadius` is a plain `float32`, not `Opt[float32]`. Zero means no blur.
- Combine with `Radius` (corner rounding) to create orb or pill glows.
- Blur is rendered via a GPU shader pass; large radii have minimal
  extra cost on Metal/OpenGL backends.
- Distinct from `BoxShadow.BlurRadius` which blurs a drop shadow,
  not the shape fill itself.
