Async data-source backed grid with CRUD operations, pagination,
abort handling, and ORM integration. Implement the `DataGridDataSource`
interface or use the built-in `InMemoryDataSource` / `GridOrmDataSource`.

## Usage

```go
source := gui.NewInMemoryDataSource([]datagrid.GridRow{
    {
        ID: "1",
        Cells: map[string]string{
            "name": "Alice", "team": "Core", "status": "Open",
        },
    },
})

datagrid.New(w, datagrid.DataGridCfg{
    ID:              "ds-grid",
    Columns:         showcaseDataGridColumns(),
    DataSource:      source,
    PaginationKind:  datagrid.GridPaginationCursor,
    PageLimit:       50,
    ShowQuickFilter: true,
    ShowCRUDToolbar: true,
})
```

## ORM Data Source

```go
src, err := gui.NewGridOrmDataSource(gui.GridOrmDataSource{
    Columns: []gui.GridOrmColumnSpec{
        {ID: "name", DBField: "name", Sortable: true, Filterable: true},
    },
    FetchFn: func(spec gui.GridOrmQuerySpec, sig *gui.GridAbortSignal) (gui.GridOrmPage, error) {
        // Query database using spec.Sorts, spec.Filters, spec.Limit, spec.Offset
        return gui.GridOrmPage{Rows: rows, RowCount: total}, nil
    },
    CreateFn:  myCreateFn,
    UpdateFn:  myUpdateFn,
    DeleteFn:  myDeleteFn,
})
```

## DataGridDataSource Interface

```go
type DataGridDataSource interface {
    Capabilities() GridDataCapabilities
    FetchData(req GridDataRequest) (GridDataResult, error)
    MutateData(req GridMutationRequest) (GridMutationResult, error)
}
```

## GridDataCapabilities

| Field                    | Type | Description                          |
|--------------------------|------|--------------------------------------|
| SupportsCursorPagination | bool | Supports cursor-based pagination     |
| SupportsOffsetPagination | bool | Supports offset-based pagination     |
| SupportsNumberedPages    | bool | Supports numbered page navigation    |
| RowCountKnown            | bool | Total row count is available         |
| SupportsCreate           | bool | Supports row creation                |
| SupportsUpdate           | bool | Supports row updates                 |
| SupportsDelete           | bool | Supports single row deletion         |
| SupportsBatchDelete      | bool | Supports multi-row deletion          |

## InMemoryDataSource

| Field          | Type      | Description                          |
|----------------|-----------|--------------------------------------|
| Rows           | []GridRow | In-memory row data                   |
| DefaultLimit   | int       | Default page size (100)              |
| LatencyMs      | int       | Simulated latency in ms              |
| RowCountKnown  | bool      | Expose total row count               |
| SupportsCursor | bool      | Enable cursor pagination             |
| SupportsOffset | bool      | Enable offset pagination             |

## GridOrmDataSource

| Field          | Type              | Description                          |
|----------------|-------------------|--------------------------------------|
| Columns        | []GridOrmColumnSpec | Column specs with DB mapping       |
| FetchFn        | GridOrmFetchFn    | Fetch callback (required)            |
| CreateFn       | GridOrmCreateFn   | Create row callback                  |
| UpdateFn       | GridOrmUpdateFn   | Update row callback                  |
| DeleteFn       | GridOrmDeleteFn   | Delete row callback                  |
| DeleteManyFn   | GridOrmDeleteManyFn | Batch delete callback              |
| DefaultLimit   | int               | Default page size                    |
| SupportsOffset | bool              | Enable offset pagination             |
| RowCountKnown  | bool              | Expose total row count               |

Runtime stats available via `w.DataGridSourceStats(id)`.
