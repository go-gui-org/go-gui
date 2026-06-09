package datagrid

import (
	"strconv"

	. "github.com/go-gui-org/go-gui/gui"
)

// SourceStats provides runtime stats for a data-source-backed grid.
type SourceStats struct {
	RowCount       *int
	LoadError      string
	RequestCount   int
	CancelledCount int
	StaleDropCount int
	ReceivedCount  int
	Loading        bool
	HasMore        bool
}

// GetSourceStats returns async stats for the named grid.
func GetSourceStats(w *Window, gridID string) SourceStats {
	dgSrc := StateMapRead[string, dataGridSourceState](w, nsDgSource)
	if dgSrc == nil {
		return SourceStats{}
	}
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return SourceStats{}
	}
	return SourceStats{
		Loading:        state.Loading,
		LoadError:      state.LoadError,
		RequestCount:   state.RequestCount,
		CancelledCount: state.CancelledCount,
		StaleDropCount: state.StaleDropCount,
		HasMore:        state.HasMore,
		ReceivedCount:  state.ReceivedCount,
		RowCount:       state.RowCount,
	}
}

func dataGridSourceApplyLocalMutation(gridID string, rows []GridRow, rowCount int, w *Window) {
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	// ok ignored: zero state → cancelActive returns immediately,
	// then state is fully overwritten below.
	state, _ := dgSrc.Get(gridID)
	dataGridSourceCancelActive(&state)
	rows = dataGridSourceRowsWithStableIDs(rows, state.PaginationKind, state)
	state.RequestID++
	state.Rows = rows
	state.ReceivedCount = len(rows)
	state.HasLoaded = true
	state.Loading = false
	state.LoadError = ""
	state.RowsDirty = true
	state.RowsSignature = dataGridRowsSignature(rows, nil)
	state.ActiveAbort = nil
	if rowCount >= 0 {
		rc := rowCount
		state.RowCount = &rc
	} else {
		state.RowCount = nil
	}
	dgSrc.Set(gridID, state)
}

func dataGridSourceCancelActive(state *dataGridSourceState) {
	if !state.Loading || state.ActiveAbort == nil {
		return
	}
	state.ActiveAbort.Abort()
	state.CancelledCount++
}

func dataGridSourceForceRefetch(gridID string, w *Window) {
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	dataGridSourceCancelActive(&state)
	state.Loading = false
	state.RequestKey = ""
	state.LoadError = ""
	state.CapsCached = false
	state.ActiveAbort = nil
	dgSrc.Set(gridID, state)
	w.UpdateWindow()
}

func dataGridResolveSourceCfg(cfg DataGridCfg, w *Window) (DataGridCfg, dataGridSourceState, bool, GridDataCapabilities) {
	source := cfg.DataSource
	if source == nil {
		return cfg, dataGridSourceState{}, false, GridDataCapabilities{}
	}

	// Use cached capabilities when available.
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	// ok ignored: zero CapsCached (false) triggers fresh Capabilities() call.
	existing, _ := dgSrc.Get(cfg.ID)
	var caps GridDataCapabilities
	if existing.CapsCached {
		caps = existing.CachedCaps
	} else {
		caps = source.Capabilities()
	}
	wasDirty := existing.RowsDirty
	state := dataGridSourceResolveState(cfg, caps, dgSrc, w)

	rowCount := cfg.RowCount
	if state.RowCount != nil {
		rc := *state.RowCount
		rowCount = &rc
	}
	var rows []GridRow
	if wasDirty {
		rows = cloneRows(state.Rows)
	} else {
		rows = state.Rows
	}
	rows = dataGridSourceRowsWithStableIDs(rows, state.PaginationKind, state)
	resolved := cfg
	resolved.Rows = rows
	resolved.PageSize = 0
	resolved.PageIndex = 0
	resolved.Loading = state.Loading
	resolved.LoadError = state.LoadError
	resolved.RowCount = rowCount
	return resolved, state, true, caps
}

