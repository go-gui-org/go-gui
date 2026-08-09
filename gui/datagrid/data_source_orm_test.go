package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

func testColumns() []gridOrmColumnSpec {
	return []gridOrmColumnSpec{
		{ID: "name", dBField: "name", QuickFilter: true,
			Filterable: true, Sortable: true,
			caseInsensitive: true},
		{ID: "email", dBField: "email", QuickFilter: true,
			Filterable: true, Sortable: true,
			caseInsensitive: true},
		{ID: "status", dBField: "status", QuickFilter: false,
			Filterable: true, Sortable: true,
			caseInsensitive: false,
			allowedOps:      []string{"equals"}},
	}
}

func testOrmSource(t *testing.T) *GridOrmDataSource {
	t.Helper()
	src, err := newGridOrmDataSource(GridOrmDataSource{
		Columns:      testColumns(),
		DefaultLimit: 50,
		fetchFn: func(
			_ GridOrmQuerySpec,
			_ *gg.GridAbortSignal) (gridOrmPage, error) {
			return gridOrmPage{
				Rows:    []GridRow{{ID: "1"}},
				hasMore: false,
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func TestNewGridOrmDataSourceValidation(t *testing.T) {
	_, err := newGridOrmDataSource(GridOrmDataSource{
		Columns: []gridOrmColumnSpec{
			{ID: "", dBField: "x"},
		},
		fetchFn: func(GridOrmQuerySpec, *gg.GridAbortSignal) (gridOrmPage, error) {
			return gridOrmPage{}, nil
		},
	})
	if err == nil {
		t.Fatal("expected error for empty column id")
	}
}

func TestNewGridOrmDataSourceDuplicateColumn(t *testing.T) {
	_, err := newGridOrmDataSource(GridOrmDataSource{
		Columns: []gridOrmColumnSpec{
			{ID: "x", dBField: "x", Filterable: true, Sortable: true},
			{ID: "x", dBField: "y", Filterable: true, Sortable: true},
		},
		fetchFn: func(GridOrmQuerySpec, *gg.GridAbortSignal) (gridOrmPage, error) {
			return gridOrmPage{}, nil
		},
	})
	if err == nil {
		t.Fatal("expected error for duplicate column id")
	}
}

func TestNewGridOrmDataSourceInvalidDBField(t *testing.T) {
	_, err := newGridOrmDataSource(GridOrmDataSource{
		Columns: []gridOrmColumnSpec{
			{ID: "x", dBField: "1bad"},
		},
		fetchFn: func(GridOrmQuerySpec, *gg.GridAbortSignal) (gridOrmPage, error) {
			return gridOrmPage{}, nil
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid db_field")
	}
}

func TestNewGridOrmDataSourceNoFetchFn(t *testing.T) {
	_, err := newGridOrmDataSource(GridOrmDataSource{
		Columns: testColumns(),
	})
	if err == nil {
		t.Fatal("expected error for nil FetchFn")
	}
}

func TestGridOrmCapabilities(t *testing.T) {
	src := testOrmSource(t)
	caps := src.Capabilities()
	if !caps.supportsCursorPagination {
		t.Fatal("cursor should be supported")
	}
	if caps.supportsCreate {
		t.Fatal("create should not be supported")
	}
}

func TestGridOrmFetchData(t *testing.T) {
	src := testOrmSource(t)
	res, err := src.FetchData(GridDataRequest{
		page: gridCursorPageReq{limit: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
}

func TestGridOrmValidateQuery(t *testing.T) {
	q, err := gridOrmValidateQuery(GridQueryState{
		Sorts: []GridSort{
			{ColID: "name", Dir: gridSortAsc},
			{ColID: "unknown"},
		},
		Filters: []gridFilter{
			{ColID: "name", Op: "contains", Value: "test"},
			{ColID: "status", Op: "equals", Value: "a"},
			{ColID: "status", Op: "contains", Value: "b"},
		},
	}, testColumns())
	if err != nil {
		t.Fatal(err)
	}
	// Unknown sort dropped.
	if len(q.Sorts) != 1 {
		t.Fatalf("sorts = %d, want 1", len(q.Sorts))
	}
	// status only allows "equals", "contains" is dropped.
	if len(q.Filters) != 2 {
		t.Fatalf("filters = %d, want 2", len(q.Filters))
	}
}

func TestGridOrmValidateQueryQuickFilterTooLong(t *testing.T) {
	long := make([]byte, gridOrmMaxFilterValueLen+1)
	for i := range long {
		long[i] = 'a'
	}
	_, err := gridOrmValidateQuery(GridQueryState{
		QuickFilter: string(long),
	}, testColumns())
	if err == nil {
		t.Fatal("expected error for long quick_filter")
	}
}

func TestGridOrmBuildSQL(t *testing.T) {
	src := testOrmSource(t)
	b, err := src.buildSQL(GridOrmQuerySpec{
		QuickFilter: "test",
		Sorts: []GridSort{
			{ColID: "name", Dir: GridSortDesc},
		},
		Filters: []gridFilter{
			{ColID: "status", Op: "equals", Value: "active"},
		},
		limit:  25,
		Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.whereSQL == "" {
		t.Fatal("WhereSQL should not be empty")
	}
	if b.orderSQL == "" {
		t.Fatal("OrderSQL should not be empty")
	}
	if len(b.Params) == 0 {
		t.Fatal("Params should not be empty")
	}
}

func TestGridOrmEscapeLike(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"100%", `100\%`},
		{"a_b", `a\_b`},
		{`a\b`, `a\\b`},
		{`%_\`, `\%\_\\`},
	}
	for _, tt := range tests {
		got := gridOrmEscapeLike(tt.input)
		if got != tt.want {
			t.Errorf("EscapeLike(%q) = %q, want %q",
				tt.input, got, tt.want)
		}
	}
}

func TestGridOrmValidDBField(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"name", true},
		{"_id", true},
		{"t.name", true},
		{"", false},
		{"1bad", false},
		{"a..b", false},
		{"a.", false},
		{"a-b", false},
		{"a b", false},
		{"T1", true},
		{"users.email_addr", true},
	}
	for _, tt := range tests {
		got := gridOrmValidDBField(tt.input)
		if got != tt.want {
			t.Errorf("ValidDBField(%q) = %v, want %v",
				tt.input, got, tt.want)
		}
	}
}

func TestGridOrmMutateMissingFn(t *testing.T) {
	src := testOrmSource(t)
	_, err := src.MutateData(GridMutationRequest{
		Kind: gridMutationCreate,
		Rows: []GridRow{{ID: "x"}},
	})
	if err == nil {
		t.Fatal("expected error for nil CreateFn")
	}
}

func TestGridOrmMutateUnknownColumn(t *testing.T) {
	src, err := newGridOrmDataSource(GridOrmDataSource{
		Columns: testColumns(),
		fetchFn: func(GridOrmQuerySpec, *gg.GridAbortSignal) (gridOrmPage, error) {
			return gridOrmPage{}, nil
		},
		createFn: func(rows []GridRow, _ *gg.GridAbortSignal) ([]GridRow, error) {
			return rows, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.MutateData(GridMutationRequest{
		Kind: gridMutationCreate,
		Rows: []GridRow{
			{ID: "x", Cells: map[string]string{"bogus": "v"}},
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown column")
	}
}

func TestGridOrmDeleteMany(t *testing.T) {
	src, err := newGridOrmDataSource(GridOrmDataSource{
		Columns: testColumns(),
		fetchFn: func(GridOrmQuerySpec, *gg.GridAbortSignal) (gridOrmPage, error) {
			return gridOrmPage{}, nil
		},
		deleteManyFn: func(ids []string, _ *gg.GridAbortSignal) ([]string, error) {
			return ids, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := src.MutateData(GridMutationRequest{
		Kind:   gridMutationDelete,
		rowIDs: []string{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.deletedIDs) != 2 {
		t.Fatalf("deleted = %d, want 2", len(res.deletedIDs))
	}
}

func TestGridOrmDeleteSingle(t *testing.T) {
	src, err := newGridOrmDataSource(GridOrmDataSource{
		Columns: testColumns(),
		fetchFn: func(GridOrmQuerySpec, *gg.GridAbortSignal) (gridOrmPage, error) {
			return gridOrmPage{}, nil
		},
		deleteFn: func(id string, _ *gg.GridAbortSignal) (string, error) {
			return id, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := src.MutateData(GridMutationRequest{
		Kind:   gridMutationDelete,
		rowIDs: []string{"a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.deletedIDs) != 1 {
		t.Fatalf("deleted = %d, want 1", len(res.deletedIDs))
	}
}

func TestGridOrmFilterDedup(t *testing.T) {
	q, err := gridOrmValidateQuery(GridQueryState{
		Filters: []gridFilter{
			{ColID: "name", Op: "contains", Value: "x"},
			{ColID: "name", Op: "contains", Value: "x"},
		},
	}, testColumns())
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Filters) != 1 {
		t.Fatalf("filters = %d, want 1", len(q.Filters))
	}
}

func TestGridOrmBuildSQLQuickFilterDeterministicOrder(t *testing.T) {
	colMap := map[string]gridOrmColumnSpec{
		"z_col": {
			ID:              "z_col",
			dBField:         "z_field",
			QuickFilter:     true,
			Filterable:      true,
			Sortable:        true,
			caseInsensitive: false,
		},
		"a_col": {
			ID:              "a_col",
			dBField:         "a_field",
			QuickFilter:     true,
			Filterable:      true,
			Sortable:        true,
			caseInsensitive: false,
		},
	}
	out, err := gridOrmBuildSQL(GridOrmQuerySpec{
		QuickFilter: "x",
		limit:       10,
		Offset:      0,
	}, colMap)
	if err != nil {
		t.Fatal(err)
	}
	want := "(a_field like ? escape '\\' or z_field like ? escape '\\')"
	if out.whereSQL != want {
		t.Fatalf("WhereSQL = %q, want %q", out.whereSQL, want)
	}
	if len(out.Params) < 2 || out.Params[0] != "%x%" || out.Params[1] != "%x%" {
		t.Fatalf("unexpected quick-filter param ordering: %#v", out.Params)
	}
}

func TestGridOrmDeleteManyDeterministicIDs(t *testing.T) {
	var gotIDs []string
	src, err := newGridOrmDataSource(GridOrmDataSource{
		Columns: testColumns(),
		fetchFn: func(GridOrmQuerySpec, *gg.GridAbortSignal) (gridOrmPage, error) {
			return gridOrmPage{}, nil
		},
		deleteManyFn: func(ids []string, _ *gg.GridAbortSignal) ([]string, error) {
			gotIDs = append([]string(nil), ids...)
			return ids, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.MutateData(GridMutationRequest{
		Kind:   gridMutationDelete,
		rowIDs: []string{"c", "a", "b", "a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(gotIDs) != 3 || gotIDs[0] != "a" || gotIDs[1] != "b" || gotIDs[2] != "c" {
		t.Fatalf("delete IDs = %#v, want [a b c]", gotIDs)
	}
}

func TestGridOrmBuildSQLClampsLimitOffset(t *testing.T) {
	src := testOrmSource(t)
	out, err := src.buildSQL(GridOrmQuerySpec{
		QuickFilter: "x",
		limit:       -5,
		Offset:      -10,
	})
	if err != nil {
		t.Fatal(err)
	}
	n := len(out.Params)
	if n < 2 {
		t.Fatalf("params too short: %#v", out.Params)
	}
	if out.Params[n-2] != "100" || out.Params[n-1] != "0" {
		t.Fatalf("limit/offset params = [%s %s], want [100 0]",
			out.Params[n-2], out.Params[n-1])
	}
}
