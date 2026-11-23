Data table from string arrays with row selection, alternating row colors,
virtualized scrolling, and configurable borders.

## Usage

```go
cfg := gui.TableCfgFromData([][]string{
    {"Name", "Age"},   // header row
    {"Alice", "30"},
    {"Bob", "25"},
})
cfg.ID = "my-table"
w.Table(cfg)
```

## From CSV

Parse with `encoding/csv` and pass the records as `RawData`:

```go
records, err := csv.NewReader(strings.NewReader("Name,Age\nAlice,30\nBob,25")).ReadAll()
if err != nil { log.Fatal(err) }
w.Table(gui.TableCfg{ID: "csv-table", RawData: records})
```

`gui.TableCfgFromCSV(data)` does the parse and builds the `TableCfg` with the
first row as header. `w.TableFromCSV(data)` goes one step further and returns
the view, showing the parse error as a one-cell table when the input is
malformed; `gui.TableCfgError(msg)` builds that error table.

## Custom Rows

```go
alignEnd := gui.HAlignEnd

w.Table(gui.TableCfg{
    ID:          "custom",
    BorderStyle: gui.TableBorderAll,
    Data: []gui.TableRowCfg{
        {Cells: []gui.TableCellCfg{
            {Value: "Name", HeadCell: true},
            {Value: "Score", HeadCell: true},
        }},
        {Cells: []gui.TableCellCfg{
            {Value: "Alice"},
            {Value: "95", HAlign: &alignEnd},
        }},
    },
})
```

`gui.TH(value)` and `gui.TD(value)` build a header cell and a data cell, so the
rows above can be written as `gui.TH("Name"), gui.TH("Score")` and
`gui.TD("Alice"), gui.TD("95")`.

## Virtualization

Renders only visible rows. The table always scrolls, so `Height` or
`MaxHeight > 0` is the only requirement. Scroll state is keyed by `cfg.ID`, or
`cfg.ID + ":scroll"` when `FreezeHeader` is set.

```go
cfg.MaxHeight = 260
```

## Wide Tables

A table takes the width its `Sizing` gives it; the default is `FillFit`, the
width of its parent. When its columns are wider, it scrolls sideways on its own
and shows a horizontal scrollbar. With `FreezeHeader`, the header stays at the
top but moves sideways with the rows, so each header stays above its column, and
the vertical scrollbar stays at the right edge of the table. A wheel or trackpad
scroll over the header scrolls the rows. A `Fit` width grows the table to its
columns instead, so it never scrolls sideways; use the default, another `Fill`
sizing or a fixed width.

```go
cfg.Sizing = gui.FillFit
cfg.FreezeHeader = true
```

## Stdlib Data Binding

Set `RawData` for CSV-style string data. First row is the header:

```go
w.Table(gui.TableCfg{
    ID: "simple-table",
    RawData: [][]string{
        {"Name", "Age"},
        {"Alice", "30"},
        {"Bob",   "25"},
    },
})
```

When `RawData` is set, `Data` is ignored.

## Border Styles

| Constant              | Description                   |
| --------------------- | ----------------------------- |
| TableBorderNone       | No borders                    |
| TableBorderAll        | Full grid                     |
| TableBorderHorizontal | Horizontal lines between rows |
| TableBorderHeaderOnly | Single line under header row  |

## Key Properties

| Property           | Type              | Description                   |
| ------------------ | ----------------- | ----------------------------- |
| RawData            | [][]string        | CSV-style data (alt. to Data) |
| Data               | []TableRowCfg     | Row data with cells           |
| ColumnAlignments   | []HorizontalAlign | Per-column alignment          |
| ColumnWidthDefault | float32           | Default column width          |
| ColumnWidthMin     | float32           | Minimum column width          |
| AlignHead          | HorizontalAlign   | Header row alignment          |
| MultiSelect        | bool              | Allow multi-row selection     |
| Selected           | map[int]bool      | Selected row indices          |
| Scrollbar          | ScrollbarOverflow | Overflow mode, both bars      |
| BorderStyle        | TableBorderStyle  | Cell border style             |
| Sizing             | Sizing            | Axis sizing, default FillFit  |
| Width              | float32           | Fixed width                   |
| Height             | float32           | Fixed height                  |
| MinWidth           | float32           | Minimum width                 |
| MaxWidth           | float32           | Maximum width                 |
| MinHeight          | float32           | Minimum height                |
| MaxHeight          | float32           | Maximum height                |
| FreezeHeader       | bool              | Pin header above the rows     |
| Focusable          | bool              | Opt in to keyboard focus      |
| RowHeader          | bool              | Row selector column + corner  |
| RowHeaderWidth     | float32           | Row header width              |
| CurrentRow         | Opt[int]          | App-owned marker row          |
| Sound              | SoundCue          | Row-activation cue            |
| SoundDisabled      | bool              | Suppress row sounds           |

## Appearance

| Property           | Type      | Description                    |
| ------------------ | --------- | ------------------------------ |
| Colors             | ColorSet  | Hover, Selected, Border        |
| ColorSelect        | Color     | Deprecated: Colors.Selected    |
| ColorRowAlt        | *Color    | Alternating row background     |
| CellPadding        | Padding   | Padding inside each cell       |
| TextStyle          | TextStyle | Body cell text style           |
| TextStyleHead      | TextStyle | Header cell text style         |
| RowHeaderTextStyle | TextStyle | Row header marker/corner style |
| RowHeaderMarker    | string    | Current-row glyph, default ▶   |
| SizeBorder         | float32   | Border line width              |
| SizeBorderHeader   | float32   | Header border line width       |

## Events

| Callback | Signature                         | Fired when            |
| -------- | --------------------------------- | --------------------- |
| OnSelect | func(map[int]bool, int, EventCtx) | Row selection changes |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
