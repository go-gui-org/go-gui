package datagrid

import (
	"slices"
	"strconv"
	"strings"
	"time"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- Cell editors ---

func dataGridCellEditorView(cfg *DataGridCfg, rowID string, rowIdx int, col GridColumnCfg, value string, editorFocusID, gridFocusID string, _ *gg.Window) gg.View {
	editorID := editorFocusID
	colID := col.ID
	gridID := cfg.ID
	crudEnabled := dataGridCrudEnabled(cfg)
	onCellEdit := cfg.OnCellEdit

	var editor gg.View
	switch col.Editor {
	case GridCellEditorSelect:
		options := make([]string, len(col.EditorOptions))
		copy(options, col.EditorOptions)
		if len(options) == 0 && value != "" {
			options = []string{value}
		}
		var selectVal []string
		if value != "" {
			selectVal = []string{value}
		}
		editor = gg.Select(gg.SelectCfg{
			ID:         editorID,
			Selected:   selectVal,
			Options:    options,
			Sizing:     gg.FillFill,
			Padding:    gg.NoPadding,
			SizeBorder: gg.NoBorder,
			Radius:     gg.SomeF(0),
			OnSelect: func(selected []string, e *gg.Event, w *gg.Window) {
				nextValue := ""
				if len(selected) > 0 {
					nextValue = selected[0]
				}
				if rowID != "" && colID != "" {
					dataGridCrudApplyCellEdit(gridID, crudEnabled, onCellEdit, GridCellEdit{
						RowID:  rowID,
						RowIdx: rowIdx,
						ColID:  colID,
						Value:  nextValue,
					}, e, w)
				}
			},
		})
	case GridCellEditorDate:
		date := dataGridParseEditorDate(value)
		editor = gg.InputDate(gg.InputDateCfg{
			ID:      editorID,
			Date:    date,
			Sizing:  gg.FillFill,
			Padding: gg.NoPadding,
			OnSelect: func(dates []time.Time, e *gg.Event, w *gg.Window) {
				if len(dates) == 0 {
					return
				}
				nextValue := dates[0].Format("1/2/2006")
				if rowID != "" && colID != "" {
					dataGridCrudApplyCellEdit(gridID, crudEnabled, onCellEdit, GridCellEdit{
						RowID:  rowID,
						RowIdx: rowIdx,
						ColID:  colID,
						Value:  nextValue,
					}, e, w)
				}
			},
		})
	case GridCellEditorCheckbox:
		checked := dataGridEditorBoolValue(value)
		editorTrueValue := col.EditorTrueValue
		editorFalseValue := col.EditorFalseValue
		editor = gg.Toggle(gg.ToggleCfg{
			ID:       editorID,
			Selected: checked,
			Padding:  gg.NoPadding,
			OnClick: func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
				nextValue := editorFalseValue
				if !checked {
					nextValue = editorTrueValue
				}
				if rowID != "" && colID != "" {
					dataGridCrudApplyCellEdit(gridID, crudEnabled, onCellEdit, GridCellEdit{
						RowID:  rowID,
						RowIdx: rowIdx,
						ColID:  colID,
						Value:  nextValue,
					}, e, w)
				}
				e.IsHandled = true
			},
		})
	default: // GridCellEditorText
		editor = gg.Input(gg.InputCfg{
			ID:         editorID,
			Text:       value,
			Sizing:     gg.FillFill,
			Padding:    gg.NoPadding,
			SizeBorder: gg.NoBorder,
			Radius:     gg.SomeF(0),
			OnTextChanged: func(_ *gg.Layout, text string, w *gg.Window) {
				if rowID != "" && colID != "" {
					e := &gg.Event{}
					dataGridCrudApplyCellEdit(gridID, crudEnabled, onCellEdit, GridCellEdit{
						RowID:  rowID,
						RowIdx: rowIdx,
						ColID:  colID,
						Value:  text,
					}, e, w)
				}
			},
			OnEnter: func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
				dataGridClearEditingRow(gridID, w)
				if gridFocusID != "" {
					w.SetFocus(gridFocusID)
				}
				e.IsHandled = true
			},
		})
	}

	return gg.Row(gg.ContainerCfg{
		ID:        editorID + ":wrap",
		Focusable: true,
		FocusSkip: true,
		Sizing:    gg.FillFill,
		Padding:   gg.NoPadding,
		Spacing:   gg.SomeF(0),
		OnKeyDown: dataGridMakeEditorOnKeydown(cfg.ID, gridFocusID),
		Content:   []gg.View{editor},
	})
}

