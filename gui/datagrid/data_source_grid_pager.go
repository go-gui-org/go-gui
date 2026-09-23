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
	return gg.CurrentLocale().StrRows + " " + strconv.Itoa(state.ReceivedCount) + "/" + totalText
}

func dataGridSourceFormatRows(start, count int, total *int) string {
	totalText := "?"
	if total != nil {
		totalText = strconv.Itoa(*total)
	}
	if count <= 0 {
		return gg.CurrentLocale().StrRows + " 0/" + totalText
	}
	end := start + count
	if total != nil && end > *total {
		end = *total
	}
	return gg.CurrentLocale().StrRows + " " + strconv.Itoa(start+1) + "-" + strconv.Itoa(end) + "/" + totalText
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
	w.InvalidateLayout()
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
		if state.RowCount != nil && pageLimit > 0 {
			// Clamp to the last page start, not the last row:
			// RowCount-1 lands mid-page (99 with limit 10).
			total := max(0, *state.RowCount)
			lastStart := 0
			if total > 0 {
				lastStart = ((total - 1) / pageLimit) * pageLimit
			}
			state.OffsetStart = min(state.OffsetStart, max(0, lastStart))
		}
	}
	state.RequestKey = ""
	state.LoadError = ""
	dgSrc.Set(gridID, state)
	w.InvalidateLayout()
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
	w.InvalidateLayout()
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
	w.InvalidateLayout()
}

func dataGridSourcePagerRow(cfg *DataGridCfg, focusID string, state dataGridSourceState, caps GridDataCapabilities, jumpText string) gg.View {
	kind := dataGridSourceEffectivePaginationKind(cfg.PaginationKind, caps)
	pageLimit := dataGridPageLimit(cfg)
	content := make([]gg.View, 0, 10)
	content = append(content, dataGridSourcePagerNav(cfg, focusID, kind, pageLimit, state)...)
	if retry := dataGridSourcePagerRetry(cfg, focusID, state); retry != nil {
		content = append(content, retry)
	}
	content = append(content, dataGridPagerRowsStatus(cfg,
		dataGridSourceRowsText(kind, state)))
	if jump := dataGridSourcePagerJump(cfg, focusID, state, caps, kind, pageLimit, jumpText); jump != nil {
		content = append(content, jump...)
	}
	return dataGridPagerShell(cfg, content)
}

// dataGridSourcePagerNav builds the prev/mode/next buttons plus the
// spacer that pushes the row counts right.
func dataGridSourcePagerNav(cfg *DataGridCfg, focusID string, kind GridPaginationKind, pageLimit int, state dataGridSourceState) []gg.View {
	gridID := cfg.ID
	hasPrev := dataGridSourceCanPrev(kind, state, pageLimit)
	hasNext := dataGridSourceCanNext(kind, state, pageLimit)
	var modeText string
	if kind == GridPaginationCursor {
		modeText = "Cursor"
	} else {
		modeText = "Offset"
	}
	var status string
	if state.Loading {
		status = gg.CurrentLocale().StrLoading
	} else if state.LoadError != "" {
		status = gg.CurrentLocale().StrError
	} else {
		status = modeText
	}
	// Shared arrows: the local pager swaps them for RTL locales.
	prevArrow, nextArrow := dataGridPagerArrows()
	return []gg.View{
		dataGridIndicatorButton(gg.ScopeID(gridID, "src_prev"), prevArrow, cfg.TextStyleHeader, cfg.ColorsHeader.Hover,
			state.Loading || !hasPrev, dataGridHeaderControlWidth+10, cfg.sounds.click, func(ctx gg.EventCtx) {
				dataGridSourcePrevPage(gridID, kind, pageLimit, ctx.Window)
				if focusID != "" {
					ctx.Window.SetFocus(focusID)
				}
				ctx.Consume()
			}),
		gg.Text(gg.TextCfg{
			Text:      status,
			Mode:      gg.TextModeSingleLine,
			TextStyle: cfg.TextStyleFilter,
		}),
		dataGridIndicatorButton(gg.ScopeID(gridID, "src_next"), nextArrow, cfg.TextStyleHeader, cfg.ColorsHeader.Hover,
			state.Loading || !hasNext, dataGridHeaderControlWidth+10, cfg.sounds.click, func(ctx gg.EventCtx) {
				dataGridSourceNextPage(gridID, kind, pageLimit, ctx.Window)
				if focusID != "" {
					ctx.Window.SetFocus(focusID)
				}
				ctx.Consume()
			}),
		dataGridPagerSpacer(),
	}
}

// dataGridSourcePagerRetry builds the error retry button, or nil
// when there is no load error.
func dataGridSourcePagerRetry(cfg *DataGridCfg, focusID string, state dataGridSourceState) gg.View {
	if state.LoadError == "" {
		return nil
	}
	gridID := cfg.ID
	return gg.Button(gg.ButtonCfg{
		ID:         gg.ScopeID(gridID, "src_retry"),
		Sizing:     gg.FitFill,
		Padding:    gg.NoPadding,
		SizeBorder: gg.SomeF(0),
		Radius:     gg.SomeF(0),
		Color:      gg.ColorTransparent,
		Colors:     gg.ColorSet{Base: gg.ColorTransparent, Hover: cfg.ColorsHeader.Hover, Click: cfg.ColorsHeader.Hover, Focus: gg.ColorTransparent, Border: gg.ColorTransparent, BorderFocus: gg.ColorTransparent},
		OnClick: func(ctx gg.EventCtx) {
			dataGridSourceRetry(gridID, ctx.Window)
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
			ctx.Consume()
		},
		Content: []gg.View{
			gg.Text(gg.TextCfg{
				Text:      "Retry",
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	})
}

// dataGridSourcePagerJump builds the jump label and input for offset
// mode, or nil otherwise.
func dataGridSourcePagerJump(cfg *DataGridCfg, focusID string, state dataGridSourceState, caps GridDataCapabilities, kind GridPaginationKind, pageLimit int, jumpText string) []gg.View {
	if kind != GridPaginationOffset {
		return nil
	}
	gridID := cfg.ID
	onSelectionChange := cfg.OnSelectionChange
	rowCount := state.RowCount
	loading := state.Loading
	loadError := state.LoadError
	jumpEnabled := dataGridSourceJumpEnabled(onSelectionChange, rowCount, loading, loadError, kind, pageLimit)
	jumpInputID := gg.ScopeID(gridID, "jump")
	return []gg.View{
		dataGridPagerJumpLabel(cfg),
		gg.Input(gg.InputCfg{
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
			Colors:      gg.ColorSet{Hover: cfg.ColorFilter, Border: cfg.ColorsRow.Border},
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
		}),
	}
}

func dataGridSourceStatusRow(cfg *DataGridCfg, message string) gg.View {
	return gg.Row(gg.ContainerCfg{
		Height:      cfg.RowHeight,
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorsRow.Border,
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
