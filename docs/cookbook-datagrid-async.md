# Async data binding in DataGrid

How to back a `DataGrid` with an asynchronous data source — paged
server results, a database, or any custom backend.

## 1. Three-tier data binding

`DataGrid` accepts data through three mutually-exclusive fields on
`DataGridCfg`. Priority: `DataSource` > `RowsData` > `Rows`.

| Tier | Field        | Type                  | When to use                                                                                         |
| ---- | ------------ | --------------------- | --------------------------------------------------------------------------------------------------- |
| 1    | `RowsData`   | `[]map[string]string` | Prototypes, tiny datasets. Columns auto-generated from first row's keys. Capped at 100,000 entries. |
| 2    | `Rows`       | `[]GridRow`           | Explicit row IDs, full control over cell values. All rows held in memory.                           |
| 3    | `DataSource` | `DataGridDataSource`  | Paged/remote data. Lazy loading, filtering, sorting, and CRUD all happen outside the grid.          |

This cookbook covers tier 3.

## 2. The DataGridDataSource interface

```go
// File: gui/datagrid/data_source.go

type DataGridDataSource interface {
    Capabilities() GridDataCapabilities
    FetchData(req GridDataRequest) (GridDataResult, error)
    MutateData(req GridMutationRequest) (GridMutationResult, error)
}
```

Three methods:

- **`Capabilities()`** — declares what the source supports (pagination
  style, whether row counts are known, which CRUD operations are
  available). Called on the main goroutine before every fetch.

- **`FetchData()`** — called from a background goroutine. Receives the
  page request, query state (filters, sorts, quick-filter), and an
  abort signal. Returns a page of rows.

- **`MutateData()`** — called from a background goroutine when the user
  creates, updates, or deletes rows via the CRUD toolbar or inline
  editing. Receives the mutation kind, affected rows, cell edits, and
  an abort signal.

### Supporting types

```go
type GridDataCapabilities struct {
    SupportsCursorPagination bool
    SupportsOffsetPagination bool
    SupportsNumberedPages    bool
    RowCountKnown            bool
    SupportsCreate           bool
    SupportsUpdate           bool
    SupportsDelete           bool
    SupportsBatchDelete      bool
}

type GridDataRequest struct {
    Page      GridPageRequest      // cursor or offset
    Signal    *GridAbortSignal     // check IsAborted() to stop work
    Query     GridQueryState       // filters + sorts + quick-filter
    GridID    string
    RequestID uint64               // monotonically increasing
}

type GridDataResult struct {
    NextCursor    string
    PrevCursor    string
    Rows          []GridRow
    RowCount      int               // -1 when unknown
    ReceivedCount int
    HasMore       bool
}
```

## 3. Quick start: InMemoryDataSource

For prototypes and small-to-medium datasets, `InMemoryDataSource`
provides filtering, sorting, cursor/offset pagination, and full CRUD
without writing a custom source.

```go
import (
    "github.com/go-gui-org/go-gui/gui"
    datagrid "github.com/go-gui-org/go-gui/gui/datagrid"
)

type App struct {
    Source datagrid.DataGridDataSource
}

func makeApp() *App {
    // Build row data once.
    rows := make([]datagrid.GridRow, 500)
    for i := range rows {
        rows[i] = datagrid.GridRow{
            ID:    fmt.Sprintf("row-%d", i),
            Cells: map[string]string{
                "name":  fmt.Sprintf("Item %d", i),
                "score": strconv.Itoa(i * 7 % 100),
            },
        }
    }

    src := datagrid.NewInMemoryDataSource(rows)
    src.DefaultLimit = 100     // rows per page (default 100)
    src.SupportsCursor = true  // cursor-based pagination (default)
    src.SupportsOffset = false // disable offset pagination
    src.LatencyMs = 80         // simulate network latency (0 = real production use)

    return &App{Source: src}
}
```

Wire it into the grid:

```go
func mainView(w *gui.Window) gui.View {
    app := gui.State[App](w)
    return datagrid.New(w, datagrid.DataGridCfg{
        ID:             "my-grid",
        Columns:        columns,
        DataSource:     app.Source,
        PaginationKind: datagrid.GridPaginationCursor,
        PageLimit:      100,
    })
}
```

`InMemoryDataSource` applies `Query.Filters`, `Query.Sorts`, and
`Query.QuickFilter` automatically. Mutations (create, update, delete)
modify the in-memory slice. Set `LatencyMs` > 0 to simulate async
latency for development; set it to 0 in production.

## 4. Custom data source

Implement `DataGridDataSource` directly when the data lives in a
remote API, database, or any custom store.

