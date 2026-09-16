# Layout sizing rules

Status: in progress (issue #634)

## Why this exists

Three rounds of review, one deep, kept finding bugs in the sizing pass (#628,
#629, #630, #632). The cause was not weak reviewing. Nothing stated what the
sizing pass was supposed to do, so a reviewer had no oracle and could only check
the code against itself.

This document is that oracle. `gui/layout_invariants.go` is its executable half,
and the two are written to say the same thing.

## The invariants

These hold on every frame, for every shape, after `layoutPipeline` returns. The
`DebugLayoutInvariants` category checks 1 to 4 at dev time and the layout fuzz
targets check them in CI. The category is opt-in, outside `DebugAll`, and is
asked for by name:

```go
found := w.TestFindings(gui.DebugAll | gui.DebugLayoutInvariants)
```

1. **A child stays inside its parent's bounds** — unless the parent clips.
   `sizingClips` (`gui/layout_sizing_clamp.go:33`) decides: a `Clip` container,
   or a `Scrollable` one on an axis its `ScrollMode` does not exclude.
   Out-of-flow children are exempt, because they are positioned against
   something else: `Float`, `OverDraw` and `shapeNone`, the set
   `skipLayoutChild` drops (`gui/layout.go:115-120`).

   **Bounds, not the content box.** Padding says where content prefers to sit,
   not where it may not go. Widgets place children inside the padding on purpose
   — a `Button` optically centres its label through `AmendLayout` (#346),
   landing it a pixel past the content edge. The hard boundary is the parent's
   own rect, which is what the clip is cut from. A child escaping by a pixel of
   padding is not a defect; a child escaping its parent is.

   The check runs at the end of `layoutPipeline`, after `layoutAmend`,
   `applyLayoutTransition` and `applyHeroTransition`
   (`gui/layout_pipeline.go:42-44`), so it reads the tree the renderer gets.
   Checking before them would exempt every callback-positioned and animated
   shape — the geometry hardest to verify by reading.

   **Text is exempt from containment when no `TextMeasurer` is injected.** Two
   approximations disagree by construction in that state: an unmeasured text
   shape takes `fallbackLineHeight`, `style.Size * 1.4`
   (`gui/text_layout.go:14`), while the parent fitting around it takes
   `fontHeight`, `style.Size * 1.2` (`gui/render_text.go:510`). The label
   overshoots by 0.2em per line through no fault of the layout pass.
   `plainTextHeightNoMeasurer` already states the contract — "an estimate of an
   estimate — right for 'does this overflow', wrong for any pixel assertion".
   Invariants 2 and 3 still apply: a NaN, or a minimum above a maximum, is a
   defect whether or not the glyph metrics are real.

2. **After clamping, `min <= max`** on both axes. Every sizing site resolves a
   conflict with `effectiveMinSize` then `clampSize`, so a shape still carrying
   `min > max` never went through them.
3. **Every emitted dimension and position is finite and non-negative.** A NaN
   wins every later `f32Max` — it returns its second argument for NaN
   (`gui/math.go:63-74`) — and poisons the scroll range.
4. **Fill children sum to the parent's content box minus spacing**
   (`checkFillSum`, #638). Main axis only, and only when at least one in-flow
   child is Fill: cross-axis Fill is stretch, where every child takes the full
   content box, and a row with no Fill leaves its slack to alignment.
   Comparisons use `f32Tolerance` scaled by the child count, since the sum
   accumulates one rounding per child. Over-constraint is a defect when the
   parent neither clips nor scrolls and is not a Wrap or Overflow row, and is
   reported; it is a supported outcome when the parent clips or scrolls
   (overflow is what the clip and the scroll range are for) or when a Wrap row
   breaks rows or an Overflow row hides trailing children instead of shrinking.
   A gap left while every Fill child sits at its maximum is also supported: the
   caps are explicit and the remainder is alignment slack. A gap with room still
   to distribute, and an overflow outside the exemptions above, mean
   `distributeSpace` gave up with space still unallocated.

Comparisons use `f32Tolerance` (`gui/math.go:5`), the repo's existing epsilon.
The pass carries fractional sizes through four phases, so an exact compare would
report rounding, not defects.

## The rules

### Clamping

**Max wins a conflict with Min.** When both are set and `Min > Max`,
`effectiveMinSize` returns `Max` (`gui/layout_sizing_clamp.go:8-13`). Every
sizing site applies it, so a computed minimum — the sum of a container's
children's minimums — can never push that container past a stated maximum.

**Zero or negative means unset,** for both `Min` and `Max`. The cost of this
choice: a size cannot be pinned to exactly 0 through `Min`.

**A stated minimum is border-box; a computed one is content-box** (#385). The
same number means two different things depending on who wrote it. A caller's
`MinWidth` already counts padding and spacing; a floor summed from children has
them added.

### Fit and Fixed

**A Fixed axis with an explicit 0 degrades to content sizing** (#94). A
zero-size Fixed box would collapse the `shapeClip`, and therefore the hit-test
region, of every descendant. A childless box still resolves to 0. What a
_negative_ Fixed size means is unspecified — it currently takes the same branch
(`gui/layout_sizing.go:457`).

**A Fixed axis discards the caller's stated Min and Max.**
`applyFixedSizingConstraints` pins `Min = Max = Width` (`gui/sizing.go`), which
is why Fixed loses to nothing downstream. A conflicting stated bound reports
through the `DebugSizing` category (`warnFixedSizingConflict`); a bound equal to
the size is redundant but harmless and stays quiet, as does a Fixed axis with no
positive size, which degrades to content sizing. `Text` is exempt: it merges the
caller's `MinWidth` as a floor before the pin, so the pin is a no-op there.
`Image` and `Svg` carry no Fixed sizing and never pin.

**A container with no children collapses to 0 rather than to its padding.**
Padding is conditional on child count (`gui/layout_sizing.go:517-519`), which is
what a closed sidebar relies on to stay shut.

**A container reserves space for its border whether or not it paints one**
(`gui/shape.go:669-676`). A structural wrapper must set `SizeBorder: NoBorder`
or it silently gains height.

**A Fit child wider than its Fill parent escapes the parent.** A Fill parent
cannot grow to enclose Fit content. This is a defect when the parent neither
clips nor scrolls. A Fill width on the child lets the scroll body or the clip
absorb the excess (issue #642).

### Fill distribution

**Fill siblings equalize.** The algorithm is water-filling on the extremum
(`gui/layout_sizing.go:289-407`): repeatedly move the smallest child up, or the
largest down, toward the next extremum. It is not proportional to current size,
and there are no weights or flex factors anywhere in the package
(`gui/sizing.go:6-11`). Two Fill siblings end up equal regardless of content.

**A Wrap or Overflow row never shrinks its Fill children**
(`gui/layout_sizing.go:693`). It breaks rows or hides trailing children instead.

**Cross-axis Fill is stretch, not distribution**
(`gui/layout_sizing.go:246-252`). Every cross-axis Fill child gets the same
size: the parent's content box.

### Scrolling

**A scrollable container drops its children's minimum floor** on a scrolling
axis (`sizingClips`). Keeping the floor is what left a Scrollable FillFill
column as wide as its content with nothing to scroll (#584). An axis the
`ScrollMode` excludes keeps its floor, because it cannot reveal hidden content.

**A Scrollable Fill container never resolves below `spacingSmall` (5px)**
(`gui/layout_sizing.go:112-122`). A collapsed scroll area stays big enough to
hit-test and grab. Every axis routes through `scrollFillResetMin`, so an axis
the `ScrollMode` excludes keeps its floor — including the Column main axis
(#637).

**Scrollbars overlay content and reserve no gutter.** They are `OverDraw`
(`gui/view_scrollbar.go:87`), so `skipLayoutChild` excludes them from every
sizing calculation. This is the macOS convention: turning a scrollbar on never
reflows content.

### Wrap

**A wrap row is the full container width**, its height is its tallest child, and
there is no baseline alignment and no uniform height across rows
(`gui/layout_wrap.go:191-201`). The full-width row is what `HAlignCenter`
centres against — not the row's own content width.

**Wrapping is first-fit and horizontal only.** No lookahead, no balancing, and a
single child wider than the available width is never shrunk or broken; it gets
its own overflowing row (`gui/layout_wrap.go:154`).

**Wrap beats Overflow** when a container sets both (#380), with a `debugWarn`.

### Floats

**A float consumes no space**: no fence post in `spacing()`, no contribution to
a Fit parent's size, no advance of the position cursor. It is lifted out of the
tree entirely and becomes a sibling layer (`gui/layout_float.go:184-216`),
leaving a placeholder behind. A parentless float anchors to the window rect.

### Rounding

**The layout pass never rounds.** There is no pixel snapping anywhere in
`gui/layout*.go`; sizes stay fractional `float32` through to `RenderCmd`.
Snapping, if any, belongs to the backend, which alone knows the device pixel
ratio. This is a decision, not an oversight.

### Alignment

**Alignment does not shift content that overflows** — or rather, it should not.
`childCrossAxisVAlign` and `childCrossAxisHAlign` guard `remaining > 0`
(`gui/layout_position.go:178`, `:198`); `applyContainerAlignment` does not
(`:147-165`), so an overflowing centred row shifts further off-screen. The two
disagree and cannot both be the rule. Tracked as #636.

## Rejected approaches

- **Proportional or weighted Fill distribution.** Equalization is the shipped
  behaviour and changing it would move every existing layout. Weights are a
  feature request, not a bug fix, and belong in their own issue.
- **Pixel snapping in the layout pass.** Rounding container rects would change
  nearly every golden file and risks cumulative drift in nested trees, to fix
  seams that belong to the backend.
- **Checking invariants from `debugAudit`.** By audit time `composeLayout` has
  lifted floats into sibling layers (`gui/window_update.go:395`), so the
  composed tree no longer says which parent a float was written under and
  containment cannot be judged. The check hooks `layoutPipeline` instead, the
  same reasoning that puts `debugCheckStamp` on `resolveFocusOwners`.
- **Putting `DebugLayoutInvariants` in `DebugAll`.** Containment has
  correct-by-design exceptions that no flag on the shape marks. A `Slider` sizes
  its wrapper to the larger of track and thumb and then places the thumb from
  `AmendLayout`, so the thumb overhangs its track on purpose; the same holds for
  a `Button`'s optically centred label. A default-on check with permanent known
  false positives teaches people to ignore it, which is worth less than no
  check. The category follows `DebugUnscopedIDs` and is asked for by name
  instead.
- **Dropping the 5px scrollable Fill floor.** A zero-width scroll area has
  nothing to hit-test or grab.
