# Spec: scroll window (frozen and pinned columns)

Issue #1363 (`Table`) and #1364 (`DataGrid`). Status: **implemented**
(unreleased).

## Problem

Since #1293 a `Table` scrolls sideways on its own and its frozen header follows.
The row header cell (#1156) is part of each row, so it scrolled away with the
columns, and no leading column could be kept in place. A WinForms `DataGridView`
keeps the row header at the left edge and can freeze columns. An app could not
build this itself: the row header cell is built inside every row, and
`ScrollLinkX` has no counterpart for the Y axis.

`DataGrid` had `GridColumnCfg.Pin`, but it only changed the column order.

## Decision

`TableCfg.FreezeRowHeader` keeps the row header in place.
`TableCfg.FreezeColumns` keeps the first N data columns in place, and the row
header with them. Leading means left-most, or right-most under RTL. Both are off
by default.

A frozen table's rows stay in place and only part of each row scrolls:

```text
body (Scrollable, states its extent; does not move its rows)
  row  [ row header | frozen cells | edge | scroll window -> other cells ]
  row  [ row header | frozen cells | edge | scroll window -> other cells ]
```

- **Extent.** The body states how wide its content is (`ScrollExtentX`).
  `contentWidth` answers that width, so the clamp, the scrollbar and the wheel
  need no change. The body keeps its children in place.
- **Window.** A window is a clipping container with `ScrollLinkX` to the body
  (`docs/specs/scroll-link.md`) and `scrollWindow` set. It moves its cells by
  the body's offset. The cells are cut off at the window's edge, not covered by
  the frozen ones. A table is transparent and its selected rows are a tint, so
  nothing has to know the color behind it.
- **The row stays one container.** It fills the viewport, up to its columns. Its
  fill, hover, click and height are those of one row, so the two sides cannot
  drift apart.
- **Narrow tables.** A window keeps 80px. When the frozen cells and the window
  do not fit, the rows are wider than the viewport (`ScrollExtentMin`). The
  first part of the scroll range, the lead, then moves the rows as a plain
  scroll container would, and the windows move by the rest. Every column can be
  reached at any width. The frozen header follows by its `ScrollLinkX`, which
  takes the part the body moves its own children by.
- **Clip.** A window clips sideways only when drawn. The focus ring of a control
  in a cell is a glow around it, and a clip as high as the row would cut it. Its
  sides snap to the device pixels that a box's sides round to (`snapClipX`). A
  backend rounds a clip outward, and at 125% a filled cell drew over the border
  of the frozen cell beside it: header and filter cells have a fill.
- **The edge.** With `TableBorderAll` the cell borders mark the frozen edge. The
  window's start padding keeps scrolling cells off the last frozen cell's
  border. With the other border styles a 1px line in the border color follows
  the last frozen cell, the row header too when only that is frozen.
- **Nothing to freeze.** A count of 0 or less, or one that covers every column,
  freezes nothing. A table that freezes nothing builds the same rows as before.

### Allocations

The windows come from a frame pool (`scratchPools.viewWindows`). A row and its
window share one slice. A table's row and cell slices now come from the view
arena (`takeViews`), frozen or not. The next frame's reset clears the arena and
the windows' content, so the views of a table that is gone are freed. A table of
12 columns and a row header with 48 built rows went from 2267 to 1643
allocations per frame, and to 1644 with two frozen columns. A window adds two
clip commands per row.

### DataGrid

`Window.ScrollWindow` builds the window, and `Table` and `DataGrid` both call
it. `datagrid` is a subpackage, so the method and the two `ContainerCfg` fields
are exported. The geometry of the edge is in one place, and the pool serves
both. An app can build a grid of its own with them.

A row of a grid with pinned columns is its left-pinned cells, the window and its
right-pinned cells. The start padding of a window keeps its content off the
border of the cell before it. The content itself starts at the window's edge
(`scrollWindowInset`), so it still overlaps that border at rest. With cells
after the window the end padding does the same (`ClipEnd`).

Not every row is made of cells:

| Row                                           | With pinned columns                  |
| --------------------------------------------- | ------------------------------------ |
| Header, filter row, frozen top rows, data row | Pinned cells stay, the rest scrolls  |
| Row-delete button                             | Stays after right pins, else scrolls |
| Group header, status line                     | Stays, so its text shows             |
| Quick filter                                  | Stays, with its Clear button         |
| Detail row                                    | Scrolls sideways, as without pins    |
| Column chooser                                | Stays; its list scrolls on its own   |

Without pins a detail row is wider than a data row by its padding, and the
scroll range is that wide. With pins its content is narrower by that much. A
range longer than a data row would open a gap between the last scrolling column
and the right-pinned ones.

`GridColumnPinLeft` means the start of a row, the right edge under RTL. The
column order already worked that way, and `FreezeColumns` does. The pin button's
arrows mirror, like the reorder arrows.

The quick filter stays in a grid without pins too, as wide as the grid. A wheel
over it still scrolls the rows: without pins it is in a scroll window, which
does not move when the body moves its own rows, and passes the wheel on.

The grid's scroll body has no border of its own any more. It took the theme's
container border, which is transparent, so every row started 1px beside the
bands above it.

## Testing

- `gui/scroll_window_test.go`: the stated extent is the scroll range; the body
  moves no child of any sizing; a window moves and clips its content; a window
  without an extent does nothing; the lead with non-finite widths.
- `gui/view_table_freeze_test.go`: frozen cells stay, the others move, header
  and rows stay in line, for every border style, LTR and RTL, with and without a
  frozen header; a narrow table reaches every column; the first frame after a
  resize; ragged rows; row heights; `Fit` width; clicks at the edge; selection;
  hover; the wheel over each part; virtualization; a frozen table in a frozen
  table's cell; no allocation per row.
- `gui/datagrid/view_data_grid_pins_test.go`: the same for left pins, right pins
  and both, with and without borders, RTL, the row-delete button and a frozen
  header; the rows that are not made of cells; a narrow grid.
- Goldens `table_freeze_columns_grid`, `table_freeze_columns_edge` and
  `table_freeze_row_header`, and in `gui/datagrid` `data_grid_pins` and
  `data_grid_pins_edge`, dark and light.

## Known Limits

- iOS and Android do not set `Window.BackingScale`, so the clip is not snapped
  there. Where the frozen edge falls between two device pixels, a filled cell
  still draws over one of them.
- `FreezeHeader` freezes one row. A table cannot freeze more rows at the top.
- A popup or other float an app anchors to a scrolling cell is not cut at the
  frozen edge, as a float is not cut by any scroll container.

## Rejected Approaches

- **Two row lists side by side, linked on the Y axis (`ScrollLinkY`).** A row
  becomes two containers: hover, selection, clicks and accessibility have to be
  kept in sync. `Table` rows are as high as their content, so the two halves
  drift apart.
- **Frozen cells pushed back into view and drawn over the others.** They need an
  opaque fill in the color behind the table. A table is transparent, a selected
  row is a tint, and a gradient or an image has no one color.
- **Offset the frozen cells while building the view.** One frame late whenever
  the clamp moves the offset (#1293).
- **Move the frozen cells in `AmendLayout`.** They have children, and moving a
  parent there does not move them (`gui/CLAUDE.md`).
- **Unfreeze columns when the table is too narrow.** Needs the width of the
  frame before, and the layout jumps at a threshold. The lead needs neither.
- **Export the window as a `ContainerCfg` flag (#1364).** Every caller would
  have to repeat the padding that keeps content off the neighbouring border, and
  a subpackage could not reach the pool.
- **Reach the window through a `gui/internal` package (#1364).** Needs untyped
  hooks between the packages, and the export policy rejects the move
  (`docs/specs/exportaudit-surface-policy.md`).
- **Freeze the row header by default, as WinForms does.** Changes every table
  that has a row header today.
