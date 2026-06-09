package datagrid

import (
	"strconv"
	"time"

	. "github.com/go-gui-org/go-gui/gui"
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
type GridColumnPin uint8

// GridColumnPin values.
const (
	GridColumnPinNone GridColumnPin = iota
	GridColumnPinLeft
	GridColumnPinRight
)

// gridAggregateOp specifies the aggregation operation.
type gridAggregateOp uint8

// gridAggregateOp values.
const (
	gridAggregateCount gridAggregateOp = iota
	gridAggregateSum
	gridAggregateAvg
	gridAggregateMin
	gridAggregateMax
)

func (op gridAggregateOp) String() string {
	switch op {
	case gridAggregateCount:
		return "count"
	case gridAggregateSum:
		return "sum"
	case gridAggregateAvg:
		return "avg"
	case gridAggregateMin:
		return "min"
	case gridAggregateMax:
		return "max"
	}
	return "count"
}

// GridCellEditorKind specifies the type of inline cell editor.
type GridCellEditorKind uint8

// GridCellEditorKind values.
const (
	GridCellEditorText GridCellEditorKind = iota
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
	TextStyle        *TextStyle
	ID               string
	Title            string
	EditorTrueValue  string
	EditorFalseValue string
	DefaultValue     string
	EditorOptions    []string
	Width            Opt[float32]
	MinWidth         Opt[float32]
	MaxWidth         Opt[float32]
	Resizable        bool
	Reorderable      bool
	Sortable         bool
	Filterable       bool
	Editable         bool
	Editor           GridCellEditorKind
	Pin              GridColumnPin
	Align            HorizontalAlign
}

// gridColumnCfgDefaults applies V-style defaults to a
// GridColumnCfg zero value. Called once per cfg construction.
func gridColumnCfgDefaults(c *GridColumnCfg) {
	if !c.Width.IsSet() {
		c.Width = SomeF(120)
	}
	if !c.MinWidth.IsSet() {
		c.MinWidth = SomeF(60)
	}
	if !c.MaxWidth.IsSet() {
		c.MaxWidth = SomeF(600)
	}
	if c.EditorTrueValue == "" {
		c.EditorTrueValue = "true"
	}
	if c.EditorFalseValue == "" {
		c.EditorFalseValue = "false"
	}
}

// GridAggregateCfg configures an aggregate operation.
type GridAggregateCfg struct {
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
type GridCellFormat struct {
	HasBGColor   bool
	BGColor      Color
	HasTextColor bool
	TextColor    Color
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
	TextStyle              TextStyle
	TextStyleHeader        TextStyle
	TextStyleFilter        TextStyle
	Selection              GridSelection
	DataSource             DataGridDataSource
	RowCount               *int
	AllowCreate            *bool
	AllowDelete            *bool
	MultiSort              *bool
	MultiSelect            *bool
	RangeSelect            *bool
	ShowHeader             *bool
	ShowGroupCounts        *bool
	HiddenColumnIDs        map[string]bool
	DetailExpandedRowIDs   map[string]bool
	OnQueryChange          func(GridQueryState, *Event, *Window)
	OnSelectionChange      func(GridSelection, *Event, *Window)
	OnColumnOrderChange    func([]string, *Event, *Window)
	OnColumnPinChange      func(string, GridColumnPin, *Event, *Window)
	OnHiddenColumnsChange  func(map[string]bool, *Event, *Window)
	OnPageChange           func(int, *Event, *Window)
	OnDetailExpandedChange func(map[string]bool, *Event, *Window)
	OnCellEdit             func(GridCellEdit, *Event, *Window)
	OnRowsChange           func([]GridRow, *Event, *Window)
	OnCRUDError            func(string, *Event, *Window)
	OnCellFormat           func(GridRow, int, GridColumnCfg, string, *Window) GridCellFormat
	OnDetailRowView        func(GridRow, *Window) View
	OnCopyRows             func([]GridRow, *Event, *Window) (string, bool)
	OnRowActivate          func(GridRow, *Event, *Window)
	Query                  GridQueryState
	ID                     string `gui:"required"`
	Cursor                 string
	LoadError              string
	QuickFilterPlaceholder string
	A11YLabel              string
	A11YDescription        string
	Columns                []GridColumnCfg
	ColumnOrder            []string
	GroupBy                []string
	Aggregates             []GridAggregateCfg
	// RowsData is a convenience field for key-value row data.
	// Map keys must match Column IDs. When set, RowsData takes
	// precedence over Rows. If Columns is empty, column
	// definitions are auto-generated from sorted keys of the
	// first map entry (default width 150px).
	RowsData            []map[string]string
	Rows                []GridRow
	FrozenTopRowIDs     []string
	PageLimit           int
	PageSize            int
	PageIndex           int
	QuickFilterDebounce time.Duration
	PaddingCell         Opt[Padding]
	PaddingHeader       Opt[Padding]
	PaddingFilter       Opt[Padding]
	Radius              Opt[float32]
	SizeBorder          Opt[float32]
	IDFocus             uint32
	IDScroll            uint32
	RowHeight           float32
	HeaderHeight        float32
	Width               float32
	Height              float32
	MinWidth            float32
	MaxWidth            float32
	MinHeight           float32
	MaxHeight           float32
	ColorBackground     Color
	ColorHeader         Color
	ColorHeaderHover    Color
	ColorFilter         Color
	ColorQuickFilter    Color
	ColorRowHover       Color
	ColorRowAlt         Color
	ColorRowSelected    Color
	ColorBorder         Color
	ColorResizeHandle   Color
	ColorResizeActive   Color
	Sizing              Opt[Sizing]
	PaginationKind      GridPaginationKind
	Loading             bool
	ShowCRUDToolbar     bool
	FreezeHeader        bool
	ShowFilterRow       bool
	ShowQuickFilter     bool
	ShowColumnChooser   bool
	Scrollbar           ScrollbarOverflow
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
	s := DefaultDataGridStyle
	if !cfg.Sizing.IsSet() {
		cfg.Sizing = Some(FillFill)
	}
	if cfg.RowHeight == 0 {
		cfg.RowHeight = dataGridDefaultRowHeight
	}
	if cfg.HeaderHeight == 0 {
		cfg.HeaderHeight = dataGridDefaultHeaderHeight
	}
	if cfg.PageLimit == 0 {
		cfg.PageLimit = dataGridDefaultPageLimit
	}
	if cfg.QuickFilterDebounce == 0 {
		cfg.QuickFilterDebounce = 200 * time.Millisecond
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
		cfg.PaddingCell = Some(s.PaddingCell)
	}
	if !cfg.PaddingHeader.IsSet() {
		cfg.PaddingHeader = Some(s.PaddingHeader)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = s.TextStyle
	}
	if cfg.TextStyleHeader == (TextStyle{}) {
		cfg.TextStyleHeader = s.TextStyleHeader
	}
	if cfg.TextStyleFilter == (TextStyle{}) {
		cfg.TextStyleFilter = s.TextStyleFilter
	}
	if !cfg.PaddingFilter.IsSet() {
		cfg.PaddingFilter = Some(s.PaddingFilter)
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = SomeF(s.Radius)
	}
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = SomeF(s.SizeBorder)
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
	AnchorRowID string
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
}

type dataGridSourceState struct {
	RowCount       *int
	ActiveAbort    *GridAbortController
	LoadError      string
	RequestKey     string
	CurrentCursor  string
	NextCursor     string
	PrevCursor     string
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
	HasMore        bool
	PaginationKind GridPaginationKind
	CapsCached     bool
	RowsDirty      bool
}

// dataGridCtx bundles commonly repeated DataGrid parameters
// to reduce function signatures. Constructed once per frame
// in DataGrid() and passed by value.
type dataGridCtx struct {
	cfg          *DataGridCfg
	columnWidths map[string]float32
	w            *Window
	editingRowID string
	columns      []GridColumnCfg
	rowHeight    float32
	focusID      uint32
	scrollID     uint32
}

// New creates a controlled, virtualized data grid view.
func New(w *Window, cfg DataGridCfg) View {
	RequireID("DataGrid", cfg.ID)
	applyDataGridDefaults(&cfg)
	if len(cfg.RowsData) > 0 && cfg.DataSource == nil {
		n := min(len(cfg.RowsData), maxDataConvLen)
		// Auto-generate columns from sorted keys of first row
		// when Columns is empty.
		if len(cfg.Columns) == 0 && len(cfg.RowsData[0]) > 0 {
			cfg.Columns = dataGridColumnsFromMap(cfg.RowsData[0])
		}
		cfg.Rows = make([]GridRow, n)
		for i := range n {
			cfg.Rows[i] = GridRow{ID: strconv.Itoa(i),
				Cells: cfg.RowsData[i]}
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
			dgSrc := StateMap[string, dataGridSourceState](w, nsDgSource, capModerate)
			if latestState, ok := dgSrc.Get(resolvedCfg.ID); ok {
				sourceState = latestState
			}
		}
	}

	// Interaction state.
	rowDeleteEnabled := dataGridCrudRowDeleteEnabled(&resolvedCfg, hasSource, sourceCaps)
	focusID := dataGridFocusID(&resolvedCfg)
	scrollID := dataGridScrollID(&resolvedCfg)
	dgHH := StateMap[string, string](w, nsDgHeaderHover, capModerate)
	hoveredColID, _ := dgHH.Get(resolvedCfg.ID)
	resizingColID := dataGridActiveResizeColID(resolvedCfg.ID, w)
	dgCO := StateMap[string, bool](w, nsDgChooserOpen, capModerate)
	chooserOpen, _ := dgCO.Get(resolvedCfg.ID)

	// Height/layout waterfall.
	rowHeight := dataGridRowHeight(&resolvedCfg, w)
	headerInScrollBody := boolDefault(resolvedCfg.ShowHeader, true) && !resolvedCfg.FreezeHeader
	staticTop := dataGridStaticTopHeight(&resolvedCfg, rowHeight, chooserOpen, headerInScrollBody)
	pageStart, pageEnd, pageIndex, pageCount := dataGridPageBounds(len(resolvedCfg.Rows),
		resolvedCfg.PageSize, resolvedCfg.PageIndex)
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
		sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
		if v, ok := sy.Get(scrollID); ok {
			scrollY = -v
		}
	}

	// Build columns and presentation.
	columns := dataGridEffectiveColumns(resolvedCfg.Columns, resolvedCfg.ColumnOrder,
		resolvedCfg.HiddenColumnIDs)
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
	focusedColID := dataGridHeaderFocusedColID(&resolvedCfg, columns, w.IDFocus())

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
	sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
	scrollX, _ := sx.Get(scrollID)

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
	scrollbarCfg := ScrollbarCfg{Overflow: resolvedCfg.Scrollbar}
	scrollBody := Column(ContainerCfg{
		ID:            resolvedCfg.ID + ":scroll",
		IDScroll:      scrollID,
		ScrollbarCfgX: &scrollbarCfg,
		ScrollbarCfgY: &scrollbarCfg,
		Color:         resolvedCfg.ColorBackground,
		Padding:       Some(dataGridScrollPadding(&resolvedCfg)),
		Spacing:       SomeF(0),
		Sizing:        FillFill,
		Content:       rows,
	})

	// Final assembly.
	content := dataGridFinalContent(dctx, scrollBody, headerView,
		headerHeight, totalWidth, scrollX, gridHeight, staticTop,
		frozenTopViews, frozenTopDisplayRows, crudEnabled, crudState,
		sourceCaps, hasSource, pagerEnabled, sourcePagerEnabled,
		pageIndex, pageCount, pageStart, pageEnd, presentation,
		sourceState)

	return Column(ContainerCfg{
		ID:              resolvedCfg.ID,
		IDFocus:         focusID,
		A11YRole:        AccessRoleGrid,
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
		Padding:     NoPadding,
		Spacing:     SomeF(0),
		Disabled:    resolvedCfg.Disabled,
		Invisible:   resolvedCfg.Invisible,
		Sizing:      resolvedCfg.Sizing.Get(FillFill),
		Width:       resolvedCfg.Width,
		Height:      resolvedCfg.Height,
		MinWidth:    resolvedCfg.MinWidth,
		MaxWidth:    resolvedCfg.MaxWidth,
		MinHeight:   resolvedCfg.MinHeight,
		MaxHeight:   resolvedCfg.MaxHeight,
		Content:     content,
	})
}
