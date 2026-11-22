# Spec: scroll link

Issue #1293. Status: **implemented** (unreleased).

## Problem

A `Table` with `FreezeHeader` keeps its header in a zone above the scrolling
rows. The header zone did not scroll, so the table could not scroll sideways on
its own. Its minimum width was the sum of its columns, and in a narrower parent
the table grew past the parent. The last columns and the vertical scrollbar were
then out of reach. Wrapping the table in a sideways-scrolling `Column` made the
columns reachable, but moved the vertical scrollbar off screen until the user
scrolled to the last column.

`datagrid` had a frozen header that did scroll sideways. It read the rows'
offset with `Window.ScrollX` while the view was built and moved the header by
it. That offset is clamped later in the frame, in `layoutAdjustScrollOffsets`.
When the clamp moved it, for example when the grid got wider while scrolled to
the end, the header sat out of line with the columns for one frame. Measured:
100px off for one frame after the grid grew by 100px. During a live window
resize the header was off on every frame.

## Decision

`ContainerCfg.ScrollLinkX` names a `Scrollable` container by its effective ID.
The container that sets it does not scroll, but its children move sideways with
the named container's content.

- **Read at positioning.** `layoutChildStartPos` applies the linked offset to
  the linking container's children, with the same arithmetic a scroll container
  uses for its own children (an RTL row subtracts it). This runs after
  `layoutAdjustScrollOffsets`, so the offset is the clamped one of this frame,
  and the header can never lag the rows.
- **Live targets only.** The offset store keeps an entry after its container is
  gone. `layoutAdjustScrollOffsets` records the keys of the frame's scroll
  containers in `w.scratch.scrollLive`, which `layoutArrange` clears each frame.
  A link applies an offset only when its key is in that set. Clearing keeps the
  map's buckets, so a steady frame does not allocate.
- **Wheel.** `mouseScrollFallbackHandlerDepth` treats a container with a link as
  its target for the wheel: the delta scrolls the linked container. A frozen
  header scrolls the table like its rows do.
- **Storage.** The key lives on `eventHandlers`, not on `Shape`. `Shape` is 312
  bytes in a 320-byte size class (`TestShapeFitsSizeClass`); a string would push
  every shape to the next class. A linking container allocates its handlers
  record, which a frozen header has anyway.
- **Ignored** on a container that is `Scrollable` itself, and when no
  `Scrollable` container in the frame has the key.

The linking container must clip (`Clip: true`), or its children draw past its
edges. It must also have the same axis and horizontal padding as the linked
container, so both place their children at the same x. A Row and a Column place
a wider child differently under RTL: a Row starts at its right edge, a Column
pins it at its left edge.

### Table

- The header zone sets `Clip` and `ScrollLinkX` to the body. Clip lets the zone
  be narrower than its columns (`sizingClips`), so the table fits its parent.
  The body was already a `Fill` scroll container, whose minimum width is reset
  (`scrollFillResetMin`).
- The header zone takes the body's side gutter, so under RTL, where the gutter
  moves to the left, both start at the same x.
- Both layouts keep a strip at the bottom for the horizontal scrollbar, as they
  kept one on the right for the vertical bar. `Scrollbar` applies to both bars.
- Row separators are at least one row wide, so a scrolled table's lines reach
  the last column.
- Keyboard scrolling uses the arranged body's height less its padding, not
  `cfg.Height`. With a frozen header the height counted the header, and with the
  new strip it counted the strip.
- No `TableCfg` flag. Only a `Fill` table in a narrower parent changes, and that
  layout was broken. `Fit` parents and fixed-width tables measured unchanged.

### datagrid

The header, quick-filter and frozen-top-row bands are Columns with the rows'
padding and `ScrollLinkX` to the rows. They were Rows, which under RTL started
their content at the right edge while the rows were pinned at the left, so the
header showed other columns than the rows. A band is as wide as a row: the
columns, plus the CRUD delete button when it shows, less one border per pair of
neighbouring cells (`dataGridRowWidth`).

## Testing

- `gui/scroll_link_test.go`: a live target moves the linked children; a missing
  target, or one that stopped scrolling, leaves them where they are.
- `gui/view_table_hscroll_test.go`: a wide table fits its parent; header and
  body cells line up unscrolled, scrolled to the end, on the first frame after a
  resize, under RTL, and after a wheel over the header; separators reach the
  last column; a table without `FreezeHeader` scrolls both ways.
- `gui/view_table_keys_scroll_test.go`: End shows the whole last row, with and
  without `FreezeHeader`.
- `gui/datagrid/view_data_grid_hscroll_test.go`: header in line on the first
  frame after a resize, under RTL with and without the delete button, and after
  a wheel over the header.
- Goldens `table_freeze_wide` and `table_freeze_wide_scrolled`, dark and light.

## Known Limits

- Under RTL a scroll container whose content is wider than itself opens at the
  end of the content (#1345). This is general layout behaviour; the link keeps
  header and rows in step there, but does not move the start.
- The table's row header (the ▶ column) scrolls away with the columns. A
  DataGridView keeps it pinned on the left. This was decided in #1293.

## Rejected Approaches

- **Read the offset while building the view** (datagrid's old way). One frame
  late whenever the clamp moves the offset, on every frame of a live resize.
- **Make the header a second scroll container and copy the offset to it.** The
  two have different scroll limits, because only the body keeps the gutter for
  the vertical bar, so the copy drifts at the end of the range. A wheel over the
  header would scroll the header alone unless it wrote back to the body: two
  offsets kept in sync in both directions.
- **Move the header in `AmendLayout`.** The header has children, and moving a
  parent after positioning does not move them (`gui/CLAUDE.md`).
- **A sticky first row inside the body.** That needs a new layout feature, and
  it breaks virtualization's assumption that the frozen header row sits outside
  the scrolling body (`indexBase`).
- **An unexported link first, exported later.** Built that way first, but
  `datagrid` is a subpackage and can use only the exported API, and it had the
  same lag. Exported in the same change.
- **A `TableCfg` opt-in flag** (the issue's suggestion). The only changed layout
  is one that was broken, so the flag would have been a second field for the
  default behaviour.
