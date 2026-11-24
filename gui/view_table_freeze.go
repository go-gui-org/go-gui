package gui

// Frozen leading columns for Table (issue #1363).
//
// With FreezeColumns or FreezeRowHeader set, a row holds the frozen
// cells and a scroll window with the rest (scroll_window.go). The row
// itself stays in place, so its fill, hover, click and height stay those
// of one row. A table that freezes nothing builds its rows as before.

// tableFreezeEdgeWidth is the width of the line between the frozen and
// the scrolling cells.
const tableFreezeEdgeWidth float32 = 1

// tableFreezeMinWindow is the room the scrolling columns keep in a
// narrow table: about one column.
const tableFreezeMinWindow float32 = 80

// tableFreeze is what each row needs to keep its leading cells in place.
// It is computed once per view in tableView. With on == false it freezes
// nothing, and only w is set: the rows take their slices from its arena.
type tableFreeze struct {
	w *Window
	// edge is the line after the last frozen cell, or nil. One view
	// serves every row: generation only reads it.
	edge View
	// scrollID is the key of the container the windows follow.
	scrollID string
	// rowWidth is the width of a whole row, the scroll extent.
	rowWidth float32
	// rowMin is the narrowest a row can be.
	rowMin float32
	// windowWidth and windowMin are the window's widest and narrowest.
	windowWidth, windowMin float32
	// border is the cell border, which neighbouring cells share.
	border float32
	// cols is how many leading data columns are frozen.
	cols int
	on   bool
}

// tableFreezeOf derives the frozen columns for a view. Freezing every
// column leaves nothing to scroll past them, so it is the same as
// freezing none: the table scrolls as a whole. A table with no scroll
// key has no offset to follow and freezes nothing either.
func tableFreezeOf(
	cfg *TableCfg, w *Window, columnWidths []float32, cellBorder float32,
	rh tableRowHeader, scrollID string,
) tableFreeze {
	cols := max(cfg.FreezeColumns, 0)
	if cols >= len(columnWidths) {
		cols = 0
	}
	on := cols > 0 || (cfg.FreezeRowHeader && rh.on)
	if !on || scrollID == "" || len(columnWidths) == 0 {
		return tableFreeze{w: w}
	}
	fz := tableFreeze{
		w:        w,
		scrollID: scrollID,
		rowWidth: tableRowWidth(columnWidths, cellBorder, rh),
		border:   cellBorder,
		cols:     cols,
		on:       true,
	}
	scrolled := columnWidths[cols:]
	for _, cw := range scrolled {
		fz.windowWidth += cw
	}
	fz.windowWidth -= float32(len(scrolled)-1) * cellBorder
	if cellBorder <= 0 {
		// Cell borders already mark the edge of a full grid. Without
		// them a cut cell would sit right next to the frozen one.
		fz.rowWidth += tableFreezeEdgeWidth
		fz.edge = Rectangle(RectangleCfg{
			Color:  cfg.Colors.Border,
			Width:  tableFreezeEdgeWidth,
			Sizing: FixedFill,
		})
	}
	fz.windowMin = f32Min(fz.windowWidth, tableFreezeMinWindow)
	fz.rowMin = fz.rowWidth - fz.windowWidth + fz.windowMin
	return fz
}

// rowCells returns the empty cell slice of a row with n cells, sized for
// the window and the edge too when the row is split.
func (fz *tableFreeze) rowCells(n int) []View {
	if fz.on {
		n += 2 // edge and window
	}
	return rowContent(fz.w, n)
}

// split opens the window after the frozen cells: it appends the edge and
// an empty window slot to cells and returns the index of that slot. The
// cells appended after it are the window's.
func (fz *tableFreeze) split(cells []View) ([]View, int) {
	if fz.edge != nil {
		cells = append(cells, fz.edge)
	}
	slot := len(cells)
	return append(cells, nil), slot
}

// closeWindow fills the window slot with the cells after it and returns
// the row's own children. Both are parts of one slice, so a split row
// takes no slice of its own.
func (fz *tableFreeze) closeWindow(cells []View, slot int) []View {
	cells[slot] = fz.w.ScrollWindow(ScrollWindowCfg{
		ScrollID: fz.scrollID,
		Content:  cells[slot+1:],
		MinWidth: fz.windowMin,
		MaxWidth: fz.windowWidth,
		Border:   fz.border,
	})
	return cells[: slot+1 : slot+1]
}
