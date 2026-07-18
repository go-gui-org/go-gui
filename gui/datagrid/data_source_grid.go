package datagrid

import (
	"strconv"

	gg "github.com/go-gui-org/go-gui/gui"
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
func GetSourceStats(w *gg.Window, gridID string) SourceStats {
	dgSrc := gg.StateMapRead[string, dataGridSourceState](w, nsDgSource)
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

func dataGridSourceApplyLocalMutation(gridID string, rows []GridRow, rowCount int, w *gg.Window) {
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	// Default zero state: absent entry means no prior mutation state;
	// cancelActive no-ops, then state is fully overwritten below.
	state := dgSrc.GetOr(gridID, dataGridSourceState{})
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

func dataGridSourceForceRefetch(gridID string, w *gg.Window) {
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
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

func dataGridResolveSourceCfg(cfg DataGridCfg, w *gg.Window) (DataGridCfg, dataGridSourceState, bool, GridDataCapabilities) {
	source := cfg.DataSource
	if source == nil {
		return cfg, dataGridSourceState{}, false, GridDataCapabilities{}
	}

	// Use cached capabilities when available.
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
	// Default zero state: absent entry means CapsCached is false,
	// triggering fresh Capabilities() call.
	existing := dgSrc.GetOr(cfg.ID, dataGridSourceState{})
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

func dataGridSourceResolveState(cfg DataGridCfg, caps GridDataCapabilities, dgSrc *gg.BoundedMap[string, dataGridSourceState], w *gg.Window) dataGridSourceState {
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

func dataGridSourceApplyPendingJumpSelection(cfg *DataGridCfg, state dataGridSourceState, w *gg.Window) {
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
	e := &gg.Event{}
	cfg.OnSelectionChange(next, e, w)
	dataGridSetAnchor(cfg.ID, rowID, w)
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
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

func dataGridSourceStartRequest(cfg DataGridCfg, caps GridDataCapabilities, kind GridPaginationKind, requestKey string, state *dataGridSourceState, w *gg.Window) {
	source := cfg.DataSource
	if source == nil {
		return
	}
	dataGridSourceCancelActive(state)
	limit := dataGridPageLimit(&cfg)
	controller := gg.NewGridAbortController()
	nextRequestID := state.RequestID + 1
	var page GridPageRequest
	switch kind {
	case GridPaginationCursor:
		page = GridCursorPageReq{
			Cursor: state.CurrentCursor,
			Limit:  limit,
		}
	default:
		page = GridOffsetPageReq{
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
			w.QueueCommand(func(w *gg.Window) {
				dataGridSourceApplyError(gridID, nextRequestID, errMsg, w)
			})
			return
		}
		w.QueueCommand(func(w *gg.Window) {
			dataGridSourceApplySuccess(gridID, nextRequestID, result, caps, w)
		})
	}()
}

func dataGridSourceDropIfStale(requestID uint64, state *dataGridSourceState, w *gg.Window, gridID string) bool {
	if requestID != state.RequestID {
		state.StaleDropCount++
		dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
		dgSrc.Set(gridID, *state)
		return true
	}
	return false
}

func dataGridSourceApplySuccess(gridID string, requestID uint64, result GridDataResult, caps GridDataCapabilities, w *gg.Window) {
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
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
		h := gg.Fnv64Str(gg.Fnv64Offset, state.CurrentCursor)
		return "__src_cx_" + zeroPadHex16(h) + "_" + strconv.Itoa(localIdx)
	}
}

func dataGridSourceApplyError(gridID string, requestID uint64, errMsg string, w *gg.Window) {
	dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
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

func dataGridSourceJumpEnabled(onSelectionChange func(GridSelection, *gg.Event, *gg.Window), rowCount *int, loading bool, loadError string, kind GridPaginationKind, pageLimit int) bool {
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

func dataGridSourceSubmitJump(onSelectionChange func(GridSelection, *gg.Event, *gg.Window), rowCount *int, loading bool, loadError string, kind GridPaginationKind, pageLimit int, gridID string, focusID string, e *gg.Event, w *gg.Window) {
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
	jumpInputID := gridID + ":jump"
	content := make([]gg.View, 0, 10)

	// Prev button.
	content = append(content, dataGridIndicatorButton("\u25C0", cfg.TextStyleHeader, cfg.ColorHeaderHover,
		state.Loading || !hasPrev, dataGridHeaderControlWidth+10, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
			dataGridSourcePrevPage(gridID, kind, pageLimit, w)
			if focusID != "" {
				w.SetFocus(focusID)
			}
			e.IsHandled = true
		}))
	// Status.
	content = append(content, gg.Text(gg.TextCfg{
		Text:      status,
		Mode:      gg.TextModeSingleLine,
		TextStyle: cfg.TextStyleFilter,
	}))
	// Next button.
	content = append(content, dataGridIndicatorButton("\u25B6", cfg.TextStyleHeader, cfg.ColorHeaderHover,
		state.Loading || !hasNext, dataGridHeaderControlWidth+10, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
			dataGridSourceNextPage(gridID, kind, pageLimit, w)
			if focusID != "" {
				w.SetFocus(focusID)
			}
			e.IsHandled = true
		}))
	// Spacer.
	content = append(content, gg.Row(gg.ContainerCfg{
		Sizing:  gg.FillFill,
		Padding: gg.NoPadding,
	}))
	// Retry button on error.
	if state.LoadError != "" {
		content = append(content, gg.Button(gg.ButtonCfg{
			Sizing:      gg.FitFill,
			Padding:     gg.NoPadding,
			SizeBorder:  gg.SomeF(0),
			Radius:      gg.SomeF(0),
			Color:       gg.ColorTransparent,
			ColorHover:  cfg.ColorHeaderHover,
			ColorFocus:  gg.ColorTransparent,
			ColorClick:  cfg.ColorHeaderHover,
			ColorBorder: gg.ColorTransparent,
			OnClick: func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
				dataGridSourceRetry(gridID, w)
				if focusID != "" {
					w.SetFocus(focusID)
				}
				e.IsHandled = true
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
		Padding: gg.SomeP(0, 6, 0, 0),
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
			OnTextChanged: func(_ *gg.Layout, text string, w *gg.Window) {
				digits := dataGridJumpDigits(text)
				dgJI := gg.StateMap[string, string](w, nsDgJump, capModerate)
				dgJI.Set(gridID, digits)
				e := &gg.Event{}
				dataGridSourceSubmitJump(onSelectionChange, rowCount, loading,
					loadError, kind, pageLimit, gridID, "", e, w)
			},
			OnEnter: func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
				dataGridSourceSubmitJump(onSelectionChange, rowCount, loading,
					loadError, kind, pageLimit, gridID, focusID, e, w)
			},
		}))
	}
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