func dataGridSourceResolveState(cfg DataGridCfg, caps GridDataCapabilities, dgSrc *BoundedMap[string, dataGridSourceState], w *Window) dataGridSourceState {
	state, ok := dgSrc.Get(cfg.ID)
	if !ok {
		state = dataGridSourceState{
			CurrentCursor:  cfg.Cursor,
			OffsetStart:    max(0, cfg.PageIndex*dataGridPageLimit(&cfg)),
			PaginationKind: cfg.PaginationKind,
			ConfigCursor:   cfg.Cursor,
		}
	}
	if !state.CapsCached {
		state.CachedCaps = caps
		state.CapsCached = true
	}
	kind := dataGridSourceEffectivePaginationKind(cfg.PaginationKind, caps)
	if state.PaginationKind != kind {
		state.PaginationKind = kind
		dataGridSourceResetPagination(&state, cfg.Cursor)
		state.Rows = nil
	}
	if cfg.Cursor != state.ConfigCursor {
		state.ConfigCursor = cfg.Cursor
		state.CurrentCursor = cfg.Cursor
		state.RequestKey = ""
	}
	querySig := gridQuerySignature(cfg.Query)
	dataGridSourceApplyQueryReset(&state, &cfg, querySig)
	if kind == GridPaginationOffset && cfg.PageSize > 0 {
		desiredStart := max(0, cfg.PageIndex*cfg.PageSize)
		if desiredStart != state.OffsetStart {
			state.OffsetStart = desiredStart
			state.RequestKey = ""
		}
	}
	requestKey := dataGridSourceRequestKey(&cfg, state, kind, querySig)
	if requestKey != state.RequestKey {
		dataGridSourceStartRequest(cfg, caps, kind, requestKey, &state, w)
	}
	state.RowsDirty = false
	dgSrc.Set(cfg.ID, state)
	return state
}

func dataGridSourceApplyPendingJumpSelection(cfg *DataGridCfg, state dataGridSourceState, w *Window) {
	if cfg.OnSelectionChange == nil || state.PendingJumpRow < 0 {
		return
	}
	if state.Loading {
		return
	}
	localIdx := state.PendingJumpRow - state.OffsetStart
	if localIdx < 0 || localIdx >= len(cfg.Rows) {
		return
	}
	rowID := dataGridRowID(cfg.Rows[localIdx], localIdx)
	next := GridSelection{
		AnchorRowID:    rowID,
		ActiveRowID:    rowID,
		SelectedRowIDs: map[string]bool{rowID: true},
	}
	e := &Event{}
	cfg.OnSelectionChange(next, e, w)
	dataGridSetAnchor(cfg.ID, rowID, w)
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	nextState, ok := dgSrc.Get(cfg.ID)
	if !ok {
		return
	}
	nextState.PendingJumpRow = -1
	dgSrc.Set(cfg.ID, nextState)
}

func dataGridSourceApplyQueryReset(state *dataGridSourceState, cfg *DataGridCfg, querySig uint64) {
	if querySig == state.QuerySignature {
		return
	}
	state.QuerySignature = querySig
	dataGridSourceResetPagination(state, cfg.Cursor)
	state.PendingJumpRow = -1
}

func dataGridSourceResetPagination(state *dataGridSourceState, cursor string) {
	state.CurrentCursor = cursor
	state.NextCursor = ""
	state.PrevCursor = ""
	state.OffsetStart = 0
	state.RequestKey = ""
}

func dataGridSourceEffectivePaginationKind(preferred GridPaginationKind, caps GridDataCapabilities) GridPaginationKind {
	if preferred == GridPaginationCursor {
		if caps.SupportsCursorPagination {
			return GridPaginationCursor
		}
		if caps.SupportsOffsetPagination {
			return GridPaginationOffset
		}
		return GridPaginationNone
	}
	if caps.SupportsOffsetPagination {
		return GridPaginationOffset
	}
	if caps.SupportsCursorPagination {
		return GridPaginationCursor
	}
	return GridPaginationNone
}

func dataGridPageLimit(cfg *DataGridCfg) int {
	if cfg.PageLimit > 0 {
		return cfg.PageLimit
	}
	if cfg.PageSize > 0 {
		return cfg.PageSize
	}
	return dataGridDefaultPageLimit
}

func dataGridSourceRequestKey(cfg *DataGridCfg, state dataGridSourceState, kind GridPaginationKind, querySig uint64) string {
	limit := dataGridPageLimit(cfg)
	switch kind {
	case GridPaginationCursor:
		return "k:cursor|cursor:" + state.CurrentCursor + "|limit:" + strconv.Itoa(limit) + "|q:" + strconv.FormatUint(querySig, 10)
	default: // offset
		end := state.OffsetStart + limit
		return "k:offset|start:" + strconv.Itoa(state.OffsetStart) + "|end:" + strconv.Itoa(end) + "|q:" + strconv.FormatUint(querySig, 10)
	}
}

