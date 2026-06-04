package gui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

var gridOrmDefaultFilterOps = []string{
	"contains", "equals", "starts_with", "ends_with",
}

const (
	gridOrmMaxFilterValueLen = 500
	gridOrmMaxFilterCount    = 100
)

// GridOrmColumnSpec describes a column for ORM-backed queries.
type GridOrmColumnSpec struct {
	ID              string
	DBField         string
	AllowedOps      []string
	normalizedOps   []string // populated by validation
	QuickFilter     bool
	Filterable      bool
	Sortable        bool
	CaseInsensitive bool
}

// GridOrmQuerySpec is the validated query sent to ORM callbacks.
type GridOrmQuerySpec struct {
	QuickFilter string
	Cursor      string
	Sorts       []GridSort
	Filters     []GridFilter
	Limit       int
	Offset      int
}

// GridOrmPage is the result from an ORM fetch callback.
type GridOrmPage struct {
	NextCursor string
	PrevCursor string
	Rows       []GridRow
	RowCount   int // -1 when unknown
	HasMore    bool
}

// GridOrmFetchFn is a callback that fetches a page of ORM data.
type GridOrmFetchFn func(spec GridOrmQuerySpec, signal *GridAbortSignal) (GridOrmPage, error)

// GridOrmCreateFn is a callback that creates rows.
type GridOrmCreateFn func(rows []GridRow, signal *GridAbortSignal) ([]GridRow, error)

// GridOrmUpdateFn is a callback that updates rows.
type GridOrmUpdateFn func(rows []GridRow, edits []GridCellEdit, signal *GridAbortSignal) ([]GridRow, error)

// GridOrmDeleteFn is a callback that deletes a single row.
type GridOrmDeleteFn func(rowID string, signal *GridAbortSignal) (string, error)

// GridOrmDeleteManyFn is a callback that deletes multiple rows.
type GridOrmDeleteManyFn func(rowIDs []string, signal *GridAbortSignal) ([]string, error)

// GridOrmDataSource wraps user-provided ORM callbacks with
// column validation, query normalization, and abort handling.
type GridOrmDataSource struct {
	columnMap      map[string]GridOrmColumnSpec
	FetchFn        GridOrmFetchFn
	CreateFn       GridOrmCreateFn
	UpdateFn       GridOrmUpdateFn
	DeleteFn       GridOrmDeleteFn
	DeleteManyFn   GridOrmDeleteManyFn
	Columns        []GridOrmColumnSpec
	DefaultLimit   int
	SupportsOffset bool
	RowCountKnown  bool
}

// NewGridOrmDataSource validates columns and builds the
// cached column map.
func NewGridOrmDataSource(src GridOrmDataSource) (*GridOrmDataSource, error) {
	if src.FetchFn == nil {
		return nil, errors.New("grid orm: fetch_fn is required")
	}
	colMap, err := gridOrmValidateColumnMap(src.Columns)
	if err != nil {
		return nil, err
	}
	validated := make([]GridOrmColumnSpec, len(src.Columns))
	for i, col := range src.Columns {
		validated[i] = colMap[strings.TrimSpace(col.ID)]
	}
	out := src
	out.Columns = validated
	out.columnMap = colMap
	return &out, nil
}

func (s *GridOrmDataSource) resolvedColumnMap() (map[string]GridOrmColumnSpec, error) {
	if len(s.columnMap) > 0 {
		return s.columnMap, nil
	}
	return gridOrmValidateColumnMap(s.Columns)
}

// Capabilities returns the data capabilities of the ORM source.
func (s *GridOrmDataSource) Capabilities() GridDataCapabilities {
	return GridDataCapabilities{
		SupportsCursorPagination: true,
		SupportsOffsetPagination: s.SupportsOffset,
		SupportsNumberedPages:    s.SupportsOffset,
		RowCountKnown:            s.RowCountKnown,
		SupportsCreate:           s.CreateFn != nil,
		SupportsUpdate:           s.UpdateFn != nil,
		SupportsDelete:           s.DeleteFn != nil || s.DeleteManyFn != nil,
		SupportsBatchDelete:      s.DeleteManyFn != nil,
	}
}

