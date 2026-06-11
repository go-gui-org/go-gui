Custom scrollbar styling for scrollable containers. Applied
via ScrollbarCfgX/ScrollbarCfgY on ContainerCfg, or used directly.
Supports drag, gutter click, and auto-hide behavior.

## Usage (Container Override)

```go
gui.Column(gui.ContainerCfg{
    IDScroll:      myScrollID,
    ScrollbarCfgY: &gui.ScrollbarCfg{
        GapEdge:  4,
        Overflow: gui.ScrollbarOnHover,
    },
    Content: views,
})
```

## Hide Scrollbar

```go
gui.Column(gui.ContainerCfg{
    IDScroll:      myScrollID,
    ScrollbarCfgX: &gui.ScrollbarCfg{
        Overflow: gui.ScrollbarHidden,
    },
    Content: views,
})
```

## Key Properties

| Property        | Type               | Description                      |
|-----------------|--------------------|----------------------------------|
| ID              | string             | Unique identifier                |
| IDScroll        | uint32             | Scroll container to attach to    |
| Orientation     | ScrollbarOrientation | Horizontal or vertical         |
| Size            | float32            | Scrollbar thickness              |
| MinThumbSize    | float32            | Minimum thumb length             |
| Radius          | float32            | Track corner radius              |
| RadiusThumb     | float32            | Thumb corner radius              |
| GapEdge         | float32            | Gap from container edge          |
| GapEnd          | float32            | Gap from track ends              |
| Overflow        | ScrollbarOverflow  | Visibility mode                  |

## Appearance

| Property        | Type  | Description                          |
|-----------------|-------|--------------------------------------|
| ColorThumb      | Color | Thumb color                          |
| ColorBackground | Color | Track background color               |

## Overflow Modes

| Constant         | Behavior                               |
|------------------|----------------------------------------|
| ScrollbarAuto    | Show when content overflows (default)  |
| ScrollbarHidden  | Never show                             |
| ScrollbarVisible | Always show                            |
| ScrollbarOnHover | Show on mouse hover only               |
