# Markdown render callback

Status: proposed. Written 2026-08-24. Revised 2026-08-28 after review against
`gui/view_markdown.go`. Modeled on Bun's `bun:markdown`
`renderMarkdown({ data, callback })`: the caller gets every parsed markdown
element in a callback and can replace its output. Additive, not a breaking
change.

## Problem

The `Markdown` widget cannot express new block types or per-element rendering
overrides. An app that wants admonitions (`> [!NOTE]`), numbered headings,
annotated paragraphs, or dropped sections must rewrite the source string before
calling `Markdown()`. Structure-aware hooks exist per concern —
`CodeHighlighter`, `MathFetcher`, `MermaidFetcher` — but there is no generic
one, so the most-requested markdown feature (callouts) has no first-class path.

`markdownBuildContent` (`gui/view_markdown.go:417`) is already a bun-style
element switch over `markdownBlock` flags. A render callback is a new first
branch of it — but the branch is not free: the loop carries three pieces of
cross-block state that the hook must not bypass. See [Loop state](#loop-state).

## Design

`RenderBlock` goes on `MarkdownCfg`, not `MarkdownStyle`: `CodeHighlighter`
lives on the style because it runs at parse time (cached), while `RenderBlock`
runs at generation time and needs the resolved document scope and the `*Window`.
`MathFetcher`/`MermaidFetcher` set the precedent.

The types go in `gui/markdown_types.go`, beside the `markdownBlock` and
`parsedTable` they mirror — `view_markdown.go` is 502 lines against an 800-line
gate and takes only the dispatch branch plus two small helpers.

### The element

The hook receives the _styled_ block, so it reuses `MarkdownStyle` instead of
re-styling raw runs.

```go
// MarkdownBlockKind names a parsed markdown block for MarkdownElement.Kind.
// Paragraph is zero — it is the render switch's default branch, so a zero
// MarkdownElement reads as the generic case, not as a heading.
type MarkdownBlockKind int

const (
	MarkdownKindParagraph MarkdownBlockKind = iota
	MarkdownKindHeading
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
// MarkdownCfg.RenderBlock. It mirrors markdownBlock field-for-field except
// the unexported baseStyle, plus Index, DocID and PlainText. A curated
// subset would omit fields real hooks need — list indent, task state, image
// dimensions, code language — and force the writer to re-derive them.
type MarkdownElement struct {
	Kind      MarkdownBlockKind
	Index     int            // source-order position, for ScopeIDN
	DocID     string         // the document's effective ID
	Content   RichText       // styled runs
	PlainText string         // unstyled text, derived for hooked blocks
	TableData *MarkdownTable // nil for non-tables

	HeaderLevel  int
	IsCode       bool
	IsHR         bool
	IsBlockquote bool
	IsImage      bool
	IsTable      bool
	IsList       bool
	IsMath       bool
	IsDefTerm    bool
	IsDefValue   bool
	IsTaskItem   bool
	TaskChecked  bool

	ListPrefix      string
	ListIndent      int
	BlockquoteDepth int

	ImageSrc    string
	ImageAlt    string
	ImageWidth  float32
	ImageHeight float32

	CodeLanguage string
	MathLatex    string
	AnchorSlug   string
}

// MarkdownTable mirrors parsedTable exactly. No column count: that belongs
// to the parser's markdown.Table, not the styled form. Exported because
// mdRenderTable cannot be rebuilt from RichText alone.
type MarkdownTable struct {
	Headers    []RichText
	Alignments []HorizontalAlign
	Rows       [][]RichText
}
```

Four derivation rules:

- **`Kind` comes from one helper, `mdKindOf(block)`, whose test order copies the
  render switch exactly** (`IsMath`, `IsCode`, `IsTable`, `IsHR`,
  `IsBlockquote`, `IsImage`, `HeaderLevel > 0`, `IsDefTerm`, `IsDefValue`,
  `IsList`, paragraph). The flags are not mutually exclusive, so a second
  hand-written order would silently disagree with the fallthrough dispatch.
- **Mermaid has no kind of its own.** `renderMdMermaid` dispatches from inside
  the code branch on `CodeLanguage == "mermaid"`
  (`gui/view_markdown_blocks.go:248`), so a hook sees `MarkdownKindCode` with
  that language.
- **`DocID` is already resolved.** `Markdown` assigns `cfg.ID = w.EffID(cfg.ID)`
  (`gui/view_markdown.go:307`) before the build pass, so `DocID` is that value
  copied out — no second `EffID` call. `Index` indexes the cached styled blocks,
  so it shifts when the source changes; headings should compose IDs from
  `AnchorSlug`.
- **`PlainText` is derived per hooked block, not cached on `markdownBlock`.** A
  cached field costs a string header and a `richTextPlain` copy per block for
  every markdown document in the app. The hook branch calls `richTextPlain` only
  when `RenderBlock != nil`, so the cost lands on the feature's users and is
  comparable to building the replacement view in the same frame.

### The callback

```go
// On MarkdownCfg:
// RenderBlock, when non-nil, is consulted for every block during layout.
// The Window parameter provides state access and EffID for composing IDs.
// Return ok=false to fall back to the default renderer. Return ok=true to
// replace the block's output: with a view, or with nil to drop the block.
RenderBlock func(w *Window, el MarkdownElement) (View, bool)
```

A single `View` plus `ok`, not `[]View`: a slice allocates per hooked block per
frame, and the one multi-view default renderer (`mdRenderHeading`, spacer +
heading) is expressible as a `Column`. One callback with a `Kind` discriminator,
not one field per kind — handling one kind would otherwise need ten no-op
fields, and the discriminator is the bun shape.

The hook runs every frame. Blocks are parsed and styled once and cached
(`markdownCache`); the callback fires per frame over the cached blocks, so it
must be cheap and pure — build `View` structs, do not fetch or parse. Fetch
triggers are unaffected: `markdownTriggerMathFetches` runs over the cached
blocks before the build, so hooked mermaid/math blocks still fetch their
diagrams. `mdRenderImage` is suppressed when the hook replaces an image block.
`markdownToRichText` (the RTF path) is a separate pipeline and is untouched.

Hook writers own their IDs and a11y: compose with `ScopeID(el.DocID, …)` or
`ScopeIDN(el.DocID, …, el.Index)` (the `gui.Debug` gate catches collisions), and
set `A11YRole`/`A11YLabel` where appropriate — `AccessRoleNote` for callouts.

The hook runs during `GenerateLayout`, under the frame lock. `SetFocus`,
`ClearFocus`, `UpdateView`, `ClearDrawCanvasCache` and `Window.Lock` all take
`w.mu` and panic naming themselves. `QueueCommand` **is** permitted and is the
documented remedy — it takes `w.commandsMu` (`gui/window_update.go:53`), never
`w.mu`. See `docs/specs/frame-lock-callback-deferral.md`.

### Loop state

`markdownBuildContent` carries three pieces of cross-block state. A hook branch
placed naively at the top of the loop body corrupts all three.

1. **Rune offsets advance only for the kinds that would have advanced them.**
   `makeCtx` is called on six branches only — blockquote, heading, def term, def
   value, list, paragraph. Math, code, table, HR and image blocks never advance
   `runeOffset`. Advancing it for every hooked block would give a document with
   a hooked code block different offsets from the same document unhooked, so
   selection would break in the hooked case only. The hook branch calls
   `makeCtx(block)` (discarding the ctx) only when `cfg.Focusable` and
   `mdKindSelectable(kind)`; that helper lists the same six kinds and sits next
   to `mdKindOf` so the two cannot drift.
2. **Pending list items flush before hook output is appended.** List items
   accumulate in `listItems` and are flushed by the next non-list block.
   Appending hook output straight to `content` while earlier siblings sit
   unflushed renders them out of source order.
3. **The tail flush moves out of the `IsList` branch.** Today the
   `i == len(blocks)-1` flush lives inside the list case, so a hooked final list
   item skips it and the pending items are silently dropped. It moves to after
   the loop, where every exit path reaches it. Output for unhooked documents is
   byte-identical, which the existing golden cases assert.

The blockquote spacer is the deliberate exception: `prevWasBQ` and the
half-height closing spacer stay computed from the parsed flags before the hook
runs, so a hook that replaces a blockquote keeps the quote's surrounding air —
the right behavior for a callout, which is a blockquote by another name. A hook
that _drops_ a blockquote therefore leaves the closing spacer behind. Accepted,
not special-cased.

### Selection and lists

Hooked blocks are not selectable text: their views carry no `TC.markdownID` /
`markdownBlockStart`, so `markdownContainerAmendLayout` and `mdHitAbsRune` skip
them. Because offsets still advance for selectable kinds, a selection spanning a
hooked block copies that block's **original markdown source**. Deliberate: the
selection is over the document, and the document is the source. The
document-level copy button likewise copies raw `Source`.

`mdFlushListItems` supplies the bullet or prefix, so a hooked list item drops
out of prefix processing and must supply its own. Mixed lists render
inconsistently; hooks that override list items should handle every item.

### Style cache limitation (pre-existing)

`markdownCache` is keyed on `MathHash(cfg.Source)` + theme ID only, not on
`CodeHighlighter` or any other `MarkdownStyle` field, so two documents in one
window with the same source but different `Style` share cached styled blocks.
The hook receives the styled `Content`, so this surfaces when one source is
rendered with varying styles. The hook neither worsens nor fixes it.

## Dog-fooding

go-gui never renders markdown internally — the in-tree consumers are `examples/`
— so this is a demo dog-food, not a first-party consumer.
`examples/showcase/demo_text.go` (`demoMarkdown`) implements a `> [!NOTE]` /
`> [!WARNING]` callout convention via `RenderBlock`, a tinted container per
kind. It is the most-requested markdown feature and exercises the hook on
blockquotes, headings, and inline content. A heading-derived table of contents
needs a document-level pass a per-block callback cannot express; out of scope.

## Testing

Unit tests (`gui/markdown_test.go`): the callback receives the right `Kind` and
content per block; `ok == false` renders the default; a replacement renders in
place; `(nil, true)` drops the block. Plus one test per loop-state rule —

- **Offsets.** One source rendered twice, plain and with a hook replacing a
  fenced code block: rune offsets of every following selectable block must
  match. The pre-review design failed this.
- **List ordering.** A list whose second item is hooked renders in source order;
  a list whose _last_ item is hooked still renders its earlier items.
- **Kind derivation.** A ` ```mermaid ` fence arrives as `MarkdownKindCode` with
  `CodeLanguage == "mermaid"`; a block with overlapping flags resolves to the
  branch the render switch would take.

Golden: add a `gui/golden_test.go` case with a `RenderBlock` (callout or heading
wrap) in both `ThemeDark` and `ThemeLight`. Existing markdown goldens must be
unchanged by the tail-flush move — that is the proof it is behavior-preserving.
`TestDuplicateIDs` over a hooked document with an ID collision proves the debug
gate covers hook-built views. New exports (`MarkdownElement`,
`MarkdownBlockKind` and its eleven constants, `MarkdownTable`, `RenderBlock`)
need consumers or `// exportaudit:keep`; the showcase covers `RenderBlock`,
`MarkdownElement` and a few kinds, the rest take markers.

## Out of scope

- Table-of-contents widget (needs a document-level block inspection pass).
- Caching hook output: the hook is per-frame by design, and a closure identity
  is unhashable, so no cache key exists. Consistent with the rest of the
  immediate-mode pipeline.
- `MarkdownStyle`-level (parse-time) hooks; `CodeHighlighter` keeps its place.

## Phases

1. `gui/markdown_types.go`: `MarkdownElement`, `MarkdownBlockKind`,
   `MarkdownTable`.
2. `gui/view_markdown.go`: the `RenderBlock` field, `mdKindOf`,
   `mdKindSelectable`, the hook branch, and the loop-state fixes (rules 2 and
   3). Unhooked golden output must not move.
3. Unit tests + golden case.
4. Dog-food: callouts in `examples/showcase/demo_text.go`.
5. Gate: `make export-audit`, `make check`, `make prepush`.

## Review corrections (2026-08-28)

The first draft was wrong on four points, all found by reading the loop:

1. "Rune offsets always advance" — `makeCtx` runs on six branches, not all
   eleven. Offsets now advance for selectable kinds only.
2. The list caveat hid two defects: out-of-order rendering, and silently dropped
   items when the last block is hooked. Both are fixed in the dispatch, not
   documented as caveats.
3. `QueueCommand` was banned from the hook. It takes `w.commandsMu`, not `w.mu`,
   and is the documented remedy.
4. `PlainText` "cached at style time" taxed every document for a feature few
   use. Derived per hooked block instead.

Also changed: return `(View, bool)` rather than `[]View` (one allocation fewer
per hooked block per frame); `MarkdownKindParagraph`, not heading, is the zero
value.
