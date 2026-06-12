package datagrid

import (
	"math"
	"slices"
	"strconv"
	"strings"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- FNV-1a helpers (64-bit, presentation cache) ---

func dataGridFnv64U64(h, val uint64) uint64 {
	h = (h ^ (val & 0xff)) * gg.Fnv64Prime
	h = (h ^ ((val >> 8) & 0xff)) * gg.Fnv64Prime
	h = (h ^ ((val >> 16) & 0xff)) * gg.Fnv64Prime
	h = (h ^ ((val >> 24) & 0xff)) * gg.Fnv64Prime
	h = (h ^ ((val >> 32) & 0xff)) * gg.Fnv64Prime
	h = (h ^ ((val >> 40) & 0xff)) * gg.Fnv64Prime
	h = (h ^ ((val >> 48) & 0xff)) * gg.Fnv64Prime
	h = (h ^ ((val >> 56) & 0xff)) * gg.Fnv64Prime
	return h
}

// --- Presentation building ---

func dataGridCachedPresentation(cfg *DataGridCfg, columns []GridColumnCfg, rowIndices []int, w *gg.Window) dataGridPresentation {
	groupCols := dataGridGroupColumns(cfg.GroupBy, columns)
	valueCols := dataGridPresentationValueCols(groupCols, cfg.Aggregates)
	visibleIndices := dataGridVisibleRowIndices(len(cfg.Rows), rowIndices)
	groupTitles := dataGridGroupTitles(columns)
	signature := dataGridPresentationSignature(cfg, columns, visibleIndices, groupCols, valueCols, groupTitles)
	dgPC := gg.StateMap[string, dataGridPresentationCache](w, nsDgPresentation, capModerate)
	if cached, ok := dgPC.Get(cfg.ID); ok {
		if cached.Signature == signature {
			return dataGridPresentation{
				Rows:          cached.Rows,
				DataToDisplay: cached.DataToDisplay,
			}
		}
	}
	var groupRanges map[string]int
	if len(groupCols) > 0 && len(visibleIndices) > 0 {
		groupRanges = dataGridGroupRanges(cfg.Rows, visibleIndices, groupCols)
	} else {
		groupRanges = map[string]int{}
	}
	pres := dataGridPresentationRowsWithGroupRanges(cfg, columns, visibleIndices, groupCols, groupRanges, groupTitles)
	dgPC.Set(cfg.ID, dataGridPresentationCache{
		Signature:     signature,
		Rows:          pres.Rows,
		DataToDisplay: pres.DataToDisplay,
		GroupRanges:   groupRanges,
		GroupCols:     groupCols,
	})
	return pres
}

func dataGridPresentationSignature(cfg *DataGridCfg, _ []GridColumnCfg, visibleIndices []int, groupCols []string, valueCols []string, groupTitles map[string]string) uint64 {
	h := gg.Fnv64Offset
	if len(groupCols) == 0 && len(cfg.Aggregates) == 0 && cfg.OnDetailRowView == nil {
		h = gg.Fnv64Str(h, cfg.ID)
		h = gg.Fnv64Byte(h, 0x1e)
		for _, idx := range visibleIndices {
			h = dataGridFnv64U64(h, uint64(idx))
			h = gg.Fnv64Byte(h, 0x1f)
			row := cfg.Rows[idx]
			if row.ID != "" {
				h = gg.Fnv64Str(h, row.ID)
			}
			h = gg.Fnv64Byte(h, 0x1f)
		}
		return h
	}
	h = gg.Fnv64Str(h, cfg.ID)
	h = gg.Fnv64Byte(h, 0x1e)
	for _, idx := range visibleIndices {
		h = dataGridFnv64U64(h, uint64(idx))
		h = gg.Fnv64Byte(h, 0x1f)
	}
	h = gg.Fnv64Byte(h, 0x1e)
	for _, colID := range groupCols {
		h = gg.Fnv64Str(h, colID)
		h = gg.Fnv64Byte(h, 0x1f)
		h = gg.Fnv64Str(h, groupTitles[colID])
		h = gg.Fnv64Byte(h, 0x1f)
	}
	h = gg.Fnv64Byte(h, 0x1e)
	for _, agg := range cfg.Aggregates {
		h = gg.Fnv64Str(h, agg.ColID)
		h = gg.Fnv64Byte(h, 0x1f)
		h = gg.Fnv64Byte(h, byte(agg.Op))
		h = gg.Fnv64Byte(h, 0x1f)
		h = gg.Fnv64Str(h, agg.Label)
		h = gg.Fnv64Byte(h, 0x1f)
	}
	detailEnabled := cfg.OnDetailRowView != nil
	if detailEnabled {
		h = gg.Fnv64Byte(h, '1')
	} else {
		h = gg.Fnv64Byte(h, '0')
	}
	for _, rowIdx := range visibleIndices {
		row := cfg.Rows[rowIdx]
		rowID := dataGridRowID(row, rowIdx)
		h = gg.Fnv64Byte(h, 0x1e)
		h = dataGridFnv64U64(h, uint64(rowIdx))
		h = gg.Fnv64Byte(h, 0x1f)
		h = gg.Fnv64Str(h, rowID)
		h = gg.Fnv64Byte(h, 0x1f)
		if detailEnabled && dataGridDetailRowExpanded(cfg, rowID) {
			h = gg.Fnv64Byte(h, '1')
		} else {
			h = gg.Fnv64Byte(h, '0')
		}
		for _, colID := range valueCols {
			h = gg.Fnv64Byte(h, 0x1f)
			h = gg.Fnv64Str(h, colID)
			h = gg.Fnv64Byte(h, '=')
			h = gg.Fnv64Str(h, row.Cells[colID])
		}
	}
	return h
}

func dataGridPresentationValueCols(groupCols []string, aggregates []GridAggregateCfg) []string {
	seen := map[string]bool{}
	cols := make([]string, 0, len(groupCols)+len(aggregates))
	for _, colID := range groupCols {
		if colID == "" || seen[colID] {
			continue
		}
		seen[colID] = true
		cols = append(cols, colID)
	}
	for _, agg := range aggregates {
		if agg.Op == GridAggregateCount || agg.ColID == "" || seen[agg.ColID] {
			continue
		}
		seen[agg.ColID] = true
		cols = append(cols, agg.ColID)
	}
	slices.Sort(cols)
	return cols
}

func dataGridPresentationRows(cfg *DataGridCfg, columns []GridColumnCfg, rowIndices []int) dataGridPresentation {
	visibleIndices := dataGridVisibleRowIndices(len(cfg.Rows), rowIndices)
	groupCols := dataGridGroupColumns(cfg.GroupBy, columns)
	groupTitles := dataGridGroupTitles(columns)
	var groupRanges map[string]int
	if len(groupCols) > 0 && len(visibleIndices) > 0 {
		groupRanges = dataGridGroupRanges(cfg.Rows, visibleIndices, groupCols)
	} else {
		groupRanges = map[string]int{}
	}
	return dataGridPresentationRowsWithGroupRanges(cfg, columns, visibleIndices, groupCols, groupRanges, groupTitles)
}

func dataGridPresentationRowsWithGroupRanges(cfg *DataGridCfg, _ []GridColumnCfg, visibleIndices []int, groupCols []string, groupRanges map[string]int, groupTitles map[string]string) dataGridPresentation {
	rows := make([]dataGridDisplayRow, 0, len(cfg.Rows)+8)
	dataToDisplay := map[int]int{}
	if len(groupCols) == 0 || len(visibleIndices) == 0 {
		for _, rowIdx := range visibleIndices {
			row := cfg.Rows[rowIdx]
			dataToDisplay[rowIdx] = len(rows)
			rows = append(rows, dataGridDisplayRow{
				Kind:       dataGridDisplayRowData,
				DataRowIdx: rowIdx,
			})
			if cfg.OnDetailRowView != nil && dataGridDetailRowExpanded(cfg, dataGridRowID(row, rowIdx)) {
				rows = append(rows, dataGridDisplayRow{
					Kind:       dataGridDisplayRowDetail,
					DataRowIdx: rowIdx,
				})
			}
		}
		return dataGridPresentation{Rows: rows, DataToDisplay: dataToDisplay}
	}

	prevValues := make([]string, len(groupCols))
	values := make([]string, len(groupCols))
	hasPrev := false

	for localIdx, rowIdx := range visibleIndices {
		row := cfg.Rows[rowIdx]
		for i, colID := range groupCols {
			values[i] = row.Cells[colID]
		}
		changeDepth := -1
		if !hasPrev {
			changeDepth = 0
		} else {
			for depth, value := range values {
				if value != prevValues[depth] {
					changeDepth = depth
					break
				}
			}
		}
		if changeDepth >= 0 {
			for depth := changeDepth; depth < len(groupCols); depth++ {
				colID := groupCols[depth]
				rangeEndLocal, ok := groupRanges[dataGridGroupRangeKey(depth, localIdx)]
				if !ok {
					rangeEndLocal = localIdx
				}
				count := max(0, rangeEndLocal-localIdx+1)
				rangeEnd := visibleIndices[rangeEndLocal]
				rows = append(rows, dataGridDisplayRow{
					Kind:          dataGridDisplayRowGroupHeader,
					GroupColID:    colID,
					GroupValue:    values[depth],
					GroupColTitle: groupTitles[colID],
					GroupDepth:    depth,
					GroupCount:    count,
					AggregateText: dataGridGroupAggregateText(cfg, rowIdx, rangeEnd),
				})
			}
		}
		dataToDisplay[rowIdx] = len(rows)
		rows = append(rows, dataGridDisplayRow{
			Kind:       dataGridDisplayRowData,
			DataRowIdx: rowIdx,
		})
		if cfg.OnDetailRowView != nil && dataGridDetailRowExpanded(cfg, dataGridRowID(row, rowIdx)) {
			rows = append(rows, dataGridDisplayRow{
				Kind:       dataGridDisplayRowDetail,
				DataRowIdx: rowIdx,
			})
		}
		copy(prevValues, values)
		hasPrev = true
	}
	return dataGridPresentation{Rows: rows, DataToDisplay: dataToDisplay}
}

func dataGridGroupColumns(groupBy []string, columns []GridColumnCfg) []string {
	if len(groupBy) == 0 {
		return nil
	}
	available := map[string]bool{}
	for _, col := range columns {
		if col.ID != "" {
			available[col.ID] = true
		}
	}
	seen := map[string]bool{}
	cols := make([]string, 0, len(groupBy))
	for _, colID := range groupBy {
		if colID == "" || seen[colID] || !available[colID] {
			continue
		}
		seen[colID] = true
		cols = append(cols, colID)
	}
	return cols
}

func dataGridGroupTitles(columns []GridColumnCfg) map[string]string {
	titles := make(map[string]string, len(columns))
	for _, col := range columns {
		if col.ID != "" {
			titles[col.ID] = col.Title
		}
	}
	return titles
}

func dataGridGroupRangeKey(depth, startIdx int) string {
	return strconv.Itoa(depth) + ":" + strconv.Itoa(startIdx)
}

func dataGridGroupRanges(rows []GridRow, indices []int, groupCols []string) map[string]int {
	ranges := map[string]int{}
	if len(indices) == 0 || len(groupCols) == 0 {
		return ranges
	}
	starts := make([]int, len(groupCols))
	values := make([]string, len(groupCols))
	for depth, colID := range groupCols {
		values[depth] = rows[indices[0]].Cells[colID]
	}
	for i := 1; i < len(indices); i++ {
		row := rows[indices[i]]
		changeDepth := -1
		for depth, colID := range groupCols {
			if row.Cells[colID] != values[depth] {
				changeDepth = depth
				break
			}
		}
		if changeDepth < 0 {
			continue
		}
		for depth := len(groupCols) - 1; depth >= changeDepth; depth-- {
			ranges[dataGridGroupRangeKey(depth, starts[depth])] = i - 1
		}
		for dep := changeDepth; dep < len(groupCols); dep++ {
			starts[dep] = i
			values[dep] = row.Cells[groupCols[dep]]
		}
	}
	last := len(indices) - 1
	for depth := range slices.Backward(groupCols) {
		ranges[dataGridGroupRangeKey(depth, starts[depth])] = last
	}
	return ranges
}

func dataGridGroupAggregateText(cfg *DataGridCfg, startIdx, endIdx int) string {
	if len(cfg.Aggregates) == 0 || startIdx < 0 || endIdx < startIdx || endIdx >= len(cfg.Rows) {
		return ""
	}
	parts := make([]string, 0, len(cfg.Aggregates))
	for _, agg := range cfg.Aggregates {
		value, ok := dataGridAggregateValue(cfg.Rows, startIdx, endIdx, agg)
		if !ok {
			continue
		}
		parts = append(parts, dataGridAggregateLabel(agg)+": "+value)
	}
	return strings.Join(parts, "  ")
}

func dataGridAggregateLabel(agg GridAggregateCfg) string {
	if agg.Label != "" {
		return agg.Label
	}
	if agg.Op == GridAggregateCount {
		return "count"
	}
	if agg.ColID == "" {
		return agg.Op.String()
	}
	return agg.Op.String() + " " + agg.ColID
}

func dataGridAggregateValue(rows []GridRow, startIdx, endIdx int, agg GridAggregateCfg) (string, bool) {
	if agg.Op == GridAggregateCount {
		return strconv.Itoa(endIdx - startIdx + 1), true
	}
	if agg.ColID == "" {
		return "", false
	}
	var values []float64
	for idx := startIdx; idx <= endIdx; idx++ {
		raw := rows[idx].Cells[agg.ColID]
		n, ok := dataGridParseNumber(raw)
		if !ok {
			continue
		}
		values = append(values, n)
	}
	if len(values) == 0 {
		return "", false
	}
	var result float64
	switch agg.Op {
	case GridAggregateSum, GridAggregateAvg:
		for _, v := range values {
			result += v
		}
		if agg.Op == GridAggregateAvg {
			result /= float64(len(values))
		}
	case GridAggregateMin:
		result = values[0]
		for _, v := range values[1:] {
			if v < result {
				result = v
			}
		}
	case GridAggregateMax:
		result = values[0]
		for _, v := range values[1:] {
			if v > result {
				result = v
			}
		}
	}
	return dataGridFormatNumber(result), true
}

func dataGridParseNumber(value string) (float64, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

func dataGridFormatNumber(value float64) string {
	text := strconv.FormatFloat(value, 'f', 4, 64)
	for strings.Contains(text, ".") && strings.HasSuffix(text, "0") {
		text = text[:len(text)-1]
	}
	text = strings.TrimSuffix(text, ".")
	return text
}

// --- Assembly functions ---

func dataGridScrollBodyRows(
	dctx dataGridCtx,
	presentation dataGridPresentation,
	rowDeleteEnabled bool,
	headerInScrollBody bool,
	headerView gg.View, chooserOpen, hasSource, virtualize bool,
	firstVisible, lastVisible int,
) []gg.View {
	cfg := dctx.cfg
	rows := make([]gg.View, 0, len(presentation.Rows)+8)
	if cfg.ShowColumnChooser {
		rows = append(rows,
			dataGridColumnChooserRow(cfg, chooserOpen, dctx.focusID))
	}
	if headerInScrollBody {
		rows = append(rows, headerView)
	}
	if cfg.ShowFilterRow {
		rows = append(rows,
			dataGridFilterRow(cfg, dctx.columns, dctx.columnWidths))
	}
	if hasSource && cfg.Loading && len(presentation.Rows) == 0 {
		rows = append(rows,
			dataGridSourceStatusRow(cfg, gg.ActiveLocale.StrLoading))
	}
	if hasSource && cfg.LoadError != "" && len(presentation.Rows) == 0 {
		rows = append(rows, dataGridSourceStatusRow(cfg,
			gg.ActiveLocale.StrLoadError+": "+cfg.LoadError))
	}

	lastRowIdx := len(presentation.Rows) - 1
	if virtualize && firstVisible > 0 {
		rows = append(rows, gg.Rectangle(gg.RectangleCfg{
			Color:  gg.ColorTransparent,
			Height: float32(firstVisible) * dctx.rowHeight,
			Sizing: gg.FillFixed,
		}))
	}

	for rowIdx := firstVisible; rowIdx <= lastVisible; rowIdx++ {
		if rowIdx < 0 || rowIdx >= len(presentation.Rows) {
			continue
		}
		entry := presentation.Rows[rowIdx]
		if entry.Kind == dataGridDisplayRowGroupHeader {
			rows = append(rows,
				dataGridGroupHeaderRowView(cfg, entry, dctx.rowHeight))
			continue
		}
		if entry.Kind == dataGridDisplayRowDetail {
			if entry.DataRowIdx < 0 ||
				entry.DataRowIdx >= len(cfg.Rows) {
				continue
			}
			rows = append(rows, dataGridDetailRowView(dctx,
				cfg.Rows[entry.DataRowIdx], entry.DataRowIdx))
			continue
		}
		if entry.DataRowIdx < 0 ||
			entry.DataRowIdx >= len(cfg.Rows) {
			continue
		}
		rows = append(rows, dataGridRowView(dctx,
			cfg.Rows[entry.DataRowIdx], entry.DataRowIdx,
			rowDeleteEnabled))
	}

	if virtualize && lastVisible < lastRowIdx {
		remaining := lastRowIdx - lastVisible
		rows = append(rows, gg.Rectangle(gg.RectangleCfg{
			Color:  gg.ColorTransparent,
			Height: float32(remaining) * dctx.rowHeight,
			Sizing: gg.FillFixed,
		}))
	}
	return rows
}

func dataGridFinalContent(
	dctx dataGridCtx,
	scrollBody, headerView gg.View, headerHeight, totalWidth, scrollX float32,
	gridHeight, staticTop float32,
	frozenTopViews []gg.View, frozenTopDisplayRows int,
	crudEnabled bool,
	crudState dataGridCrudState,
	sourceCaps GridDataCapabilities,
	hasSource bool,
	pagerEnabled, sourcePagerEnabled bool,
	pageIndex, pageCount, pageStart, pageEnd int,
	presentation dataGridPresentation,
	sourceState dataGridSourceState,
) []gg.View {
	cfg := dctx.cfg
	content := make([]gg.View, 0, 6)
	if crudEnabled {
		content = append(content, dataGridCrudToolbarRow(cfg,
			crudState, sourceCaps, hasSource, dctx.focusID))
	}
	if cfg.ShowQuickFilter {
		qfHeight := dataGridQuickFilterHeight(cfg)
		content = append(content, dataGridFrozenTopZone(cfg,
			[]gg.View{dataGridQuickFilterRow(cfg)},
			qfHeight, totalWidth, scrollX))
	}
	if boolDefault(cfg.ShowHeader, true) && cfg.FreezeHeader {
		content = append(content, dataGridFrozenTopZone(cfg,
			[]gg.View{headerView}, headerHeight, totalWidth, scrollX))
	}
	if frozenTopDisplayRows > 0 {
		frozenHeight := float32(frozenTopDisplayRows) * dctx.rowHeight
		content = append(content, dataGridFrozenTopZone(cfg,
			frozenTopViews, frozenHeight, totalWidth, scrollX))
	}
	content = append(content, scrollBody)
	if pagerEnabled {
		totalRows := len(cfg.Rows)
		if cfg.RowCount != nil {
			totalRows = *cfg.RowCount
		}
		dgJump := gg.StateMap[string, string](dctx.w, nsDgJump, capModerate)
		jumpText, _ := dgJump.Get(cfg.ID)
		content = append(content, dataGridPagerRow(cfg, dctx.focusID,
			pageIndex, pageCount, pageStart, pageEnd, totalRows,
			gridHeight, dctx.rowHeight, staticTop, dctx.scrollID,
			presentation.DataToDisplay, jumpText))
	}
	if sourcePagerEnabled {
		dgJump := gg.StateMap[string, string](dctx.w, nsDgJump, capModerate)
		jumpText, _ := dgJump.Get(cfg.ID)
		content = append(content, dataGridSourcePagerRow(cfg,
			dctx.focusID, sourceState, sourceCaps, jumpText))
	}
	return content
}