// FetchData retrieves a page of grid data from the ORM source.
func (s *GridOrmDataSource) FetchData(
	req GridDataRequest,
) (GridDataResult, error) {
	if err := gridAbortCheck(req.Signal); err != nil {
		return GridDataResult{}, err
	}
	colMap, err := s.resolvedColumnMap()
	if err != nil {
		return GridDataResult{}, err
	}
	query, err := gridOrmValidateQueryWithMap(req.Query, colMap)
	if err != nil {
		return GridDataResult{}, err
	}
	limit, offset, cursor := gridOrmResolvePage(
		req.Page, s.DefaultLimit)
	page, err := s.FetchFn(GridOrmQuerySpec{
		QuickFilter: query.QuickFilter,
		Sorts:       query.Sorts,
		Filters:     query.Filters,
		Limit:       limit,
		Offset:      offset,
		Cursor:      cursor,
	}, req.Signal)
	if err != nil {
		return GridDataResult{}, err
	}
	if err := gridAbortCheck(req.Signal); err != nil {
		return GridDataResult{}, err
	}
	nextCursor := page.NextCursor
	prevCursor := page.PrevCursor
	if _, ok := req.Page.(GridCursorPageReq); ok {
		if nextCursor == "" && page.HasMore {
			nextCursor = dataGridSourceCursorFromIndex(
				offset + len(page.Rows))
		}
		if prevCursor == "" {
			prevCursor = dataGridSourcePrevCursor(offset, limit)
		}
	}
	return GridDataResult{
		Rows:          page.Rows,
		NextCursor:    nextCursor,
		PrevCursor:    prevCursor,
		RowCount:      page.RowCount,
		HasMore:       page.HasMore,
		ReceivedCount: len(page.Rows),
	}, nil
}

// MutateData applies create/update/delete mutations via the ORM.
func (s *GridOrmDataSource) MutateData(
	req GridMutationRequest,
) (GridMutationResult, error) {
	if err := gridAbortCheck(req.Signal); err != nil {
		return GridMutationResult{}, err
	}
	colMap, err := s.resolvedColumnMap()
	if err != nil {
		return GridMutationResult{}, err
	}
	switch req.Kind {
	case GridMutationCreate:
		if s.CreateFn == nil {
			return GridMutationResult{},
				errors.New("grid orm: create not supported")
		}
		if err := gridOrmValidateMutationColumns(
			req.Rows, nil, colMap); err != nil {
			return GridMutationResult{}, err
		}
		rowsCopy := make([]GridRow, len(req.Rows))
		copy(rowsCopy, req.Rows)
		created, err := s.CreateFn(rowsCopy, req.Signal)
		if err != nil {
			return GridMutationResult{}, err
		}
		if err := gridAbortCheck(req.Signal); err != nil {
			return GridMutationResult{}, err
		}
		return GridMutationResult{Created: created}, nil

	case GridMutationUpdate:
		if s.UpdateFn == nil {
			return GridMutationResult{},
				errors.New("grid orm: update not supported")
		}
		if err := gridOrmValidateMutationColumns(
			req.Rows, req.Edits, colMap); err != nil {
			return GridMutationResult{}, err
		}
		rowsCopy := make([]GridRow, len(req.Rows))
		copy(rowsCopy, req.Rows)
		editsCopy := make([]GridCellEdit, len(req.Edits))
		copy(editsCopy, req.Edits)
		updated, err := s.UpdateFn(
			rowsCopy, editsCopy, req.Signal)
		if err != nil {
			return GridMutationResult{}, err
		}
		if err := gridAbortCheck(req.Signal); err != nil {
			return GridMutationResult{}, err
		}
		return GridMutationResult{Updated: updated}, nil

	case GridMutationDelete:
		idSet := gridDeduplicateRowIDs(req.Rows, req.RowIDs)
		ids := make([]string, 0, len(idSet))
		for k := range idSet {
			ids = append(ids, k)
		}
		slices.Sort(ids)
		if len(ids) == 0 {
			return GridMutationResult{}, nil
		}
		var deletedIDs []string
		if s.DeleteManyFn != nil {
			deletedIDs, err = s.DeleteManyFn(ids, req.Signal)
			if err != nil {
				return GridMutationResult{}, err
			}
		} else if s.DeleteFn != nil {
			out := make([]string, 0, len(ids))
			var deleted string
			for _, rowID := range ids {
				deleted, err = s.DeleteFn(rowID, req.Signal)
				if err != nil {
					return GridMutationResult{}, err
				}
				if err := gridAbortCheck(req.Signal); err != nil {
					return GridMutationResult{}, err
				}
				if deleted != "" {
					out = append(out, deleted)
				}
			}
			deletedIDs = out
		} else {
			return GridMutationResult{},
				errors.New("grid orm: delete not supported")
		}
		if err := gridAbortCheck(req.Signal); err != nil {
			return GridMutationResult{}, err
		}
		return GridMutationResult{DeletedIDs: deletedIDs}, nil
	}
	return GridMutationResult{},
		errors.New("grid orm: unknown mutation kind")
}

