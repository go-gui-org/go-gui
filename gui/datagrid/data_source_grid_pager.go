package datagrid

import (
	"strconv"

	gg "github.com/go-gui-org/go-gui/gui"
)

func dataGridSourceRowsText(kind GridPaginationKind, state dataGridSourceState) string {
	if kind == GridPaginationOffset {
		return dataGridSourceFormatRows(state.OffsetStart, state.ReceivedCount, state.RowCount)
	}
	if start, ok := dataGridSourceCursorToIndexOpt(state.CurrentCursor); ok {
		return dataGridSourceFormatRows(start, state.ReceivedCount, state.RowCount)
	}
	totalText := "?"
	if state.RowCount != nil {
		totalText = strconv.Itoa(*state.RowCount)
	}
	return gg.ActiveLocale.StrRows + " " + strconv.Itoa(state.ReceivedCount) + "/" + totalText
}

func dataGridSourceFormatRows(start, count int, total *int) string {
	totalText := "?"
	if total != nil {
		totalText = strconv.Itoa(*total)
	}
	if count <= 0 {
		return gg.ActiveLocale.StrRows + " 0/" + totalText
	}
	end := start + count
	if total != nil && end > *total {
		end = *total
	}
	return gg.ActiveLocale.StrRows + " " + strconv.Itoa(start+1) + "-" + strconv.Itoa(end) + "/" + totalText
}

func dataGridSourceCanPrev(kind GridPaginationKind, state dataGridSourceState, pageLimit int) bool {
	if kind == GridPaginationCursor {
		return state.prevCursor != ""
	}
	return state.OffsetStart > 0 && pageLimit > 0
}

func dataGridSourceCanNext(kind GridPaginationKind, state dataGridSourceState, pageLimit int) bool {
	if kind == GridPaginationCursor {
		return state.nextCursor != ""
	}
	if state.RowCount != nil {
		return state.OffsetStart+state.ReceivedCount < *state.RowCount
	}
	if state.hasMore {
		return true
	}
	return state.ReceivedCount >= max(1, pageLimit)
}

func dataGridSourcePrevPage(gridID string, kind GridPaginationKind, pageLimit int, w *gg.Window) {
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	if state.Loading {
		return
	}
	if kind == GridPaginationCursor {
		if state.prevCursor == "" {
			return
		}
		state.CurrentCursor = state.prevCursor
	} else {
		if pageLimit <= 0 {
			return
		}
		state.OffsetStart = max(0, state.OffsetStart-pageLimit)
	}
	state.RequestKey = ""
	state.LoadError = ""
	dgSrc.Set(gridID, state)
	w.UpdateWindow()
}

func dataGridSourceNextPage(gridID string, kind GridPaginationKind, pageLimit int, w *gg.Window) {
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	if state.Loading {
		return
	}
	if kind == GridPaginationCursor {
		if state.nextCursor == "" {
			return
		}
		state.CurrentCursor = state.nextCursor
	} else {
		state.OffsetStart += max(1, pageLimit)
		if state.RowCount != nil {
			state.OffsetStart = min(state.OffsetStart, max(0, *state.RowCount-1))
		}
	}
	state.RequestKey = ""
	state.LoadError = ""
	dgSrc.Set(gridID, state)
	w.UpdateWindow()
}

func dataGridSourceJumpToRow(gridID string, targetIdx, pageLimit int, w *gg.Window) {
	if pageLimit <= 0 || targetIdx < 0 {
		return
	}
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	if state.Loading {
		return
	}
	state.PendingJumpRow = targetIdx
	pageStart := (targetIdx / pageLimit) * pageLimit
	if pageStart != state.OffsetStart {
		state.OffsetStart = pageStart
		state.RequestKey = ""
		state.LoadError = ""
	}
	dgSrc.Set(gridID, state)
	w.UpdateWindow()
}

