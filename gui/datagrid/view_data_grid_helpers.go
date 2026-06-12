package datagrid

import (
	"slices"
	"strconv"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- Helper functions ---

func dataGridIndicatorTextStyle(base gg.TextStyle) gg.TextStyle {
	s := base
	s.Color = dataGridDimColor(base.Color)
	return s
}

func dataGridDimColor(c gg.Color) gg.Color {
	return gg.RGBA(c.R, c.G, c.B, dataGridIndicatorAlpha)
}

func dataGridRowID(row GridRow, idx int) string {
	if row.ID != "" {
		return row.ID
	}
	autoID := dataGridRowAutoID(row)
	if autoID != "" {
		return autoID
	}
	return strconv.Itoa(idx)
}

func dataGridRowAutoID(row GridRow) string {
	if len(row.Cells) == 0 {
		return ""
	}
	keys := make([]string, 0, len(row.Cells))
	for k := range row.Cells {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	h := gg.Fnv64Offset
	for i, key := range keys {
		if i > 0 {
			h = gg.Fnv64Byte(h, dataGridUnitSep[0])
		}
		h = gg.Fnv64Str(h, key)
		h = gg.Fnv64Byte(h, '=')
		h = gg.Fnv64Str(h, row.Cells[key])
	}
	return "__auto_" + zeroPadHex16(h)
}

func dataGridHeight(cfg *DataGridCfg) float32 {
	if cfg.Height > 0 {
		return cfg.Height
	}
	if cfg.MaxHeight > 0 {
		return cfg.MaxHeight
	}
	return 0
}

func dataGridPagerEnabled(cfg *DataGridCfg, pageCount int) bool {
	return cfg.PageSize > 0 && pageCount > 1
}

func dataGridPagerHeight(cfg *DataGridCfg) float32 {
	if cfg.RowHeight > 0 {
		return cfg.RowHeight
	}
	return dataGridHeaderHeight(cfg)
}

func dataGridPagerPadding(cfg *DataGridCfg) gg.Padding {
	pc := cfg.PaddingCell.Get(gg.Padding{})
	pf := cfg.PaddingFilter.Get(gg.Padding{})
	left := f32Max(pf.Left, pc.Left)
	right := f32Max(pf.Right, pc.Right)
	return gg.NewPadding(pf.Top, right, pf.Bottom, left)
}

func dataGridHeaderHeight(cfg *DataGridCfg) float32 {
	if cfg.HeaderHeight > 0 {
		return cfg.HeaderHeight
	}
	return cfg.RowHeight
}

func dataGridFilterHeight(cfg *DataGridCfg) float32 {
	return dataGridHeaderHeight(cfg)
}

func dataGridQuickFilterHeight(cfg *DataGridCfg) float32 {
	return dataGridHeaderHeight(cfg)
}

func dataGridRowHeight(cfg *DataGridCfg, _ *gg.Window) float32 {
	if cfg.RowHeight > 0 {
		return cfg.RowHeight
	}
	return cfg.TextStyle.Size + cfg.PaddingCell.Get(gg.Padding{}).Height() + cfg.SizeBorder.Get(0)
}

func dataGridStaticTopHeight(cfg *DataGridCfg, _ float32, chooserOpen bool, includeHeader bool) float32 {
	top := float32(0)
	if cfg.ShowColumnChooser {
		top += dataGridColumnChooserHeight(cfg, chooserOpen)
	}
	if includeHeader {
		top += dataGridHeaderHeight(cfg)
	}
	if cfg.ShowFilterRow {
		top += dataGridFilterHeight(cfg)
	}
	return top
}

func dataGridFocusID(cfg *DataGridCfg) uint32 {
	if cfg.IDFocus > 0 {
		return cfg.IDFocus
	}
	return gg.FnvSum32(cfg.ID + ":focus")
}

func dataGridScrollID(cfg *DataGridCfg) uint32 {
	if cfg.IDScroll > 0 {
		return cfg.IDScroll
	}
	return gg.FnvSum32(cfg.ID + ":scroll")
}

// dataGridVisibleRangeForScroll converts scroll position to
// the range of row indices to render.
func dataGridVisibleRangeForScroll(scrollY, viewportHeight, rowHeight float32, rowCount int, staticTop float32, buffer int) (int, int) {
	if rowCount == 0 || rowHeight <= 0 || viewportHeight <= 0 {
		return 0, -1
	}
	bodyScroll := max(scrollY-staticTop, 0)
	first := int(bodyScroll / rowHeight)
	visibleRows := int(viewportHeight/rowHeight) + 1
	firstVisible := max(first-buffer, 0)
	lastVisible := min(first+visibleRows+buffer, rowCount-1)
	firstVisible = min(firstVisible, lastVisible)
	return firstVisible, lastVisible
}

func dataGridDetailRowExpanded(cfg *DataGridCfg, rowID string) bool {
	return rowID != "" && cfg.DetailExpandedRowIDs[rowID]
}

func dataGridHasSource(cfg *DataGridCfg) bool {
	return cfg.DataSource != nil
}

func dataGridColumnChooserHeight(cfg *DataGridCfg, isOpen bool) float32 {
	base := cfg.RowHeight
	if base <= 0 {
		base = dataGridHeaderHeight(cfg)
	}
	if isOpen {
		return base * 2
	}
	return base
}

// --- Page bounds ---

func dataGridPageBounds(totalRows, pageSize, requestedPage int) (start, end, pageIndex, pageCount int) {
	if totalRows <= 0 {
		return 0, 0, 0, 1
	}
	if pageSize <= 0 {
		return 0, totalRows, 0, 1
	}
	ps := min(pageSize, totalRows)
	pageCount = max(1, (totalRows+ps-1)/ps)
	pageIndex = max(0, min(pageCount-1, requestedPage))
	start = pageIndex * ps
	end = min(totalRows, start+ps)
	return
}

func dataGridPageRowIndices(start, end int) []int {
	if end <= start || start < 0 {
		return nil
	}
	indices := make([]int, 0, end-start)
	for idx := start; idx < end; idx++ {
		indices = append(indices, idx)
	}
	return indices
}

func dataGridVisibleRowIndices(rowCount int, pageIndices []int) []int {
	if len(pageIndices) > 0 {
		return pageIndices
	}
	return dataGridPageRowIndices(0, max(0, rowCount))
}

func dataGridHasRowID(rows []GridRow, rowID string) bool {
	if rowID == "" {
		return false
	}
	for idx, row := range rows {
		if dataGridRowID(row, idx) == rowID {
			return true
		}
	}
	return false
}

func dataGridActiveRowIndex(rows []GridRow, selection GridSelection) int {
	res := dataGridActiveRowIndexStrict(rows, selection)
	if res >= 0 {
		return res
	}
	if len(rows) > 0 {
		return 0
	}
	return -1
}

func dataGridActiveRowIndexStrict(rows []GridRow, selection GridSelection) int {
	if len(rows) == 0 {
		return -1
	}
	hasActive := selection.ActiveRowID != ""
	hasSelected := len(selection.SelectedRowIDs) > 0
	if !hasActive && !hasSelected {
		return -1
	}
	firstSelected := -1
	for idx, row := range rows {
		id := dataGridRowID(row, idx)
		if hasActive && id == selection.ActiveRowID {
			return idx
		}
		if firstSelected < 0 && hasSelected && selection.SelectedRowIDs[id] {
			firstSelected = idx
		}
	}
	return firstSelected
}

func dataGridSelectedRows(rows []GridRow, selection GridSelection) []GridRow {
	if len(selection.SelectedRowIDs) == 0 {
		return nil
	}
	var selected []GridRow
	for idx, row := range rows {
		if selection.SelectedRowIDs[dataGridRowID(row, idx)] {
			selected = append(selected, row)
		}
	}
	return selected
}

func dataGridPageRows(cfg *DataGridCfg, rowHeight float32) int {
	if rowHeight <= 0 {
		return 1
	}
	page := int(dataGridHeight(cfg) / rowHeight)
	if page < 1 {
		return 1
	}
	return page
}

func dataGridEditingEnabled(cfg *DataGridCfg) bool {
	if cfg.OnCellEdit == nil && !cfg.ShowCRUDToolbar {
		return false
	}
	for _, col := range cfg.Columns {
		if col.Editable {
			return true
		}
	}
	return false
}

func dataGridCrudEnabled(cfg *DataGridCfg) bool {
	return cfg.ShowCRUDToolbar
}
