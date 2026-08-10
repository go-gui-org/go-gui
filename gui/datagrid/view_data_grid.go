package datagrid

import (
	"strconv"
	"time"

	gg "github.com/go-gui-org/go-gui/gui"
)

// Data grid constants.
const (
	dataGridVirtualBufferRows       = 2
	dataGridResizeDoubleClickFrames = uint64(24) // ~400ms at 60fps
	dataGridEditDoubleClickFrames   = uint64(36) // ~600ms at 60fps
	dataGridResizeHandleWidth       = float32(6)
	dataGridAutofitPadding          = float32(18)
	dataGridAutofitMaxRows          = 1000
	dataGridIndicatorAlpha          = uint8(140)
	dataGridHeaderControlWidth      = float32(12)
	dataGridHeaderReorderSpacing    = float32(1)
	dataGridHeaderLabelMinWidth     = float32(24)
	dataGridGroupIndentStep         = float32(14)
	dataGridDetailIndentGap         = float32(4)
	dataGridRecordSep               = "\x1e"
	dataGridUnitSep                 = "\x1f"
	dataGridGroupSep                = "\x1d"
	dataGridDefaultRowHeight        = float32(30)
	dataGridDefaultHeaderHeight     = float32(34)
	dataGridMaxAutoColumns          = 1_000 // safety cap for auto-gen columns

	// maxDataConvLen caps convenience-field slice conversions
	// (RowsData→Rows, Items→Options, RawData→Data, ItemPaths→Nodes)
	// to prevent OOM from unbounded inputs.
	maxDataConvLen           = 100_000
	dataGridDefaultPageLimit = 100
	dataGridJumpInputWidth   = float32(68)
)

// dataGridColumnsFromMap builds column definitions from the keys
// of the first RowsData entry. Keys are sorted for deterministic
// output; capped at dataGridMaxAutoColumns.
func dataGridColumnsFromMap(row map[string]string) []GridColumnCfg {
	keys := sortedMapKeys(row)
	if len(keys) > dataGridMaxAutoColumns {
		keys = keys[:dataGridMaxAutoColumns]
	}
	cols := make([]GridColumnCfg, len(keys))
	for i, k := range keys {
		cols[i] = GridColumnCfg{
			ID:       k,
			Title:    k,
			Sortable: true,
		}
	}
	return cols
}

// GridColumnPin specifies column pinning position.
type gridColumnPin uint8

// GridColumnPin values.
const (
	gridColumnPinNone gridColumnPin = iota
	gridColumnPinLeft
	gridColumnPinRight
)

// GridCellEditorKind specifies the type of inline cell editor.
type gridCellEditorKind uint8

// GridCellEditorKind values.
const (
	gridCellEditorText gridCellEditorKind = iota
	GridCellEditorSelect
	GridCellEditorDate
	GridCellEditorCheckbox
)

// dataGridDisplayRowKind classifies display rows.
type dataGridDisplayRowKind uint8

const (
	dataGridDisplayRowData dataGridDisplayRowKind = iota
	dataGridDisplayRowGroupHeader
	dataGridDisplayRowDetail
)

// GridColumnCfg configures a single data grid column.
type GridColumnCfg struct {
	TextStyle        *gg.TextStyle
	ID               string
	Title            string
	editorTrueValue  string
	editorFalseValue string
	DefaultValue     string
	EditorOptions    []string
	Width            gg.Opt[float32]
	MinWidth         gg.Opt[float32]
	MaxWidth         gg.Opt[float32]
	resizable        bool
	Reorderable      bool
	Sortable         bool
	Filterable       bool
	Editable         bool
	Editor           gridCellEditorKind
	pin              gridColumnPin
	Align            gg.HorizontalAlign // ergonomics-audit:opt-plain — zero (HAlignStart) is the natural default; no distinct unset behavior
}