func dataGridSourceRowPositionText(cfg *DataGridCfg, state dataGridSourceState, kind GridPaginationKind) string {
	totalText := "?"
	if state.RowCount != nil {
		totalText = strconv.Itoa(*state.RowCount)
	}
	if len(cfg.Rows) == 0 {
		return "Row 0 of " + totalText
	}
	localIdx := dataGridActiveRowIndexStrict(cfg.Rows, cfg.Selection)
	if localIdx < 0 || localIdx >= len(cfg.Rows) {
		localIdx = 0
	}
	current := localIdx + 1
	if kind == GridPaginationOffset {
		current = state.OffsetStart + localIdx + 1
	} else if start, ok := dataGridSourceCursorToIndexOpt(state.CurrentCursor); ok {
		current = start + localIdx + 1
	}
	if state.RowCount != nil {
		current = max(1, min(*state.RowCount, current))
	}
	return "Row " + strconv.Itoa(current) + " of " + totalText
}

func dataGridSourceJumpEnabled(onSelectionChange func(GridSelection, gg.EventCtx), rowCount *int, loading bool, loadError string, kind GridPaginationKind, pageLimit int) bool {
	if onSelectionChange == nil || pageLimit <= 0 {
		return false
	}
	if kind != GridPaginationOffset || loading || loadError != "" {
		return false
	}
	if rowCount != nil {
		return *rowCount > 0
	}
	return false
}

func dataGridSourceSubmitJump(onSelectionChange func(GridSelection, gg.EventCtx), rowCount *int, loading bool, loadError string, kind GridPaginationKind, pageLimit int, gridID string, focusID string, e *gg.Event, w *gg.Window) {
	if !dataGridSourceJumpEnabled(onSelectionChange, rowCount, loading, loadError, kind, pageLimit) {
		return
	}
	if rowCount == nil {
		return
	}
	total := *rowCount
	dgJI := gg.StateMap[string, string](w, nsDgJump, capModerate)
	// Default "": absent entry means no jump text typed yet.
	jumpText := dgJI.GetOr(gridID, "")
	targetIdx, ok := dataGridParseJumpTarget(jumpText, total)
	if !ok {
		return
	}
	dgJI.Set(gridID, strconv.Itoa(targetIdx+1))
	dataGridSourceJumpToRow(gridID, targetIdx, pageLimit, w)
	if focusID != "" {
		w.SetFocus(focusID)
	}
	e.IsHandled = true
}

func dataGridSourceRetry(gridID string, w *gg.Window) {
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	state.RequestKey = ""
	state.LoadError = ""
	dgSrc.Set(gridID, state)
	w.UpdateWindow()
}

