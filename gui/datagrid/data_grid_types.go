package datagrid

// GridSortDir specifies ascending or descending sort order.
type GridSortDir uint8

// GridSortDir constants.
const (
	GridSortAsc GridSortDir = iota
	GridSortDesc
)

// GridPaginationKind selects cursor- or offset-based paging.
type GridPaginationKind uint8

// GridPaginationKind constants.
const (
	GridPaginationNone GridPaginationKind = iota
	GridPaginationCursor
	GridPaginationOffset
)

// GridMutationKind identifies a create, update, or delete.
type GridMutationKind uint8

// GridMutationKind constants.
const (
	GridMutationCreate GridMutationKind = iota
	GridMutationUpdate
	GridMutationDelete
)

// GridSort describes a single sort criterion.
type GridSort struct {
	ColID string
	Dir   GridSortDir
}

// GridFilter describes a single column filter.
type GridFilter struct {
	ColID string
	Op    string // "contains", "equals", "starts_with", "ends_with"
	Value string
}

// GridQueryState holds the active sorts, filters, and quick
// filter for a data grid query.
type GridQueryState struct {
	QuickFilter string
	Sorts       []GridSort
	Filters     []GridFilter
}

// GridSelection tracks the selected rows in a data grid.
type GridSelection struct {
	SelectedRowIDs map[string]bool
	AnchorRowID    string
	ActiveRowID    string
}

// GridRow represents a single data row with an ID and
// column-keyed cell values.
type GridRow struct {
	Cells map[string]string
	ID    string
}

// GridCellEdit describes a single cell edit operation.
type GridCellEdit struct {
	RowID  string
	ColID  string
	Value  string
	RowIdx int
}

// GridCursorPageReq requests a cursor-based page.
type GridCursorPageReq struct {
	Cursor string
	Limit  int
}

func (GridCursorPageReq) gridPageRequest() {}

// GridOffsetPageReq requests an offset-based page.
type GridOffsetPageReq struct {
	StartIndex int
	EndIndex   int
}

func (GridOffsetPageReq) gridPageRequest() {}

// GridPageRequest is satisfied by GridCursorPageReq or
// GridOffsetPageReq. External code can type-switch but cannot
// implement their own pagination types.
type GridPageRequest interface {
	gridPageRequest()
}

// GridAggregateOp specifies the aggregation operation.
type GridAggregateOp uint8

// GridAggregateOp values.
const (
	GridAggregateCount GridAggregateOp = iota
	GridAggregateSum
	GridAggregateAvg
	GridAggregateMin
	GridAggregateMax
)

// String returns the SQL-like name for the aggregate operation.
func (op GridAggregateOp) String() string {
	switch op {
	case GridAggregateCount:
		return "count"
	case GridAggregateSum:
		return "sum"
	case GridAggregateAvg:
		return "avg"
	case GridAggregateMin:
		return "min"
	case GridAggregateMax:
		return "max"
	default:
		return "unknown"
	}
}
