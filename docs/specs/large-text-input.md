# Spec: large text in `Input` and `Text`

Issues #1190 and #1187. Status: **implemented** — chunked layout (this
spec's design A) landed with #1190; the byte-indexed edit core and the
`InputCfg.MaxLength` budget follow in the same series (#1187).

## Problem

A `Text` or `Input` whose text was longer than 10,240 bytes drew nothing
(#1190). go-glyph caps one layout at `MaxTextLength` (10,240 bytes) and refuses
longer text; go-gui shaped a text shape's whole text as one layout, so the draw
failed as a whole. Below the cap the whole text was re-shaped on every rebuild
and on every keystroke, because the layout cache lived on the per-frame
`shapeTextConfig` arena, and the 65,536-rune paste budget (#1187) stood in for
a performance ceiling nothing could raise.

Measured on a desktop with the pure-Go shaper (Segoe UI, 16 px):

| Text                                | Time        | Allocated |
| ----------------------------------- | ----------- | --------- |
| 200 B paragraph, wrapped            | 80 µs       | 32 KB     |
| 1 KB paragraph, wrapped             | 0.41 ms     | 185 KB    |
| 10 KB paragraph, wrapped            | 5.4 ms      | 6.8 MB    |
| 50 × 200 B chunks (= 10 KB)         | 3.9 ms      | 1.5 MB    |
| Rich text of 20 × 10 KB runs        | 132 ms      | 131 MB    |

Layout memory grows superlinearly with paragraph size, so one layout for a
document can never scale: 64 KB would be ~35 ms and ~40 MB per rebuild.

## Design

A text shape's text is laid out as a `textBlock` (`gui/text_block.go`): a list
of chunks, each one glyph layout within the budget `textChunkBytes` (2048).

- **Chunking.** Whole paragraphs (`\n`-delimited) pack into one chunk while it
  stays within the budget. An empty paragraph is its own chunk with no layout
  and one synthetic line box, so a text ending in `\n` keeps its caret line
  (go-glyph gives it an empty last line too). A paragraph over the budget is
  one *open* chunk that is cut when first shaped.
- **Seams.** In wrap mode a cut is reflowed: the budgeted text plus one more
  budget is shaped once, the last line start within the limit becomes the next
  piece's start, and the piece ends where the line before ends. Greedy wrapping
  is prefix-stable, so the lines before the seam are the whole paragraph's
  lines, and no short line appears. The space the wrapper consumed at the break
  belongs to no chunk, as a `\n` does. In no-wrap mode the pieces of a
  paragraph sit side by side on one row.
- **Shaping on demand, bounded.** A chunk is shaped when a query or the
  renderer reaches it. A new block up to 64 KB (`textBlockEagerBytes`) shapes
  every chunk once for exact geometry; above that the unseen chunks are
  estimated from an average advance and measured as they come into view. At
  most 48 shaped chunks (`textBlockLiveLayouts`) stay live; the rest keep
  their geometry and are shaped again when needed.
- **Persistence.** The block lives in `nsTextBlock` under the shape's identity
  (an input's owner, a `Text`'s ID, or for a large anonymous `Text` the
  string's own memory, content-checked on every hit). The frame resolves it
  through `plainTextBlock`, which brings it up to date: an edit is located as
  the bytes outside the common prefix and suffix of the old and new text, the
  paragraphs it touched are re-chunked, the chunks after it shift, and nothing
  else is shaped. A width, mode or shaping-style change rebuilds the block.
- **Queries.** `text_block_query.go` answers the layout queries the widgets
  used on a whole-text layout — caret position, hit test, selection rects,
  caret and word moves, line start and end, vertical moves, grapheme delete —
  in whole-text byte offsets and block coordinates, crossing seams where the
  single layout had none. `TestTextBlockMatchesWholeLayout` checks every query
  against a whole-text layout of the same stub shaper at a 12-byte budget.
- **Drawing.** `renderTextBlock` emits one glyph-layout draw per chunk that
  reaches into the clip. A single-line text over the budget takes the layout
  path too, since one `RenderText` command would hand the backend a string the
  shaper refuses.

The exit criterion of the spike holds: after the first frame (23 ms for a
100 KB text), a rebuilt frame of that text costs 1.9 ms
(`TestRenderLargeTextRebuildIsCheap`, soft backend). A keypress in a 10 KB
single-line field stays within the allocation gate
(`TestInputKeypressAllocBudget`).

## Rejected Approaches

- **Raise `glyph.MaxTextLength`.** A single 64 KB layout would cost ~35 ms and
  ~40 MB per rebuild, and the renderer's draw scratch is sized for layouts up
  to the cap; the field would be unusable exactly where it claimed to work.
- **A widget-owned text buffer** (`InputCfg.Buffer`, go-edit style). Two
  sources of truth beside `Text`, a migration of every wrapper
  (`NumericInput`, `InputDate`, `InputGroup`, the datagrid editors) and either
  a materialized string on every `OnTextChanged` anyway or a breaking signature.
  The only gain over the block is the per-edit string copy, a memmove.
- **The block inside go-glyph** (`glyph.Document`). The cache policy is
  per-window, per-identity, immediate-mode state that belongs to the widget;
  go-glyph's unit is one paragraph; a cross-module dependency in every step.
  The string helpers go-glyph already exports (`WordStartLeft`,
  `WordBoundsInString`) cover the seam cases.
- **`Input` on a `DrawCanvas`** the way go-edit draws. Loses the layout
  pipeline — sizing, `Fill`, scroll container, theme roles, focus ring, a11y,
  IME rect — and duplicates go-edit, which exists for that use.
- **Fixed-size chunks ignoring `\n`.** Bidi reordering is per paragraph, so
  paragraph seams are exact and free; a forced seam inside a paragraph can
  split a bidi run, which only a paragraph longer than the budget with no
  newline in it ever pays for.
- **Laying out everything up front.** 1 MB is ~0.4 s and ~150 MB of layouts.
- **Keeping the layout cache on `shapeTextConfig`.** The arena is per frame,
  so every rebuild re-shaped the whole text: today's cost class.

## Known limits

- The scroll extent of a text over 64 KB is estimated until its chunks have
  been seen; it settles as the user scrolls.
- A paragraph longer than the budget in no-wrap mode is drawn as pieces side by
  side, cut after a space when one is near; kerning across the seam is lost.
- At a character-level wrap of a run with no break opportunity the chunk
  boundary is the break the shaper would also make, and the caret at that
  offset sits on the later line, where glyph's single layout puts it on the
  earlier one.
