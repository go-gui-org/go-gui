Full-featured data grid with sorting, filtering, pagination,
virtualized scrolling, column chooser, grouping, aggregation, inline editing,
row selection, and CRUD toolbar support.

## Usage

```go
datagrid.New(w, datagrid.DataGridCfg{
    ID:       "grid",
    PageSize: 10,
    Columns: []datagrid.GridColumnCfg{
        {ID: "name", Title: "Name", Sortable: true, Filterable: true},
        {ID: "age",  Title: "Age",  Sortable: true, Align: gui.HAlignEnd},
    },
    Rows: []datagrid.GridRow{
        {ID: "1", Cells: map[string]string{"name": "Alice", "age": "30"}},
        {ID: "2", Cells: map[string]string{"name": "Bob",   "age": "25"}},
    },
    ShowQuickFilter:  true,
    ShowColumnChooser: true,
})
```

## With DataSource

```go
source := gui.NewInMemoryDataSource(rows)
datagrid.New(w, datagrid.DataGridCfg{
    ID:             "ds-grid",
    Columns:        cols,
    DataSource:     source,
    PaginationKind: datagrid.GridPaginationCursor,
    PageLimit:      50,
})
```

## Stdlib Data Binding

Use `RowsData []map[string]string` for key-value data.
When `Columns` is empty, columns are auto-generated from the
first row's keys:

```go
datagrid.New(w, datagrid.DataGridCfg{
    ID: "simple-grid",
    RowsData: []map[string]string{
        {"name": "Alice", "age": "30"},
        {"name": "Bob",   "age": "25"},
    },
})
```

When `RowsData` is set, `Rows` is ignored.
`DataSource` still wins over `RowsData`.

## Virtualization

Automatically virtualizes when `Height` or `MaxHeight > 0`. Scroll
state is keyed by the widget's string ID + `":scroll"` suffix.

```go
datagrid.New(w, datagrid.DataGridCfg{
    ID:        "grid",
    MaxHeight: 300,
    // Virtualization is automatic.
})
```

## Key Properties

| Property               | Type               | Description                          |
|------------------------|--------------------|--------------------------------------|
| Columns                | []GridColumnCfg    | Column definitions                   |
| ColumnOrder            | []string           | Column display order by ID           |
| RowsData               | []map[string]string | Key-value row data (alt. to Rows)    |
| Rows                   | []GridRow          | Static row data                      |
| DataSource             | DataGridDataSource | Async data backend                   |
| PaginationKind         | GridPaginationKind | Cursor or offset pagination          |
| PageSize               | int                | Rows per page (static)               |
| PageLimit              | int                | Rows per page (data source)          |
| PageIndex              | int                | Current page index                   |
| Query                  | GridQueryState     | Active sorts, filters, search        |
| Selection              | GridSelection      | Current row selection state          |
| GroupBy                | []string           | Column IDs for row grouping          |
| Aggregates             | []GridAggregateCfg | Column aggregate definitions         |
| ShowQuickFilter        | bool               | Show search bar                      |
| ShowFilterRow          | bool               | Show per-column filters              |
| ShowColumnChooser      | bool               | Show column visibility picker        |
| ShowCRUDToolbar        | bool               | Show create/delete toolbar           |
| FreezeHeader           | bool               | Sticky header during scroll          |
| MultiSort              | *bool              | Allow multi-column sort              |
| MultiSelect            | *bool              | Allow multi-row selection            |
| RangeSelect            | *bool              | Allow shift-click range select       |
| ShowHeader             | *bool              | Show/hide header row                 |
| HiddenColumnIDs        | map[string]bool    | Columns hidden by ID                 |
| FrozenTopRowIDs        | []string           | Row IDs pinned to top                |
| DetailExpandedRowIDs   | map[string]bool    | Expanded detail row IDs              |
| QuickFilterPlaceholder | string             | Quick filter placeholder text        |
| QuickFilterDebounce    | time.Duration      | Quick filter debounce delay          |
| RowHeight              | float32            | Row height in pixels                 |
| HeaderHeight           | float32            | Header height in pixels              |
| IDFocus                | uint32             | Tab-order focus ID (> 0 to enable)   |
| Scrollable             | bool               | Opt into the scroll system (state keyed by ID)|
| Disabled               | bool               | Disable interaction                  |
| Invisible              | bool               | Hide without removing from layout    |
| Sizing                 | Sizing             | Combined axis sizing mode            |
| Width                  | float32            | Fixed width                          |
| Height                 | float32            | Fixed height                         |
| MinWidth               | float32            | Minimum width                        |
| MaxWidth               | float32            | Maximum width                        |
| MinHeight              | float32            | Minimum height                       |
| MaxHeight              | float32            | Maximum height                       |

