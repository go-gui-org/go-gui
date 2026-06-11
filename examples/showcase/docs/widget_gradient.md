Linear and radial gradients with configurable direction
and color stops. Applied via the `Gradient` field on any container
or rectangle. `BorderGradient` applies a gradient to the border.

## Linear Gradient

```go
gui.Column(gui.ContainerCfg{
    Gradient: &gui.GradientDef{
        Direction: gui.GradientToRight,
        Stops: []gui.GradientStop{
            {Pos: 0, Color: gui.ColorFromString("#3b82f6")},
            {Pos: 1, Color: gui.ColorFromString("#8b5cf6")},
        },
    },
})
```

## Radial Gradient

```go
gui.Column(gui.ContainerCfg{
    Gradient: &gui.GradientDef{
        Type: gui.GradientRadial,
        Stops: []gui.GradientStop{
            {Pos: 0, Color: gui.White},
            {Pos: 1, Color: gui.ColorFromString("#3b82f6")},
        },
    },
})
```

## Border Gradient

```go
gui.Column(gui.ContainerCfg{
    SizeBorder: gui.SomeF(2),
    BorderGradient: &gui.GradientDef{
        Direction: gui.GradientToRight,
        Stops: []gui.GradientStop{
            {Pos: 0, Color: gui.ColorFromString("#3b82f6")},
            {Pos: 1, Color: gui.ColorFromString("#8b5cf6")},
        },
    },
})
```

## Custom Angle

```go
gui.Column(gui.ContainerCfg{
    Gradient: &gui.GradientDef{
        HasAngle: true,
        Angle:    135,
        Stops: []gui.GradientStop{
            {Pos: 0, Color: gui.ColorFromString("#f97316")},
            {Pos: 1, Color: gui.ColorFromString("#ef4444")},
        },
    },
})
```

## GradientDef Properties

| Property  | Type              | Description                          |
|-----------|-------------------|--------------------------------------|
| Type      | GradientType      | GradientLinear (default) or GradientRadial |
| Direction | GradientDirection | Named direction constant              |
| Angle     | float32           | Explicit angle in degrees            |
| HasAngle  | bool              | True when Angle overrides Direction  |
| Stops     | []GradientStop    | Color stops (Pos 0.0-1.0)           |

## Gradient Types

| Type           | Description              |
|----------------|--------------------------|
| GradientLinear | Linear gradient (default) |
| GradientRadial | Radial gradient          |

## Directions

| Constant              | Angle |
|-----------------------|-------|
| GradientToTop         | 0°    |
| GradientToTopRight    | 45°   |
| GradientToRight       | 90°   |
| GradientToBottomRight | 135°  |
| GradientToBottom      | 180°  |
| GradientToBottomLeft  | 225°  |
| GradientToLeft        | 270°  |
| GradientToTopLeft     | 315°  |

## GradientStop

| Field | Type    | Description                |
|-------|---------|----------------------------|
| Color | Color   | Stop color                 |
| Pos   | float32 | Position along gradient (0.0-1.0) |
