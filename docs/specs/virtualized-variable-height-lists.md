# Spec: variable-height virtualized lists and index-addressed scrolling

Status: **implemented** — issue #332.

Two halves of one problem.

**Virtualization was arithmetic over one scalar.** `listCoreVisibleRange`
(`gui/list_core.go`) divides the scroll offset by a single `rowHeight`, and the
content height is faked with transparent spacer rectangles sized
`rowCount × rowHeight`. That is exact for a widget that owns its row shape and
useless for rows the caller builds — a chat transcript, a feed, a log view, a
card list.

**Rows could not be addressed by index.** `scrollToView` resolves through
`FindByID`, so a row virtualization never built is structurally unreachable.
`ScrollVerticalToPct` inherits the fictional content height, so `pct = 1` lands
near the end rather than at it and pin-to-bottom drifts with every append.

The outcome is a `VirtualList` widget, a per-list height model, and a public
index-addressed scroll API that works on the new widget **and** on the five
existing uniform ones.

## The height model

`gui/list_height_model.go`. One type, two shapes:

- **uniform** — every row is `rowH` tall. No per-item storage, every query is
  arithmetic. This is what the existing widgets already assume, which is why the
  retrofit costs them one line.
- **variable** — per-item heights in a Fenwick (binary indexed) tree, seeded
  from an estimate and refined by measurement. `Prefix` and `IndexAt` are O(log
  n), a point update O(log n), a full rebuild O(n).

### Why an exact tree rather than a sampled average

go-shirei's `widgets.VirtualList` (zlib; design reference, not copied) samples
the first N and last N rows and extrapolates an average. That is cheaper and
stateless, and its own documentation records what it costs: a thumb that jumps
and a false bottom on a corpus whose row heights are regionally skewed — a feed
of short acknowledgements followed by long posts is exactly that shape.

An exact prefix-sum tree pays 12 B/item and a staleness problem in exchange for
a scrollbar that means what it shows. Both costs are answered rather than
accepted:

- staleness — `ItemKey` (below);
- memory — above `listHeightMaxVariable` (~4M items) the model falls back to
  uniform and reports it under `gui.Debug` (the `DebugListBoxNoHeight`
  category).

### Why the tree is float64 while the heights are float32

At a million rows the running prefix reaches ~3×10⁷ px, where one float32 ULP is
~4 px. That is visible jitter, and worse, it makes the prefix sequence
non-monotonic — so the `IndexAt` binary-lifting search can return an index whose
span does not contain `y`. The heights themselves stay float32 because they are
pixel measurements; only the accumulation needs the range.

### Why shirei's per-frame `Measure` does not port

shirei measures every candidate row each frame and caches nothing. In go-gui the
layout passes mutate ID-keyed window state (`layoutOverflow`'s overflow map,
`resolveShapeIDs`) and draw shapes from the frame-scoped pool. Measuring N rows
per frame would both allocate and corrupt per-window state. Measurement here is
therefore observation of rows that were built anyway — the `listBoxAmendLayout`
idiom — which is what makes the write-back and the re-anchor necessary.

### Heights are keyed, positions are indexed

`heights`/`tree` hold the current frame's index order; `byKey` holds what was
actually measured, under the caller's `ItemKey`. A count change rebuilds the
tree by walking `ItemKey(i)` and taking `byKey[k]`, so an insert, a delete or a
reorder keeps every measured height on its own item. Without `ItemKey` the model
is index-keyed: an insert at the front shifts every measured height by one and
rows render at a neighbour's height until re-measured.
`Window.InvalidateListHeights` is the escape hatch, and is also the only answer
for content edited under a _stable_ key — nothing detects that.

`byKey` is bounded: when it outgrows the live count by `listHeightKeyLoadFactor`
it is replaced by one holding only live keys, so a long-running feed does not
accumulate forever.

## The write-back and the re-anchor

One `AmendLayout` on the list's scroll container, not one per row. `layoutAmend`
fires children-first, so every row's `Shape.Height` is final. Child position
`leadOffset + k` maps to item `builtFirst + k` — recorded by the view phase,
never reverse-parsed from a row ID.

The re-anchor keys on the **row under the viewport top**, not on `first`:

```
anchor = IndexAt(-scrollY)
before = Prefix(anchor)
... write each built row's measured height ...
delta  = Prefix(anchor) - before
scrollY -= delta
```

Keying on `first` cannot work: rows above `first` sit inside the leading spacer
and are never measured, so `Prefix(first)` does not move and the delta is
structurally always zero.

`IndexAt` carries a boundary tolerance for this, and it is not cosmetic. The
offset a re-anchor stores **is** a `Prefix`, rounded to float32 on its way out
while the tree sums in float64. Exactly on a row boundary that rounding lands a
hair below it and the walk answers with the row above — one ULP of error, a
whole row of displacement, and the correction then moves the reader by that
row's height. The tolerance is one float32 ULP of `y` plus a sub-pixel floor.
`TestListHeightModelIndexAtMonotonic` asserts `IndexAt(Prefix(i)) == i` over a
corpus of deliberately fractional heights; whole-pixel heights give prefixes
that are exact in float32 and ask nothing.

The already-positioned children are shifted by the same amount, as scroll
anchoring shifts them. This is the part that is easy to get wrong. This frame's
rows were laid out at their _actual_ heights above a leading spacer sized from
the _stale_ model, so they already sit off by exactly that delta; correcting
only the stored offset fixes the next frame and leaves this one visibly
displaced. `TestVirtualListWriteBackHoldsTheAnchorRow` is the layout-level
guard.

## Index-addressed scrolling

