package gui

// Row header column and select-all corner for Table (issue #1156).
//
// With TableCfg.RowHeader set, every row gets a narrow leading cell, in
// the style of the WinForms DataGridView row selector. The cell is not
// part of TableRowCfg.Cells, so it never takes part in column widths,
// alignments or anything else a caller does with the data. The cell on
// a header row is the corner: a click there selects every data row. The
// cell on a data row selects that row; Ctrl (Cmd on macOS) toggles it
// and Shift extends a range from the anchor. The current row, the one a
// click or arrow key went to last, shows a marker.
//
// Selection keys on cfg.Data indices, so it stays stable when
// virtualization drops rows out of view.

// tableRowHeaderWidthDefault is the row header width when the caller
// leaves RowHeaderWidth at zero. Wide enough for the marker glyph at
// body text size with no horizontal padding.
const tableRowHeaderWidthDefault float32 = 24

// Glyphs drawn in the row header. The splitter already draws ▶ as
// plain text, so the bundled fonts cover it.
const (
	tableRowHeaderCorner  = "◢"
	tableRowHeaderCurrent = "▶"
)

// tableRowHeader is what each row needs to build its header cell. It is
// computed once per view in tableView. The zero value (on == false)
// builds no header cell.
type tableRowHeader struct {
	// data is cfg.Data, kept for the corner (select all) and for range
	// selection, which skip header rows.
	data  []TableRowCfg
	key   string // table effective ID; keys the anchor and current row
	width float32
	// current is the cfg.Data index that shows the marker, or -1 for
	// none.
	current int
	on      bool
}

// tableRowHeaderOf derives the row header state for a view. activeRowIdx
// is the keyboard position of a focusable table (-1 otherwise); a
// table that is not focusable reads the current row that clicks store
// instead, with no default, so no row shows the marker before the
// first click.
func tableRowHeaderOf(cfg *TableCfg, activeRowIdx int, w *Window) tableRowHeader {
	if !cfg.RowHeader {
		return tableRowHeader{current: -1}
	}
	width := cfg.RowHeaderWidth
	// NaN and Inf would poison the row layout; treat them as unset.
	if width <= 0 || !f32IsFinite(width) {
		width = tableRowHeaderWidthDefault
	}
	current := activeRowIdx
	if !cfg.Focusable {
		current = -1
		// The state store hangs off the Window, so a view built with
		// no Window has no current row.
		if w != nil {
			current = StateReadOr(w, nsTableFocus, cfg.ID, -1)
		}
	}
	return tableRowHeader{
		data:    cfg.Data,
		key:     cfg.ID,
		width:   width,
		current: current,
		on:      true,
	}
}

// tableIsHeaderRow reports whether every cell of r is a header cell.
// Such a row is the column header (TableCfgFromData makes row 0 one),
// so it takes the corner instead of a row selector and select all
// leaves it out. A row with no cells is not a header.
func tableIsHeaderRow(r TableRowCfg) bool {
	if len(r.Cells) == 0 {
		return false
	}
	for i := range r.Cells {
		if !r.Cells[i].HeadCell {
			return false
		}
	}
	return true
}

// tableSelectAll returns a selection that holds every data row, header
// rows left out.
func tableSelectAll(data []TableRowCfg) map[int]bool {
	sel := make(map[int]bool, len(data))
	for i := range data {
		if !tableIsHeaderRow(data[i]) {
			sel[i] = true
		}
	}
	return sel
}

// tableRowHeaderSelection returns the selection after a click on the
// row header of row, with mods held. It never changes selected.
//
//   - Plain click: row becomes the whole selection.
//   - Shortcut (Ctrl, or Cmd on macOS): row toggles; the rest stays.
//   - Shift: the range anchor..row, header rows left out, replaces the
//     selection. Shortcut+Shift adds the range to it instead.
//
// Single select has room for one row, so every click selects row alone.
// Only the Shift and shortcut bits count, so a held mouse button that a
// backend puts in mods changes nothing.
func tableRowHeaderSelection(
	data []TableRowCfg, selected map[int]bool, multiSelect bool,
	row, anchor int, mods Modifier,
) map[int]bool {
	if !multiSelect {
		return map[int]bool{row: true}
	}
	shortcut := isShortcut(mods)
	if mods.Has(ModShift) {
		next := make(map[int]bool)
		if shortcut {
			next = copySelected(selected)
		}
		// Clamp the span to the data first: the anchor comes from state
		// and can be stale after Data shrinks, and an unclamped span
		// would walk every index up to it.
		lo := max(min(anchor, row), 0)
		hi := min(max(anchor, row), len(data)-1)
		for i := lo; i <= hi; i++ {
			if !tableIsHeaderRow(data[i]) {
				next[i] = true
			}
		}
		return next
	}
	if shortcut {
		// The same toggle a click on the row's cells applies.
		return tableNextSelection(selected, true, row)
	}
	return map[int]bool{row: true}
}