// gridColumnCfgDefaults applies V-style defaults to a
// GridColumnCfg zero value. Called once per cfg construction.
func gridColumnCfgDefaults(c *GridColumnCfg) {
	if !c.Width.IsSet() {
		c.Width = gg.SomeF(120)
	}
	if !c.MinWidth.IsSet() {
		c.MinWidth = gg.SomeF(60)
	}
	if !c.MaxWidth.IsSet() {
		c.MaxWidth = gg.SomeF(600)
	}
	if c.editorTrueValue == "" {
		c.editorTrueValue = "true"
	}
	if c.editorFalseValue == "" {
		c.editorFalseValue = "false"
	}
}

// GridAggregateCfg configures an aggregate operation.
type gridAggregateCfg struct {
	ColID string
	Label string
	Op    gridAggregateOp
}

// gridCsvData holds parsed CSV data.
type gridCsvData struct {
	Columns []GridColumnCfg
	Rows    []GridRow
}

// gridExportCfg configures export behavior.
type gridExportCfg struct {
	SanitizeSpreadsheetFormulas bool
	XLSXAutoType                bool
}

// GridCellFormat describes conditional cell formatting.
type gridCellFormat struct {
	hasBGColor   bool
	bGColor      gg.Color
	hasTextColor bool
	textColor    gg.Color
}

// dataGridDisplayRow is a flat display entry (data, group
// header, or detail expansion).
type dataGridDisplayRow struct {
	GroupColID    string
	GroupValue    string
	GroupColTitle string
	AggregateText string
	DataRowIdx    int
	GroupDepth    int
	GroupCount    int
	Kind          dataGridDisplayRowKind
}

// dataGridPresentation is the flattened display row list
// with a data-row-index → display-index map.
type dataGridPresentation struct {
	DataToDisplay map[int]int
	Rows          []dataGridDisplayRow
}

// DataGridCfg configures a data grid widget.
//
//nolint:revive // DataGrid prefix intentional
type DataGridCfg struct {
	TextStyle              gg.TextStyle
	TextStyleHeader        gg.TextStyle
	TextStyleFilter        gg.TextStyle
	Selection              GridSelection
	DataSource             DataGridDataSource
	RowCount               *int
	allowCreate            *bool
	allowDelete            *bool
	multiSort              *bool
	MultiSelect            *bool
	rangeSelect            *bool
	showHeader             *bool
	showGroupCounts        *bool
	hiddenColumnIDs        map[string]bool
	detailExpandedRowIDs   map[string]bool
	OnQueryChange          func(GridQueryState, gg.EventCtx)
	OnSelectionChange      func(GridSelection, gg.EventCtx)
	onColumnOrderChange    func([]string, gg.EventCtx)
	onColumnPinChange      func(string, gridColumnPin, gg.EventCtx)
	onHiddenColumnsChange  func(map[string]bool, gg.EventCtx)
	onPageChange           func(int, gg.EventCtx)
	onDetailExpandedChange func(map[string]bool, gg.EventCtx)
	OnCellEdit             func(GridCellEdit, gg.EventCtx)
	onRowsChange           func([]GridRow, gg.EventCtx)
	OnCRUDError            func(string, gg.EventCtx)
	cellFormat             func(GridRow, int, GridColumnCfg, string, *gg.Window) gridCellFormat
	detailRowView          func(GridRow, *gg.Window) gg.View
	onCopyRows             func([]GridRow, gg.EventCtx) (string, bool)
	onRowActivate          func(GridRow, gg.EventCtx)
	Query                  GridQueryState
	ID                     string `gui:"required"`
	Cursor                 string
	loadError              string
	quickFilterPlaceholder string
	A11YLabel              string
	A11YDescription        string
	Columns                []GridColumnCfg
	columnOrder            []string
	groupBy                []string
	aggregates             []gridAggregateCfg
	// RowsData is a convenience field for key-value row data.
	// Map keys must match Column IDs. When set, RowsData takes
	// precedence over Rows. If Columns is empty, column
	// definitions are auto-generated from sorted keys of the
	// first map entry (default width 150px).
	rowsData        []map[string]string
	Rows            []GridRow
	frozenTopRowIDs []string
	PageLimit       int
	pageSize        int
	pageIndex       int
	// QuickFilterDebounce delays the quick filter query commit.
	// Defaults to 200ms on grids with a DataSource, 0 (immediate)
	// otherwise; negative opts a sourced grid out of debouncing.
	quickFilterDebounce time.Duration
	PaddingCell         gg.Opt[gg.Padding]
	PaddingHeader       gg.Opt[gg.Padding]
	PaddingFilter       gg.Opt[gg.Padding]
	Radius              gg.Opt[float32]
	SizeBorder          gg.Opt[float32]
	rowHeight           float32
	HeaderHeight        float32
	Width               float32
	Height              float32
	MinWidth            float32
	MaxWidth            float32
	MinHeight           float32
	MaxHeight           float32
	ColorBackground     gg.Color
	ColorHeader         gg.Color
	ColorHeaderHover    gg.Color
	ColorFilter         gg.Color
	ColorQuickFilter    gg.Color
	ColorRowHover       gg.Color
	ColorRowAlt         gg.Color
	ColorRowSelected    gg.Color
	ColorBorder         gg.Color
	ColorResizeHandle   gg.Color
	ColorResizeActive   gg.Color
	Sizing              gg.Opt[gg.Sizing]
	PaginationKind      gridPaginationKind
	Loading             bool
	ShowCRUDToolbar     bool
	FreezeHeader        bool
	ShowFilterRow       bool
	ShowQuickFilter     bool
	ShowColumnChooser   bool
	Scrollbar           gg.ScrollbarOverflow
	Disabled            bool
	Invisible           bool
}

