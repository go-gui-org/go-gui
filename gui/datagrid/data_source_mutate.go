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

	gg "github.com/go-gui-org/go-gui/gui"
)

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
