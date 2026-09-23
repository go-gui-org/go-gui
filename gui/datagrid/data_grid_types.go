package datagrid

// GridSortDir specifies ascending or descending sort order.
type gridSortDir uint8

// GridSortDir is the caller-facing spelling of the sort direction.
// Alias, not a new type: existing references keep working.
// exportaudit:keep — caller-facing sort spelling
type GridSortDir = gridSortDir

// GridSortDir constants.
const (
	gridSortAsc gridSortDir = iota
	GridSortDesc
	// GridSortAsc sorts ascending. Alias for the zero value, so
	// callers can spell both directions explicitly. Declared
	// after GridSortDesc: sharing the iota chain would collapse
	// Desc onto Asc.
	// exportaudit:keep — caller-facing sort spelling
	GridSortAsc = gridSortAsc
)

// GridPaginationKind selects cursor- or offset-based paging.
// exportaudit:keep — caller-facing config (issue #372)
type GridPaginationKind uint8

// GridPaginationKind constants.
const (
	// exportaudit:keep — caller-facing config (issue #372)
	GridPaginationNone GridPaginationKind = iota
	GridPaginationCursor
	GridPaginationOffset
)

// GridMutationKind identifies a create, update, or delete.
type gridMutationKind uint8

// GridMutationKind constants.
const (
	gridMutationCreate gridMutationKind = iota
	gridMutationUpdate
	gridMutationDelete
)

// GridSort describes a single sort criterion.
type GridSort struct {
	ColID string
	Dir   gridSortDir
}

// GridFilter describes a single column filter.
type gridFilter struct {
	ColID string
	Op    string // "contains", "equals", "starts_with", "ends_with"
	Value string
}

// GridFilter is the caller-facing spelling of a column filter.
// Alias: GridQueryState.Filters is rangeable today, and now also
// constructible outside the package.
// exportaudit:keep — caller-facing filter construction
type GridFilter = gridFilter

// GridQueryState holds the active sorts, filters, and quick
// filter for a data grid query.
type GridQueryState struct {
	QuickFilter string
	Sorts       []GridSort
	Filters     []gridFilter
}

// Clone deep-copies the sorts and filters so a handoff across
// goroutines or frames never shares backing arrays with the
// caller. Every async or debounced commit site uses it; a missed
// field here is a data race there.
func (q GridQueryState) Clone() GridQueryState {
	return GridQueryState{
		QuickFilter: q.QuickFilter,
		Sorts:       append([]GridSort(nil), q.Sorts...),
		Filters:     append([]gridFilter(nil), q.Filters...),
	}
}

// GridSelection tracks the selected rows in a data grid.
type GridSelection struct {
	SelectedRowIDs map[string]bool
	anchorRowID    string
	activeRowID    string
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
	rowIdx int
}

// GridCursorPageReq requests a cursor-based page.
type gridCursorPageReq struct {
	Cursor string
	limit  int
}

func (gridCursorPageReq) gridPageRequest() {}

// GridOffsetPageReq requests an offset-based page.
type gridOffsetPageReq struct {
	StartIndex int
	endIndex   int
}

func (gridOffsetPageReq) gridPageRequest() {}

// GridPageRequest is satisfied by GridCursorPageReq or
// GridOffsetPageReq. External code can type-switch but cannot
// implement their own pagination types.
// exportaudit:keep — collides with the gridPageRequest marker method
type GridPageRequest interface {
	gridPageRequest()
}

// GridAggregateOp specifies the aggregation operation.
type gridAggregateOp uint8

// GridAggregateOp values.
const (
	gridAggregateCount gridAggregateOp = iota
	gridAggregateSum
	gridAggregateAvg
	gridAggregateMin
	gridAggregateMax
)

// String returns the SQL-like name for the aggregate operation.
func (op gridAggregateOp) String() string {
	switch op {
	case gridAggregateCount:
		return "count"
	case gridAggregateSum:
		return "sum"
	case gridAggregateAvg:
		return "avg"
	case gridAggregateMin:
		return "min"
	case gridAggregateMax:
		return "max"
	default:
		return "unknown"
	}
}
