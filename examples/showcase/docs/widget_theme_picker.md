Palette icon that opens a floating dropdown of all
registered themes. Clicking a theme applies it immediately via
w.SetTheme. The dropdown supports keyboard navigation (arrow keys,
Enter, Escape).

## Usage

```go
gui.ThemePicker(gui.ThemePickerCfg{
    ID:          "my-theme-picker",
    FloatAnchor: gui.FloatBottomLeft,
    FloatTieOff: gui.FloatTopLeft,
    OnSelect: func(name string, _ *gui.Event, w *gui.Window) {
        fmt.Println("switched to", name)
    },
})
```

## Float Positioning

The dropdown is a floating panel. Control its anchor and tie-off
to place it relative to the icon:

```go
gui.ThemePicker(gui.ThemePickerCfg{
    ID:          "tp",
    FloatAnchor: gui.FloatTopRight,
    FloatTieOff: gui.FloatBottomRight,
})
```

## Key Properties

| Property     | Type                               | Description                          |
|--------------|------------------------------------|--------------------------------------|
| ID           | string                             | Unique identifier (required)         |
| IDFocus      | uint32                             | Tab-order focus ID (> 0 to enable)   |
| Sizing       | Sizing                             | Combined axis sizing mode            |
| OnSelect     | func(string, *Event, *Window)      | Called with theme name on selection   |
| FloatAnchor  | FloatAttach                        | Dropdown anchor point on parent      |
| FloatTieOff  | FloatAttach                        | Dropdown tie-off point on dropdown   |
| FloatOffsetX | float32                            | Horizontal offset from anchor        |
| FloatOffsetY | float32                            | Vertical offset from anchor          |

## Accessibility

| Property        | Type   | Description                            |
|-----------------|--------|----------------------------------------|
| A11YLabel       | string | Accessible label                       |
| A11YDescription | string | Accessible description                 |