// GridOrmValidateQuery validates a query against columns.
func GridOrmValidateQuery(
	query GridQueryState, columns []GridOrmColumnSpec,
) (GridQueryState, error) {
	colMap, err := gridOrmValidateColumnMap(columns)
	if err != nil {
		return GridQueryState{}, err
	}
	return gridOrmValidateQueryWithMap(query, colMap)
}

func gridOrmValidateQueryWithMap(
	query GridQueryState,
	colMap map[string]GridOrmColumnSpec,
) (GridQueryState, error) {
	if len(query.QuickFilter) > gridOrmMaxFilterValueLen {
		return GridQueryState{}, fmt.Errorf(
			"grid orm: quick_filter exceeds max length (%d)",
			gridOrmMaxFilterValueLen)
	}
	if len(query.Filters) > gridOrmMaxFilterCount {
		return GridQueryState{}, fmt.Errorf(
			"grid orm: too many filters (%d > %d)",
			len(query.Filters), gridOrmMaxFilterCount)
	}
	var sorts []GridSort
	for _, s := range query.Sorts {
		col, ok := colMap[s.ColID]
		if !ok || !col.Sortable {
			continue
		}
		sorts = append(sorts, GridSort{
			ColID: s.ColID, Dir: s.Dir,
		})
	}
	var filters []GridFilter
	for _, f := range query.Filters {
		if len(f.Value) > gridOrmMaxFilterValueLen {
			return GridQueryState{}, fmt.Errorf(
				"grid orm: filter value exceeds max length (%d)",
				gridOrmMaxFilterValueLen)
		}
		col, ok := colMap[f.ColID]
		if !ok || !col.Filterable {
			continue
		}
		op := gridOrmNormalizeFilterOp(f.Op)
		if !gridOrmColumnAllowsFilterOp(col, op) {
			continue
		}
		isDup := false
		for _, existing := range filters {
			if existing.ColID == f.ColID &&
				existing.Op == op &&
				existing.Value == f.Value {
				isDup = true
				break
			}
		}
		if isDup {
			continue
		}
		filters = append(filters, GridFilter{
			ColID: f.ColID, Op: op, Value: f.Value,
		})
	}
	return GridQueryState{
		Sorts:       sorts,
		Filters:     filters,
		QuickFilter: query.QuickFilter,
	}, nil
}

func gridOrmResolvePage(
	page GridPageRequest, configuredLimit int,
) (limit, offset int, cursor string) {
	defLimit := max(1, min(dataGridSourceMaxPageLimit,
		nonZero(configuredLimit, 100)))
	switch p := page.(type) {
	case GridCursorPageReq:
		limit = max(1, min(dataGridSourceMaxPageLimit,
			nonZero(p.Limit, defLimit)))
		offset = max(0,
			dataGridSourceCursorToIndex(p.Cursor))
		cursor = p.Cursor
	case GridOffsetPageReq:
		offset = max(0, p.StartIndex)
		limit = max(1, min(dataGridSourceMaxPageLimit,
			nonZero(p.EndIndex-p.StartIndex, defLimit)))
	default:
		limit = defLimit
	}
	return
}