```go
type RESTDataSource struct {
    BaseURL string
}

func (s *RESTDataSource) Capabilities() datagrid.GridDataCapabilities {
    return datagrid.GridDataCapabilities{
        SupportsCursorPagination: true,
        RowCountKnown:            true,
        SupportsUpdate:           true,
    }
}

func (s *RESTDataSource) FetchData(
    req datagrid.GridDataRequest,
) (datagrid.GridDataResult, error) {
    // 1. Poll the abort signal so the grid can cancel stale requests.
    if req.Signal.IsAborted() {
        return datagrid.GridDataResult{}, errors.New("aborted")
    }

    // 2. Build the API request from req.Query and req.Page.
    cursor := ""
    if cp, ok := req.Page.(datagrid.GridCursorPageReq); ok {
        cursor = cp.Cursor
    }
    // ... HTTP call to s.BaseURL with cursor, filters, sorts ...

    // 3. Return the page.
    return datagrid.GridDataResult{
        Rows:          rows,
        NextCursor:    resp.NextCursor,
        RowCount:      resp.TotalCount,
        ReceivedCount: len(rows),
        HasMore:       resp.NextCursor != "",
    }, nil
}

func (s *RESTDataSource) MutateData(
    req datagrid.GridMutationRequest,
) (datagrid.GridMutationResult, error) {
    if req.Signal.IsAborted() {
        return datagrid.GridMutationResult{}, errors.New("aborted")
    }
    switch req.Kind {
    case datagrid.GridMutationUpdate:
        // ... HTTP PATCH with req.Edits ...
    case datagrid.GridMutationCreate:
        // ... HTTP POST with req.Rows ...
    case datagrid.GridMutationDelete:
        // ... HTTP DELETE with req.RowIDs ...
    }
    return datagrid.GridMutationResult{RowCount: newTotal}, nil
}
```

### Key rules for FetchData

- **Check `req.Signal.IsAborted()`** before expensive work and
  periodically during long operations. The grid cancels stale requests
  when the user pages quickly or changes filters.
- **`req.Page` is a sealed interface.** Type-switch on
  `GridCursorPageReq` (has `Cursor` and `Limit`) or
  `GridOffsetPageReq` (has `StartIndex` and `EndIndex`). Fall back to
  a default first page when neither matches.
- **`RowCount` = -1** when the total is unknown. The grid shows "?" in
  the status bar.
- **Return a proper `NextCursor`** for cursor pagination so the "next
  page" button works.

## 5. GridOrmDataSource

For SQL-backed grids, `GridOrmDataSource` wraps user-provided callback
functions with column validation, query normalization, and abort
handling.

```go
src := &datagrid.GridOrmDataSource{
    Columns: []datagrid.GridOrmColumnSpec{
        {
            ID:              "name",
            DBField:         "full_name",
            QuickFilter:     true,
            Filterable:      true,
            Sortable:        true,
            CaseInsensitive: true,
        },
        {
            ID:      "score",
            DBField: "score",
            AllowedOps: []string{
                "equals", "contains",
            },
            Sortable: true,
        },
    },
    FetchFn: func(
        spec datagrid.GridOrmQuerySpec,
        signal *gg.GridAbortSignal,
    ) (datagrid.GridOrmPage, error) {
        if signal.IsAborted() {
            return datagrid.GridOrmPage{}, errors.New("aborted")
        }
        // Build and execute SQL from spec:
        //   spec.QuickFilter  → WHERE ... ILIKE
        //   spec.Filters      → WHERE clauses
        //   spec.Sorts        → ORDER BY
        //   spec.Limit/Offset → LIMIT / OFFSET
        //   spec.Cursor       → WHERE id > cursor
        rows, err := db.Query(buildQuery(spec))
        // ...
        return datagrid.GridOrmPage{
            Rows:     rows,
            RowCount: total,
        }, nil
    },
    CreateFn: func(
        rows []datagrid.GridRow,
        signal *gg.GridAbortSignal,
    ) ([]datagrid.GridRow, error) {
        // INSERT INTO ... RETURNING id
        return inserted, nil
    },
    UpdateFn: func(
        rows []datagrid.GridRow,
        edits []datagrid.GridCellEdit,
        signal *gg.GridAbortSignal,
    ) ([]datagrid.GridRow, error) {
        // UPDATE ... SET ... WHERE id = ?
        return updated, nil
    },
    DeleteFn: func(
        rowID string,
        signal *gg.GridAbortSignal,
    ) (string, error) {
        // DELETE ... WHERE id = ?
        return rowID, nil
    },
    DefaultLimit:   100,
    RowCountKnown:  true,
    SupportsOffset: true,
}
```

### GridOrmColumnSpec fields

| Field             | Type       | Description                                                                       |
| ----------------- | ---------- | --------------------------------------------------------------------------------- |
| `ID`              | `string`   | Matches `GridColumnCfg.ID` in the grid config.                                    |
| `DBField`         | `string`   | The database column name (used in SQL).                                           |
| `AllowedOps`      | `[]string` | Subset of `contains`, `equals`, `starts_with`, `ends_with`. Defaults to all four. |
| `QuickFilter`     | `bool`     | Include this column in quick-filter (full-text) searches.                         |
| `Filterable`      | `bool`     | Allow per-column filter UI.                                                       |
| `Sortable`        | `bool`     | Allow sorting on this column.                                                     |
| `CaseInsensitive` | `bool`     | Wrap comparisons in `LOWER()`.                                                    |

### Callback signatures

