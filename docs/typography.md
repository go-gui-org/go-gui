# Typography — the type scale

Read text styles from the theme. Name a role for the job the text does. Never
spell a size or a dimming alpha at a call site. The `when` for each decision
lives in `docs/style-guide.md`. The token map lives in `docs/theme-tokens.md`.
This file is the scale itself: which roles exist and what each one is for.

`ThemeMaker` builds every role from `ThemeCfg.TextStyleDef` and the size ladder
(`SizeTextTiny` … `SizeTextXLarge`). A custom theme states the body size and
gets the full ladder. The dark and light themes use body 14.

## Read the scale

Read a role style where the text color is the theme's. The block below is the
showcase specimen builder, quoted from `examples/showcase/demo_typography.go`. A
test compares this block against that file byte for byte: what you read here is
what compiles.

```go
func typographyScaleSpecimens(t gui.Theme) []gui.View {
	return []gui.View{
		gui.Text(gui.TextCfg{Text: "Display", TextStyle: t.TextStyleDisplay}),
		gui.Text(gui.TextCfg{Text: "Title", TextStyle: t.TextStyleTitle}),
		gui.Text(gui.TextCfg{Text: "Body", TextStyle: t.TextStyleBody}),
		gui.Text(gui.TextCfg{Text: "Caption", TextStyle: t.TextStyleCaption}),
		gui.Text(gui.TextCfg{Text: "Code", TextStyle: t.TextStyleCode}),
	}
}
```

Outside generation, read the theme from the window (`w.Theme()`). In factories
and `GenerateLayout`, keep the `guiTheme` read, as the theme rules require.

## Regular and bold roles

| Role                    | Face    | Ladder rung      | Use for                      |
| ----------------------- | ------- | ---------------- | ---------------------------- |
| `TextStyleDisplay`      | Bold    | `SizeTextXLarge` | Largest heading on a surface |
| `TextStyleTitle`        | Bold    | `SizeTextLarge`  | Dialog titles                |
| `TextStyleTitleSmall`   | Bold    | `SizeTextMedium` | Group-box and toast titles   |
| `TextStyleBodyLarge`    | Regular | `SizeTextLarge`  | Large body text              |
| `TextStyleBody`         | Regular | `SizeTextMedium` | Default body text            |
| `TextStyleBodySmall`    | Regular | `SizeTextSmall`  | Secondary body text          |
| `TextStyleCaption`      | Regular | `SizeTextXSmall` | Hints and captions           |
| `TextStyleCaptionSmall` | Regular | `SizeTextTiny`   | Smallest supporting text     |

Headings take the bold roles. Body and value text stays regular. The
widget-by-widget heading table lives in the style guide.

## Code roles

| Role                 | Rung                 |
| -------------------- | -------------------- |
| `TextStyleCode`      | `SizeTextMedium` + 1 |
| `TextStyleCodeSmall` | `SizeTextXSmall` + 1 |
| `TextStyleCodeTiny`  | `SizeTextTiny` + 1   |

Mono faces draw smaller than regular faces at the same point size. Each code
rung adds 1 point to compensate. The offset is uniform because every rung uses
the same face.

## Icon roles

`TextStyleIconXLarge` … `TextStyleIconTiny` render glyphs, not text. They carry
the icon family and one rung each, from `SizeTextXLarge` down to `SizeTextTiny`.
Centring treats a run in one of these roles as a glyph and centres its ink.

## De-emphasis roles

Four roles cover every quiet text. Pick by what the text does.

| Role                   | Use when the text …                   |
| ---------------------- | ------------------------------------- |
| `TextStyleSecondary`   | Supports primary text beside it       |
| `TextStyleLabel`       | Names a nearby value; also steps size |
| `TextStyleDisabled`    | Sits in a control the user cannot use |
| `TextStylePlaceholder` | Stands in for a value not yet entered |

The values derive from the body color and the theme polarity. A theme states
`ThemeCfg.ColorText*` only to override them.

## Modifiers

A role fixes face, size and color for one purpose. A modifier re-faces a role
for the rest: `Bold()`, `Italic()`, `Regular()`. Each modifier keeps size and
color, so chaining works in any order. `Display.Regular()` is the extra-large
regular rung.

`Theme.Mono(style)` moves a style to the mono family. It adds the same +1
compensation the code roles use. Set `Size` after `Mono()`, not before: an
explicit size replaces the compensation instead of stacking on it. Where a code
role fits, use the role and skip the modifier.

## Custom themes

State `TextStyleDef.Size` to set the body size. The six rungs derive from it.
State `MonoFontFamily` to retarget the code roles. State `IconFontFamily` to
retarget the icon roles, and register the font before the backend starts.
