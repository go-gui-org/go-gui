Preview a short HTML snippet, such as a promo banner an admin writes for a
mobile app. `w.HTML` parses a safe HTML subset into the same blocks that
`w.Markdown` draws, so styling, selection, links and the image policy are
shared. Text stays literal: `*`, `_`, `#` and `|` are not read as Markdown.

## Usage

```go
w.HTML(gui.MarkdownCfg{
    Source: `<h2>Sale</h2><p>Save <b>50%</b> today.</p>`,
    Style:  gui.DefaultMarkdownStyle(),
})
```

`HTML` takes the same `MarkdownCfg` as `Markdown`. See the Markdown page for its
fields.

## Supported Elements

- `p`, `div` and structure tags (`section`, `td`, …) break paragraphs
- `br`, `hr`, `h1`–`h6`
- `b`/`strong`, `i`/`em`, `u`/`ins`, `s`/`del`/`strike`, `mark`, `sup`, `sub`
- `code`/`kbd`/`samp`/`tt` inline, `pre` as a code block
- `blockquote`, `ul`, `ol` (with `start`), `li`
- `a href` (only safe URLs become links)
- `img src alt width height` (loads only when `AllowRemoteImages` or
  `AllowLocalImages` is set; otherwise the alt text shows)

## Dropped

- `script`, `style`, `template`, `head`, `title`, `textarea`, `iframe`,
  `noscript`, `object`, `select`: the tag and its content
- Inline SVG, MathML and comments
- Any other tag: the tag goes, its text stays
- CSS, `style` and `class` attributes, tables and forms: not supported
