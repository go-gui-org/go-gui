Sortable data table from string arrays with row selection,
alternating row colors, virtualized scrolling, and configurable borders.

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

```go
cfg, err := gui.TableCfgFromCSV("Name,Age\nAlice,30\nBob,25")
if err != nil { log.Fatal(err) }
cfg.ID = "csv-table"
w.Table(cfg)
```

## Custom Rows

```go
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
            {Value: "95", HAlign: gui.HAlignEndPtr()},
        }},
    },
})
```

## Virtualization

Renders only visible rows when scrolling is enabled. Requires
`Scrollable: true` and `Height` or `MaxHeight > 0`.
Scroll state is keyed by `cfg.ID`, or `cfg.ID + ":scroll"` when
`FreezeHeader` is set.

```go
cfg.Scrollable = true
cfg.MaxHeight = 260
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

| Constant             | Description                        |
|----------------------|------------------------------------|
| TableBorderNone      | No borders                         |
| TableBorderAll       | Full grid                          |
| TableBorderHorizontal| Horizontal lines between rows      |
| TableBorderHeaderOnly| Single line under header row       |

## Key Properties

| Property           | Type              | Description                          |
|--------------------|-------------------|--------------------------------------|
| RawData            | [][]string        | CSV-style data (alt. to Data)        |
| Data               | []TableRowCfg     | Row data with cells                  |
| ColumnAlignments   | []HorizontalAlign | Per-column alignment                 |
| ColumnWidthDefault | float32           | Default column width                 |
| ColumnWidthMin     | float32           | Minimum column width                 |
| AlignHead          | HorizontalAlign   | Header row alignment                 |
| MultiSelect        | bool              | Allow multi-row selection            |
| Selected           | map[int]bool      | Selected row indices                 |
| Scrollable         | bool              | Opt into the scroll system (state keyed by ID)|
| Scrollbar          | ScrollbarOverflow | Scrollbar overflow mode              |
| BorderStyle        | TableBorderStyle  | Cell border style                    |
| Sizing             | Sizing            | Combined axis sizing mode            |
| Width              | float32           | Fixed width                          |
| Height             | float32           | Fixed height                         |
| MinWidth           | float32           | Minimum width                        |
| MaxWidth           | float32           | Maximum width                        |
| MinHeight          | float32           | Minimum height                       |
| MaxHeight          | float32           | Maximum height                       |

## Appearance

| Property         | Type         | Description                          |
|------------------|--------------|--------------------------------------|
| ColorBorder      | Color        | Border/grid line color               |
| ColorSelect      | Color        | Selected row background              |
| ColorHover       | Color        | Hovered row background               |
| ColorRowAlt      | *Color       | Alternating row background           |
| CellPadding      | Opt[Padding] | Padding inside each cell             |
| TextStyle        | TextStyle    | Body cell text style                 |
| TextStyleHead    | TextStyle    | Header cell text style               |
| SizeBorder       | float32      | Border line width                    |
| SizeBorderHeader | float32      | Header border line width             |

## Events

| Callback | Signature                                    | Fired when             |
|----------|----------------------------------------------|------------------------|
| OnSelect | func(map[int]bool, int, *Event, *Window)     | Row selection changes  |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
