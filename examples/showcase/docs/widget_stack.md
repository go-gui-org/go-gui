Layered container: children go one over the other. The first child is the bottom
layer and the last child is the top layer. Uses ContainerCfg for all sizing,
alignment, scrolling, borders, and event handling.

## Usage

```go
gui.Stack(gui.ContainerCfg{
    Sizing: gui.FillFixed,
    Height: 160,
    HAlign: gui.HAlignCenter,
    VAlign: gui.VAlignMiddle,
    Content: []gui.View{
        photo,   // bottom layer
        caption, // top layer, placed by the Stack's HAlign and VAlign
    },
})
```

## Place one layer on its own

The Stack's `HAlign` and `VAlign` place every layer. To put one layer in a
different place, wrap it in a `FillFill` Column with its own alignment:

```go
gui.Column(gui.ContainerCfg{
    Sizing:     gui.FillFill,
    SizeBorder: gui.NoBorder,
    HAlign:     gui.HAlignEnd,
    VAlign:     gui.VAlignTop,
    Content:    []gui.View{badge},
})
```

A wrapper with no click handler does not stop a press. The press goes on to the
layers under it.

## Rules

| Topic        | Behavior                                                        |
| ------------ | --------------------------------------------------------------- |
| Position     | Every layer starts at the content origin (inside the padding).  |
| Fit size     | The largest layer on each axis, plus padding.                   |
| Fill layer   | Takes the whole content box on both axes.                       |
| Alignment    | `HAlign` and `VAlign` place each layer on both axes.            |
| Spacing      | Has no effect.                                                  |
| Draw order   | First child first. The last child is drawn on top.              |
| Pointer      | The top layer gets a press first.                               |
| Float child  | Out of flow, as in a Row or Column. It does not size the Stack. |
| Scroll, clip | Layers stay in the tree. They scroll and clip with the Stack.   |

## Stack or Float

Use a Stack for layers that belong inside a container: a background with
controls over it, a badge on an avatar, a game HUD. Use `Float` for content that
must escape its container and draw over everything, such as a dropdown, a menu
or a toast.

## Key Properties

| Property   | Type            | Description                           |
| ---------- | --------------- | ------------------------------------- |
| Content    | []View          | Layers, bottom first                  |
| Sizing     | Sizing          | Combined axis sizing mode             |
| Width      | float32         | Fixed width                           |
| Height     | float32         | Fixed height                          |
| Padding    | Padding         | Inner padding                         |
| HAlign     | HorizontalAlign | Horizontal place of every layer       |
| VAlign     | VerticalAlign   | Vertical place of every layer         |
| TextDir    | TextDirection   | In RTL, `HAlignStart` is the right    |
| Scrollable | bool            | Opt into the scroll system (needs ID) |
| Clip       | bool            | Clip layers to bounds                 |
