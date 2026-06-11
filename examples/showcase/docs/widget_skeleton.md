Shimmer placeholder for content that is still loading.
Displays an animated gradient sweep over a solid base color,
signaling that real content will appear once data arrives.

## Usage

```go
gui.Skeleton(gui.SkeletonCfg{
    ID:     "sk-line",
    Sizing: gui.FillFixed,
    Height: 14,
})
```

## Circle Variant

```go
gui.Skeleton(gui.SkeletonCfg{
    ID:      "sk-avatar",
    Variant: gui.SkeletonCircle,
    Width:   48,
    Height:  48,
})
```

## Card Placeholder

```go
gui.Row(gui.ContainerCfg{
    Sizing:  gui.FillFit,
    Spacing: gui.SomeF(12),
    Content: []gui.View{
        gui.Skeleton(gui.SkeletonCfg{
            ID: "avatar", Variant: gui.SkeletonCircle,
            Width: 48, Height: 48,
        }),
        gui.Column(gui.ContainerCfg{
            Sizing:  gui.FillFit,
            Spacing: gui.SomeF(6),
            Content: []gui.View{
                gui.Skeleton(gui.SkeletonCfg{ID: "l1", Sizing: gui.FillFixed, Height: 14}),
                gui.Skeleton(gui.SkeletonCfg{ID: "l2", Width: 200, Height: 14}),
            },
        }),
    },
})
```

## Custom Colors

```go
gui.Skeleton(gui.SkeletonCfg{
    ID:             "sk-custom",
    Sizing:         gui.FillFixed,
    Height:         24,
    Color:          gui.RGB(60, 60, 80),
    ColorHighlight: gui.RGB(100, 100, 140),
})
```

## Key Properties

| Property  | Type            | Description                          |
|-----------|-----------------|--------------------------------------|
| ID        | string          | Unique identifier (drives animation) |
| Variant   | SkeletonVariant | Shape variant (rect or circle)       |
| Sizing    | Sizing          | Combined axis sizing mode            |
| Width     | float32         | Fixed width                          |
| Height    | float32         | Fixed height                         |
| MinWidth  | float32         | Minimum width                        |
| MaxWidth  | float32         | Maximum width                        |
| MinHeight | float32         | Minimum height                       |
| MaxHeight | float32         | Maximum height                       |
| Disabled  | bool            | Disable the shimmer animation        |
| Invisible | bool            | Hide without removing from layout    |

## Appearance

| Property       | Type         | Description                       |
|----------------|--------------|-----------------------------------|
| Color          | Color        | Base skeleton color               |
| ColorHighlight | Color        | Shimmer highlight color           |
| Radius         | Opt[float32] | Corner radius                     |

## Variants

| Variant        | Description                                |
|----------------|--------------------------------------------|
| SkeletonRect   | Rounded rectangle (default)                |
| SkeletonCircle | Circle, sized by Width and Height          |

## Accessibility

| Property        | Type   | Description                            |
|-----------------|--------|----------------------------------------|
| A11YLabel       | string | Accessible label (default "Loading")   |
| A11YDescription | string | Accessible description                 |
