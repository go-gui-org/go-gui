# Markdown render callback

Status: proposed. Written 2026-08-24. Modeled on Bun's `bun:markdown`
`renderMarkdown({ data, callback })`: the caller gets every parsed markdown
element (heading, list, …) in a callback and can replace its output. The
feature is additive. It is not a breaking change.

## Problem

The `Markdown` widget cannot express new block types or per-element rendering
overrides. An app that wants admonitions (`> [!NOTE]`), numbered headings,
annotated paragraphs, or dropped sections must rewrite the source string before
calling `Markdown()`. Structure-aware extension hooks already exist per concern
— `CodeHighlighter`, `MathFetcher`, `MermaidFetcher` — but there is no generic
one, so the most-requested markdown feature (callouts) has no first-class path.

The render pass is already a bun-style element switch:
`markdownBuildContent` (`gui/view_markdown.go`) discriminates each
`markdownBlock` by flags (`IsCode`, `IsTable`, `HeaderLevel`, `IsList`, …) and
dispatches to `mdRender*`. A render callback is a new first branch of that
switch — no new machinery.

## Design

### Hook location: `MarkdownCfg.RenderBlock`

On `MarkdownCfg`, not `MarkdownStyle`. `CodeHighlighter` lives on the style
because it runs at parse/style time (cached). `RenderBlock` runs at generation
time — it needs the resolved document scope and the `*Window` for building
views, which only the render pass has. `MathFetcher`/`MermaidFetcher` set the
precedent for behavioral hooks on the Cfg.

### The element

`MarkdownElement` is an exported, styled view of one parsed block. The hook
receives the *styled* form so it reuses `MarkdownStyle` instead of re-styling
raw runs:

```go
// MarkdownBlockKind names a parsed markdown block for MarkdownElement.Kind.
type MarkdownBlockKind int

const (
	MarkdownKindHeading MarkdownBlockKind = iota
	MarkdownKindParagraph
	MarkdownKindCode
	MarkdownKindTable
	MarkdownKindHR
	MarkdownKindBlockquote
	MarkdownKindImage
	MarkdownKindMath
	MarkdownKindList
	MarkdownKindDefTerm
	MarkdownKindDefValue
)

// MarkdownElement is one parsed, styled markdown block, passed to
// MarkdownCfg.RenderBlock. Kind is the discriminator; Kind-specific data
// (HeaderLevel, TableData, ImageSrc, …) is zero for other kinds.
type MarkdownElement struct {
	Kind        MarkdownBlockKind
	HeaderLevel int
	Content     RichText   // styled runs
	PlainText   string     // unstyled source text
	TableData   *MarkdownTable // parsed+styled table; nil for non-tables
	DocID       string     // effective markdown widget ID, for ScopeID
	// Task flags, image dims, CodeLanguage, MathLatex, AnchorSlug and the
	// remaining markdownBlock fields mirror the parser; see markdownBlock.
}
```

`TableData` is an exported mirror of the internal `parsedTable`
(headers/rows/alignments as `RichText`), since `parsedTable` itself is private.

### The callback

```go
// On MarkdownCfg:
// RenderBlock, when non-nil, is consulted for every block during layout.
// Return nil to fall back to the default renderer.
RenderBlock func(el MarkdownElement) []View
```

- Consulted first in the `markdownBuildContent` loop, per block, before the
  kind switch. `nil` return = default rendering.
- Single callback + `Kind` discriminator, not one field per kind: handling one
  kind requires twelve no-op fields. The discriminator is the bun shape.
- **Runs every frame.** Blocks are parsed and styled once and cached
  (`markdownCache`, keyed on source hash + theme). The callback fires per
  frame over the cached styled blocks. It must be cheap and pure — build
  `View` structs, do not fetch or parse. Document this in the field comment.
- **Selection is unaffected.** `makeCtx` advances rune offsets from
  `block.Content`, never from rendered views, so replaced blocks keep correct
  offsets. They are not selectable text.
- **IDs in custom views are the hook writer's responsibility.** Compose inner
  IDs with `ScopeID(el.DocID, …)`. The `gui.Debug` gate
  (`TestDuplicateIDs`) catches collisions at runtime.
- **List caveat.** List items accumulate through `mdFlushListItems`, which
  supplies the bullet/prefix. A non-nil return for a `MarkdownKindList` block
  replaces that item wholesale and drops it from prefix processing — the hook
  supplies its own prefix. Mixed lists (some items hooked, some not) therefore
  render inconsistently. Document that hooks which override list items must
  handle every item of a list.
- Images/math: `mdRenderImage` and the mermaid/math fetch triggers run on the
  cached blocks regardless of the hook, so custom replacements do not disable
  diagram fetching.
- `markdownToRichText` (the RTF path) is a separate pipeline and is untouched.

## Dog-fooding

go-gui itself never renders markdown internally — the in-tree consumers are
`examples/` — so this is a demo dog-food, not a first-party consumer.

- `examples/showcase/demo_text.go` (`demoMarkdown`): implement a
  `> [!NOTE]` / `> [!WARNING]` callout convention via `RenderBlock` — a
  tinted container per kind. This is the most-requested markdown feature and
  exercises the hook on blockquotes, headings, and inline content.
- A heading-derived table of contents needs a document-level pass that a
  per-block callback cannot express. It is out of scope (see below).

## Testing

- Unit tests (`gui/markdown_test.go`) cover each behavior: the callback
  receives the right `Kind` and content per block. A `nil` fallback renders
  the default. A replacement renders in place. Selection rune offsets still
  advance. The list caveat is covered.
- Golden: add a `gui/golden_test.go` case with a `RenderBlock` (callout or
  heading wrap) recorded in both `ThemeDark` and `ThemeLight`.
- `TestDuplicateIDs` run over a hooked document with an ID collision proves
  the debug gate covers hook-built views.
- New exports need consumers or `// exportaudit:keep` (`make export-audit`).

## Out of scope

- Table-of-contents widget (needs a document-level block inspection pass).
- Caching hook output: the hook is per-frame by design; its closure identity
  is unhashable, so a cache key is not available. Per-frame cost is a
  documented constraint, consistent with the rest of the immediate-mode
  pipeline.
- `MarkdownStyle`-level hooks (parse-time) stay as-is; `CodeHighlighter`
  keeps its place.

## Phases

1. `gui/view_markdown.go`: add `MarkdownElement`, `MarkdownBlockKind`,
   `MarkdownTable`, `RenderBlock` field, and the dispatch branch in
   `markdownBuildContent`.
2. Unit tests + golden case.
3. Dog-food: callout rendering in `examples/showcase/demo_text.go`.
4. Gate: `make export-audit`, `make check`, `make prepush`.

## Unresolved questions

1. Export `MarkdownTable` now, or exclude table data from `MarkdownElement`
   in v1 (tables keep default rendering unless the hook wraps)? Leaning:
   include — `mdRenderTable` is the one renderer that cannot be reproduced
   from `RichText` alone.
2. `MarkdownElement` mirrors `markdownBlock` field-for-field, or a curated
   subset? Leaning: curated subset + the raw parser block exposed as an
   unexported-adjacent escape hatch, to keep the exported struct small.