// boolDefault returns *p if non-nil, else def.
func boolDefault(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

// applyDataGridDefaults fills zero-valued fields with theme
// defaults and sensible fallbacks.
func applyDataGridDefaults(cfg *DataGridCfg) {
	s := gg.DefaultDataGridStyle
	if !cfg.Sizing.IsSet() {
		cfg.Sizing = gg.Some(gg.FillFill)
	}
	if cfg.rowHeight == 0 {
		cfg.rowHeight = dataGridDefaultRowHeight
	}
	if cfg.HeaderHeight == 0 {
		cfg.HeaderHeight = dataGridDefaultHeaderHeight
	}
	if cfg.PageLimit == 0 {
		cfg.PageLimit = dataGridDefaultPageLimit
	}
	// Debounce only pays off when filtering triggers async fetches;
	// on in-memory grids it just delays the query round-trip. A
	// negative value opts a sourced grid out of debouncing (the
	// handler treats <= 0 as immediate).
	if cfg.quickFilterDebounce == 0 && dataGridHasSource(cfg) {
		cfg.quickFilterDebounce = 200 * time.Millisecond
	}
	if !cfg.ColorBackground.IsSet() {
		cfg.ColorBackground = s.ColorBackground
	}
	if !cfg.ColorHeader.IsSet() {
		cfg.ColorHeader = s.ColorHeader
	}
	if !cfg.ColorHeaderHover.IsSet() {
		cfg.ColorHeaderHover = s.ColorHeaderHover
	}
	if !cfg.ColorFilter.IsSet() {
		cfg.ColorFilter = s.ColorFilter
	}
	if !cfg.ColorQuickFilter.IsSet() {
		cfg.ColorQuickFilter = s.ColorQuickFilter
	}
	if !cfg.ColorRowHover.IsSet() {
		cfg.ColorRowHover = s.ColorRowHover
	}
	if !cfg.ColorRowAlt.IsSet() {
		cfg.ColorRowAlt = s.ColorRowAlt
	}
	if !cfg.ColorRowSelected.IsSet() {
		cfg.ColorRowSelected = s.ColorRowSelected
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = s.ColorBorder
	}
	if !cfg.ColorResizeHandle.IsSet() {
		cfg.ColorResizeHandle = s.ColorResizeHandle
	}
	if !cfg.ColorResizeActive.IsSet() {
		cfg.ColorResizeActive = s.ColorResizeActive
	}
	if !cfg.PaddingCell.IsSet() {
		cfg.PaddingCell = gg.Some(s.PaddingCell)
	}
	if !cfg.PaddingHeader.IsSet() {
		cfg.PaddingHeader = gg.Some(s.PaddingHeader)
	}
	if cfg.TextStyle == (gg.TextStyle{}) {
		cfg.TextStyle = s.TextStyle
	}
	if cfg.TextStyleHeader == (gg.TextStyle{}) {
		cfg.TextStyleHeader = s.TextStyleHeader
	}
	if cfg.TextStyleFilter == (gg.TextStyle{}) {
		cfg.TextStyleFilter = s.TextStyleFilter
	}
	if !cfg.PaddingFilter.IsSet() {
		cfg.PaddingFilter = gg.Some(s.PaddingFilter)
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = gg.SomeF(s.Radius)
	}
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = gg.SomeF(s.SizeBorder)
	}
	for i := range cfg.Columns {
		gridColumnCfgDefaults(&cfg.Columns[i])
	}
}