```go
type GridOrmFetchFn func(
    spec GridOrmQuerySpec,
    signal *GridAbortSignal,
) (GridOrmPage, error)

type GridOrmCreateFn func(
    rows []GridRow,
    signal *GridAbortSignal,
) ([]GridRow, error)

type GridOrmUpdateFn func(
    rows []GridRow, edits []GridCellEdit,
    signal *GridAbortSignal,
) ([]GridRow, error)

type GridOrmDeleteFn func(
    rowID string,
    signal *GridAbortSignal,
) (string, error)

type GridOrmDeleteManyFn func(
    rowIDs []string,
    signal *GridAbortSignal,
) ([]string, error)
```

## 6. Pagination

Two pagination modes, selected by `DataGridCfg.PaginationKind`:

### Cursor-based (`GridPaginationCursor`)

- `GridDataCapabilities.SupportsCursorPagination` must be `true`.
- Each page returns a `NextCursor` and `PrevCursor` (opaque strings).
- The grid sends `GridCursorPageReq{Cursor, Limit}`.
- Best for large, append-only datasets. Cursors survive
  insertions/deletions better than offsets.

### Offset-based (`GridPaginationOffset`)

- `GridDataCapabilities.SupportsOffsetPagination` must be `true`.
- The grid sends `GridOffsetPageReq{StartIndex, EndIndex}`.
- `SupportsNumberedPages` enables numbered page buttons (1, 2, 3, …)
  in addition to prev/next arrows.
- Simpler to implement but can miss or duplicate rows if data changes
  between pages.

The `PaginationKind` field on `DataGridCfg` determines the mode. The
grid validates it against `Capabilities()` and falls back to what the
source supports.

## 7. Abort signals and staleness

The grid automatically cancels in-flight requests when the user
changes pages, filters, or sorts before the previous request
completes. Your data source must cooperate:

```go
func (s *MySource) FetchData(
    req datagrid.GridDataRequest,
) (datagrid.GridDataResult, error) {
    // Check before starting work.
    if req.Signal.IsAborted() {
        return datagrid.GridDataResult{}, errors.New("aborted")
    }

    // Poll during long operations. InMemoryDataSource polls every 20ms.
    for i, chunk := range chunks {
        if i%10 == 0 && req.Signal.IsAborted() {
            return datagrid.GridDataResult{}, errors.New("aborted")
        }
        process(chunk)
    }
    // ...
}
```

The grid also drops stale responses — if a newer request was issued
while this one was in flight, the result is discarded even if the
source didn't check the signal.

`GetSourceStats()` exposes cancellation and staleness counters:

```go
stats := datagrid.GetSourceStats(w, "my-grid")
fmt.Printf("requests=%d cancelled=%d stale=%d\n",
    stats.RequestCount, stats.CancelledCount, stats.StaleDropCount)
```

## 8. Loading and error states

### Loading indicator

`DataGridCfg.Loading` is set automatically by the grid when a source
request is in flight. The grid renders a spinner in the status bar.
You can also read it to disable other UI:

```go
stats := datagrid.GetSourceStats(w, "my-grid")
if stats.Loading {
    // Show a global loading indicator, disable submit buttons, etc.
}
```

### Error display

Return an `error` from `FetchData()` or `MutateData()`. The grid
displays the error message in the status bar and keeps the last
successful page visible. Read it back via `GetSourceStats()`:

```go
stats := datagrid.GetSourceStats(w, "my-grid")
if stats.LoadError != "" {
    // Show a toast or banner.
}
```

### Row count

When `RowCountKnown` is `true`, the grid shows "Page 3 of 42 (4,200
rows)". When `false`, it shows "Page 3 (200 rows)" with a "?"
placeholder for the total.

## 9. Source rebuilds

When toggling pagination mode or switching data sets, create a new
source and assign it to `DataGridCfg.DataSource`. The grid detects the
change and issues a fresh fetch:

```go
app.Source = &datagrid.InMemoryDataSource{
    Rows:           app.AllRows,
    DefaultLimit:   220,
    SupportsCursor: !app.UseOffset,
}
```

The old source's in-flight requests are aborted automatically.

## 10. Reference example

Full working example: `examples/data_grid_data_source/main.go` —
50,000 rows, cursor/offset pagination toggle, simulated latency,
editable columns, inline CRUD.

## Checklist

- [ ] `Capabilities()` accurately reflects pagination and CRUD support.
- [ ] `FetchData()` checks `req.Signal.IsAborted()` before and during work.
- [ ] `RowCount` is -1 when the total is genuinely unknown.
- [ ] Cursors are stable opaque strings that survive data changes.
- [ ] `NextCursor` is empty on the last page.
- [ ] `MutateData()` handles all three mutation kinds (`GridMutationCreate`,
      `GridMutationUpdate`, `GridMutationDelete`).
- [ ] Mutation callbacks validate column IDs against the grid config.
- [ ] Source rebuilds create a new `DataGridDataSource` instance — don't
      mutate the old one.
- [ ] `OnCRUDError` callback is wired for user-visible error feedback.
- [ ] `GetSourceStats()` is called only from the main goroutine (it reads
      from `StateMap` without a lock).