func gridOrmValidateColumnMap(
	columns []GridOrmColumnSpec,
) (map[string]GridOrmColumnSpec, error) {
	out := make(map[string]GridOrmColumnSpec, len(columns))
	for _, col := range columns {
		id := strings.TrimSpace(col.ID)
		if id == "" {
			return nil, errors.New(
				"grid orm: column id is required")
		}
		dbField := strings.TrimSpace(col.DBField)
		if dbField == "" {
			return nil, fmt.Errorf(
				"grid orm: column %q requires db_field", id)
		}
		if !GridOrmValidDBField(dbField) {
			return nil, fmt.Errorf(
				"grid orm: column %q has invalid db_field: %s",
				id, dbField)
		}
		if _, exists := out[id]; exists {
			return nil, fmt.Errorf(
				"grid orm: duplicate column id: %s", id)
		}
		normOps := make([]string, len(col.AllowedOps))
		for i, rawOp := range col.AllowedOps {
			normOps[i] = gridOrmNormalizeFilterOp(rawOp)
		}
		validated := col
		validated.ID = id
		validated.DBField = dbField
		validated.normalizedOps = normOps
		out[id] = validated
	}
	return out, nil
}

func gridOrmNormalizeFilterOp(op string) string {
	normalized := strings.ToLower(strings.TrimSpace(op))
	if normalized == "" {
		return "contains"
	}
	return normalized
}

func gridOrmColumnAllowsFilterOp(
	col GridOrmColumnSpec, op string,
) bool {
	if op == "" {
		return false
	}
	if len(col.normalizedOps) > 0 {
		return slices.Contains(col.normalizedOps, op)
	}
	return slices.Contains(gridOrmDefaultFilterOps, op)
}

func gridOrmValidateMutationColumns(
	rows []GridRow, edits []GridCellEdit,
	colMap map[string]GridOrmColumnSpec,
) error {
	if len(colMap) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	for _, row := range rows {
		for colID := range row.Cells {
			if seen[colID] {
				continue
			}
			if _, ok := colMap[colID]; !ok {
				return fmt.Errorf(
					"grid orm: unknown column id: %s", colID)
			}
			seen[colID] = true
		}
	}
	for _, edit := range edits {
		if seen[edit.ColID] {
			continue
		}
		if _, ok := colMap[edit.ColID]; !ok {
			return fmt.Errorf(
				"grid orm: unknown column id: %s", edit.ColID)
		}
		seen[edit.ColID] = true
	}
	return nil
}

// GridOrmSQLBuilder holds SQL fragments built from a query spec.
type GridOrmSQLBuilder struct {
	WhereSQL  string
	OrderSQL  string
	LimitSQL  string
	OffsetSQL string
	Params    []string
}

// BuildSQL validates the query spec against the source's
// columns and builds SQL fragments.
func (s *GridOrmDataSource) BuildSQL(
	spec GridOrmQuerySpec,
) (GridOrmSQLBuilder, error) {
	colMap, err := s.resolvedColumnMap()
	if err != nil {
		return GridOrmSQLBuilder{}, err
	}
	return GridOrmBuildSQL(spec, colMap)
}

// GridOrmBuildSQL builds SQL fragments from a query spec and
// pre-validated column map. No SQL keywords (WHERE, ORDER BY)
// are included.
func GridOrmBuildSQL(
	spec GridOrmQuerySpec,
	colMap map[string]GridOrmColumnSpec,
) (GridOrmSQLBuilder, error) {
	query, err := gridOrmValidateQueryWithMap(GridQueryState{
		QuickFilter: spec.QuickFilter,
		Sorts:       spec.Sorts,
		Filters:     spec.Filters,
	}, colMap)
	if err != nil {
		return GridOrmSQLBuilder{}, err
	}
	var params []string
	var whereParts []string
	qf := gridOrmBuildQuickFilter(
		query.QuickFilter, colMap, &params)
	if qf != "" {
		whereParts = append(whereParts, qf)
	}
	for _, f := range query.Filters {
		col, ok := colMap[f.ColID]
		if !ok {
			continue
		}
		clause := gridOrmBuildFilterClause(
			col.DBField, f.Op, f.Value,
			col.CaseInsensitive, &params)
		whereParts = append(whereParts, clause)
	}
	order := gridOrmBuildOrder(query.Sorts, colMap)
	limit := max(1, min(dataGridSourceMaxPageLimit, nonZero(spec.Limit, 100)))
	offset := max(0, spec.Offset)
	params = append(params,
		strconv.Itoa(limit),
		strconv.Itoa(offset))
	return GridOrmSQLBuilder{
		WhereSQL:  strings.Join(whereParts, " and "),
		OrderSQL:  order,
		LimitSQL:  "limit ?",
		OffsetSQL: "offset ?",
		Params:    params,
	}, nil
}

