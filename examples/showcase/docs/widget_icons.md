256 icons from the Feather icon font. Each icon is a named
constant (e.g. `gui.IconCheck`, `gui.IconFolder`, `gui.IconSearch`).
Render with `gui.Text` using one of the six theme Icon styles.
Icons are Unicode glyphs — no image files required.

## Usage

```go
// Single icon
gui.Text(gui.TextCfg{Text: gui.IconCheck, TextStyle: t.Icon4})

// Icon with custom color
gui.Text(gui.TextCfg{
    Text:      gui.IconAlertCircle,
    TextStyle: t.Icon3,
    Color:     gui.ColorRed,
})

// Icon inside a button
gui.Button(gui.ButtonCfg{
    ID: "save",
    Content: []gui.View{
        gui.Text(gui.TextCfg{Text: gui.IconSave, TextStyle: t.Icon4}),
        gui.Text(gui.TextCfg{Text: "Save"}),
    },
})
```

## Icon Styles

| Style   | Size   | Maps to        |
|---------|--------|----------------|
| t.Icon1 | XLarge | SizeTextXLarge |
| t.Icon2 | Large  | SizeTextLarge  |
| t.Icon3 | Medium | SizeTextMedium |
| t.Icon4 | Small  | SizeTextSmall  |
| t.Icon5 | XSmall | SizeTextXSmall |
| t.Icon6 | Tiny   | SizeTextTiny   |

## Programmatic Access

`gui.IconLookup` is a `map[string]string` mapping snake_case names to
Unicode glyphs:

```go
for name, glyph := range gui.IconLookup {
    gui.Text(gui.TextCfg{Text: glyph, TextStyle: t.Icon4})
}
```

## Common Icons

| Constant        | Glyph | Usage                |
|-----------------|-------|----------------------|
| IconCheck       | check | Confirmation, success |
| IconX           | x     | Close, dismiss       |
| IconFolder      | folder | File browser         |
| IconSearch      | search | Search fields        |
| IconSave        | save  | Save actions         |
| IconTrash       | trash | Delete actions       |
| IconAlertCircle | alert | Warnings, errors     |
| IconEdit        | edit  | Edit actions         |
