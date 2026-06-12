package datagrid

import (
	"cmp"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	gg "github.com/go-gui-org/go-gui/gui"
)

const dataGridSourceMaxPageLimit = 10000

// GridDataRequest is the request payload for FetchData.
type GridDataRequest struct {
	Page      GridPageRequest
	Signal    *gg.GridAbortSignal
	Query     GridQueryState
	GridID    string
	RequestID uint64
}

// GridDataResult is the response from FetchData.
type GridDataResult struct {
	NextCursor    string
	PrevCursor    string
	Rows          []GridRow
	RowCount      int // -1 when unknown
	ReceivedCount int
	HasMore       bool
}

// GridDataCapabilities describes what a data source supports.
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

// GridMutationRequest is the request payload for MutateData.
type GridMutationRequest struct {
	Signal    *gg.GridAbortSignal
	Query     GridQueryState
	GridID    string
	Rows      []GridRow
	RowIDs    []string
	Edits     []GridCellEdit
	RequestID uint64
	Kind      GridMutationKind
}

// GridMutationResult is the response from MutateData.
type GridMutationResult struct {
	Errors     map[string]string
	Created    []GridRow
	Updated    []GridRow
	DeletedIDs []string
	FailedIDs  []string
	RowCount   int // -1 when unknown
}

// DataGridDataSource is the interface for grid data providers.
//
//nolint:revive // DataGrid prefix intentional
type DataGridDataSource interface {
	Capabilities() GridDataCapabilities
	FetchData(req GridDataRequest) (GridDataResult, error)
	MutateData(req GridMutationRequest) (GridMutationResult, error)
}

// InMemoryDataSource implements DataGridDataSource using an
// in-memory row slice.
type InMemoryDataSource struct {
	Rows           []GridRow
	DefaultLimit   int
	LatencyMs      int
	mu             sync.RWMutex
	RowCountKnown  bool
	SupportsCursor bool
	SupportsOffset bool
}

// NewInMemoryDataSource creates an InMemoryDataSource with
// sensible defaults.
func NewInMemoryDataSource(rows []GridRow) *InMemoryDataSource {
	return &InMemoryDataSource{
		Rows:           rows,
		DefaultLimit:   100,
		RowCountKnown:  true,
		SupportsCursor: true,
		SupportsOffset: true,
	}
}

// Capabilities returns the supported operations for in-memory data.
func (s *InMemoryDataSource) Capabilities() GridDataCapabilities {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return GridDataCapabilities{
		SupportsCursorPagination: s.SupportsCursor,
		SupportsOffsetPagination: s.SupportsOffset,
		SupportsNumberedPages:    s.SupportsOffset,
		RowCountKnown:            s.RowCountKnown,
		SupportsCreate:           true,
		SupportsUpdate:           true,
		SupportsDelete:           true,
		SupportsBatchDelete:      true,
	}
}

// FetchData returns paginated rows from in-memory storage.
func (s *InMemoryDataSource) FetchData(req GridDataRequest) (GridDataResult, error) {
	if err := dataGridSourceSleepWithAbort(req.Signal, s.LatencyMs); err != nil {
		return GridDataResult{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]GridRow, len(s.Rows))
	copy(rows, s.Rows)
	defaultLimit := s.DefaultLimit
	rowCountKnown := s.RowCountKnown
	// latencyMs=0: sleep already applied above; inner call
	// degenerates to abort-check only.
	return dataGridSourceInMemoryFetch(
		rows, defaultLimit, 0, rowCountKnown, req)
}