func dataGridSourcePagerRow(cfg *DataGridCfg, focusID string, state dataGridSourceState, caps GridDataCapabilities, jumpText string) gg.View {
	kind := dataGridSourceEffectivePaginationKind(cfg.PaginationKind, caps)
	pageLimit := dataGridPageLimit(cfg)
	hasPrev := dataGridSourceCanPrev(kind, state, pageLimit)
	hasNext := dataGridSourceCanNext(kind, state, pageLimit)
	rowsText := dataGridSourceRowsText(kind, state)
	onSelectionChange := cfg.OnSelectionChange
	rowCount := state.RowCount
	loading := state.Loading
	loadError := state.LoadError
	jumpEnabled := dataGridSourceJumpEnabled(onSelectionChange, rowCount, loading, loadError, kind, pageLimit)
	var modeText string
	if kind == GridPaginationCursor {
		modeText = "Cursor"
	} else {
		modeText = "Offset"
	}
	var status string
	if state.Loading {
		status = gg.ActiveLocale.StrLoading
	} else if state.LoadError != "" {
		status = gg.ActiveLocale.StrError
	} else {
		status = modeText
	}

	gridID := cfg.ID
	jumpInputID := gg.ScopeID(gridID, "jump")
	content := make([]gg.View, 0, 10)

	// Prev button.
	content = append(content, dataGridIndicatorButton(gg.ScopeID(gridID, "src_prev"), "\u25C0", cfg.TextStyleHeader, cfg.ColorHeaderHover,
		state.Loading || !hasPrev, dataGridHeaderControlWidth+10, cfg.sounds.click, func(ctx gg.EventCtx) {
			dataGridSourcePrevPage(gridID, kind, pageLimit, ctx.Window)
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
			ctx.Consume()
		}))
	// Status.
	content = append(content, gg.Text(gg.TextCfg{
		Text:      status,
		Mode:      gg.TextModeSingleLine,
		TextStyle: cfg.TextStyleFilter,
	}))
	// Next button.
	content = append(content, dataGridIndicatorButton(gg.ScopeID(gridID, "src_next"), "\u25B6", cfg.TextStyleHeader, cfg.ColorHeaderHover,
		state.Loading || !hasNext, dataGridHeaderControlWidth+10, cfg.sounds.click, func(ctx gg.EventCtx) {
			dataGridSourceNextPage(gridID, kind, pageLimit, ctx.Window)
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
			ctx.Consume()
		}))
	// Spacer.
	content = append(content, gg.Row(gg.ContainerCfg{
		Sizing:  gg.FillFill,
		Padding: gg.NoPadding,
	}))
	// Retry button on error.
	if state.LoadError != "" {
		content = append(content, gg.Button(gg.ButtonCfg{
			ID:         gg.ScopeID(gridID, "src_retry"),
			Sizing:     gg.FitFill,
			Padding:    gg.NoPadding,
			SizeBorder: gg.SomeF(0),
			Radius:     gg.SomeF(0),
			Color:      gg.ColorTransparent,
			Colors:     gg.ColorSet{Base: gg.ColorTransparent, Hover: cfg.ColorHeaderHover, Click: cfg.ColorHeaderHover, Focus: gg.ColorTransparent, Border: gg.ColorTransparent, BorderFocus: gg.ColorTransparent},
			OnClick: func(ctx gg.EventCtx) {
				dataGridSourceRetry(gridID, ctx.Window)
				if focusID != "" {
					ctx.Window.SetFocus(focusID)
				}
			},
			Content: []gg.View{
				gg.Text(gg.TextCfg{
					Text:      "Retry",
					Mode:      gg.TextModeSingleLine,
					TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
				}),
			},
		}))
	}
	// Rows status.
	content = append(content, gg.Row(gg.ContainerCfg{
		Sizing:  gg.FitFill,
		Padding: gg.NewPadding(0, 6, 0, 0),
		VAlign:  gg.VAlignMiddle,
		Content: []gg.View{
			gg.Text(gg.TextCfg{
				Text:      rowsText,
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	}))
	// Jump input for offset mode.
	if kind == GridPaginationOffset {
		content = append(content, gg.Text(gg.TextCfg{
			Text:      gg.ActiveLocale.StrJump,
			Mode:      gg.TextModeSingleLine,
			TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
		}))
		content = append(content, gg.Input(gg.InputCfg{
			ID:          jumpInputID,
			Text:        jumpText,
			Placeholder: "#",
			Disabled:    !jumpEnabled,
			Width:       dataGridJumpInputWidth,
			Sizing:      gg.FixedFill,
			Padding:     gg.NoPadding,
			SizeBorder:  gg.SomeF(0),
			Radius:      gg.SomeF(0),
			Color:       cfg.ColorFilter,
			ColorHover:  cfg.ColorFilter,
			ColorBorder: cfg.ColorBorder,
			TextStyle:   cfg.TextStyleFilter,
			OnTextChanged: func(text string, ctx gg.EventCtx) {
				digits := dataGridJumpDigits(text)
				dgJI := gg.StateMap[string, string](ctx.Window, nsDgJump, capModerate)
				dgJI.Set(gridID, digits)
				e := &gg.Event{}
				dataGridSourceSubmitJump(onSelectionChange, rowCount, loading,
					loadError, kind, pageLimit, gridID, "", e, ctx.Window)
			},
			OnEnter: func(ctx gg.EventCtx) {
				dataGridSourceSubmitJump(onSelectionChange, rowCount, loading,
					loadError, kind, pageLimit, gridID, focusID, ctx.Event, ctx.Window)
			},
		}))
	}
	return gg.Row(gg.ContainerCfg{
		Height:      dataGridPagerHeight(cfg),
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  gg.SomeF(0),
		Padding:     dataGridPagerPadding(cfg),
		Spacing:     gg.SomeF(6),
		VAlign:      gg.VAlignMiddle,
		Content:     content,
	})
}

func dataGridSourceStatusRow(cfg *DataGridCfg, message string) gg.View {
	return gg.Row(gg.ContainerCfg{
		Height:      cfg.RowHeight,
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  gg.SomeF(0),
		Padding:     cfg.PaddingFilter,
		VAlign:      gg.VAlignMiddle,
		Content: []gg.View{
			gg.Text(gg.TextCfg{
				Text:      message,
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	})
}
