package datagrid

import (
	gg "github.com/go-gui-org/go-gui/gui"
)

// SourceStats provides runtime stats for a data-source-backed grid.
// exportaudit:keep — reachable from an exported signature
type SourceStats struct {
	RowCount       *int
	loadError      string
	RequestCount   int
	CancelledCount int
	StaleDropCount int
	ReceivedCount  int
	Loading        bool
	hasMore        bool
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
		loadError:      state.LoadError,
		RequestCount:   state.RequestCount,
		CancelledCount: state.CancelledCount,
		StaleDropCount: state.StaleDropCount,
		hasMore:        state.hasMore,
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
		anchorRowID:    rowID,
		activeRowID:    rowID,
		SelectedRowIDs: map[string]bool{rowID: true},
	}
	e := &gg.Event{}
	cfg.OnSelectionChange(next, gg.EventCtx{Layout: nil, Event: e, Window: w})
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
	state.nextCursor = ""
	state.prevCursor = ""
	state.OffsetStart = 0
	state.RequestKey = ""
}

func dataGridSourceEffectivePaginationKind(preferred GridPaginationKind, caps GridDataCapabilities) GridPaginationKind {
	if preferred == GridPaginationCursor {
		if caps.supportsCursorPagination {
			return GridPaginationCursor
		}
		if caps.supportsOffsetPagination {
			return GridPaginationOffset
		}
		return GridPaginationNone
	}
	if caps.supportsOffsetPagination {
		return GridPaginationOffset
	}
	if caps.supportsCursorPagination {
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