// MutateData applies create/update/delete mutations to in-memory rows.
func (s *InMemoryDataSource) MutateData(req GridMutationRequest) (GridMutationResult, error) {
	if err := dataGridSourceSleepWithAbort(req.Signal, s.LatencyMs); err != nil {
		return GridMutationResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return dataGridSourceInMemoryMutate(
		&s.Rows, 0, s.RowCountKnown, req)
}

func dataGridSourceInMemoryFetch(
	rows []GridRow, defaultLimit, latencyMs int,
	rowCountKnown bool, req GridDataRequest,
) (GridDataResult, error) {
	if err := dataGridSourceSleepWithAbort(req.Signal, latencyMs); err != nil {
		return GridDataResult{}, err
	}
	filtered := dataGridSourceApplyQuery(rows, req.Query)
	limit := max(1, min(dataGridSourceMaxPageLimit,
		nonZero(defaultLimit, 100)))
	var start, end int
	switch p := req.Page.(type) {
	case GridCursorPageReq:
		s := max(0, min(len(filtered),
			dataGridSourceCursorToIndex(p.Cursor)))
		chunk := max(1, min(dataGridSourceMaxPageLimit,
			nonZero(p.Limit, limit)))
		start, end = s, min(len(filtered), s+chunk)
	case GridOffsetPageReq:
		start, end = dataGridSourceOffsetBounds(
			p.StartIndex, p.EndIndex, len(filtered), limit)
	default:
		start, end = 0, min(len(filtered), limit)
	}
	page := make([]GridRow, end-start)
	copy(page, filtered[start:end])
	if err := gridAbortCheck(req.Signal); err != nil {
		return GridDataResult{}, err
	}
	_, isCursor := req.Page.(GridCursorPageReq)
	rc := -1
	if rowCountKnown {
		rc = len(filtered)
	}
	var nextCursor, prevCursor string
	if isCursor && end < len(filtered) {
		nextCursor = dataGridSourceCursorFromIndex(end)
	}
	if isCursor {
		prevCursor = dataGridSourcePrevCursor(start, end-start)
	}
	return GridDataResult{
		Rows:          page,
		NextCursor:    nextCursor,
		PrevCursor:    prevCursor,
		RowCount:      rc,
		HasMore:       end < len(filtered),
		ReceivedCount: len(page),
	}, nil
}

func dataGridSourceInMemoryMutate(
	rows *[]GridRow, latencyMs int,
	rowCountKnown bool, req GridMutationRequest,
) (GridMutationResult, error) {
	if err := dataGridSourceSleepWithAbort(req.Signal, latencyMs); err != nil {
		return GridMutationResult{}, err
	}
	work := make([]GridRow, len(*rows))
	copy(work, *rows)
	result, err := dataGridSourceApplyMutation(
		&work, req.Kind, req.Rows, req.RowIDs, req.Edits)
	if err != nil {
		return GridMutationResult{}, err
	}
	if err := gridAbortCheck(req.Signal); err != nil {
		return GridMutationResult{}, err
	}
	*rows = work
	rc := -1
	if rowCountKnown {
		rc = len(*rows)
	}
	return GridMutationResult{
		Created:    result.created,
		Updated:    result.updated,
		DeletedIDs: result.deletedIDs,
		RowCount:   rc,
	}, nil
}

// dataGridSourceOffsetBounds clamps start/end to [0,total]
// and falls back to defaultLimit when the range is empty.
func dataGridSourceOffsetBounds(
	startIndex, endIndex, total, defaultLimit int,
) (int, int) {
	start := max(0, min(total, startIndex))
	end := max(start, min(total, endIndex))
	if end <= start {
		end = min(total, start+defaultLimit)
	}
	return start, end
}

var errGridAborted = errors.New("grid: request aborted")

func gridAbortCheck(signal *gg.GridAbortSignal) error {
	if signal.IsAborted() {
		return errGridAborted
	}
	return nil
}

func dataGridSourceSleepWithAbort(
	signal *gg.GridAbortSignal, ms int,
) error {
	if ms <= 0 {
		return gridAbortCheck(signal)
	}
	// Cap to prevent near-infinite sleep from a misconfigured
	// LatencyMs. 60s is more than any reasonable test simulation.
	if ms > 60_000 {
		ms = 60_000
	}
	remaining := ms
	for remaining > 0 {
		if err := gridAbortCheck(signal); err != nil {
			return err
		}
		step := min(remaining, 20)
		time.Sleep(time.Duration(step) * time.Millisecond)
		remaining -= step
	}
	return gridAbortCheck(signal)
}

func dataGridSourceCursorFromIndex(index int) string {
	return "i:" + strconv.Itoa(max(0, index))
}

func dataGridSourcePrevCursor(start, pageSize int) string {
	if start <= 0 {
		return ""
	}
	return dataGridSourceCursorFromIndex(
		max(0, start-pageSize))
}

func dataGridSourceCursorToIndex(cursor string) int {
	idx, ok := dataGridSourceCursorToIndexOpt(cursor)
	if !ok {
		return 0
	}
	return idx
}

func dataGridSourceCursorToIndexOpt(cursor string) (int, bool) {
	trimmed := strings.TrimSpace(cursor)
	if trimmed == "" {
		return 0, true
	}
	if strings.HasPrefix(trimmed, "i:") {
		val := trimmed[2:]
		if !dataGridSourceIsDecimal(val) {
			return 0, false
		}
		n, _ := strconv.Atoi(val)
		return max(0, n), true
	}
	if !dataGridSourceIsDecimal(trimmed) {
		return 0, false
	}
	n, _ := strconv.Atoi(trimmed)
	return max(0, n), true
}

func dataGridSourceIsDecimal(input string) bool {
	if input == "" {
		return false
	}
	for i := range len(input) {
		if input[i] < '0' || input[i] > '9' {
			return false
		}
	}
	return true
}

// dataGridSourceApplyQuery filters and sorts rows in memory.
func dataGridSourceApplyQuery(
	rows []GridRow, query GridQueryState,
) []GridRow {
	if query.QuickFilter == "" && len(query.Filters) == 0 &&
		len(query.Sorts) == 0 {
		return rows
	}
	hasFilters := query.QuickFilter != "" ||
		len(query.Filters) > 0
	var filtered []GridRow
	if hasFilters {
		needle := strings.ToLower(query.QuickFilter)
		lowered := make([]gridFilterLowered, len(query.Filters))
		for i, f := range query.Filters {
			lowered[i] = gridFilterLowered{
				colID: f.ColID,
				op:    f.Op,
				value: strings.ToLower(f.Value),
			}
		}
		filtered = make([]GridRow, 0, len(rows))
		for _, row := range rows {
			if dataGridSourceRowMatchesQuery(
				row, needle, lowered) {
				filtered = append(filtered, row)
			}
		}
	} else {
		filtered = rows
	}
	if len(query.Sorts) == 0 {
		return filtered
	}
	n := len(filtered)
	if n <= 1 {
		return filtered
	}
	idxs := make([]int, n)
	for i := range idxs {
		idxs[i] = i
	}
	if len(query.Sorts) == 1 {
		s0 := query.Sorts[0]
		keys := make([]string, n)
		for i, row := range filtered {
			keys[i] = row.Cells[s0.ColID]
		}
		dir := 1
		if s0.Dir == GridSortDesc {
			dir = -1
		}
		slices.SortFunc(idxs, func(a, b int) int {
			ka, kb := keys[a], keys[b]
			if ka == kb {
				return cmp.Compare(a, b)
			}
			return dir * cmp.Compare(ka, kb)
		})
	} else {
		sorts := query.Sorts
		keyCols := make([][]string, len(sorts))
		for si, s := range sorts {
			col := make([]string, n)
			for i, row := range filtered {
				col[i] = row.Cells[s.ColID]
			}
			keyCols[si] = col
		}
		slices.SortFunc(idxs, func(a, b int) int {
			for si, s := range sorts {
				ka := keyCols[si][a]
				kb := keyCols[si][b]
				if c := cmp.Compare(ka, kb); c != 0 {
					if s.Dir != GridSortAsc {
						return -c
					}
					return c
				}
			}
			return cmp.Compare(a, b)
		})
	}
	result := make([]GridRow, n)
	for i, idx := range idxs {
		result[i] = filtered[idx]
	}
	return result
}

type gridFilterLowered struct {
	colID string
	op    string
	value string
}

func dataGridSourceRowMatchesQuery(
	row GridRow, needle string, filters []gridFilterLowered,
) bool {
	if needle != "" {
		matched := false
		for _, value := range row.Cells {
			if gridContainsLower(value, needle) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, f := range filters {
		cell := row.Cells[f.colID]
		var matched bool
		switch f.op {
		case "equals":
			matched = gridEqualsLower(cell, f.value)
		case "starts_with":
			matched = gridStartsWithLower(cell, f.value)
		case "ends_with":
			matched = gridEndsWithLower(cell, f.value)
		default:
			matched = gridContainsLower(cell, f.value)
		}
		if !matched {
			return false
		}
	}
	return true
}

// gridContainsLower checks haystack.toLower().contains(needle)
// without allocating. needle must already be lowered.
func gridContainsLower(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	limit := len(haystack) - len(needle)
	for i := 0; i <= limit; i++ {
		found := true
		for j := range len(needle) {
			if gg.ASCIILower(haystack[i+j]) != needle[j] {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}

// gridEqualsLower checks haystack.toLower() == needle
// without allocating. needle must already be lowered.
func gridEqualsLower(haystack, needle string) bool {
	if len(haystack) != len(needle) {
		return false
	}
	for i := range len(haystack) {
		if gg.ASCIILower(haystack[i]) != needle[i] {
			return false
		}
	}
	return true
}

// gridStartsWithLower checks haystack.toLower().hasPrefix(needle)
// without allocating. needle must already be lowered.
func gridStartsWithLower(haystack, needle string) bool {
	if len(haystack) < len(needle) {
		return false
	}
	for i := range len(needle) {
		if gg.ASCIILower(haystack[i]) != needle[i] {
			return false
		}
	}
	return true
}

// gridEndsWithLower checks haystack.toLower().hasSuffix(needle)
// without allocating. needle must already be lowered.
func gridEndsWithLower(haystack, needle string) bool {
	if len(haystack) < len(needle) {
		return false
	}
	off := len(haystack) - len(needle)
	for i := range len(needle) {
		if gg.ASCIILower(haystack[i+off]) != needle[i] {
			return false
		}
	}
	return true
}

// gridQuerySignature returns a stable FNV-1a 64-bit hash of
// the query state. Filters are sorted by col_id for order
// independence.
func gridQuerySignature(query GridQueryState) uint64 {
	h := gg.Fnv64Offset
	h = gg.Fnv64Str(h, query.QuickFilter)
	h = gg.Fnv64Byte(h, '|')
	h = gg.Fnv64Byte(h, 's')
	for _, s := range query.Sorts {
		h = gg.Fnv64Byte(h, 0x1e)
		h = gg.Fnv64Str(h, s.ColID)
		if s.Dir == GridSortDesc {
			h = gg.Fnv64Byte(h, 'd')
		} else {
			h = gg.Fnv64Byte(h, 'a')
		}
	}
	h = gg.Fnv64Byte(h, '|')
	h = gg.Fnv64Byte(h, 'f')
	filters := query.Filters
	if len(filters) <= 1 {
		for _, f := range filters {
			h = gridHashFilter(h, f)
		}
		return h
	}
	idxs := make([]int, len(filters))
	for i := range idxs {
		idxs[i] = i
	}
	slices.SortFunc(idxs, func(a, b int) int {
		fa, fb := filters[a], filters[b]
		if c := cmp.Compare(fa.ColID, fb.ColID); c != 0 {
			return c
		}
		if c := cmp.Compare(fa.Op, fb.Op); c != 0 {
			return c
		}
		return cmp.Compare(fa.Value, fb.Value)
	})
	for _, i := range idxs {
		h = gridHashFilter(h, filters[i])
	}
	return h
}

// zeroPadHex16 formats a uint64 as a zero-padded 16-char
// lowercase hex string, equivalent to fmt.Sprintf("%016x", v).
func zeroPadHex16(v uint64) string {
	s := strconv.FormatUint(v, 16)
	const pad = "0000000000000000" // 16 zeros
	if len(s) < 16 {
		s = pad[:16-len(s)] + s
	}
	return s
}

func gridHashFilter(h uint64, f GridFilter) uint64 {
	hash := gg.Fnv64Byte(h, 0x1e)
	hash = gg.Fnv64Str(hash, f.ColID)
	hash = gg.Fnv64Byte(hash, 0x1f)
	hash = gg.Fnv64Str(hash, f.Op)
	hash = gg.Fnv64Byte(hash, 0x1f)
	hash = gg.Fnv64Str(hash, f.Value)
	return hash
}

type gridMutationApplyResult struct {
	created    []GridRow
	updated    []GridRow
	deletedIDs []string
}

func dataGridSourceApplyMutation(
	rows *[]GridRow, kind GridMutationKind,
	reqRows []GridRow, reqRowIDs []string,
	edits []GridCellEdit,
) (gridMutationApplyResult, error) {
	switch kind {
	case GridMutationCreate:
		return dataGridSourceApplyCreate(rows, reqRows)
	case GridMutationUpdate:
		return dataGridSourceApplyUpdate(rows, reqRows, edits)
	case GridMutationDelete:
		return dataGridSourceApplyDelete(rows, reqRows, reqRowIDs)
	}
	return gridMutationApplyResult{}, errors.New(
		"grid: unknown mutation kind")
}

func dataGridSourceApplyCreate(
	rows *[]GridRow, reqRows []GridRow,
) (gridMutationApplyResult, error) {
	if len(reqRows) == 0 {
		return gridMutationApplyResult{}, nil
	}
	existing := make(map[string]bool, len(*rows))
	for idx, row := range *rows {
		existing[dataGridRowID(row, idx)] = true
	}
	created := make([]GridRow, 0, len(reqRows))
	for _, row := range reqRows {
		nextID, err := dataGridSourceNextCreateRowID(
			*rows, existing, row.ID)
		if err != nil {
			return gridMutationApplyResult{}, err
		}
		cells := make(map[string]string, len(row.Cells))
		maps.Copy(cells, row.Cells)
		nextRow := GridRow{ID: nextID, Cells: cells}
		*rows = append(*rows, nextRow)
		existing[nextID] = true
		created = append(created, nextRow)
	}
	return gridMutationApplyResult{created: created}, nil
}

func dataGridSourceApplyUpdate(
	rows *[]GridRow, reqRows []GridRow, edits []GridCellEdit,
) (gridMutationApplyResult, error) {
	updatedIDs := make(map[string]bool)
	editsByRow := make(map[string][]GridCellEdit)
	for _, edit := range edits {
		if edit.RowID == "" {
			return gridMutationApplyResult{},
				errors.New("grid: row id is required")
		}
		if edit.ColID == "" {
			return gridMutationApplyResult{},
				errors.New("grid: edit has empty col id")
		}
		editsByRow[edit.RowID] = append(
			editsByRow[edit.RowID], edit)
	}
	updated := make([]GridRow, 0,
		len(reqRows)+len(editsByRow))
	rowIdx := make(map[string]int, len(*rows))
	for idx, row := range *rows {
		rowIdx[dataGridRowID(row, idx)] = idx
	}
	for _, reqRow := range reqRows {
		if reqRow.ID == "" {
			return gridMutationApplyResult{},
				errors.New("grid: row id is required")
		}
		idx, ok := rowIdx[reqRow.ID]
		if !ok {
			return gridMutationApplyResult{},
				fmt.Errorf("grid: update row not found: %s",
					reqRow.ID)
		}
		cells := make(map[string]string,
			len((*rows)[idx].Cells))
		maps.Copy(cells, (*rows)[idx].Cells)
		maps.Copy(cells, reqRow.Cells)
		if rowEdits, hasEdits := editsByRow[reqRow.ID]; hasEdits {
			for _, edit := range rowEdits {
				cells[edit.ColID] = edit.Value
			}
		}
		(*rows)[idx] = GridRow{ID: (*rows)[idx].ID, Cells: cells}
		updated = append(updated, (*rows)[idx])
		updatedIDs[reqRow.ID] = true
	}
	pendingIDs := make([]string, 0, len(editsByRow))
	for rowID := range editsByRow {
		if updatedIDs[rowID] {
			continue
		}
		pendingIDs = append(pendingIDs, rowID)
	}
	slices.Sort(pendingIDs)
	for _, rowID := range pendingIDs {
		rowEdits := editsByRow[rowID]
		idx, ok := rowIdx[rowID]
		if !ok {
			return gridMutationApplyResult{},
				fmt.Errorf("grid: edit row not found: %s", rowID)
		}
		cells := make(map[string]string,
			len((*rows)[idx].Cells))
		maps.Copy(cells, (*rows)[idx].Cells)
		for _, edit := range rowEdits {
			cells[edit.ColID] = edit.Value
		}
		(*rows)[idx] = GridRow{
			ID: (*rows)[idx].ID, Cells: cells,
		}
		updated = append(updated, (*rows)[idx])
	}
	return gridMutationApplyResult{updated: updated}, nil
}

func dataGridSourceApplyDelete(
	rows *[]GridRow, reqRows []GridRow, reqRowIDs []string,
) (gridMutationApplyResult, error) {
	idSet := gridDeduplicateRowIDs(reqRows, reqRowIDs)
	if len(idSet) == 0 {
		return gridMutationApplyResult{}, nil
	}
	kept := make([]GridRow, 0, len(*rows))
	deletedIDs := make([]string, 0, len(idSet))
	for idx, row := range *rows {
		rowID := dataGridRowID(row, idx)
		if idSet[rowID] {
			deletedIDs = append(deletedIDs, rowID)
			continue
		}
		kept = append(kept, row)
	}
	*rows = kept
	return gridMutationApplyResult{deletedIDs: deletedIDs}, nil
}

// gridDeduplicateRowIDs collects unique non-empty IDs from
// GridRow.ID values and raw ID strings.
func gridDeduplicateRowIDs(
	rows []GridRow, rowIDs []string,
) map[string]bool {
	seen := make(map[string]bool)
	for _, row := range rows {
		if row.ID != "" {
			seen[row.ID] = true
		}
	}
	for _, rowID := range rowIDs {
		id := strings.TrimSpace(rowID)
		if id != "" {
			seen[id] = true
		}
	}
	return seen
}

// dataGridRowID is in view_data_grid.go (auto-hash fallback).

func dataGridSourceNextCreateRowID(
	rows []GridRow, existing map[string]bool, preferredID string,
) (string, error) {
	id := strings.TrimSpace(preferredID)
	if id != "" && !existing[id] {
		return id, nil
	}
	maxID := len(rows) + 1000
	next := len(rows) + 1
	for next <= maxID {
		candidate := strconv.Itoa(next)
		if !existing[candidate] {
			return candidate, nil
		}
		next++
	}
	// Numeric range exhausted; try random hex IDs.
	for range 10 {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return "", fmt.Errorf(
				"grid: random id generation failed: %w", err)
		}
		candidate := fmt.Sprintf("__gen_%016x",
			binary.BigEndian.Uint64(buf[:]))
		if !existing[candidate] {
			return candidate, nil
		}
	}
	return "", errors.New("grid: unable to generate unique row id")
}

func nonZero(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}
