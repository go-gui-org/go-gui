package datagrid

import (
	"math"
	"slices"
	"strings"
	"time"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- Cell editors ---

func dataGridCellEditorView(cfg *DataGridCfg, rowID string, rowIdx int, col GridColumnCfg, value string, editorFocusID, gridFocusID uint32, _ *gg.Window) gg.View {
	editorID := cfg.ID + ":editor:" + rowID + ":" + col.ID
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
			IDFocus:    editorFocusID,
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
			IDFocus: editorFocusID,
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
			IDFocus:  editorFocusID,
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
			IDFocus:    editorFocusID,
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
				if gridFocusID > 0 {
					w.SetIDFocus(gridFocusID)
				}
				e.IsHandled = true
			},
		})
	}

	return gg.Row(gg.ContainerCfg{
		ID:        editorID + ":wrap",
		IDFocus:   editorFocusID,
		FocusSkip: true,
		Sizing:    gg.FillFill,
		Padding:   gg.NoPadding,
		Spacing:   gg.SomeF(0),
		OnKeyDown: dataGridMakeEditorOnKeydown(cfg.ID, gridFocusID),
		Content:   []gg.View{editor},
	})
}

func dataGridMakeEditorOnKeydown(gridID string, gridFocusID uint32) func(*gg.Layout, *gg.Event, *gg.Window) {
	return func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
		if e.Modifiers != 0 || e.KeyCode != gg.KeyEscape {
			return
		}
		dataGridClearEditingRow(gridID, w)
		if gridFocusID > 0 {
			w.SetIDFocus(gridFocusID)
		}
		e.IsHandled = true
	}
}

func dataGridTrackRowEditClick(gridID string, editEnabled bool, editorFocusBase uint32, colCount int, columns []GridColumnCfg, _ int, rowID string, gridFocusID uint32, e *gg.Event, w *gg.Window) {
	if !editEnabled || dataGridHasKeyboardModifiers(e) {
		return
	}
	firstColIdx := dataGridFirstEditableColumnIndexEx(columns)
	if firstColIdx < 0 {
		return
	}
	dgES := gg.StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	state, _ := dgES.Get(gridID)

	isDoubleClick := state.LastClickRowID == rowID && state.LastClickFrame > 0 &&
		e.FrameCount-state.LastClickFrame <= dataGridEditDoubleClickFrames
	if isDoubleClick {
		state.EditingRowID = rowID
		state.LastClickRowID = ""
		state.LastClickFrame = 0
		dgES.Set(gridID, state)
		editorFocusID := dataGridEditorFocusIDFromBase(editorFocusBase, colCount, firstColIdx)
		if editorFocusID > 0 {
			w.SetIDFocus(editorFocusID)
		} else if gridFocusID > 0 {
			w.SetIDFocus(gridFocusID)
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

// dataGridCellEditorFocusBaseID returns the first focus ID for
// editor cells. Header cells occupy [base+1 .. base+col_count];
// editor cells start at base+col_count+1.
func dataGridCellEditorFocusBaseID(cfg *DataGridCfg, colCount int) uint32 {
	if colCount <= 0 {
		return 0
	}
	headerBase := dataGridHeaderFocusBaseID(cfg, colCount)
	if headerBase == 0 {
		return 0
	}
	if headerBase > math.MaxUint32-uint32(colCount) {
		return 0
	}
	return headerBase + uint32(colCount)
}

func dataGridCellEditorFocusID(cfg *DataGridCfg, colCount, rowIdx, colIdx int) uint32 {
	if colCount <= 0 || rowIdx < 0 || colIdx < 0 || colIdx >= colCount {
		return 0
	}
	base := dataGridCellEditorFocusBaseID(cfg, colCount)
	if base == 0 {
		return 0
	}
	cellOffset := uint64(colIdx)
	if cellOffset > uint64(math.MaxUint32-base) {
		return 0
	}
	return base + uint32(cellOffset)
}

func dataGridEditorFocusIDFromBase(base uint32, colCount, colIdx int) uint32 {
	if base == 0 || colCount <= 0 || colIdx < 0 || colIdx >= colCount {
		return 0
	}
	cellOffset := uint64(colIdx)
	if cellOffset > uint64(math.MaxUint32-base) {
		return 0
	}
	return base + uint32(cellOffset)
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
	state, _ := dgES.Get(gridID)
	state.EditingRowID = rowID
	dgES.Set(gridID, state)
}

func dataGridClearEditingRow(gridID string, w *gg.Window) {
	dgES := gg.StateMap[string, dataGridEditState](w, nsDgEdit, capModerate)
	state, _ := dgES.Get(gridID)
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