// --- Internal state structs ---

type dataGridResizeState struct {
	ColID          string
	LastClickColID string
	LastClickFrame uint64
	StartMouseX    float32
	StartWidth     float32
	Active         bool
}

type dataGridColWidths struct {
	Widths map[string]float32
}

type dataGridPresentationCache struct {
	DataToDisplay map[int]int
	GroupRanges   map[string]int
	Rows          []dataGridDisplayRow
	GroupCols     []string
	Signature     uint64
}

type dataGridRangeState struct {
	anchorRowID string
}

type dataGridEditState struct {
	EditingRowID   string
	LastClickRowID string
	LastClickFrame uint64
}

type dataGridCrudState struct {
	DirtyRowIDs             map[string]bool
	DraftRowIDs             map[string]bool
	DeletedRowIDs           map[string]bool
	SaveError               string
	CommittedRows           []GridRow
	WorkingRows             []GridRow
	SourceSignature         uint64
	LocalRowsLen            int
	LocalRowsIDSignature    uint64
	NextDraftSeq            int
	LocalRowsSignatureValid bool
	Saving                  bool
	SourceChanged           bool
	ActiveAbort             *gg.GridAbortController
	RequestID               uint64
}

type dataGridSourceState struct {
	RowCount       *int
	ActiveAbort    *gg.GridAbortController
	loadError      string
	RequestKey     string
	CurrentCursor  string
	nextCursor     string
	prevCursor     string
	ConfigCursor   string
	Rows           []GridRow
	RequestID      uint64
	QuerySignature uint64
	OffsetStart    int
	ReceivedCount  int
	RequestCount   int
	CancelledCount int
	StaleDropCount int
	PendingJumpRow int
	RowsSignature  uint64
	CachedCaps     GridDataCapabilities
	Loading        bool
	HasLoaded      bool
	hasMore        bool
	PaginationKind gridPaginationKind
	CapsCached     bool
	RowsDirty      bool
}

// dataGridCtx bundles commonly repeated DataGrid parameters
// to reduce function signatures. Constructed once per frame
// in DataGrid() and passed by value.
type dataGridCtx struct {
	cfg          *DataGridCfg
	columnWidths map[string]float32
	w            *gg.Window
	editingRowID string
	columns      []GridColumnCfg
	rowHeight    float32
	focusID      string
	scrollID     string
}

