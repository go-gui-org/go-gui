Custom scrollbar styling for scrollable containers. Applied via
ScrollbarCfgX/ScrollbarCfgY on ContainerCfg, or used directly. Supports drag,
gutter click, and auto-hide behavior.

## Usage (Container Override)

```go
gui.Column(gui.ContainerCfg{
    ID:            "scrolling-panel",
    Scrollable:    true,
    ScrollbarCfgY: &gui.ScrollbarCfg{
        GapEdge:  gui.SomeF(4),
    },
    Content: views,
})
```

## Hide Scrollbar

```go
gui.Column(gui.ContainerCfg{
    ID:            "hidden-scroll",
    Scrollable:    true,
    ScrollbarCfgX: &gui.ScrollbarCfg{
        Overflow: gui.ScrollbarHidden,
    },
    Content: views,
})
```

## Key Properties

| Property     | Type                 | Description                                    |
| ------------ | -------------------- | ---------------------------------------------- |
| ID           | string               | Unique identifier                              |
| Orientation  | ScrollbarOrientation | Horizontal or vertical                         |
| Size         | float32              | Scrollbar thickness                            |
| MinThumbSize | float32              | Minimum thumb length                           |
| Radius       | float32              | Track corner radius                            |
| RadiusThumb  | float32              | Thumb corner radius                            |
| GapEdge      | Opt[float32]         | Gap from container edge; unset takes the theme |
| GapEnd       | Opt[float32]         | Gap from track ends; unset takes the theme     |
| Overflow     | ScrollbarOverflow    | Visibility mode                                |

## Appearance

| Property        | Type  | Description            |
| --------------- | ----- | ---------------------- |
| ColorThumb      | Color | Thumb color            |
| ColorBackground | Color | Track background color |

## Overflow Modes

| Constant         | Behavior                              |
| ---------------- | ------------------------------------- |
| ScrollbarAuto    | Show when content overflows (default) |
| ScrollbarHidden  | Never show                            |
| ScrollbarVisible | Always show                           |
