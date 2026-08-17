package gui

// Keyboard navigation for a focusable Table (issue #345, audit §6.1).
//
// Focus lands on the table as a whole, and Up/Down/Home/End move an
// "active row" — the keyboard position, tinted with the hover color.
// Selection follows the active row when OnSelect is set: plain
// movement selects the row it lands on, and Shift extends a range
// under MultiSelect. Enter/Space activate the active row exactly the
// way a click on it would (row OnClick plus the selection toggle).
// This mirrors gui/datagrid's row-level answer, so the two grids do
// not contradict each other.

// tableFocusState is the focus surface a focusable table's outer
// container takes: the flag that joins the tab order, the ring that
// paints focus, and the key handler that navigates. An all-zero value
// (or one taken from a non-focusable cfg) wires nothing.
type tableFocusState struct {
	focusable bool
	ring      func(EventCtx)
	onKeyDown func(EventCtx)
}

// tableFocusWiring builds the focus surface for a table's outer
// container. scrollID/rowH/listH carry the scroll target for
// virtualization: rowH == 0 means rows are not uniform, so arrow
// movement highlights but does not scroll. bodyRowOffset is 1 for the
// freeze layout, whose scrollable body holds data rows 1..n.
func tableFocusWiring(
	cfg *TableCfg, selected map[int]bool, multiSelect bool,
	onSelect func(map[int]bool, int, EventCtx),
	scrollID string, rowH, listH float32,
	bodyRowOffset int,
) tableFocusState {
	if !cfg.Focusable {
		return tableFocusState{}
	}
	data := cfg.Data
	activeKey := cfg.ID
	return tableFocusState{
		focusable: true,
		ring:      focusRingAmend(Color{}, cfg.ColorBorderFocus),
		onKeyDown: func(ctx EventCtx) {
			tableOnKeyDown(data, selected, multiSelect, onSelect,
				activeKey, scrollID, rowH, listH, bodyRowOffset,
				ctx.Event, ctx.Window)
		},
	}
}

// freezeBodyOffset is the scroll-index shift a table's keyboard
// movement needs: the freeze body holds data rows 1..n, so a data
// index maps to a body row one below it.
func freezeBodyOffset(freeze bool) int {
	if freeze {
		return 1
	}
	return 0
}

// tableWireFocus copies a table's focus surface onto its outer
// container cfg. The callers differ (freeze vs non-freeze layout) but
// the surface is the same.
func tableWireFocus(cfg *ContainerCfg, fw tableFocusState) {
	cfg.Focusable = fw.focusable
	cfg.AmendLayout = fw.ring
	cfg.OnKeyDown = fw.onKeyDown
}

// tableNextSelection builds the selection after a row is activated —
// the row is toggled in, or out when already selected. Under
// MultiSelect the rest of the selection survives; otherwise the
// activation becomes the whole selection. Shared by the row click
// handler (tableBuildRow) and keyboard activation, so click and
// Space/Enter cannot drift apart.
func tableNextSelection(selected map[int]bool, multiSelect bool, row int) map[int]bool {
	next := make(map[int]bool)
	if multiSelect {
		next = copySelected(selected)
	}
	if next[row] {
		delete(next, row)
	} else {
		next[row] = true
	}
	return next
}

// tableOnKeyDown navigates a table from the keyboard.
//
// bodyRowOffset is 1 for the freeze layout — the scrollable body
// holds data rows 1..n, so a data index maps to a body row below it —
// and 0 otherwise. It is only consulted when the table scrolls with
// uniform rows (rowH > 0).
func tableOnKeyDown(
	data []TableRowCfg,
	selected map[int]bool,
	multiSelect bool,
	onSelect func(map[int]bool, int, EventCtx),
	activeKey, scrollID string,
	rowH, listH float32,
	bodyRowOffset int,
	e *Event, w *Window,
) {
	if len(data) == 0 {
		return
	}
	// Shift extends a range; anything else modified travels on.
	if e.Modifiers != 0 && !e.Modifiers.Has(ModShift) {
		return
	}

	action := listCoreNavigate(e.KeyCode, len(data))
	if e.KeyCode == KeySpace {
		action = listCoreSelectItem
	}
	// Escape (listCoreDismiss) has no popup to dismiss here: it and
	// unrecognized keys travel on.
	if action == listCoreNone || action == listCoreDismiss {
		return
	}

	// The active row persists in state across frames, so focus can
	// leave and return to the same position.
	fm := StateMap[string, int](w, nsTableFocus, capModerate)
	cur := fm.GetOr(activeKey, 0)
	if cur < 0 || cur >= len(data) {
		cur = 0
	}

	if action == listCoreSelectItem {
		// Activate the active row as a click on it would.
		if row := data[cur]; row.OnClick != nil {
			row.OnClick(EventCtx{nil, e, w})
		}
		if onSelect != nil {
			onSelect(tableNextSelection(selected, multiSelect, cur),
				cur, EventCtx{nil, e, w})
		}
		e.IsHandled = true
		return
	}

	next, changed := listCoreApplyNav(action, cur, len(data))
	if !changed {
		// Already clamped at an edge (Up at row 0, End at the last):
		// still consume, so the key does not fall through to the
		// page — ListBox and datagrid do the same.
		e.IsHandled = true
		return
	}
	fm.Set(activeKey, next)

	// Selection follows movement. Shift anchors the range at the row
	// movement last started from; every other move re-anchors.
	if onSelect != nil {
		sel := make(map[int]bool)
		if multiSelect && e.Modifiers.Has(ModShift) {
			am := StateMap[string, int](w, nsTableAnchor, capModerate)
			anchor := am.GetOr(activeKey, next)
			for i := min(anchor, next); i <= max(anchor, next); i++ {
				sel[i] = true
			}
		} else {
			sel[next] = true
			StateMap[string, int](
				w, nsTableAnchor, capModerate,
			).Set(activeKey, next)
		}
		onSelect(sel, next, EventCtx{nil, e, w})
	}

	if scrollID != "" && rowH > 0 && listH > 0 {
		scrollEnsureVisible(scrollID,
			max(next-bodyRowOffset, 0), rowH, listH, w)
	}
	e.IsHandled = true
}
