package datagrid

import (
	"maps"
	"strings"

	gg "github.com/go-gui-org/go-gui/gui"
)

// dataGridEffectiveColumns resolves the final visible column
// list: apply columnOrder (fallback to declaration order),
// filter hidden columns, ensure at least one column remains,
// then partition into [left-pinned, unpinned, right-pinned].
func dataGridEffectiveColumns(columns []GridColumnCfg, columnOrder []string, hiddenColumnIDs map[string]bool) []GridColumnCfg {
	if len(columns) == 0 {
		return nil
	}
	order, byID := dataGridColumnOrderAndMap(columns, columnOrder)
	ordered := make([]GridColumnCfg, 0, len(columns))
	for _, id := range order {
		if hiddenColumnIDs[id] {
			continue
		}
		col, ok := byID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, col)
	}
	if len(ordered) == 0 {
		for _, id := range order {
			if col, ok := byID[id]; ok {
				ordered = append(ordered, col)
				break
			}
		}
	}
	return dataGridPartitionPins(ordered)
}

// dataGridColumnOrderAndMap builds the normalized column
// order list and the id→column map in a single pass.
func dataGridColumnOrderAndMap(columns []GridColumnCfg, columnOrder []string) ([]string, map[string]GridColumnCfg) {
	byID := make(map[string]GridColumnCfg, len(columns))
	colIDs := map[string]bool{}
	for _, col := range columns {
		if col.ID != "" {
			colIDs[col.ID] = true
			byID[col.ID] = col
		}
	}
	seen := map[string]bool{}
	order := make([]string, 0, len(columns))
	for _, id := range columnOrder {
		if id == "" || seen[id] {
			continue
		}
		if colIDs[id] {
			seen[id] = true
			order = append(order, id)
		}
	}
	for _, col := range columns {
		if col.ID == "" || seen[col.ID] {
			continue
		}
		seen[col.ID] = true
		order = append(order, col.ID)
	}
	return order, byID
}

func dataGridPartitionPins(columns []GridColumnCfg) []GridColumnCfg {
	var left, center, right []GridColumnCfg
	for _, col := range columns {
		switch col.Pin {
		case GridColumnPinLeft:
			left = append(left, col)
		case GridColumnPinRight:
			right = append(right, col)
		default:
			center = append(center, col)
		}
	}
	merged := make([]GridColumnCfg, 0, len(columns))
	merged = append(merged, left...)
	merged = append(merged, center...)
	merged = append(merged, right...)
	return merged
}

func dataGridColumnNextPin(pin GridColumnPin) GridColumnPin {
	switch pin {
	case GridColumnPinNone:
		return GridColumnPinLeft
	case GridColumnPinLeft:
		return GridColumnPinRight
	default:
		return GridColumnPinNone
	}
}

// dataGridColumnOrderMove moves colID in order by delta
// (-1 left, +1 right). Returns a new slice.
func dataGridColumnOrderMove(order []string, colID string, delta int) []string {
	if len(order) == 0 || delta == 0 {
		return order
	}
	idx := -1
	for i, id := range order {
		if id == colID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return order
	}
	target := max(0, min(len(order)-1, idx+delta))
	if target == idx {
		return order
	}
	next := make([]string, len(order))
	copy(next, order)
	val := next[idx]
	// Remove at idx.
	next = append(next[:idx], next[idx+1:]...)
	// Insert at target.
	next = append(next[:target], append([]string{val}, next[target:]...)...)
	return next
}

func dataGridVisibleColumnCount(columns []GridColumnCfg, hidden map[string]bool) int {
	count := 0
	for _, col := range columns {
		if col.ID == "" || hidden[col.ID] {
			continue
		}
		count++
	}
	return count
}

func dataGridNextHiddenColumns(hidden map[string]bool, colID string, columns []GridColumnCfg) map[string]bool {
	next := make(map[string]bool, len(hidden))
	maps.Copy(next, hidden)
	if colID == "" {
		return next
	}
	if next[colID] {
		delete(next, colID)
		return next
	}
	visibleCount := dataGridVisibleColumnCount(columns, next)
	if visibleCount <= 1 {
		return next
	}
	next[colID] = true
	return next
}

// --- Sort / Filter ---

func dataGridToggleSort(query GridQueryState, colID string, multiSort, appendSort bool) GridQueryState {
	next := GridQueryState{
		Sorts:       make([]GridSort, len(query.Sorts)),
		Filters:     make([]GridFilter, len(query.Filters)),
		QuickFilter: query.QuickFilter,
	}
	copy(next.Sorts, query.Sorts)
	copy(next.Filters, query.Filters)

	idx := dataGridSortIndex(next.Sorts, colID)
	newDir := GridSortAsc
	remove := false
	if idx >= 0 {
		if next.Sorts[idx].Dir == GridSortAsc {
			newDir = GridSortDesc
		} else {
			remove = true
		}
	}
	if appendSort && multiSort {
		if idx >= 0 {
			if remove {
				next.Sorts = append(next.Sorts[:idx], next.Sorts[idx+1:]...)
			} else {
				next.Sorts[idx] = GridSort{ColID: colID, Dir: newDir}
			}
		} else {
			next.Sorts = append(next.Sorts, GridSort{ColID: colID, Dir: GridSortAsc})
		}
		return next
	}
	if idx >= 0 {
		if remove {
			next.Sorts = nil
		} else {
			next.Sorts = []GridSort{{ColID: colID, Dir: newDir}}
		}
	} else {
		next.Sorts = []GridSort{{ColID: colID, Dir: GridSortAsc}}
	}
	return next
}

