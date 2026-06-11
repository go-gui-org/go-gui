Mixed styles, links, abbreviations, footnotes, and decorations
within a single text block. Build a paragraph from `RichTextRun`
slices rendered by `gui.RTF`.

## Usage

```go
t := gui.CurrentTheme()
gui.RTF(gui.RtfCfg{
    RichText: gui.RichText{
        Runs: []gui.RichTextRun{
            gui.RichRun("Normal, ", t.N3),
            gui.RichRun("bold, ", t.B3),
            gui.RichRun("italic", t.I3),
        },
    },
})
```

## With Links and Abbreviations

```go
gui.RTF(gui.RtfCfg{
    RichText: gui.RichText{
        Runs: []gui.RichTextRun{
            gui.RichRun("Visit ", t.N3),
            gui.RichLink("Go docs", "https://go.dev", t.N3),
            gui.RichRun(". ", t.N3),
            gui.RichAbbr("HTML", "HyperText Markup Language", t.N3),
        },
    },
    Mode: gui.TextModeWrap,
})
```

## Run Helpers

| Helper                                     | Description                              |
|--------------------------------------------|------------------------------------------|
| `RichRun(text, style)`           | Styled text run                          |
| `RichLink(text, url, style)`     | Hyperlink (auto underline + theme color) |
| `RichAbbr(text, expansion, style)` | Abbreviation with tooltip              |
| `RichFootnote(id, content, style)` | Superscript footnote with tooltip      |
| `RichBr()`                       | Line break                               |

## Text Decorations

Set `Underline` or `Strikethrough` on a `TextStyle`:

```go
gui.RichRun("underlined", gui.TextStyle{
    Color: t.N3.Color, Size: t.N3.Size,
    Underline: true,
})
```

## Key Properties

| Property      | Type       | Description                          |
|---------------|------------|--------------------------------------|
| RichText      | RichText   | Runs of styled text                  |
| MinWidth      | float32    | Minimum block width                  |
| Mode          | TextMode   | Text wrapping mode                   |
| HangingIndent | float32    | Negative indent for wrapped lines    |
| Clip          | bool       | Clip text to bounds                  |
| BaseTextStyle | *TextStyle | Fallback style for runs              |
| IDFocus       | uint32     | Tab-order focus ID (enables select)  |
| Disabled      | bool       | Disable interaction                  |
| Invisible     | bool       | Hide without removing from layout    |
| FocusSkip     | bool       | Skip in tab-order navigation         |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
