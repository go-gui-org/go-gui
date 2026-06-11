Quick command search overlay with fuzzy filtering, keyboard
navigation, and grouped items. Shows a centered floating card with a
search input and scrollable results list.

## Usage

```go
gui.CommandPalette(gui.CommandPaletteCfg{
    ID:       "cmd",
    IDFocus:  focusPalette,
    IDScroll: scrollPalette,
    Items:    items,
    OnAction: func(id string, _ *gui.Event, w *gui.Window) {
        // handle action
    },
})

// Toggle with Ctrl+K or programmatically:
gui.CommandPaletteToggle("cmd", focusPalette, w)
```

## API

| Function                                     | Description              |
|----------------------------------------------|--------------------------|
| CommandPaletteShow(id, idFocus, idScroll, w) | Show and focus palette   |
| CommandPaletteDismiss(id, w)                 | Hide palette             |
| CommandPaletteToggle(id, idFocus, idScroll, w) | Toggle visibility      |
| CommandPaletteIsVisible(id, w) bool          | Check if visible         |

## Key Properties

| Property    | Type                 | Description                      |
|-------------|----------------------|----------------------------------|
| ID          | string               | Unique identifier                |
| Items       | []CommandPaletteItem | Available commands               |
| Placeholder | string               | Search input hint text           |
| Width       | float32              | Palette width                    |
| MaxHeight   | float32              | Maximum dropdown height          |
| IDFocus     | uint32               | Focus ID for input               |
| IDScroll    | uint32               | Scroll ID for results list       |
| FloatZIndex | int                  | Z-index for float layering       |

## Appearance

| Property       | Type         | Description                      |
|----------------|--------------|----------------------------------|
| Color          | Color        | Card background color            |
| ColorBorder    | Color        | Card border color                |
| ColorHighlight | Color        | Highlighted item color           |
| BackdropColor  | Color        | Semi-transparent backdrop        |
| SizeBorder     | Opt[float32] | Border width                     |
| Radius         | Opt[float32] | Corner radius                    |
| TextStyle      | TextStyle    | Item label text styling          |
| DetailStyle    | TextStyle    | Item detail text styling         |

## CommandPaletteItem

| Property | Type   | Description                          |
|----------|--------|--------------------------------------|
| ID       | string | Action identifier                    |
| Label    | string | Display text                         |
| Detail   | string | Secondary description                |
| Icon     | string | Icon glyph                           |
| Group    | string | Group heading                        |
| Disabled | bool   | Disable this item                    |

## Events

| Callback  | Signature                        | Fired when                   |
|-----------|----------------------------------|------------------------------|
| OnAction  | func(string, *Event, *Window)    | Command selected             |
| OnDismiss | func(*Window)                    | Palette dismissed            |