func dataGridSourceStartRequest(cfg DataGridCfg, caps GridDataCapabilities, kind GridPaginationKind, requestKey string, state *dataGridSourceState, w *Window) {
	source := cfg.DataSource
	if source == nil {
		return
	}
	dataGridSourceCancelActive(state)
	limit := dataGridPageLimit(&cfg)
	controller := NewGridAbortController()
	nextRequestID := state.RequestID + 1
	var page gridPageRequest
	switch kind {
	case GridPaginationCursor:
		page = gridCursorPageReq{
			Cursor: state.CurrentCursor,
			Limit:  limit,
		}
	default:
		page = gridOffsetPageReq{
			StartIndex: state.OffsetStart,
			EndIndex:   state.OffsetStart + limit,
		}
	}
	req := GridDataRequest{
		GridID:    cfg.ID,
		Query:     cfg.Query,
		Page:      page,
		Signal:    controller.Signal,
		RequestID: nextRequestID,
	}
	state.Loading = true
	state.LoadError = ""
	state.RequestID = nextRequestID
	state.RequestKey = requestKey
	state.ActiveAbort = controller
	state.RequestCount++
	state.PaginationKind = kind

	gridID := cfg.ID
	go func() {
		if req.Signal.IsAborted() {
			return
		}
		result, err := source.FetchData(req)
		if req.Signal.IsAborted() {
			return
		}
		if err != nil {
			errMsg := err.Error()
			w.QueueCommand(func(w *Window) {
				dataGridSourceApplyError(gridID, nextRequestID, errMsg, w)
			})
			return
		}
		w.QueueCommand(func(w *Window) {
			dataGridSourceApplySuccess(gridID, nextRequestID, result, caps, w)
		})
	}()
}

func dataGridSourceDropIfStale(requestID uint64, state *dataGridSourceState, w *Window, gridID string) bool {
	if requestID != state.RequestID {
		state.StaleDropCount++
		dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
		dgSrc.Set(gridID, *state)
		return true
	}
	return false
}

func dataGridSourceApplySuccess(gridID string, requestID uint64, result GridDataResult, caps GridDataCapabilities, w *Window) {
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	if dataGridSourceDropIfStale(requestID, &state, w, gridID) {
		return
	}
	result.Rows = dataGridSourceRowsWithStableIDs(result.Rows, state.PaginationKind, state)
	state.Loading = false
	state.LoadError = ""
	state.HasLoaded = true
	state.RowsSignature = dataGridRowsSignature(result.Rows, nil)
	state.RowsDirty = true
	state.Rows = result.Rows
	state.NextCursor = result.NextCursor
	state.PrevCursor = result.PrevCursor
	state.HasMore = result.HasMore
	if result.ReceivedCount > 0 {
		state.ReceivedCount = result.ReceivedCount
	} else {
		state.ReceivedCount = len(result.Rows)
	}
	if result.RowCount >= 0 {
		rc := result.RowCount
		state.RowCount = &rc
	} else if !caps.RowCountKnown {
		state.RowCount = nil
	}
	state.ActiveAbort = nil
	dgSrc.Set(gridID, state)
	w.UpdateWindow()
}

func dataGridSourceRowsWithStableIDs(rows []GridRow, kind GridPaginationKind, state dataGridSourceState) []GridRow {
	if len(rows) == 0 {
		return rows
	}
	needsClone := false
	for _, row := range rows {
		if row.ID == "" {
			needsClone = true
			break
		}
	}
	if !needsClone {
		return rows
	}
	out := cloneRows(rows)
	for localIdx := range out {
		if out[localIdx].ID != "" {
			continue
		}
		out[localIdx].ID = dataGridSourceSyntheticRowID(kind, state, localIdx)
	}
	return out
}

func dataGridSourceSyntheticRowID(kind GridPaginationKind, state dataGridSourceState, localIdx int) string {
	localIdx = max(localIdx, 0)
	switch kind {
	case GridPaginationOffset:
		absIdx := max(0, state.OffsetStart) + localIdx
		return "__src_o_" + strconv.Itoa(absIdx)
	default:
		if start, ok := dataGridSourceCursorToIndexOpt(state.CurrentCursor); ok {
			return "__src_c_" + strconv.Itoa(max(0, start)+localIdx)
		}
		h := Fnv64Str(Fnv64Offset, state.CurrentCursor)
		return "__src_cx_" + zeroPadHex16(h) + "_" + strconv.Itoa(localIdx)
	}
}

