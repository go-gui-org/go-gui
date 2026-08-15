Palette icon that opens a floating dropdown of all registered themes. Clicking a
theme applies it immediately via w.SetTheme. The dropdown supports keyboard
navigation (arrow keys, Enter, Escape).

## Usage

```go
gui.ThemePicker(gui.ThemePickerCfg{
    ID:          "my-theme-picker",
    FloatAnchor: gui.FloatBottomLeft,
    FloatTieOff: gui.FloatTopLeft,
    OnSelect: func(name string, ctx gui.EventCtx) {
        fmt.Println("switched to", name)
    },
})
```

## Float Positioning

The dropdown is a floating panel. Control its anchor and tie-off to place it
relative to the icon:

```go
gui.ThemePicker(gui.ThemePickerCfg{
    ID:          "tp",
    FloatAnchor: gui.FloatTopRight,
    FloatTieOff: gui.FloatBottomRight,
})
```

## Key Properties

| Property     | Type                   | Description                         |
| ------------ | ---------------------- | ----------------------------------- |
| ID           | string                 | Unique identifier (required)        |
| Sizing       | Sizing                 | Combined axis sizing mode           |
| OnSelect     | func(string, EventCtx) | Called with theme name on selection |
| FloatAnchor  | FloatAttach            | Dropdown anchor point on parent     |
| FloatTieOff  | FloatAttach            | Dropdown tie-off point on dropdown  |
| FloatOffsetX | float32                | Horizontal offset from anchor       |
| FloatOffsetY | float32                | Vertical offset from anchor         |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
