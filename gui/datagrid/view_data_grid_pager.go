package datagrid

import (
	gg "github.com/go-gui-org/go-gui/gui"
)

// --- Pager ---

type dataGridPagerContext struct {
	cfg           *DataGridCfg
	dataToDisplay map[int]int
	jumpText      string
	pageIndex     int
	pageCount     int
	pageStart     int
	pageEnd       int
	totalRows     int
	focusID       string
	viewportH     float32
	rowHeight     float32
	staticTop     float32
	scrollID      string
}

func dataGridPagerRow(cfg *DataGridCfg, focusID string, pageIndex, pageCount, pageStart, pageEnd, totalRows int, viewportH, rowHeight, staticTop float32, scrollID string, dataToDisplay map[int]int, jumpText string) gg.View {
	return dataGridBuildPagerRow(dataGridPagerContext{
		cfg: cfg, focusID: focusID, pageIndex: pageIndex, pageCount: pageCount,
		pageStart: pageStart, pageEnd: pageEnd, totalRows: totalRows,
		viewportH: viewportH, rowHeight: rowHeight, staticTop: staticTop,
		scrollID: scrollID, dataToDisplay: dataToDisplay, jumpText: jumpText,
	})
}

func dataGridBuildPagerRow(pctx dataGridPagerContext) gg.View {
	cfg := pctx.cfg
	content := dataGridPagerContent(pctx)
	return gg.Row(gg.ContainerCfg{
		Height:      dataGridPagerHeight(cfg),
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  gg.SomeF(0),
		Padding:     gg.Some(dataGridPagerPadding(cfg)),
		Spacing:     gg.SomeF(6),
		VAlign:      gg.VAlignMiddle,
		Content:     content,
	})
}

func dataGridPagerContent(pctx dataGridPagerContext) []gg.View {
	cfg := pctx.cfg
	onPageChange := cfg.OnPageChange
	isFirst := pctx.pageIndex <= 0
	isLast := pctx.pageIndex >= pctx.pageCount-1
	pageText := gg.LocalePageFmt(pctx.pageIndex+1, pctx.pageCount)
	rowsText := dataGridPagerRowsText(pctx.pageStart, pctx.pageEnd, pctx.totalRows)
	jumpCtx := dataGridJumpContextFromPager(pctx)
	jumpEnabled := dataGridJumpEnabledLocal(len(cfg.Rows), cfg.OnSelectionChange, cfg.OnPageChange, cfg.PageSize, pctx.totalRows)
	jumpInputID := cfg.ID + ":jump"
	jumpFocusID := jumpInputID
	prevArrow, nextArrow := dataGridPagerArrows()

	content := make([]gg.View, 0, 9)
	content = append(content, dataGridPagerPrevButton(cfg, onPageChange, pctx.pageIndex, pctx.focusID, isFirst, prevArrow))
	content = append(content, gg.Text(gg.TextCfg{
		Text:      pageText,
		Mode:      gg.TextModeSingleLine,
		TextStyle: cfg.TextStyleFilter,
	}))
	content = append(content, dataGridPagerNextButton(cfg, onPageChange, pctx.pageIndex, pctx.pageCount, pctx.focusID, isLast, nextArrow))
	content = append(content, dataGridPagerSpacer())
	content = append(content, dataGridPagerRowsStatus(cfg, rowsText))
	content = append(content, dataGridPagerJumpLabel(cfg))
	content = append(content, dataGridPagerJumpInput(cfg, jumpInputID, jumpFocusID, pctx.jumpText, jumpEnabled, jumpCtx, pctx.focusID))
	return content
}

func dataGridPagerRowsText(pageStart, pageEnd, totalRows int) string {
	if totalRows == 0 || pageEnd <= pageStart {
		return gg.ActiveLocale.StrRows + " 0/0"
	}
	return gg.LocaleRowsFmt(pageStart+1, pageEnd, totalRows)
}

