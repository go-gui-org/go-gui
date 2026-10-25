# HTML subset view

Issue #1158. Status: **landed**.

## Problem

An app ported from a webview (Wails + React) previews a short HTML snippet that
an admin writes for a mobile app. Go-Gui had no way to show it. Converting the
HTML to Markdown text in the app breaks text that holds Markdown syntax: `*`,
`_`, `#` and `|` change meaning when the Markdown parser reads the result.

## Decision

`func (w *Window) HTML(cfg MarkdownCfg) View`, beside `Window.Markdown`.

- `markdown.ParseHTML(source string) []Block` (`gui/markdown/html.go`) parses
  the subset straight into the `Block`/`Run` model that `markdown.Parse` makes.
  Text goes into runs as is, so Markdown syntax in it stays literal.
- `Window.HTML` returns the same `markdownView` with an unexported `html` flag.
  Only the parse step differs; styling, layout, selection, links, the render
  hook and the image policy (#1105) are shared.
- The styled-block cache key carries the flag (`markdownCacheKey.html`). One
  string read as HTML and as Markdown gives different blocks, so the two must
  not share an entry.
- The tokenizer is `github.com/tdewolff/parse/v2/html`. That module is already a
  direct dependency (SVG CSS), so no new module is added.

## Subset

| HTML                                                                                                 | Block / Run                                      |
| ---------------------------------------------------------------------------------------------------- | ------------------------------------------------ |
| `p`, `div`, structure tags (`section`, `td`, …)                                                      | paragraph break                                  |
| `br`                                                                                                 | `"\n"` run                                       |
| `hr`                                                                                                 | `IsHR`                                           |
| `h1`–`h6`                                                                                            | `HeaderLevel`, `AnchorSlug`                      |
| `b`/`strong`, `i`/`em`                                                                               | `FormatBold`, `FormatItalic`, `FormatBoldItalic` |
| `u`/`ins`, `s`/`del`/`strike`, `mark`, `sup`, `sub`                                                  | run flags                                        |
| `code`/`kbd`/`samp`/`tt`                                                                             | `FormatCode`                                     |
| `pre`                                                                                                | code block, whitespace kept                      |
| `blockquote`                                                                                         | `IsBlockquote`, `BlockquoteDepth`                |
| `ul`, `ol` (`start`), `li`                                                                           | `IsList`, `ListPrefix`, `ListIndent`             |
| `a href`                                                                                             | `Run.Link` when `IsSafeURL`                      |
| `img src alt width height`                                                                           | image block; `src` must pass `isSafeImagePath`   |
| `script`, `style`, `template`, `head`, `title`, `textarea`, `iframe`, `noscript`, `object`, `select` | dropped with content                             |
| inline SVG/MathML, comments                                                                          | dropped                                          |
| any other tag                                                                                        | dropped, text kept                               |

Rules:

- Entities decode in text and attribute values.
- Outside `pre`, each run of ASCII whitespace becomes one space. `&nbsp;` is not
  collapsed.
- No typography pass: `--` and `...` stay as written.
- An `img` inside a paragraph splits it, because images are blocks.
- The source is capped at `maxSourceBytes`. List and quote nesting is capped at
  16 and inline nesting at 64, so hostile input costs a bounded parse.
- A close tag pops the most recent open tag of that name and every tag opened
  after it: `<b><i>x</b>y` renders `y` plain.

## Rejected Approaches

- **A `SourceHTML bool` on `MarkdownCfg`.** A forgotten flag shows the tags as
  Markdown text, and `w.Markdown` drawing HTML reads wrong at the call site.
- **An exported HTML → blocks converter only.** `Markdown` takes a source
  string, not blocks, so a converter renders nothing without a second new view
  that takes blocks. The app would also re-parse every frame unless it caches.
- **A separate `HTMLCfg`.** It would copy about 25 `MarkdownCfg` fields and
  drift from them.
- **HTML → Markdown text inside Go-Gui.** The escaping problem from the issue.
- **`golang.org/x/net/html`.** A new module; `tdewolff/parse` is already pinned.
- **CSS, `style`/`class`, tables, forms.** Out of scope: the view is not a
  layout engine.