## GridColumnCfg

| Property    | Type            | Description                          |
|-------------|-----------------|--------------------------------------|
| ID          | string          | Column identifier                    |
| Title       | string          | Header text                          |
| Width       | float32         | Column width                         |
| Sortable    | bool            | Enable sorting                       |
| Filterable  | bool            | Enable filtering                     |
| Editable    | bool            | Enable inline editing                |
| Resizable   | bool            | Enable drag-resize                   |
| Reorderable | bool            | Enable drag-reorder                  |
| Pin         | GridColumnPin   | Pin left or right                    |
| Align       | HorizontalAlign | Cell alignment                       |

## Appearance

| Property          | Type         | Description                          |
|-------------------|--------------|--------------------------------------|
| ColorBackground   | Color        | Grid background                      |
| ColorHeader       | Color        | Header background                    |
| ColorHeaderHover  | Color        | Header hover background              |
| ColorFilter       | Color        | Filter row background                |
| ColorQuickFilter  | Color        | Quick filter background              |
| ColorRowHover     | Color        | Row hover background                 |
| ColorRowAlt       | Color        | Alternating row background           |
| ColorRowSelected  | Color        | Selected row background              |
| ColorBorder       | Color        | Border/grid line color               |
| ColorResizeHandle | Color        | Column resize handle color           |
| ColorResizeActive | Color        | Active resize handle color           |
| PaddingCell       | Opt[Padding] | Cell padding                         |
| PaddingHeader     | Opt[Padding] | Header cell padding                  |
| PaddingFilter     | Padding      | Filter row padding                   |
| TextStyle         | TextStyle    | Body cell text style                 |
| TextStyleHeader   | TextStyle    | Header text style                    |
| TextStyleFilter   | TextStyle    | Filter text style                    |
| Radius            | float32      | Corner radius                        |
| SizeBorder        | float32      | Border width                         |
| Scrollbar         | ScrollbarOverflow | Scrollbar overflow mode          |

## Events

| Callback               | Signature                                              | Fired when                    |
|------------------------|--------------------------------------------------------|-------------------------------|
| OnQueryChange          | func(GridQueryState, *Event, *Window)                  | Sort/filter/search changes    |
| OnSelectionChange      | func(GridSelection, *Event, *Window)                   | Row selection changes         |
| OnColumnOrderChange    | func([]string, *Event, *Window)                        | Column reorder                |
| OnColumnPinChange      | func(string, GridColumnPin, *Event, *Window)           | Column pin changes            |
| OnHiddenColumnsChange  | func(map[string]bool, *Event, *Window)                 | Column visibility changes     |
| OnPageChange           | func(int, *Event, *Window)                             | Page navigation               |
| OnDetailExpandedChange | func(map[string]bool, *Event, *Window)                 | Detail row expand/collapse    |
| OnCellEdit             | func(GridCellEdit, *Event, *Window)                    | Inline cell edit committed    |
| OnRowsChange           | func([]GridRow, *Event, *Window)                       | Row data changes              |
| OnCellClick            | func(string, *Event, *Window)                          | Cell clicked                  |
| OnCellFormat           | func(GridRow, int, GridColumnCfg, string, *Window) GridCellFormat | Custom cell formatting |
| OnDetailRowView        | func(GridRow, *Window) View                            | Render detail row content     |
| OnCopyRows             | func([]GridRow, *Event, *Window) (string, bool)        | Copy selected rows            |
| OnRowActivate          | func(GridRow, *Event, *Window)                         | Row double-click/enter        |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