func dataGridPagerArrows() (string, string) {
	prev := "◀" // ◀
	next := "▶" // ▶
	if gg.ActiveLocale.TextDir == gg.TextDirRTL {
		prev, next = next, prev
	}
	return prev, next
}

func dataGridPagerPrevButton(cfg *DataGridCfg, onPageChange func(int, gg.EventCtx), pageIndex int, focusID string, isFirst bool, prevArrow string) gg.View {
	return dataGridIndicatorButton(prevArrow, cfg.TextStyleHeader, cfg.ColorHeaderHover,
		onPageChange == nil || isFirst, dataGridHeaderControlWidth+10,
		func(ctx gg.EventCtx) {
			if onPageChange == nil {
				return
			}
			next := max(0, pageIndex-1)
			onPageChange(next, gg.EventCtx{Layout: nil, Event: ctx.Event, Window: ctx.Window})
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
			ctx.Consume()
		})
}

func dataGridPagerNextButton(cfg *DataGridCfg, onPageChange func(int, gg.EventCtx), pageIndex, pageCount int, focusID string, isLast bool, nextArrow string) gg.View {
	return dataGridIndicatorButton(nextArrow, cfg.TextStyleHeader, cfg.ColorHeaderHover,
		onPageChange == nil || isLast, dataGridHeaderControlWidth+10,
		func(ctx gg.EventCtx) {
			if onPageChange == nil {
				return
			}
			next := min(pageCount-1, pageIndex+1)
			onPageChange(next, gg.EventCtx{Layout: nil, Event: ctx.Event, Window: ctx.Window})
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
			ctx.Consume()
		})
}

func dataGridPagerSpacer() gg.View {
	return gg.Row(gg.ContainerCfg{Sizing: gg.FillFill, Padding: gg.NoPadding})
}

func dataGridPagerRowsStatus(cfg *DataGridCfg, rowsText string) gg.View {
	return gg.Row(gg.ContainerCfg{
		Sizing:  gg.FitFill,
		Padding: gg.SomeP(0, 6, 0, 0),
		VAlign:  gg.VAlignMiddle,
		Content: []gg.View{
			gg.Text(gg.TextCfg{
				Text:      rowsText,
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	})
}

func dataGridPagerJumpLabel(cfg *DataGridCfg) gg.View {
	return gg.Text(gg.TextCfg{
		Text:      gg.ActiveLocale.StrJump,
		Mode:      gg.TextModeSingleLine,
		TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
	})
}

func dataGridJumpContextFromPager(pctx dataGridPagerContext) dataGridJumpContext {
	cfg := pctx.cfg
	return dataGridJumpContext{
		rows:              cfg.Rows,
		onSelectionChange: cfg.OnSelectionChange,
		onPageChange:      cfg.OnPageChange,
		pageSize:          cfg.PageSize,
		totalRows:         pctx.totalRows,
		pageIndex:         pctx.pageIndex,
		viewportH:         pctx.viewportH,
		rowHeight:         pctx.rowHeight,
		staticTop:         pctx.staticTop,
		scrollID:          pctx.scrollID,
		dataToDisplay:     pctx.dataToDisplay,
		gridID:            cfg.ID,
	}
}

func dataGridPagerJumpInput(cfg *DataGridCfg, inputID string, focusID string, jumpText string, jumpEnabled bool, jumpCtx dataGridJumpContext, gridFocusID string) gg.View {
	return gg.Input(gg.InputCfg{
		ID:          inputID,
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
		OnTextChanged: func(inputText string, ctx gg.EventCtx) {
			digits := dataGridJumpDigits(inputText)
			dgJI := gg.StateMap[string, string](ctx.Window, nsDgJump, capModerate)
			dgJI.Set(jumpCtx.gridID, digits)
			e := &gg.Event{}
			dataGridSubmitLocalJump(jumpCtx, e, ctx.Window)
		},
		OnEnter: func(ectx gg.EventCtx) {
			ctx := jumpCtx
			ctx.focusID = gridFocusID
			dataGridSubmitLocalJump(ctx, ectx.Event, ectx.Window)
		},
	})
}