func dataGridSortIndex(sorts []GridSort, colID string) int {
	for idx, s := range sorts {
		if s.ColID == colID {
			return idx
		}
	}
	return -1
}

func dataGridQuerySetFilter(query GridQueryState, colID, value string) GridQueryState {
	next := GridQueryState{
		Sorts:       make([]GridSort, len(query.Sorts)),
		Filters:     make([]GridFilter, len(query.Filters)),
		QuickFilter: query.QuickFilter,
	}
	copy(next.Sorts, query.Sorts)
	copy(next.Filters, query.Filters)

	idx := dataGridQueryFilterIndex(next.Filters, colID)
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if idx >= 0 {
			next.Filters = append(next.Filters[:idx], next.Filters[idx+1:]...)
		}
		return next
	}
	if idx >= 0 {
		next.Filters[idx] = GridFilter{
			ColID: colID,
			Op:    next.Filters[idx].Op,
			Value: value,
		}
	} else {
		next.Filters = append(next.Filters, GridFilter{
			ColID: colID,
			Op:    "contains",
			Value: value,
		})
	}
	return next
}

func dataGridQueryFilterIndex(filters []GridFilter, colID string) int {
	for idx, f := range filters {
		if f.ColID == colID {
			return idx
		}
	}
	return -1
}

func dataGridQueryFilterValue(query GridQueryState, colID string) string {
	idx := dataGridQueryFilterIndex(query.Filters, colID)
	if idx < 0 {
		return ""
	}
	return query.Filters[idx].Value
}

// --- Column widths ---

// dataGridColumnWidths resolves column widths from cached
// state, falling back to column config defaults. Clamps
// each width to [MinWidth, MaxWidth]. Prunes stale entries
// for removed columns. Writes back to cache only if changed.
func dataGridColumnWidths(gridID string, columns []GridColumnCfg, w *gg.Window) map[string]float32 {
	dgCW := gg.StateMap[string, dataGridColWidths](w, nsDgColWidths, capModerate)
	cached, hasCached := dgCW.Get(gridID)

	changed := !hasCached
	activeIDs := map[string]bool{}
	if !changed {
		for _, col := range columns {
			if col.ID == "" {
				continue
			}
			activeIDs[col.ID] = true
			cw, ok := cached.Widths[col.ID]
			if !ok {
				changed = true
				break
			}
			clamped := dataGridClampWidth(col, cw)
			if cw != clamped {
				changed = true
				break
			}
		}
		if !changed {
			for key := range cached.Widths {
				if !activeIDs[key] {
					changed = true
					break
				}
			}
		}
	}
	if !changed {
		return cached.Widths
	}

	widths := make(map[string]float32, len(columns))
	for _, col := range columns {
		if col.ID == "" {
			continue
		}
		base, ok := cached.Widths[col.ID]
		if !ok {
			base = dataGridInitialWidth(col)
		}
		widths[col.ID] = dataGridClampWidth(col, base)
	}
	dgCW.Set(gridID, dataGridColWidths{Widths: widths})
	return widths
}

func dataGridColumnWidth(gridID string, columns []GridColumnCfg, col GridColumnCfg, w *gg.Window) float32 {
	widths := dataGridColumnWidths(gridID, columns, w)
	return dataGridColumnWidthFor(col, widths)
}

func dataGridColumnWidthFor(col GridColumnCfg, widths map[string]float32) float32 {
	if w, ok := widths[col.ID]; ok {
		return w
	}
	return dataGridInitialWidth(col)
}

func dataGridSetColumnWidth(gridID string, col GridColumnCfg, width float32, w *gg.Window) {
	dgCW := gg.StateMap[string, dataGridColWidths](w, nsDgColWidths, capModerate)
	var widths map[string]float32
	if cached, ok := dgCW.Get(gridID); ok {
		widths = make(map[string]float32, len(cached.Widths))
		maps.Copy(widths, cached.Widths)
	} else {
		widths = map[string]float32{}
	}
	widths[col.ID] = dataGridClampWidth(col, width)
	dgCW.Set(gridID, dataGridColWidths{Widths: widths})
}

func dataGridInitialWidth(col GridColumnCfg) float32 {
	base := col.Width.Get(120)
	return dataGridClampWidth(col, base)
}

func dataGridClampWidth(col GridColumnCfg, width float32) float32 {
	minW := col.MinWidth.Get(60)
	maxW := col.MaxWidth.Get(600)
	maxW = max(maxW, minW)
	minW = max(minW, 1)
	return f32Clamp(width, minW, maxW)
}

func dataGridColumnsTotalWidth(columns []GridColumnCfg, columnWidths map[string]float32) float32 {
	total := float32(0)
	for _, col := range columns {
		total += dataGridColumnWidthFor(col, columnWidths)
	}
	return total
}