`gui/scroll_index.go`. Public on `*Window`: `ScrollToIndex`,
`ScrollToIndexAt(frac)`, `ScrollIndexIntoView`, `ScrollToEnd`, plus
`InvalidateListHeights`.

A request is applied immediately when the list has already been generated, and
queued either way. `layoutApplyVirtualScrolls` runs **after**
`layoutApplyScrollAnchors` and before `layoutDisables`. The position is not
negotiable: `layoutAdjustScrollOffsets` is the _first_ position pass and
zero-fills any scrollable with no stored entry, so an offset written before it
is clobbered on a list nobody has scrolled yet — which is the exact case
`ScrollToIndex` exists for. By the new pass the arranged height and
`contentHeight` are both known, so the clamp is real.
`TestScrollToIndexOnNeverScrolledList` pins it.

A far jump into never-measured rows lands estimate-accurate. The request stays
live for `virtualScrollMaxAge` frames and re-applies, so it converges as the
rows it caused to be built are measured. A uniform model is exact on the first
application and the request drops immediately. A jump also drops any pending
`scrollAnchor` for the same scrollable — both write the same key, and the
explicit jump is the later intent.

## The retrofit: index spaces

Each existing virtualized widget registers a uniform model with exactly the
`rowHeight` its own spacers use, so the index API agrees with the arithmetic
already on screen.

| Site                          | Index space                                                          |
| ----------------------------- | -------------------------------------------------------------------- |
| `gui/view_listbox.go`         | `cfg.Data`, subheadings included                                     |
| `gui/view_table.go`           | `cfg.Data`; `indexBase = dataStart`, so a frozen header sits outside |
| `gui/view_tree_rows.go`       | the **flat** row index, not the node index; re-registered per frame  |
| `gui/view_combobox.go`        | the **filtered** items — moves with the query                        |
| `gui/view_command_palette.go` | the **filtered** items — moves with the query                        |

Known limits inherited rather than fixed here:

- Virtualization triggers on `Scrollable && height > 0`. Only `ListBox` falls
  back to the height arrange recorded last frame, so a Fill-sized `Table` or
  `Tree` never virtualizes and therefore registers no model.
- The table emits separator rectangles between rows that may not be inside
  `tableEstimateRowHeight`. The registered model inherits that error rather than
  introducing a second one.

## VirtualList

`gui/view_virtual_list.go` and `_build.go` (split for the 800-line gate).

- **No `Virtualize` flag.** Virtualization turns on automatically when the
  scroll container has a bounded height, following the existing convention;
  `VirtualList` is scrollable by construction. Height source: `Cfg.Height` →
  `MaxHeight` → the inner height the amend hook recorded last frame.
- **Bounded first-frame probe.** Under Fill sizing frame 1 has no height. The
  `ListBox` precedent builds _every_ row there; at a million rows that is a
  hang, so a `virtualListProbeRows` window is built instead — enough for arrange
  to record a height.
- **Overscan in pixels, not rows.** A row count is meaningless when rows differ
  in height. A row count remains as a secondary cap against pathological
  corpora.
- **Spacing fixed at zero.** A gap between rows is height the model does not
  account for, and every position below would drift by one gap per row. Space
  rows from inside `ItemView`.
- **Focus is an index, not a shape.** An unbuilt row has no effective ID, so the
  focused row lives in a `StateMap` keyed by list ID. `VirtualListFocusedIndex`
  / `SetVirtualListFocusedIndex` read and write it; arrow keys move it and call
  `ScrollIndexIntoView`, so focus lands one frame later. Tab traverses only
  built rows.
- Row IDs are the caller's to compose, with `ScopeIDN(listID, "row", i)` — an ID
  containing `:` is absolute, which is what keeps a row's identity stable while
  the window it lives in scrolls.

## The width ratchet

The one mistake that makes a virtual list misbehave with nothing to catch: a row
that turns the width `ItemView` is **handed** into a width it **demands**.
`MinWidth: width - padding` looks like it cancels out, and does not — the row's
border pushes the demand a few pixels past the width the list just reported, so
the list widens, reports the wider width, and is asked for wider still. Every
frame. And because a width change invalidates the wrap, every frame also
re-wraps and re-measures every built row, which reads as text reflowing under
the pointer and a scroll position that will not sit still.
`examples/virtual_list` shipped with exactly this bug.

Nothing about it looks like a failure from inside the widget, so the widget
reports it: `virtualListNoteWidth` counts consecutive frames of widening with
the window standing still — the window moving is what separates a resize, which
stops, from a ratchet, which does not — and warns after four under [Debug]. Rows
fill the width they are given through `Sizing`; the argument is for decisions,
not for minimums.

## Known limits

1. **Without `ItemKey` the model is index-keyed** (above).
2. **Width is one frame late.** `ItemView(i, width)` receives the inner width
   recorded by the previous arrange; it is 0 on frame 1. After a resize the
   heights are stale for one frame. The width change does **not** discard them:
   a height measured for a row at a slightly different wrap predicts that row
   better than one estimate shared by the whole corpus, and the rows on screen
   are re-measured that frame anyway. Discarding them re-seated the reader from
   the estimate on every frame of a resize drag. The anchor is captured before
   the change and restored after either way.
3. **Practical row ceiling.** `Shape.Y`, `scrollY`, `contentHeight` and the
   spacer heights are float32 throughout the engine, so past ~10⁷ px of content
   the scroll system quantizes regardless of the model's precision.

## Attribution

go-shirei `widgets/scroll.go` and `docs/virtual-list.md` (zlib) were read as a
design reference for the problem shape and the failure modes worth documenting.
No code was copied; the height model here is deliberately a different design,
for the reasons above.
