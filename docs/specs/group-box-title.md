# Group-box titles

Issue #1238. Status: **implemented**.

## Problem

`ContainerCfg.Title` draws a label in a container's top border, like an HTML
fieldset legend. `addGroupBoxTitle` built it as two `Float` children, an eraser
rectangle and a text shape, positioned half the label's height above the box.

Floats are lifted out of the tree into layers drawn above it, sorted by
`FloatZIndex` and then by tree order. Every popup defaults to Z 0, so the title
was drawn over any tooltip, `Select` or `Combobox` list, menu or context menu
whose owner came earlier in the tree. The report was a tooltip on a button,
covered by the title of the radio group below it.

Being a float did more harm than the draw order:

- **Clipping.** A float escapes every ancestor clip. A titled box scrolled out
  of a viewport kept drawing its title over the header above, or below the
  footer.
- **Transforms.** A float is placed from its anchor's layout box, so the title
  of a rotated or scaled box stayed behind, unrotated.
- **Hover.** The title layer sat above the main tree in the hit test, so it took
  the hover state of whatever lay under it.
- **Disabled.** Floats are not stamped by `layoutDisables`, so the eraser was
  dimmed by hand, and the dimmed eraser let the border show through the title.

`TitleBG` defaulted to transparent, so an app that did not set it got the border
line through its title. Its doc said it must match the parent's background, and
`RadioButtonGroupCfg.TitleBG` had no doc at all.

## Decision

The code is in `gui/group_box_title.go`.

**The container draws its title itself.** The text shape hangs off
`Shape.fx.groupTitle` (with `titleBG` beside it), not off `Shape`, which sits at
312 of the 320 bytes `TestShapeFitsSizeClass` allows; it is the `imageSurface`
precedent. `renderLayoutDepth` draws the container's own shape, then calls
`renderLayoutChildren`, which opens the clip, stencil, clip-radius and
quarter-turn brackets around the children and closes them as it returns, then
calls `renderGroupBoxTitle`. So the title is drawn after the children, in the
parent's clip, but inside the box's transform and filter brackets. It is in the
main layer, under every popup, and it turns and filters with its box.

The split is not cosmetic. A first version deferred `renderGroupBoxTitle` inside
`renderLayoutDepth`. That was the function's eighth `defer`, with two `return`
statements, past the compiler's limits for open-coded defers (eight defers, and
defers times returns at most 15). Every defer in this per-shape, per-frame
function became a runtime record, about 15–19% slower on the flat render
benchmark. Check with `go build -gcflags=-d=defer ./gui/`.

**The box reserves room for its title.** With only the first change, a title
obeyed its parent's clip but still stuck out above its box. A scroll viewport
clips its children to its content box, so the first titled box in any scroll
view lost the top half of its title at rest; a `RotatedBox`, which clips at its
child's edges, did the same. Now `containerBody` draws the fill, shadow, border
and rounded stencil half the label's height below the box's top, and
`addGroupBoxTitle` sets the top padding so the content starts below the label:
half the label above the border, plus the caller's padding or the other half of
the label, whichever is larger. Both follow the label's measured height, so a
larger title font reserves more. A box sized by its content grows; a box with a
fixed or filling height keeps its size and its content area shrinks, the
breaking part of the change. HTML fieldsets, Win32 group boxes and GTK frames
all use this model. In a right-to-left box the label mirrors to the right edge
with the same inset.

**An unset `TitleBG` takes the color drawn under the border.** The border lies
at the top of the box's body, so the default eraser covers only the label's
lower half, from the border line down, and starts half a border width higher
because the web and PDF backends centre a stroke on its path. It ends at the
body's bottom in a box shorter than the label. `backdropColor` composites the
fills drawn there, the box's own first and then its ancestors', as
`renderContainer` paints them (opacity, the disabled dim, a hover fill), down to
the first opaque one or the window's clear color. It gives up, and no eraser is
drawn, at a gradient or shader fill, at the box's own blurred fill (its soft top
edge lies under the eraser), at a non-container ancestor, past a `ColorFilter`
(the eraser is drawn inside the filter, the fills further out are not), at a
float that is not opaque (a lifted float's `Parent` still points into the tree
it hangs from), and on a see-through window. It also draws none when the box has
no border to hide. An explicit `TitleBG` is a patch behind the whole label,
painted as given, never dimmed; `ColorTransparent` turns the eraser off.

A box an overflowing row hid (`shapeNone`) draws no title, and a title whose ink
is invisible (`ColorBorder` transparent beside a `BorderGradient`) draws
nothing, so no eraser cuts a gap in the border.

An empty titled box keeps its padding (`hasLayoutContent`): the sizing passes
collapse a container with no children to 0 on its cross axis, which a closed
`Sidebar` relies on, and the float children used to keep a group box out of that
rule. The clip radius a rounded clipping box hands its images
(`resolveClipRadius`) measures from the body's corner, not the box's.

## What it does not do

- A title wider than its box still overhangs the trailing edge, and a clipping
  parent cuts what overhangs it; the box's width does not count the title.
- Siblings drawn underneath, as in a `Stack`, are not part of the backdrop: the
  eraser takes the container's color over them. Set `TitleBG` there.
- There is no per-box title style; every title uses the theme's
  `TextStyleTitleSmall`.
- The title is not in the accessibility tree; it was not before either.

## Rejected approaches

- **Raise the tooltip's default `FloatZIndex`.** Fixes the report only. Select,
  Combobox, menus and context menus stay under titles, and the clipping and
  transform defects remain.
- **Give the title floats a Z below every popup.** A titled box inside a
  floating panel would draw its title under its own panel, and the clipping
  defects remain.
- **Keep the title as children, out of flow (`outOfFlow`), drawn after the
  content.** Children are positioned from the flow cursor, which includes the
  scroll offset and alignment, so each title shape needed a positioning hook,
  and a titled container that clips (any scrolling group box) cut its own title
  off, because its children draw inside its content box.
- **Keep the overhang and draw the title in the clipping parent's padding box,
  as scrollbars do.** Still cut by a parent with no padding (`RotatedBox`), a
  title scrolled past the edge lingers in the padding after its border is gone,
  and the title still overlaps whatever sits above the box.
- **Cut a gap in the border instead of drawing an eraser.** Exact on any
  background, but it takes three scissored border strokes per box, and a scissor
  is axis-aligned in the window, so it breaks under a rotation or a quarter
  turn.
