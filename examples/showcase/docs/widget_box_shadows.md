Drop shadows on containers, buttons, rectangles, and
other elements. Set via the `Shadow` field (`*BoxShadow`) on any
`ContainerCfg`, `RectangleCfg`, `ButtonCfg`, or `SidebarCfg`.

## Usage

```go
gui.Column(gui.ContainerCfg{
    Shadow: &gui.BoxShadow{
        OffsetX:    4,
        OffsetY:    4,
        BlurRadius: 16,
        Color:      gui.RGBA(0, 0, 0, 80),
    },
})
```

## Elevated Card

```go
gui.Column(gui.ContainerCfg{
    Radius: gui.SomeF(8),
    Shadow: &gui.BoxShadow{
        OffsetX:      0,
        OffsetY:      2,
        BlurRadius:   8,
        Color:        gui.RGBA(0, 0, 0, 60),
    },
    Content: []gui.View{ /* ... */ },
})
```

## BoxShadow Properties

| Property     | Type    | Description                              |
|--------------|---------|------------------------------------------|
| OffsetX      | float32 | Horizontal shadow offset                 |
| OffsetY      | float32 | Vertical shadow offset                   |
| BlurRadius   | float32 | Shadow blur amount                       |
| Color        | Color   | Shadow color (use RGBA for transparency) |

Positive OffsetX shifts right, positive OffsetY shifts down.