// New creates a controlled, virtualized data grid view.
func New(w *gg.Window, cfg DataGridCfg) gg.View {
	gg.RequireID("DataGrid", cfg.ID)
	applyDataGridDefaults(&cfg)
	if len(cfg.rowsData) > 0 && cfg.DataSource == nil {
		n := min(len(cfg.rowsData), maxDataConvLen)
		// Auto-generate columns from sorted keys of first row
		// when Columns is empty.
		if len(cfg.Columns) == 0 && len(cfg.rowsData[0]) > 0 {
			cfg.Columns = dataGridColumnsFromMap(cfg.rowsData[0])
		}
		cfg.Rows = make([]GridRow, n)
		for i := range n {
			cfg.Rows[i] = GridRow{ID: strconv.Itoa(i),
				Cells: cfg.rowsData[i]}
		}
	}

	// Resolve data source and apply pending jump/selection.
	resolvedCfg, sourceState, hasSource, sourceCaps := dataGridResolveSourceCfg(cfg, w)
	if hasSource {
		dataGridSourceApplyPendingJumpSelection(&resolvedCfg, sourceState, w)
	}

	// Overlay CRUD working copy.
	var crudState dataGridCrudState
	crudEnabled := dataGridCrudEnabled(&resolvedCfg)
	if crudEnabled {
		nextCfg, nextCrudState := dataGridCrudResolveCfg(resolvedCfg, w)
		resolvedCfg = nextCfg
		crudState = nextCrudState
		if hasSource {
			dgSrc := gg.StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
			if latestState, ok := dgSrc.Get(resolvedCfg.ID); ok {
				sourceState = latestState
			}
		}
	}

	// Interaction state.
	rowDeleteEnabled := dataGridCrudRowDeleteEnabled(&resolvedCfg, hasSource, sourceCaps)
	focusID := dataGridFocusID(&resolvedCfg)
	scrollID := dataGridScrollID(&resolvedCfg)
	dgHH := gg.StateMap[string, string](w, nsDgHeaderHover, capModerate)
	// Default "": absent entry means no column is hovered.
	hoveredColID := dgHH.GetOr(resolvedCfg.ID, "")
	resizingColID := dataGridActiveResizeColID(resolvedCfg.ID, w)
	dgCO := gg.StateMap[string, bool](w, nsDgChooserOpen, capModerate)
	// Default false: absent entry means column chooser is closed.
	chooserOpen := dgCO.GetOr(resolvedCfg.ID, false)

	// Height/layout waterfall.
	rowHeight := dataGridRowHeight(&resolvedCfg, w)
	headerInScrollBody := boolDefault(resolvedCfg.showHeader, true) && !resolvedCfg.FreezeHeader
	staticTop := dataGridStaticTopHeight(&resolvedCfg, rowHeight, chooserOpen, headerInScrollBody)
	pageStart, pageEnd, pageIndex, pageCount := dataGridPageBounds(len(resolvedCfg.Rows),
		resolvedCfg.pageSize, resolvedCfg.pageIndex)
	pageIndices := dataGridPageRowIndices(pageStart, pageEnd)
	frozenTopIndices, bodyPageIndices := dataGridSplitFrozenTopIndices(&resolvedCfg, pageIndices)
	frozenTopIDs := dataGridFrozenTopIDSet(&resolvedCfg)
	pagerEnabled := dataGridPagerEnabled(&resolvedCfg, pageCount)
	sourcePagerEnabled := hasSource
	gridHeight := dataGridHeight(&resolvedCfg)
	if (pagerEnabled || sourcePagerEnabled) && gridHeight > 0 {
		gridHeight = f32Max(0, gridHeight-dataGridPagerHeight(&resolvedCfg))
	}
	if crudEnabled {
		toolbarHeight := dataGridCrudToolbarHeight(&resolvedCfg)
		if gridHeight > 0 {
			gridHeight = f32Max(0, gridHeight-toolbarHeight)
		}
	}
	virtualize := gridHeight > 0 && len(resolvedCfg.Rows) > 0
	scrollY := float32(0)
	if virtualize {
		if v, ok := w.ScrollY().Get(scrollID); ok {
			scrollY = -v
		}
	}

	// Build columns and presentation.
	columns := dataGridEffectiveColumns(resolvedCfg.Columns, resolvedCfg.columnOrder,
		resolvedCfg.hiddenColumnIDs)
	presentation := dataGridCachedPresentation(&resolvedCfg, columns, bodyPageIndices, w)
	if !hasSource {
		dataGridApplyPendingLocalJumpScroll(&resolvedCfg, gridHeight, rowHeight,
			staticTop, scrollID, presentation.DataToDisplay, w)
	}

	// Clear stale editing state.
	editingRowID := dataGridEditingRowID(resolvedCfg.ID, w)
	if editingRowID != "" && !dataGridHasRowID(resolvedCfg.Rows, editingRowID) {
		dataGridClearEditingRow(resolvedCfg.ID, w)
		editingRowID = ""
	}
	focusedColID := dataGridHeaderFocusedColID(&resolvedCfg, columns, w.FocusID())

	// Column widths and header.
	columnWidths := dataGridColumnWidths(resolvedCfg.ID, resolvedCfg.Columns, w)
	dctx := dataGridCtx{
		cfg:          &resolvedCfg,
		columns:      columns,
		columnWidths: columnWidths,
		rowHeight:    rowHeight,
		focusID:      focusID,
		scrollID:     scrollID,
		editingRowID: editingRowID,
		w:            w,
	}
	totalWidth := dataGridColumnsTotalWidth(columns, columnWidths)
	headerView := dataGridHeaderRow(&resolvedCfg, columns, columnWidths, focusID,
		hoveredColID, resizingColID, focusedColID)
	headerHeight := dataGridHeaderHeight(&resolvedCfg)
	frozenTopViews, frozenTopDisplayRows := dataGridFrozenTopViews(dctx,
		frozenTopIndices, rowDeleteEnabled)
	// Default 0: absent entry means no horizontal scroll offset.
	scrollX := w.ScrollX().GetOr(scrollID, 0)

	// Visible range for virtualization.
	firstVisible, lastVisible := 0, len(presentation.Rows)-1
	if virtualize {
		firstVisible, lastVisible = dataGridVisibleRangeForScroll(scrollY, gridHeight,
			rowHeight, len(presentation.Rows), staticTop, dataGridVirtualBufferRows)
	}

	// Assemble scroll body rows.
	rows := dataGridScrollBodyRows(dctx, presentation,
		rowDeleteEnabled, headerInScrollBody, headerView,
		chooserOpen, hasSource, virtualize,
		firstVisible, lastVisible)

	// Scrollable body.
	scrollbarCfg := gg.ScrollbarCfg{Overflow: resolvedCfg.Scrollbar}
	scrollBody := gg.Column(gg.ContainerCfg{
		ID:            scrollID,
		Scrollable:    true,
		ScrollbarCfgX: &scrollbarCfg,
		ScrollbarCfgY: &scrollbarCfg,
		Color:         resolvedCfg.ColorBackground,
		Padding:       gg.Some(dataGridScrollPadding(&resolvedCfg)),
		Spacing:       gg.SomeF(0),
		Sizing:        gg.FillFill,
		Content:       rows,
	})

	// Final assembly.
	content := dataGridFinalContent(dctx, scrollBody, headerView,
		headerHeight, totalWidth, scrollX, gridHeight, staticTop,
		frozenTopViews, frozenTopDisplayRows, crudEnabled, crudState,
		sourceCaps, hasSource, pagerEnabled, sourcePagerEnabled,
		pageIndex, pageCount, pageStart, pageEnd, presentation,
		sourceState)

	return gg.Column(gg.ContainerCfg{
		ID:              resolvedCfg.ID,
		Focusable:       true,
		A11YRole:        gg.AccessRoleGrid,
		A11YLabel:       resolvedCfg.A11YLabel,
		A11YDescription: resolvedCfg.A11YDescription,
		OnKeyDown: dataGridMakeOnKeydown(&resolvedCfg, columns, rowHeight,
			staticTop, scrollID, pageIndices, frozenTopIDs, presentation.DataToDisplay),
		OnChar:      dataGridMakeOnChar(&resolvedCfg, columns),
		OnMouseMove: dataGridMakeOnMouseMove(resolvedCfg.ID),
		Color:       resolvedCfg.ColorBackground,
		ColorBorder: resolvedCfg.ColorBorder,
		SizeBorder:  resolvedCfg.SizeBorder,
		Radius:      resolvedCfg.Radius,
		Padding:     gg.NoPadding,
		Spacing:     gg.SomeF(0),
		Disabled:    resolvedCfg.Disabled,
		Invisible:   resolvedCfg.Invisible,
		Sizing:      resolvedCfg.Sizing.Get(gg.FillFill),
		Width:       resolvedCfg.Width,
		Height:      resolvedCfg.Height,
		MinWidth:    resolvedCfg.MinWidth,
		MaxWidth:    resolvedCfg.MaxWidth,
		MinHeight:   resolvedCfg.MinHeight,
		MaxHeight:   resolvedCfg.MaxHeight,
		Content:     content,
	})
}
