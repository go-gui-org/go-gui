# Spec: wrapped text and container alignment

Status: **implemented** — issue #577. Landed on `main` in the same pass as the
fix and the `docs/dx-cheat-sheet.md` entry.

## Problem

A `Text` with `Mode: TextModeWrap` in a column with `HAlign: HAlignCenter`
rendered at the left edge, while a plain `Text` sibling centered.

Nothing in the alignment code was wrong. `Text` defaults a wrap mode to
`FillFit` (`gui/view_text.go`), because wrapping needs a width to wrap to. The
fill pass then set that child to the parent's content width, so
`childCrossAxisHAlign` (`gui/layout_position.go`) computed `remaining == 0` and
had nothing to move. The alignment of the lines _inside_ the box is a separate
control, `TextStyle.Align`, which nothing pointed the reader at.

## Two models

| Toolkit | Box alignment        | Line alignment              | Container alignment centers wrapped text |
| ------- | -------------------- | --------------------------- | ---------------------------------------- |
| SwiftUI | `VStack(alignment:)` | `.multilineTextAlignment()` | No                                       |
| Flutter | `crossAxisAlignment` | `Text(textAlign:)`          | No                                       |
| CSS     | `align-items`        | `text-align` (inherits)     | Yes                                      |

Go-Gui keeps the SwiftUI and Flutter split. The report is reasonable because the
CSS model is equally common, and the API said nothing about which applied.

## Decision

Keep the box/line split. Make the box honest: after the wrap pass, a wrapped
text box shrinks to its longest line, so the existing `HAlign` code has slack to
move it. `shrinkWrapToInk` (`gui/layout_pipeline.go`) does this, gated so that
only the reported case changes:

1. the parent is a column whose resolved `HAlign` is not left
2. the mode is `TextModeWrap` or `TextModeWrapKeepSpaces`
3. the Fill width came from the factory, not the caller (`wrapSizingDefault`)
4. the shape is not Input's text (`overflowScrollX`)
5. the shape is not a float, which its anchor places rather than the container
   it was declared in
6. `Sizing.Width` is not `sizingFixed`
7. `TextStyle.Align` is `TextAlignLeft`
8. the measured longest line is finite and narrower than the box

Gate 1 keeps every left-aligned wrapped text byte-identical, which is nearly all
of them, the library's own included. Gate 7 matters because glyph already
centers the lines against the wrap width; moving the box as well would double
the offset.

A shrink under a scrolling parent also refreshes that parent's `contentW` cache,
which the fill pass took a pass earlier. Only a `Scrollable` or `Clip` parent
gets the refresh: every reader of the cache is a scroll one, and recomputing it
for each wrapped child of a plain column would be quadratic in the child count
for a number nothing reads.

Shrinking cannot cause a re-wrap. The glyph layout is already shaped at the old
width, and `layoutFillWidths` resets the box to the parent's content width
before the wrap pass runs again, so the width handed to the shaper is the same
every frame. `shrinkWrapToInk` also rewrites `tc.textLayoutWidth` to the new
width, so the render pass reuses the shaped layout instead of shaping the same
text a second time — the line breaks at the longest line are the same breaks,
because every line already fits it and a word that did not fit the wider box
does not fit this one.

## Not covered

Text long enough to fill every line has no slack to return, so the box stays
full width and only `TextStyle.Align` moves anything. That is documented rather
than fixed.

RTF and markdown blocks (`layoutWrapRTF`) are untouched. They need no exclusion
branch, because every markdown block names its `Sizing` explicitly and gate 3
skips it. Shrinking would also be wrong there: an RTF block paints code
backgrounds, horizontal rules, tables and list indents to `shape.Width`.

## Rejected Approaches

- **Inherit the container's `HAlign` into the text's line alignment** (the CSS
  model). Rejected: `TextAlignLeft` is the zero value of `TextAlignment`, so an
  explicit left is indistinguishable from unset, and there is no way to respect
  a caller who asked for left under a centered parent. It would also re-lay out
  every wrapped text under a centered or right-aligned container across every
  consumer, including the library's own widgets.
- **Documentation only**, closing the issue as by design. Rejected: the surprise
  comes from `Mode` silently changing `Sizing`, which is a second effect of a
  field whose name says nothing about it. Docs alone leave the trap in place.
- **A `gui.Debug` finding** for wrap text under a non-left `HAlign` parent with
  no `TextStyle.Align`. Deferred, not rejected: it adds an exported
  `DebugCategory`, which is its own design pass.
