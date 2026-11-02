# Spec: large text in `Input` and `Text`

Issues #1190 and #1187. Status: **implemented** — chunked layout (this spec's
design A) with #1190; the byte-indexed edit core with delta undo, and the
`InputCfg.MaxLength` budget, with #1187.

## Problem

A `Text` or `Input` whose text was longer than 10,240 bytes drew nothing
(#1190). go-glyph caps one layout at `MaxTextLength` (10,240 bytes) and refuses
longer text; go-gui shaped a text shape's whole text as one layout, so the draw
failed as a whole. Below the cap the whole text was re-shaped on every rebuild
and on every keystroke, because the layout cache lived on the per-frame
`shapeTextConfig` arena, and the 65,536-rune paste budget (#1187) stood in for a
performance ceiling nothing could raise.

Measured on a desktop with the pure-Go shaper (Segoe UI, 16 px):

| Text                         | Time    | Allocated |
| ---------------------------- | ------- | --------- |
| 200 B paragraph, wrapped     | 80 µs   | 32 KB     |
| 1 KB paragraph, wrapped      | 0.41 ms | 185 KB    |
| 10 KB paragraph, wrapped     | 5.4 ms  | 6.8 MB    |
| 50 × 200 B chunks (= 10 KB)  | 3.9 ms  | 1.5 MB    |
| Rich text of 20 × 10 KB runs | 132 ms  | 131 MB    |

Layout memory grows superlinearly with paragraph size, so one layout for a
document can never scale: 64 KB would be ~35 ms and ~40 MB per rebuild.

## Design

A text shape's text is laid out as a `textBlock` (`gui/text_block.go`): a list
of chunks, each one glyph layout within the budget `textChunkBytes` (2048).

- **Chunking.** Whole paragraphs (`\n`-delimited) pack into one chunk while it
  stays within the budget. An empty paragraph is its own chunk with no layout
  and one synthetic line box, so a text ending in `\n` keeps its caret line
  (go-glyph gives it an empty last line too). A paragraph over the budget is one
  _open_ chunk that is cut when first reached. A wrapped seam is found by
  shaping; a no-wrap seam is a metadata-only cut. The budget, 2,048 bytes, keeps
  a chunk under the 4,096 clusters go-glyph retains its layout scratch for
  (`maxLayoutScratch`): every chunk reuses the shaper's buffers, where a 4 KB
  ASCII chunk would reallocate them on every shape. Measured on the pure-Go
  shaper, a 2 KB layout allocates 0.4 MB and a 4 KB one 1.6 MB.
- **Seams.** In wrap mode a cut is reflowed: the budgeted text plus one more
  budget is shaped once, the last line start within the limit becomes the next
  piece's start, and the piece ends where the line before ends. Greedy wrapping
  is prefix-stable, so the lines before the seam are the whole paragraph's
  lines, and no short line appears. The space the wrapper consumed at the break
  belongs to no chunk, as a `\n` does. In no-wrap mode the pieces of a paragraph
  sit side by side on one row.
- **Shaping on demand, bounded.** A chunk is shaped when a query or the renderer
  reaches it. A new block up to 64 KB (`textBlockEagerBytes`) shapes every chunk
  once for exact geometry; above that the unseen chunks are estimated from an
  average advance and measured as they come into view. At most 48 shaped chunks
  (`textBlockLiveLayouts`) stay live; the rest keep their geometry and are
  shaped again when needed. A chunk is shaped from a copy of its bytes:
  `glyph.Layout` keeps the text it shaped, and a slice of the block's text would
  keep that whole text alive, one old generation per chunk an edit did not
  touch.
- **Scroll anchoring.** Measuring an estimated chunk moves every line below it
  by the estimate's error. `layoutAnchorTextBlocks`
  (`gui/text_block_anchor.go`), run after the position pass, measures the chunks
  the scroll view shows before the frame draws them, grows the boxes that fit
  the text by the difference, and moves the scroll offset by the height that
  appeared above the first chunk that was already on screen. The lines in view
  stay put; only the scrollbar thumb moves. An edit shapes the re-packed chunks
  that hold text before it, so the paragraphs above the caret keep their
  measured heights even when the grown paragraph leaves its chunk; a large paste
  after them stays estimated, so a 100 KB paste costs a few milliseconds.
- **Persistence.** The block lives in `nsTextBlock` under the shape's identity
  (an input's owner, a `Text`'s ID, or for a large anonymous `Text` the string's
  own memory, content-checked on every hit). The frame resolves it through
  `plainTextBlock`, which brings it up to date: an edit is located as the bytes
  outside the common prefix and suffix of the old and new text, the paragraphs
  it touched are re-chunked, the chunks after it shift, and nothing else is
  shaped. A width, mode or shaping-style change rebuilds the block. A block that
  no layout resolved in the last two view passes is dropped (`sweepTextBlocks`):
  the cache is bounded by count, and a large text that left the tree would
  otherwise stay until 256 other identities pushed it out.
- **Single-line source/display cache.** A single-line input retains its exact
  source value, but shapes CR and LF as one-byte spaces so a pasted separator
  cannot move the caret onto a clipped second row. The block caches both forms
  and the separator count. An edit finds its common prefix and suffix once,
  transforms only the changed span, updates that count, and hands the same delta
  to the chunk patcher; unchanged megabyte values are not rescanned.
- **Queries.** `text_block_query.go` answers the layout queries the widgets used
  on a whole-text layout — caret position, hit test, selection rects, caret and
  word moves, line start and end, vertical moves, grapheme delete — in
  whole-text byte offsets and block coordinates, crossing seams where the single
  layout had none. Selection, IME and spelling rectangles shape only chunks in
  the two-dimensional clip. A far cursor in one unwrapped paragraph splits the
  metadata toward its offset without shaping the intervening pieces: their
  estimated cumulative X is present in both the destination position and the
  horizontal scroll offset, so it cancels at the viewport. Only the destination
  chunk needs exact cursor geometry. `TestTextBlockMatchesWholeLayout` checks
  every query against a whole-text layout of the same stub shaper at a 12-byte
  budget; `TestTextBlockLazyNoWrapFarCursorShapesOnlyDestination` pins the
  one-layout far-cursor bound.
- **Drawing.** `renderTextBlock` emits one glyph-layout draw per chunk that
  reaches into the clip. A single-line text over the budget takes the layout
  path too, since one `RenderText` command would hand the backend a string the
  shaper refuses. glyph applies a draw's transform about the draw's origin, so
  under a rotation, a transform or an animation's motion every chunk keeps the
  block's origin and moves inside the transform: the block turns as one. The
  clip test then uses each chunk's transformed box. A content transition's fade
  draws texts past one chunk the same way, each at its alpha.

The exit criterion of the spike holds: after the first frame (23 ms for a 100 KB
text), a rebuilt frame of that text costs 1.9 ms
(`TestRenderLargeTextRebuildIsCheap`, soft backend). Milestone-3 measurements on
Windows, an i7-1270P and the soft backend give these steady whole-widget costs
(30 edits/actions per sample; timing remains advisory):

| Operation           |                   100 KB |                1 MiB |
| ------------------- | -----------------------: | -------------------: |
| edit at start       |     0.96–1.05 ms, 382 KB | 2.34–2.49 ms, 874 KB |
| edit at end         | 0.74–0.85 ms, 276–278 KB | 0.95–0.98 ms, 534 KB |
| Home/End            |     0.15–0.20 ms, 5.2 KB | 0.83–0.94 ms, 5.2 KB |
| select-all/collapse |     0.15–0.22 ms, 5.2 KB | 0.77–0.86 ms, 5.2 KB |

The first `End` on a fresh 1 MiB no-wrap value formerly shaped 511 chunks:
396–402 ms, 170–173 MB transient allocation and about 15.6 MB retained after GC.
Metadata-only traversal shapes one destination chunk: 6.4–7.1 ms, 0.4–0.9 MB
transient and about 0.36 MB retained. The corresponding 100 KB jump fell from 48
shapes and about 39 ms to one shape and 0.65–1.26 ms. go-glyph's existing
`NoHitTesting` layout mode reduced the old pass to about 54 MB but still took
about 287 ms, so the useful fix belongs in go-gui and no go-glyph branch is
needed.

`BenchmarkInputLargeEdit` and `BenchmarkInputLargeNavigate` preserve the full
matrix. `TestInputLargeEditAllocBudget` caps a steady 1 MiB edit at 1.3 MB
(857,392-byte baseline), and `TestInputLargeNavigateAllocBudget` caps Home/End
and select-all/collapse at 32 KB each (5,216/5,248-byte baselines). The legacy
10 KB allocation-count gate remains for comparison and is skipped under the race
detector, whose process-wide bookkeeping makes `AllocsPerRun` unstable.

### Edit core

The caret and selection in `inputState` are byte offsets: the unit every layout
query takes, so a key event converts nothing (`gui/input_state.go`,
`gui/input_edit.go`). An insert or delete is one splice of the text with no
`[]rune` round trip. Grapheme boundaries without a layout come from
`prevGraphemeBoundary` and `nextGraphemeBoundary` (`gui/grapheme.go`), which
segment a 64-byte window around the offset, not the whole text.

An undo step (`inputMemento`) is the bytes an edit replaced at an offset —
`removed`, `inserted`, `off` — with the caret and selection before it. Undo puts
`removed` back where `inserted` sits; redo is the inverse. A typing run
coalesces into one step whose span is the union of its edits
(`inputMemento.merged`). A small span is copied so it does not pin the text it
was cut from (`mementoSpan`).

Undo first checks that `inserted` still sits at `off`. If the app replaced the
field's text, the step no longer applies and both stacks are dropped; splicing
stale bytes into a text the history never saw would corrupt it. An edit that
leaves the text unchanged records no step.

| One `editInsert`, ASCII text | Full-copy history | Delta history |
| ---------------------------- | ----------------- | ------------- |
| 10 KB                        | 62 µs, 90 KB      | 2 µs, 10 KB   |
| 1 MiB                        | 4.9 ms, 9.2 MB    | 0.17 ms, 1 MB |

`TestEditUndoRedoRandom` drives random edits, undoes every step back to the
start and redoes every step to the end.

### Budget

`InputCfg.MaxLength` is the field's budget in runes, the unit `OnPasteRejected`
already reported; 0 takes `inputMaxRunesDefault`, 1,048,576. The budget bounds
what one edit costs, a copy of the text, not what can be shown. Every path text
grows by checks it before the edit with `inputEditFits`: typing and IME commits
(`inputTextChange`), and paste from Ctrl+V, the context menu and the macOS yank
(`inputKeyPaste`). An edit that does not grow the text always fits, so a field
holding app-set text past its budget takes deletes and replacements no longer
than the selection. App-set text is never cut; the debug category
`DebugGlyphLayoutFallback` reports such a field once. `PreTextChange` and
`PostCommitNormalize` output is capped at the budget, as it was at the old one.

Two costs scale with the whole text and are bounded instead: the inner text
shape's accessible name carries the first 65,536 runes, and a field past 64 KiB
is not handed to the platform spell checker.

## Rejected Approaches

- **Raise `glyph.MaxTextLength`.** A single 64 KB layout would cost ~35 ms and
  ~40 MB per rebuild, and the renderer's draw scratch is sized for layouts up to
  the cap; the field would be unusable exactly where it claimed to work.
- **A widget-owned text buffer** (`InputCfg.Buffer`, go-edit style). Two sources
  of truth beside `Text`, a migration of every wrapper (`NumericInput`,
  `InputDate`, `InputGroup`, the datagrid editors) and either a materialized
  string on every `OnTextChanged` anyway or a breaking signature. The only gain
  over the block is the per-edit string copy, a memmove.
- **The block inside go-glyph** (`glyph.Document`). The cache policy is
  per-window, per-identity, immediate-mode state that belongs to the widget;
  go-glyph's unit is one paragraph; a cross-module dependency in every step. The
  string helpers go-glyph already exports (`WordStartLeft`,
  `WordBoundsInString`) cover the seam cases.
- **`Input` on a `DrawCanvas`** the way go-edit draws. Loses the layout pipeline
  — sizing, `Fill`, scroll container, theme roles, focus ring, a11y, IME rect —
  and duplicates go-edit, which exists for that use.
- **Fixed-size chunks ignoring `\n`.** Bidi reordering is per paragraph, so
  paragraph seams are exact and free; a forced seam inside a paragraph can split
  a bidi run, which only a paragraph longer than the budget with no newline in
  it ever pays for.
- **Laying out everything up front.** 1 MB is ~0.4 s and ~150 MB of layouts, and
  the cost comes back on every width change: 68 ms for 90 KB, 0.6 to 0.8 s for
  0.4 to 1 MB, against about 5 ms with estimates.
- **Shaping around the caret before scrolling it into view.** It made the
  caret's position exact, but measured chunks above the view outside the layout
  pass, where nothing compensates: the lines on screen moved by the estimate's
  error, a whole view height in `TestTextAnchorCaretWalkScrollsByRows`. Measure
  in the anchor pass, which corrects the offset in the same frame.
- **Keeping the layout cache on `shapeTextConfig`.** The arena is per frame, so
  every rebuild re-shaped the whole text: today's cost class.
- **Rune offsets with a rune-to-byte table.** It keeps the old unit at the cost
  of a second index the size of the text, rebuilt or patched on every edit; byte
  offsets need neither. The mask engine stays rune-based: a masked field is
  short, and the conversion happens at its boundary (`maskRuneArgs`).
- **Full-text undo copies with a smaller depth.** Fewer copies of a large field
  are still copies of a large field, and a depth small enough to bound the
  memory is too small to be useful.
- **An unlimited budget (`MaxLength: -1`).** Every edit copies the text, so an
  unbounded field is an unbounded cost per keystroke; a large positive value is
  available to an app that wants one.
- **Cutting app-set text to the budget.** That is the silent truncation #1187
  reported, moved from paste to `Text`. The app owns the string it sets.
- **Spell checking only the visible lines.** It needs the platform's ranges
  remapped across chunk offsets on every scroll; a size bound is simpler, and a
  field that large is rarely prose a user is proofreading.

## Known limits

- The scroll extent of a text over 64 KB is estimated until its chunks have been
  seen; it settles as the user scrolls, and the scrollbar thumb moves when it
  does.
- The anchor pass moves the offset only when every box between the text and its
  scroll view is a fit-height column that takes the height change in full: a
  `MaxHeight` it would pass, or a `MinHeight` it sits at or would fall below,
  stops it. Otherwise it still measures the chunks in view, and the boxes catch
  up on the next frame.
- A paragraph longer than the budget in no-wrap mode is drawn as pieces side by
  side, cut after a space when one is near; kerning across the seam is lost.
- Off-screen no-wrap pieces keep estimated widths until reached. A far caret and
  its horizontal scroll share those estimates, so the visible destination is
  stable; the total intrinsic width settles as pieces become visible.
- A gradient fill or a shimmer spans each chunk's box, not the whole text's, so
  past one chunk it starts over in every chunk. glyph measures a gradient on the
  layout it draws (`layoutGradientExtents`) and takes no larger frame; spanning
  the block needs that in go-glyph.
- At a character-level wrap of a run with no break opportunity the chunk
  boundary is the break the shaper would also make, and the caret at that offset
  sits on the later line, where glyph's single layout puts it on the earlier
  one.
