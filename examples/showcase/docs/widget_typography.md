Typography specimens for every theme text role: the regular and bold ladder, the
code and icon rungs, the four de-emphasis roles, and the face modifiers. Use it
to pick a role by eye, then read that role in code. The canonical reference is
`docs/typography.md`.

## Read the scale

Read a role style where the text color is the theme's. Below is the specimen
builder this page renders, quoted from `demo_typography.go`. A test compares
this block against that file byte for byte: what you read here is what compiles.

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

## Scale

| Role                  | Face    |
| --------------------- | ------- |
| TextStyleDisplay      | Bold    |
| TextStyleTitle        | Bold    |
| TextStyleTitleSmall   | Bold    |
| TextStyleBodyLarge    | Regular |
| TextStyleBody         | Regular |
| TextStyleBodySmall    | Regular |
| TextStyleCaption      | Regular |
| TextStyleCaptionSmall | Regular |
| TextStyleCode         | Mono    |
| TextStyleCodeSmall    | Mono    |
| TextStyleCodeTiny     | Mono    |

Icon roles (`TextStyleIconXLarge` … `TextStyleIconTiny`) render glyphs, not
text. Code rungs sit 1 point above the matching regular rung to compensate the
mono face.

## De-emphasis roles

| Role                 | Use when the text …                   |
| -------------------- | ------------------------------------- |
| TextStyleSecondary   | Supports primary text beside it       |
| TextStyleLabel       | Names a nearby value; also steps size |
| TextStyleDisabled    | Sits in a control the user cannot use |
| TextStylePlaceholder | Stands in for a value not yet entered |

## Modifiers

`Bold()`, `Italic()` and `Regular()` re-face a role and keep its size and color.
`Theme.Mono(style)` moves a style to the mono family with the +1 compensation.
Set `Size` after `Mono()`, not before. Where a named role fits, use the role.