// tableRowHeaderCell builds the leading header cell of row rowIdx: the
// corner on a header row, a row selector on a data row. It matches the
// data cells' border and vertical padding, so the row keeps its height.
func tableRowHeaderCell(
	cfg *TableCfg, rowIdx int, rh tableRowHeader, cellBorder float32,
	selected map[int]bool, multiSelect bool,
	onSelect func(map[int]bool, int, EventCtx),
) View {
	r := cfg.Data[rowIdx]
	isHeader := tableIsHeaderRow(r)

	var glyph, label string
	ts := cfg.TextStyle
	var onClick func(EventCtx)
	if isHeader {
		glyph = tableRowHeaderCorner
		label = "Select all rows"
		ts = cfg.TextStyleHead
		// Single select cannot hold every row, so the corner is a
		// plain cell there.
		if onSelect != nil && multiSelect {
			data := rh.data
			ri := rowIdx
			onClick = func(ctx EventCtx) {
				onSelect(tableSelectAll(data), ri, ctx)
				ctx.Consume()
			}
		}
	} else {
		if rowIdx == rh.current {
			glyph = tableRowHeaderCurrent
		}
		label = "Select row"
		if onSelect != nil {
			onClick = tableRowHeaderOnClick(rh, rowIdx, r.OnClick,
				selected, multiSelect, onSelect)
		}
	}

	var onHover func(EventCtx)
	if onClick != nil {
		ch := cfg.Colors.Hover
		onHover = func(ctx EventCtx) {
			ctx.Window.SetMouseCursorPointingHand()
			ctx.Layout.Shape.Color = ch
		}
	}

	return Column(ContainerCfg{
		A11YRole:    AccessRoleGridCell,
		A11YCfg:     A11YCfg{A11YLabel: label},
		Color:       ColorTransparent,
		ColorBorder: cfg.Colors.Border,
		SizeBorder:  BorderPx(cellBorder),
		// No side padding: the cell is narrow and the glyph is
		// centered. Top and bottom match the data cells.
		Padding: NewPadding(cfg.CellPadding.Top, 0, cfg.CellPadding.Bottom, 0),
		Radius:  NoRadius,
		HAlign:  HAlignCenter,
		Sizing:  FixedFill,
		Width:   rh.width,
		OnClick: onClick,
		OnHover: onHover,
		Content: []View{Text(TextCfg{Text: glyph, TextStyle: ts})},
	})
}

// tableRowHeaderOnClick is the click handler of a data row's header
// cell. The click acts as a click on the row would — the row's own
// OnClick runs first — but selects with the row header rules, so it is
// consumed before the row's handler can toggle the selection a second
// time.
func tableRowHeaderOnClick(
	rh tableRowHeader, rowIdx int, rowOnClick func(EventCtx),
	selected map[int]bool, multiSelect bool,
	onSelect func(map[int]bool, int, EventCtx),
) func(EventCtx) {
	data, key, ri := rh.data, rh.key, rowIdx
	return func(ctx EventCtx) {
		var mods Modifier
		if ctx.Event != nil {
			mods = ctx.Event.Modifiers
		}
		w := ctx.Window
		am := StateMap[string, int](w, nsTableAnchor, capModerate)
		anchor := am.GetOr(key, ri)
		next := tableRowHeaderSelection(data, selected, multiSelect,
			ri, anchor, mods)
		// Shift extends from the anchor and leaves it in place, so a
		// second Shift click re-spans from the same row. Every other
		// click moves the anchor here.
		if !multiSelect || !mods.Has(ModShift) {
			am.Set(key, ri)
		}
		// The clicked row is now current: it shows the marker, and a
		// focusable table's arrow keys move on from it.
		StateMap[string, int](w, nsTableFocus, capModerate).Set(key, ri)
		if rowOnClick != nil {
			rowOnClick(ctx)
		}
		onSelect(next, ri, ctx)
		ctx.Consume()
	}
}

// tableSelectionKeys handles the selection keys a focusable table
// answers beyond arrow movement, and reports whether it consumed e.
//
//   - Shortcut+A (Ctrl+A, or Cmd+A on macOS) selects every data row
//     under MultiSelect. Any multi-select table answers it, row header
//     or not: it is the platform's select-all key.
//   - Shift+Space makes the current row the whole selection. Row header
//     tables only.
//   - Escape clears a selection. Row header tables only, and only when
//     something is selected, so with nothing to clear the key still
//     reaches a dialog that closes on it.
//
// It runs before tableOnKeyDown, which treats Space as a toggle and
// lets any other modified key travel on.
func tableSelectionKeys(
	data []TableRowCfg, selected map[int]bool, multiSelect, rowHeader bool,
	onSelect func(map[int]bool, int, EventCtx),
	activeKey string, e *Event, w *Window,
) bool {
	if onSelect == nil || len(data) == 0 {
		return false
	}
	cur := StateReadOr(w, nsTableFocus, activeKey, 0)
	if cur < 0 || cur >= len(data) {
		cur = 0
	}
	ctx := EventCtx{nil, e, w}
	switch {
	case e.KeyCode == KeyA && isShortcut(e.Modifiers):
		if !multiSelect {
			return false
		}
		onSelect(tableSelectAll(data), cur, ctx)
	case rowHeader && e.KeyCode == KeySpace && e.Modifiers.Has(ModShift):
		StateMap[string, int](w, nsTableAnchor, capModerate).Set(activeKey, cur)
		onSelect(map[int]bool{cur: true}, cur, ctx)
	case rowHeader && e.KeyCode == KeyEscape && e.Modifiers == ModNone:
		if len(selected) == 0 {
			return false
		}
		onSelect(map[int]bool{}, cur, ctx)
	default:
		return false
	}
	e.IsHandled = true
	return true
}
