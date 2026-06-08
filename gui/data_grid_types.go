package gui

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

// gridMutationKind identifies a create, update, or delete.
type gridMutationKind uint8

// gridMutationKind constants.
const (
	gridMutationCreate gridMutationKind = iota
	gridMutationUpdate
	gridMutationDelete
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