func dataGridMakeEditorOnKeydown(gridID string, gridFocusID string) func(*gg.Layout, *gg.Event, *gg.Window) {
	return func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
		if e.Modifiers != 0 || e.KeyCode != gg.KeyEscape {
			return
		}
		dataGridClearEditingRow(gridID, w)
		if gridFocusID != "" {
			w.SetFocus(gridFocusID)
		}
		e.IsHandled = true
	}
}

func dataGridTrackRowEditClick(gridID string, editEnabled bool, editorFocusBase string, colCount int, columns []GridColumnCfg, _ int, rowID string, gridFocusID string, e *gg.Event, w *gg.Window) {
	if !editEnabled || dataGridHasKeyboardModifiers(e) {
		return
	}
	firstColIdx := dataGridFirstEditableColumnIndexEx(columns)
	if firstColIdx < 0 {
		return
	}
	dgES := gg.StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	// Default zero state: absent entry means no prior row-click recorded.
	state := dgES.GetOr(gridID, dataGridEditState{})

	isDoubleClick := state.LastClickRowID == rowID && state.LastClickFrame > 0 &&
		e.FrameCount-state.LastClickFrame <= dataGridEditDoubleClickFrames
	if isDoubleClick {
		state.EditingRowID = rowID
		state.LastClickRowID = ""
		state.LastClickFrame = 0
		dgES.Set(gridID, state)
		editorFocusID := dataGridEditorFocusIDFromBase(editorFocusBase, colCount, firstColIdx)
		if editorFocusID != "" {
			w.SetFocus(editorFocusID)
		} else if gridFocusID != "" {
			w.SetFocus(gridFocusID)
		}
		return
	}
	if state.EditingRowID != "" && state.EditingRowID != rowID {
		state.EditingRowID = ""
	}
	state.LastClickRowID = rowID
	state.LastClickFrame = e.FrameCount
	dgES.Set(gridID, state)
}

// hasAnyModifiers checks if the event has any of the specified keyboard modifiers
func hasAnyModifiers(e *gg.Event, modifiers ...gg.Modifier) bool {
	return slices.ContainsFunc(modifiers, e.Modifiers.Has)
}

func dataGridHasKeyboardModifiers(e *gg.Event) bool {
	return hasAnyModifiers(e, gg.ModShift, gg.ModCtrl, gg.ModAlt, gg.ModSuper)
}

func dataGridFirstEditableColumnIndex(cfg *DataGridCfg, columns []GridColumnCfg) int {
	if !dataGridEditingEnabled(cfg) {
		return -1
	}
	return dataGridFirstEditableColumnIndexEx(columns)
}

func dataGridFirstEditableColumnIndexEx(columns []GridColumnCfg) int {
	for idx, col := range columns {
		if col.Editable {
			return idx
		}
	}
	return -1
}

// dataGridCellEditorFocusBaseID returns the focus-ID prefix for
// editor cells. Each cell appends its column index.
func dataGridCellEditorFocusBaseID(cfg *DataGridCfg, colCount int) string {
	if colCount <= 0 {
		return ""
	}
	return cfg.ID + ":efocus:"
}

func dataGridCellEditorFocusID(cfg *DataGridCfg, colCount, rowIdx, colIdx int) string {
	if colCount <= 0 || rowIdx < 0 || colIdx < 0 || colIdx >= colCount {
		return ""
	}
	return cfg.ID + ":efocus:" + strconv.Itoa(colIdx)
}

func dataGridEditorFocusIDFromBase(base string, colCount, colIdx int) string {
	if base == "" || colCount <= 0 || colIdx < 0 || colIdx >= colCount {
		return ""
	}
	return base + strconv.Itoa(colIdx)
}

func dataGridEditingRowID(gridID string, w *gg.Window) string {
	dgES := gg.StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	if state, ok := dgES.Get(gridID); ok {
		return state.EditingRowID
	}
	return ""
}

func dataGridSetEditingRow(gridID, rowID string, w *gg.Window) {
	dgES := gg.StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	// Default zero state: absent entry means no editing row was set yet.
	state := dgES.GetOr(gridID, dataGridEditState{})
	state.EditingRowID = rowID
	dgES.Set(gridID, state)
}

func dataGridClearEditingRow(gridID string, w *gg.Window) {
	dgES := gg.StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	// Default zero state: absent entry means no editing row to clear.
	state := dgES.GetOr(gridID, dataGridEditState{})
	state.EditingRowID = ""
	dgES.Set(gridID, state)
}

func dataGridEditorBoolValue(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

func dataGridParseEditorDate(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Now()
	}
	for _, layout := range []string{
		"1/2/2006",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC3339,
	} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed
		}
	}
	return time.Now()
}
