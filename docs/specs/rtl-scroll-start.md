# Spec: RTL scroll start

Issue #1345. Status: **implemented** (unreleased).

## Problem

Under a right-to-left locale, a scrolling `Column` whose child `Row` was wider
than the column showed the end of the row first. The row's first item was off
screen on the right, and the scrollbar thumb started at the left, which under
RTL is the end. `Table`, `DataGrid` and `Tree` are all built this way, so a wide
table opened at its last column and a tree hid its expand arrows.

Two things caused it:

- `childCrossAxisHAlign` pinned a child wider than its column at the column's
  left edge, whichever way the text ran.
- `scrollMirrorsX` mirrored the horizontal offset only for a scrolling `Row`. An
  RTL row already started at its right edge and kept its offset as a distance
  from that edge (#633). A column did not.

## Decision

An RTL column or stack works the way an RTL row already did.

- **Layout.** Under RTL, `childCrossAxisHAlign` pins a child wider than its
  `Column` or `Stack` at the right edge, its start. The rest sticks out past the
  left edge, as a browser page with `dir="rtl"` does. This holds whether or not
  the container scrolls. LTR is unchanged.
- **Offset.** `scrollMirrorsX` is true for an RTL column or stack, not only a
  row. The stored offset stays in [maxOffset, 0] and means a distance from the
  start edge, so 0 shows the start of the content. Everything that already asked
  `scrollMirrorsX` follows without change: the thumb position, thumb drag, a
  press on the track, the wheel, smooth scrolling and momentum.
  `layoutChildStartPos` subtracts the offset for a scroll container's own
  children. A container that follows one through `ScrollLinkX` takes the sign
  from the container it follows (`w.scratch.scrollLive` keeps it), not from
  itself: an LTR column or a `Canvas` that follows an RTL column would otherwise
  move against it.
- **Canvas is an exception.** A `Canvas` (`axisNone`) adds each child's own X,
  and its content width is the furthest child's X plus width. Its range runs
  from the left edge in either direction, so it neither pins a wide child at the
  right nor mirrors its offset. It behaves as it did before this change,
  including one flaw left alone: with the default alignment, an RTL canvas
  shifts a narrower child right by the free space, which the range does not
  count. Mirroring it moved a child at X 250, 100 wide, in a 300-wide canvas to
  300..400 when scrolled to the end, out of view.
- **Input is an exception.** A multiline `Input` field scrolls sideways only to
  reach the ink of an unbreakable run, which paints past the right edge of its
  text box in either direction. Its field sets the unexported
  `ContainerCfg.inkScrollX`, kept on `eventHandlers` so `Shape` does not grow,
  and `scrollMirrorsX` is false for it. The caret follow and the drag hit test
  read its offset as a distance from the left edge, as before. Mirroring it
  moved the text right on End, and the caret left the field. The field has the
  input's ID, so the scroll APIs reach it and report its offset from the left
  edge too.
- **Drag paths.** A `DragScroll` pan passes a pointer delta to
  `ScrollHorizontalBy`, which takes an offset delta, so it mirrors the delta
  when `scrollMirrorsX` holds. Before, an RTL row panned against the pointer.
  Without this step the change above would have broken RTL column panning, which
  worked only because columns were not mirrored. A horizontal drag-to-reorder
  had the same fault in its edge scroll and in the pointer mapping after a
  scroll, and both mirror now. No widget reaches them yet: `TabControl` is the
  only horizontal one and it names no scroll container.
- **API docs.** `ScrollHorizontalTo`, `ScrollHorizontalToPct`,
  `ScrollHorizontalOffset` and `ScrollHorizontalPct` say "start" and "end"
  instead of "left" and "right", and name the Canvas and Input exceptions.

No exported surface was added.

### Behaviour change

- An RTL `Column` or `Stack`, scrolling or not, lets a child wider than itself
  stick out on the left instead of the right.
- On an RTL column, `ScrollHorizontalTo(id, 0)` shows the right edge of the
  content, and a negative offset moves toward the left, as on an RTL row. An app
  that scrolled an RTL column to its far end to show the start should drop that
  workaround.

## Rejected Approaches

- **Turn an Input's offset around at the API.** `ScrollHorizontalTo` and the
  rest would convert, so 0 is the right edge for every RTL scrollable. It adds
  conversion code for one widget whose sideways scroll exists only to reach ink,
  and the caret follow and drag hit test would still need the physical offset.
  At 0 a long RTL word has its first letters off the right, as before this
  change; that is a matter of where the ink goes, not of the offset's sign.
- **Anchor a Canvas's content at the right.** Measure the furthest child and
  shift the whole canvas so its right end sits at the right edge under RTL. A
  canvas child's X is a position the app chose in left-to-right coordinates, so
  that moves every child away from where the app put it.
- **Start the offset at the far end.** The first time an RTL column overflows,
  set its offset to maxOffset and keep 0 as the left edge. `ScrollHorizontalTo`
  with 0 and `ScrollHorizontalToPct` with 0 would show the end, so offset 0
  would mean "end" on an RTL column and "start" on an RTL row. Each container
  would need a record that it was set up, and content that grows wider would
  slide the start off screen.
- **An opt-in `ContainerCfg` field.** Adds API to fix a bug, every widget and
  app has to set it, and the default stays wrong.
- **Build widget bodies as Rows.** A body has to stack its rows vertically, so
  it would take a Row scroller around a Column scroller. That moves the vertical
  scrollbar out of view until the user scrolls sideways (the #1293 problem), and
  leaves columns that apps build themselves wrong.