// GridOrmEscapeLike escapes SQL LIKE wildcard characters
// (%, _) so they match literally.
func GridOrmEscapeLike(s string) string {
	if !strings.ContainsAny(s, `%_\`) {
		return s
	}
	r := strings.ReplaceAll(s, `\`, `\\`)
	r = strings.ReplaceAll(r, `%`, `\%`)
	r = strings.ReplaceAll(r, `_`, `\_`)
	return r
}

func gridOrmBuildQuickFilter(
	needle string,
	columns map[string]GridOrmColumnSpec,
	params *[]string,
) string {
	trimmed := strings.TrimSpace(needle)
	if trimmed == "" {
		return ""
	}
	lowerNeedle := strings.ToLower(trimmed)
	escapedLower := GridOrmEscapeLike(lowerNeedle)
	escapedTrimmed := GridOrmEscapeLike(trimmed)
	var orParts []string
	keys := make([]string, 0, len(columns))
	for k := range columns {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		col := columns[k]
		if !col.QuickFilter {
			continue
		}
		if col.CaseInsensitive {
			orParts = append(orParts,
				fmt.Sprintf("lower(%s) like ? escape '\\'",
					col.DBField))
			*params = append(*params,
				"%"+escapedLower+"%")
		} else {
			orParts = append(orParts,
				fmt.Sprintf("%s like ? escape '\\'",
					col.DBField))
			*params = append(*params,
				"%"+escapedTrimmed+"%")
		}
	}
	if len(orParts) == 0 {
		return ""
	}
	return "(" + strings.Join(orParts, " or ") + ")"
}

func gridOrmBuildFilterClause(
	dbField, op, value string,
	caseInsensitive bool, params *[]string,
) string {
	targetField := dbField
	if caseInsensitive {
		targetField = "lower(" + dbField + ")"
	}
	targetValue := value
	if caseInsensitive {
		targetValue = strings.ToLower(value)
	}
	var clause, param string
	switch op {
	case "equals":
		clause = targetField + " = ?"
		param = targetValue
	case "starts_with":
		clause = targetField + " like ? escape '\\'"
		param = GridOrmEscapeLike(targetValue) + "%"
	case "ends_with":
		clause = targetField + " like ? escape '\\'"
		param = "%" + GridOrmEscapeLike(targetValue)
	default:
		clause = targetField + " like ? escape '\\'"
		param = "%" + GridOrmEscapeLike(targetValue) + "%"
	}
	*params = append(*params, param)
	return clause
}

func gridOrmBuildOrder(
	sorts []GridSort,
	colMap map[string]GridOrmColumnSpec,
) string {
	var parts []string
	for _, s := range sorts {
		col, ok := colMap[s.ColID]
		if !ok || !col.Sortable || !GridOrmValidDBField(col.DBField) {
			continue
		}
		dir := "asc"
		if s.Dir == GridSortDesc {
			dir = "desc"
		}
		parts = append(parts,
			col.DBField+" "+dir)
	}
	return strings.Join(parts, ", ")
}

// GridOrmValidDBField checks that a db_field contains only
// alphanumeric chars, underscores, and at most one dot.
// Must start with a letter or underscore.
func GridOrmValidDBField(field string) bool {
	if field == "" {
		return false
	}
	first := field[0]
	if (first < 'a' || first > 'z') &&
		(first < 'A' || first > 'Z') && first != '_' {
		return false
	}
	dotCount := 0
	for i := 1; i < len(field); i++ {
		c := field[i]
		if c == '.' {
			dotCount++
			if dotCount > 1 || i == len(field)-1 {
				return false
			}
			continue
		}
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}
