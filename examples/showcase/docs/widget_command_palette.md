Quick command search overlay with fuzzy filtering, keyboard navigation, and
grouped items. Shows a centered floating card with a search input and scrollable
results list.

## Usage

```go
gui.CommandPalette(gui.CommandPaletteCfg{
    ID:    "cmd",
    Items: items,
    OnAction: func(id string, ctx gui.EventCtx) {
        // handle action
    },
})

// Toggle with Ctrl+K or programmatically:
gui.CommandPaletteToggle("cmd", w)
```

## API

| Function                    | Description       |
| --------------------------- | ----------------- |
| CommandPaletteToggle(id, w) | Toggle visibility |

Show, dismiss and visibility read are unexported; `Toggle` is the only public
control.

## Key Properties

| Property      | Type                 | Description                |
| ------------- | -------------------- | -------------------------- |
| ID            | string               | Unique identifier          |
| Items         | []CommandPaletteItem | Available commands         |
| Placeholder   | string               | Search input hint text     |
| Width         | float32              | Palette width              |
| MaxHeight     | float32              | Maximum dropdown height    |
| FloatZIndex   | int                  | Z-index for float layering |
| Sound         | SoundCue             | Backdrop-dismiss cue       |
| SoundDisabled | bool                 | Suppress backdrop sound    |

## Appearance

| Property             | Type         | Description                 |
| -------------------- | ------------ | --------------------------- |
| Color                | Color        | Card background color       |
| ColorBorder          | Color        | Card border color           |
| ColorHighlight       | Color        | Highlighted item color      |
| ColorHighlightSubtle | Color        | Tint behind highlighted row |
| BackdropColor        | Color        | Semi-transparent backdrop   |
| SizeBorder           | Opt[float32] | Border width                |
| Radius               | Opt[float32] | Corner radius               |
| TextStyle            | TextStyle    | Item label text styling     |
| DetailStyle          | TextStyle    | Item detail text styling    |

## CommandPaletteItem

| Property | Type   | Description           |
| -------- | ------ | --------------------- |
| ID       | string | Action identifier     |
| Label    | string | Display text          |
| Detail   | string | Secondary description |
| Icon     | string | Icon glyph            |
| Group    | string | Group heading         |
| Disabled | bool   | Disable this item     |

## Events

| Callback  | Signature              | Fired when        |
| --------- | ---------------------- | ----------------- |
| OnAction  | func(string, EventCtx) | Command selected  |
| OnDismiss | func(*Window)          | Palette dismissed |