func dataGridSourceApplyError(gridID string, requestID uint64, errMsg string, w *Window) {
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	if dataGridSourceDropIfStale(requestID, &state, w, gridID) {
		return
	}
	state.Loading = false
	state.LoadError = errMsg
	state.ActiveAbort = nil
	dgSrc.Set(gridID, state)
	w.UpdateWindow()
}

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
	return ActiveLocale.StrRows + " " + strconv.Itoa(state.ReceivedCount) + "/" + totalText
}

func dataGridSourceFormatRows(start, count int, total *int) string {
	totalText := "?"
	if total != nil {
		totalText = strconv.Itoa(*total)
	}
	if count <= 0 {
		return ActiveLocale.StrRows + " 0/" + totalText
	}
	end := start + count
	if total != nil && end > *total {
		end = *total
	}
	return ActiveLocale.StrRows + " " + strconv.Itoa(start+1) + "-" + strconv.Itoa(end) + "/" + totalText
}

func dataGridSourceCanPrev(kind GridPaginationKind, state dataGridSourceState, pageLimit int) bool {
	if kind == GridPaginationCursor {
		return state.PrevCursor != ""
	}
	return state.OffsetStart > 0 && pageLimit > 0
}

func dataGridSourceCanNext(kind GridPaginationKind, state dataGridSourceState, pageLimit int) bool {
	if kind == GridPaginationCursor {
		return state.NextCursor != ""
	}
	if state.RowCount != nil {
		return state.OffsetStart+state.ReceivedCount < *state.RowCount
	}
	if state.HasMore {
		return true
	}
	return state.ReceivedCount >= max(1, pageLimit)
}

