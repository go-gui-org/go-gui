Hover hints attached to any widget. Wrap any view with
`WithTooltip` to show text on hover after a configurable delay.
Tooltips auto-flip when near screen edges.

## Usage

```go
gui.WithTooltip(w, gui.WithTooltipCfg{
    Text: "Helpful hint",
    Content: []gui.View{
        gui.Button(gui.ButtonCfg{...}),
    },
})
```

## Custom Positioning

```go
gui.WithTooltip(w, gui.WithTooltipCfg{
    Text:   "Right side tooltip",
    Anchor: gui.Some(gui.FloatMiddleRight),
    TieOff: gui.Some(gui.FloatMiddleLeft),
    Content: []gui.View{
        gui.Button(gui.ButtonCfg{...}),
    },
})
```

## WithTooltipCfg Properties

| Property | Type          | Description                          |
|----------|---------------|--------------------------------------|
| ID       | string        | Unique identifier                    |
| Text     | string        | Tooltip text content                 |
| Delay    | time.Duration | Hover delay before showing           |
| Anchor   | Opt[FloatAttach] | Tooltip anchor on trigger         |
| TieOff   | Opt[FloatAttach] | Tooltip attach point              |
| Content  | []View        | Wrapped target views                 |

## TooltipCfg Properties (Advanced)

| Property     | Type          | Description                      |
|--------------|---------------|----------------------------------|
| ID           | string        | Unique identifier                |
| Delay        | time.Duration | Hover delay before showing       |
| Content      | []View        | Custom tooltip content           |
| Anchor       | Opt[FloatAttach] | Float anchor point            |
| TieOff       | Opt[FloatAttach] | Float tie-off point           |
| OffsetX      | Opt[float32]     | Horizontal offset from anchor |
| OffsetY      | Opt[float32]     | Vertical offset from anchor   |
| FloatZIndex  | int           | Z-index for float layering       |

## Appearance

| Property     | Type         | Description                      |
|--------------|--------------|----------------------------------|
| Color        | Color        | Tooltip background color         |
| ColorBorder  | Color        | Border color                     |
| Padding      | Opt[Padding] | Inner padding                    |
| TextStyle    | TextStyle    | Tooltip text styling             |
| Radius       | Opt[float32] | Corner radius                    |
| SizeBorder   | Opt[float32] | Border width                     |
