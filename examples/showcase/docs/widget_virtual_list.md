Virtualized list of rows that the app builds, with heights that only the layout
knows: a chat transcript, a feed, a log view, a list of cards. Only the rows in
view are built.

## When to Use It

| Widget        | Row height             | Rows built by | Use for                          |
| ------------- | ---------------------- | ------------- | -------------------------------- |
| `ListBox`     | One height for all     | The widget    | Pick lists, options, selection   |
| `Table`       | One height for all     | The widget    | Rows and columns of cells        |
| `Tree`        | One height for all     | The widget    | Nested, expandable nodes         |
| `VirtualList` | Different for each row | Your app      | Cards, messages, log lines, feed |

`ListBox`, `Table` and `Tree` own the shape of their rows, so one row height
describes all of them. Use `VirtualList` when the rows are your own views and
their heights differ, for example because the text wraps.

## Usage

```go
gui.VirtualList(gui.VirtualListCfg{
    ID:        "feed",
    ItemCount: len(app.Messages),
    Height:    360,
    Sizing:    gui.FillFixed,
    ItemKey: func(i int) string {
        return strconv.Itoa(app.Messages[i].Seq)
    },
    ItemView: func(i int, _ float32) gui.View {
        return messageCard(app.Messages[i])
    },
})
```

The list needs a height to virtualize. Set `Height` or `MaxHeight`, or give it
`Fill` sizing in a parent that has a height. Inside a scrolling column, `Fill`
has no height to fill, so set `Height`.

## Heights

Two ways to get row heights:

- **`ItemHeight`** — return the height of row `i` without building it. Use it
  when the height is cheap to compute. The positions are then exact from the
  first frame, and nothing is measured.
- **Measurement** — leave `ItemHeight` nil. The list measures each row it
  builds, and starts from an estimate for the others. Positions become correct
  over the first frames that a region is in view, and the scrollbar becomes more
  accurate as you scroll.

The list puts no space between rows, because a gap is height that the model does
not know. Put the space inside the row, as padding.

## Identity

Set `ItemKey` when items can be inserted, deleted or moved. The list stores each
measured height under the key, so the height stays with its row.

Without `ItemKey`, heights are stored by index. After an insert at the top, each
row then shows at the height of the row that was at its index before, until it
is measured again. Call `Window.InvalidateListHeights(id)` to measure all rows
again.

When the content of a row changes and its key does not, the list cannot see the
change. Call `Window.InvalidateListHeights(id)` after such an edit.

## Scrolling

Scroll by index, not by percent. Most rows are not built, so the list has no
view to find by ID. The total content height is also an estimate, so a percent
of it moves as rows are measured. Do not use `ScrollVerticalToPct` on a
`VirtualList`.

| Method                           | Effect                                 |
| -------------------------------- | -------------------------------------- |
| `w.ScrollToIndex(id, i)`         | Row `i` at the top of the viewport     |
| `w.ScrollToIndexAt(id, i, frac)` | Row `i` at `frac`: 0 top, 0.5 middle   |
| `w.ScrollIndexIntoView(id, i)`   | Least scroll that shows row `i`        |
| `w.ScrollToEnd(id)`              | Bottom of the content                  |
| `w.VirtualListFocusedIndex(id)`  | Row the keyboard cursor is on          |
| `w.InvalidateListHeights(id)`    | Forget measured heights; measure again |

`id` is the effective ID. Under a panel with an ID it is the joined path, not
the leaf. Read it with `w.ResolveID("feed")`.

To keep the list at the bottom while rows are added, call `ScrollToEnd` after
each append.

When the list has focus, the arrow keys, Page Up, Page Down (10 rows), Home and
End move a keyboard cursor and scroll it into view. The cursor is an index,
because the row it is on may not be built. Read it in `ItemView` with
`w.VirtualListFocusedIndex(id)` and paint that row as focused.

## Width Warning

`ItemView` gets `width`, the inner width of the list from the previous frame. It
is 0 on the first frame, and one frame late after a resize.

Use `width` only for decisions, for example a compact layout below some width.
Do not use it as a minimum width. A row with `MinWidth: width` asks for the full
width of the list, and its own border and padding add a few pixels. The list
then becomes wider on each frame and never settles. Let rows fill the width with
`Sizing` instead.

## Key Properties

| Property      | Type                               | Description                             |
| ------------- | ---------------------------------- | --------------------------------------- |
| ID            | string                             | Scroll, height and focus state key      |
| ItemCount     | int                                | Number of rows                          |
| ItemView      | func(i int, width float32) View    | Builds row `i`                          |
| ItemHeight    | func(i int, width float32) float32 | Optional: height of row `i`, no measure |
| ItemKey       | func(i int) string                 | Optional: stable identity of row `i`    |
| OverscanPx    | float32                            | Pixels built past each edge (0 = half)  |
| Height        | float32                            | Fixed height (activates virtualization) |
| MinWidth      | float32                            | Minimum width                           |
| MaxWidth      | float32                            | Maximum width                           |
| MinHeight     | float32                            | Minimum height                          |
| MaxHeight     | float32                            | Maximum height                          |
| Sizing        | Sizing                             | Combined axis sizing mode               |
| Disabled      | bool                               | Disable interaction                     |
| Invisible     | bool                               | Hide without removing from layout       |
| FocusDisabled | bool                               | Opt out of default-on focus             |

## Appearance

| Property   | Type     | Description                         |
| ---------- | -------- | ----------------------------------- |
| Padding    | Padding  | Inner padding                       |
| Radius     | Radius   | Corner radius                       |
| SizeBorder | Border   | Border width                        |
| Color      | Color    | Background color (sets Colors.Base) |
| Colors     | ColorSet | Base, Border and BorderFocus colors |

Hover and selection colors belong to the rows. The list does not paint them.

## Events

| Callback  | Signature      | Fired when                                    |
| --------- | -------------- | --------------------------------------------- |
| OnKeyDown | func(EventCtx) | Key press not used by the built-in navigation |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Messages"}`.