func dataGridSourcePrevPage(gridID string, kind GridPaginationKind, pageLimit int, w *Window) {
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	if state.Loading {
		return
	}
	if kind == GridPaginationCursor {
		if state.PrevCursor == "" {
			return
		}
		state.CurrentCursor = state.PrevCursor
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

func dataGridSourceNextPage(gridID string, kind GridPaginationKind, pageLimit int, w *Window) {
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	if state.Loading {
		return
	}
	if kind == GridPaginationCursor {
		if state.NextCursor == "" {
			return
		}
		state.CurrentCursor = state.NextCursor
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

func dataGridSourceJumpToRow(gridID string, targetIdx, pageLimit int, w *Window) {
	if pageLimit <= 0 || targetIdx < 0 {
		return
	}
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
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

func dataGridSourceJumpEnabled(onSelectionChange func(GridSelection, *Event, *Window), rowCount *int, loading bool, loadError string, kind GridPaginationKind, pageLimit int) bool {
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

func dataGridSourceSubmitJump(onSelectionChange func(GridSelection, *Event, *Window), rowCount *int, loading bool, loadError string, kind GridPaginationKind, pageLimit int, gridID string, focusID uint32, e *Event, w *Window) {
	if !dataGridSourceJumpEnabled(onSelectionChange, rowCount, loading, loadError, kind, pageLimit) {
		return
	}
	if rowCount == nil {
		return
	}
	total := *rowCount
	dgJI := StateMap[string, string](w, nsDgJump, capModerate)
	jumpText, _ := dgJI.Get(gridID) // ok ignored: empty → parseJumpTarget returns (0, false)
	targetIdx, ok := dataGridParseJumpTarget(jumpText, total)
	if !ok {
		return
	}
	dgJI.Set(gridID, strconv.Itoa(targetIdx+1))
	dataGridSourceJumpToRow(gridID, targetIdx, pageLimit, w)
	if focusID > 0 {
		w.SetIDFocus(focusID)
	}
	e.IsHandled = true
}

func dataGridSourceRetry(gridID string, w *Window) {
	dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	state, ok := dgSrc.Get(gridID)
	if !ok {
		return
	}
	state.RequestKey = ""
	state.LoadError = ""
	dgSrc.Set(gridID, state)
	w.UpdateWindow()
}

func dataGridSourcePagerRow(cfg *DataGridCfg, focusID uint32, state dataGridSourceState, caps GridDataCapabilities, jumpText string) View {
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
		status = ActiveLocale.StrLoading
	} else if state.LoadError != "" {
		status = ActiveLocale.StrError
	} else {
		status = modeText
	}

	gridID := cfg.ID
	jumpInputID := gridID + ":jump"
	content := make([]View, 0, 10)

	// Prev button.
	content = append(content, dataGridIndicatorButton("\u25C0", cfg.TextStyleHeader, cfg.ColorHeaderHover,
		state.Loading || !hasPrev, dataGridHeaderControlWidth+10, func(_ *Layout, e *Event, w *Window) {
			dataGridSourcePrevPage(gridID, kind, pageLimit, w)
			if focusID > 0 {
				w.SetIDFocus(focusID)
			}
			e.IsHandled = true
		}))
	// Status.
	content = append(content, Text(TextCfg{
		Text:      status,
		Mode:      TextModeSingleLine,
		TextStyle: cfg.TextStyleFilter,
	}))
	// Next button.
	content = append(content, dataGridIndicatorButton("\u25B6", cfg.TextStyleHeader, cfg.ColorHeaderHover,
		state.Loading || !hasNext, dataGridHeaderControlWidth+10, func(_ *Layout, e *Event, w *Window) {
			dataGridSourceNextPage(gridID, kind, pageLimit, w)
			if focusID > 0 {
				w.SetIDFocus(focusID)
			}
			e.IsHandled = true
		}))
	// Spacer.
	content = append(content, Row(ContainerCfg{
		Sizing:  FillFill,
		Padding: NoPadding,
	}))
	// Retry button on error.
	if state.LoadError != "" {
		content = append(content, Button(ButtonCfg{
			Sizing:      FitFill,
			Padding:     NoPadding,
			SizeBorder:  SomeF(0),
			Radius:      SomeF(0),
			Color:       ColorTransparent,
			ColorHover:  cfg.ColorHeaderHover,
			ColorFocus:  ColorTransparent,
			ColorClick:  cfg.ColorHeaderHover,
			ColorBorder: ColorTransparent,
			OnClick: func(_ *Layout, e *Event, w *Window) {
				dataGridSourceRetry(gridID, w)
				if focusID > 0 {
					w.SetIDFocus(focusID)
				}
				e.IsHandled = true
			},
			Content: []View{
				Text(TextCfg{
					Text:      "Retry",
					Mode:      TextModeSingleLine,
					TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
				}),
			},
		}))
	}
	// Rows status.
	content = append(content, Row(ContainerCfg{
		Sizing:  FitFill,
		Padding: SomeP(0, 6, 0, 0),
		VAlign:  VAlignMiddle,
		Content: []View{
			Text(TextCfg{
				Text:      rowsText,
				Mode:      TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	}))
	// Jump input for offset mode.
	if kind == GridPaginationOffset {
		content = append(content, Text(TextCfg{
			Text:      ActiveLocale.StrJump,
			Mode:      TextModeSingleLine,
			TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
		}))
		content = append(content, Input(InputCfg{
			ID:          jumpInputID,
			IDFocus:     FnvSum32(jumpInputID),
			Text:        jumpText,
			Placeholder: "#",
			Disabled:    !jumpEnabled,
			Width:       dataGridJumpInputWidth,
			Sizing:      FixedFill,
			Padding:     NoPadding,
			SizeBorder:  SomeF(0),
			Radius:      SomeF(0),
			Color:       cfg.ColorFilter,
			ColorHover:  cfg.ColorFilter,
			ColorBorder: cfg.ColorBorder,
			TextStyle:   cfg.TextStyleFilter,
			OnTextChanged: func(_ *Layout, text string, w *Window) {
				digits := dataGridJumpDigits(text)
				dgJI := StateMap[string, string](w, nsDgJump, capModerate)
				dgJI.Set(gridID, digits)
				e := &Event{}
				dataGridSourceSubmitJump(onSelectionChange, rowCount, loading,
					loadError, kind, pageLimit, gridID, 0, e, w)
			},
			OnEnter: func(_ *Layout, e *Event, w *Window) {
				dataGridSourceSubmitJump(onSelectionChange, rowCount, loading,
					loadError, kind, pageLimit, gridID, focusID, e, w)
			},
		}))
	}
	return Row(ContainerCfg{
		Height:      dataGridPagerHeight(cfg),
		Sizing:      FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     Some(dataGridPagerPadding(cfg)),
		Spacing:     SomeF(6),
		VAlign:      VAlignMiddle,
		Content:     content,
	})
}

func dataGridSourceStatusRow(cfg *DataGridCfg, message string) View {
	return Row(ContainerCfg{
		Height:      cfg.RowHeight,
		Sizing:      FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(0),
		Padding:     cfg.PaddingFilter,
		VAlign:      VAlignMiddle,
		Content: []View{
			Text(TextCfg{
				Text:      message,
				Mode:      TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
		},
	})
}
