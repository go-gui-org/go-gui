package datagrid

import (
	"strconv"

	gg "github.com/go-gui-org/go-gui/gui"
)

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
		page = gridCursorPageReq{
			Cursor: state.CurrentCursor,
			limit:  limit,
		}
	default:
		page = gridOffsetPageReq{
			StartIndex: state.OffsetStart,
			endIndex:   state.OffsetStart + limit,
		}
	}
	req := GridDataRequest{
		gridID:    cfg.ID,
		Query:     cfg.Query,
		page:      page,
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
	state.nextCursor = result.nextCursor
	state.prevCursor = result.prevCursor
	state.hasMore = result.hasMore
	if result.ReceivedCount > 0 {
		state.ReceivedCount = result.ReceivedCount
	} else {
		state.ReceivedCount = len(result.Rows)
	}
	if result.RowCount >= 0 {
		rc := result.RowCount
		state.RowCount = &rc
	} else if !caps.rowCountKnown {
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
		// A row key, not a scope: it becomes a part of the grid's row
		// IDs, so it must not contain the ID separator.
		return "__src_o_" + strconv.Itoa(absIdx) // ergonomics-audit:id-part
	default:
		if start, ok := dataGridSourceCursorToIndexOpt(state.CurrentCursor); ok {
			return "__src_c_" + strconv.Itoa(max(0, start)+localIdx) // ergonomics-audit:id-part
		}
		h := gg.Fnv64Str(gg.Fnv64Offset, state.CurrentCursor)
		return "__src_cx_" + zeroPadHex16(h) + "_" + strconv.Itoa(localIdx) // ergonomics-audit:id-part
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
