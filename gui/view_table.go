package gui

import (
	"encoding/csv"
	"maps"
	"strings"

	"github.com/go-gui-org/go-glyph"
)

// TableBorderStyle controls which borders are drawn in a table.
type TableBorderStyle uint8

// TableBorderStyle constants.
const (
	TableBorderNone       TableBorderStyle = iota // no borders
	TableBorderAll                                // full grid
	TableBorderHorizontal                         // horizontal lines between rows
	TableBorderHeaderOnly                         // single line under header row
)

// TableRowCfg configures a table row.
type TableRowCfg struct {
	OnClick func(EventCtx)
	ID      string
	Cells   []TableCellCfg
}

// TableCellCfg configures a table cell.
type TableCellCfg struct {
	Content   View
	HAlign    *HorizontalAlign
	TextStyle *TextStyle
	RichText  *RichText
	OnClick   func(EventCtx)
	ID        string
	Value     string
	HeadCell  bool
}

// TableCfg configures a table layout.
type TableCfg struct {
	TextStyle     TextStyle
	TextStyleHead TextStyle
	ColorRowAlt   *Color
	// AlignHead overrides the header alignment. Unset takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	AlignHead *HorizontalAlign
	Selected  map[int]bool
	OnSelect  func(map[int]bool, int, EventCtx)
	ID        string `gui:"required"`
	A11YCfg
	// ColumnAlignments per-column horizontal alignment, in display
	// order. Shorter than the column count leaves the tail at the
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColumnAlignments []HorizontalAlign
	// RawData is a convenience field for CSV-style data. First row
	// is treated as the header. When set, RawData takes precedence
	// over Data.
	// exportaudit:keep — caller-facing config (issue #372)
	RawData [][]string
	Data    []TableRowCfg

	// CellPadding insets every cell. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	CellPadding Padding
	// ColumnWidthDefault is the fallback width for columns without
	// an explicit width. Zero takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColumnWidthDefault float32
	// ColumnWidthMin is the narrowest any column may size. Zero
	// takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColumnWidthMin   float32
	SizeBorder       float32 // ergonomics-audit:opt-plain — 0 = no borders, applied as-is; public API kept plain
	SizeBorderHeader float32 // ergonomics-audit:opt-plain — 0 = no header separator; public API kept plain

	// Scrollable enables scrolling. When set with Height or
	// MaxHeight, virtualization renders only visible rows. Scroll
	// state is keyed by Cfg.ID, or ScopeID(Cfg.ID, "scroll") when
	// FreezeHeader is set — pass that to Window.ScrollVerticalTo.
	Scrollable  bool
	Width       float32
	Height      float32
	MinWidth    float32
	MaxWidth    float32
	MinHeight   float32
	MaxHeight   float32
	ColorBorder Color
	// ColorBorderFocus is the border while the table holds focus.
	// Unset takes the theme's.
	ColorBorderFocus Color
	ColorSelect      Color
	// ColorSelectSubtle is the tint behind a selected row — the
	// wash, never the full accent slab; focus is the ring, not a
	// second fill (visual-refresh §4.3). Unset takes the theme's.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorSelectSubtle Color
	ColorHover        Color
	// Colors sets the per-state colors. The flat Color* fields
	// above win over their Colors slots. Table has no base fill,
	// so Base is unused and Hover/Border are the live slots.
	Colors ColorSet

	// Focusable opts the table into the tab order. When set, focus
	// lands on the table itself and Up/Down/Home/End move an active
	// row — tinted with the hover color — which follows the mouse
	// click and drives selection when OnSelect is set (Shift extends
	// a range under MultiSelect). Enter/Space activate the active
	// row the way a click would. Opt-in, not default: a table is a
	// display surface until its rows are navigable.
	Focusable bool

	// Sizing
	Sizing       Sizing
	BorderStyle  TableBorderStyle
	MultiSelect  bool
	Scrollbar    ScrollbarOverflow
	FreezeHeader bool
}

func applyTableDefaults(cfg *TableCfg) {
	s := &defaultTableStyle
	cfg.Colors = cfg.Colors.resolved(Color{}, themeColorSet(
		Color{}, s.ColorHover, Color{},
		Color{}, s.ColorBorder, s.ColorBorderFocus,
	))
	cfg.Colors.applyTo(nil, &cfg.ColorHover, nil, nil,
		&cfg.ColorBorder, &cfg.ColorBorderFocus)
	// A caller-set ColorSelect is an explicit override and wins over
	// the theme's wash (subtleSlot). Resolved before the theme fill
	// below, so IsSet still tells caller-set from theme-set.
	subtleSlot(&cfg.ColorSelectSubtle, cfg.ColorSelect, s.ColorSelectSubtle)
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = s.ColorSelect
	}
	if !cfg.CellPadding.IsSet() {
		cfg.CellPadding = s.cellPadding
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = s.TextStyle
	}
	if cfg.TextStyleHead == (TextStyle{}) {
		cfg.TextStyleHead = s.TextStyleHead
	}
	if cfg.AlignHead == nil {
		cfg.AlignHead = &s.alignHead
	}
	if cfg.ColumnWidthDefault == 0 {
		cfg.ColumnWidthDefault = s.columnWidthDefault
	}
	if cfg.ColumnWidthMin == 0 {
		cfg.ColumnWidthMin = s.columnWidthMin
	}
}

// tableColWidthCache stores measured column widths keyed by
// content hash.
type tableColWidthCache struct {
	widths []float32
	hash   uint64
}

// Table generates a table from the given TableCfg. Column widths
// are auto-sized when a Window is available during layout.
func Table(cfg TableCfg) View {
	RequireID("Table", cfg.ID)
	return viewFunc(func(w *Window) View {
		return tableView(cfg, w)
	})
}

// Table generates a table with text measurement, column width
// caching, and optional virtualization.
func (w *Window) Table(cfg TableCfg) View {
	return tableView(cfg, w)
}

// tableScrollID returns the scroll key for a table. The freeze path
// scrolls an inner body container that needs its own identity; the
// non-freeze path scrolls the outer Column, which carries cfg.ID.
func tableScrollID(cfg *TableCfg, freeze bool) string {
	if freeze {
		return ScopeID(cfg.ID, "scroll") // inner bodyCfg carries it
	}
	return cfg.ID // outerCfg carries it
}

// tableActiveRow returns the keyboard position for a focusable
// table, or -1 for one that never joins the tab order (so its rows
// take no active-row tint).
func tableActiveRow(cfg *TableCfg, w *Window) int {
	if !cfg.Focusable {
		return -1
	}
	return StateReadOr(w, nsTableFocus, cfg.ID, 0)
}

// tableEmptyView is the bare container an empty table resolves to.
// It still carries the tab stop and ring when focusable, so a
// keyboard user lands on it instead of skipping past a control with
// nothing to navigate yet.
func tableEmptyView(cfg *TableCfg) View {
	fw := tableFocusWiring(cfg, nil, false, nil, "", 0, 0, 0)
	return Column(ContainerCfg{
		ID:          cfg.ID,
		Focusable:   fw.focusable,
		AmendLayout: fw.ring,
		Padding:     NoPadding,
	})
}

func tableView(cfg TableCfg, w *Window) View {
	if len(cfg.RawData) > 0 {
		n := min(len(cfg.RawData), maxDataConvLen)
		cfg.Data = TableCfgFromData(cfg.RawData[:n]).Data
	}
	applyTableDefaults(&cfg)

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)

	// Keyboard focus state (issue #345): the active row is the
	// position arrow keys move and Enter/Space activate. -1 disables
	// the highlight for a table that never joins the tab order.
	activeRowIdx := tableActiveRow(&cfg, w)
	// Row clicks sync the keyboard position so arrow keys continue
	// from where the mouse went — but only a focusable table ever
	// reads it back, so leave the key empty and skip the per-click
	// write for the others.
	navKey := cfg.ID
	if !cfg.Focusable {
		navKey = ""
	}

	if len(cfg.Data) == 0 {
		return tableEmptyView(&cfg)
	}

	lastRowIdx := len(cfg.Data) - 1

	// Cell-level borders for BorderAll; negative spacing
	// collapses doubled borders between cells and rows.
	var cellBorder, rowSpacing float32
	if cfg.BorderStyle == TableBorderAll {
		cellBorder = cfg.SizeBorder
		rowSpacing = -cfg.SizeBorder
	}

	columnWidths := tableColumnWidths(&cfg, w)
	freeze := cfg.FreezeHeader && cfg.Scrollable && len(cfg.Data) > 1
	// Derived once per view: the freeze path's concatenation must not
	// be repeated at each use (virtualization read + bodyCfg literal).
	scrollID := tableScrollID(&cfg, freeze)

	// Hoist loop-invariant values.
	onSelect := cfg.OnSelect
	selected := cfg.Selected
	multiSelect := cfg.MultiSelect
	colorHover := cfg.ColorHover

	// Virtualization.
	listHeight := cfg.Height
	if listHeight <= 0 {
		listHeight = cfg.MaxHeight
	}

	dataStart := 0
	dataCount := len(cfg.Data)
	if freeze {
		dataStart = 1
		dataCount = len(cfg.Data) - 1
	}

	virtualize := cfg.Scrollable && listHeight > 0 &&
		dataCount > 0 && w != nil
	rowHeight := float32(0)
	first, last := dataStart, lastRowIdx
	if virtualize {
		rowHeight = tableEstimateRowHeight(&cfg, w)
		// Default 0: unscrolled position when no offset recorded yet.
		scrollY := w.scrollY().GetOr(scrollID, 0)
		vFirst, vLast := listCoreVisibleRange(
			dataCount, rowHeight, listHeight, scrollY)
		first = vFirst + dataStart
		last = vLast + dataStart
		// Index space: cfg.Data. With Freeze, data row 0 is the frozen
		// header and sits outside the scrollable, which is what
		// indexBase records.
		listHeightRegisterUniform(w, scrollID, dataCount,
			rowHeight, 0, dataStart)
	}

	rows := tableBuildRows(&cfg, columnWidths, cellBorder,
		selected, multiSelect, colorHover, onSelect,
		activeRowIdx, navKey, first, last, lastRowIdx, dataStart,
		rowHeight, virtualize)

	// Keyboard focus wiring for the outer container, shared by the
	// freeze and non-freeze layouts (issue #345).
	fw := tableFocusWiring(&cfg, selected, multiSelect, onSelect,
		scrollID, rowHeight, listHeight, freezeBodyOffset(freeze))

	if freeze {
		return tableFreezeLayout(&cfg, columnWidths, cellBorder,
			rowSpacing, selected, multiSelect, colorHover,
			onSelect, rows, scrollID, activeRowIdx, navKey, fw)
	}

	outerCfg := ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleGrid,
		A11YCfg:   A11YCfg{A11YLabel: cfg.A11YLabel, A11YDescription: cfg.A11YDescription},
		Color:     ColorTransparent,
		Padding:   NoPadding,
		Spacing:   Some(rowSpacing),
		Radius:    SomeF(0),
		Sizing:    cfg.Sizing,
		Width:     cfg.Width,
		Height:    cfg.Height,
		MinWidth:  cfg.MinWidth,
		MaxWidth:  cfg.MaxWidth,
		MinHeight: cfg.MinHeight,
		MaxHeight: cfg.MaxHeight,
		Content:   rows,
	}

	// Focus wiring is a no-op for a non-focusable table: the zero
	// state carries false/nil/nil, which is what outerCfg already is.
	tableWireFocus(&outerCfg, fw)

	if cfg.Scrollable {
		outerCfg.Scrollable = true
		outerCfg.Padding = NewPadding(0, DefaultScrollbarStyle.Size+PadXSmall, 0, 0)
		outerCfg.ScrollbarCfgX = &ScrollbarCfg{
			Overflow: ScrollbarHidden,
		}
		if cfg.Scrollbar != ScrollbarAuto {
			outerCfg.ScrollbarCfgY = &ScrollbarCfg{
				Overflow: cfg.Scrollbar,
			}
		}
	}

	return Column(outerCfg)
}

// TR creates a table row from the given cells.
func TR(cols []TableCellCfg) TableRowCfg {
	return TableRowCfg{Cells: cols}
}

// TH creates a header cell with bold text.
func tH(value string) TableCellCfg {
	ts := DefaultTextStyle
	ts.Typeface = glyph.TypefaceBold
	return TableCellCfg{Value: value, HeadCell: true, TextStyle: &ts}
}

// TD creates a data cell.
func tD(value string) TableCellCfg {
	return TableCellCfg{Value: value}
}

// TableCfgFromData creates a TableCfg from [][]string.
// First row is treated as a header row.
func TableCfgFromData(data [][]string) TableCfg {
	rows := make([]TableRowCfg, 0, len(data))
	for i, r := range data {
		cells := make([]TableCellCfg, 0, len(r))
		for _, cell := range r {
			if i == 0 {
				cells = append(cells, tH(cell))
			} else {
				cells = append(cells, tD(cell))
			}
		}
		rows = append(rows, TableRowCfg{Cells: cells})
	}
	return TableCfg{Data: rows}
}

// TableCfgFromCSV parses CSV data into a TableCfg. First row
// is treated as a header row.
func tableCfgFromCSV(data string) (TableCfg, error) {
	reader := csv.NewReader(strings.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return TableCfg{}, err
	}
	return TableCfgFromData(records), nil
}

// TableFromCSV parses CSV data and returns a table view.
// On parse error, returns an error table.
func (w *Window) tableFromCSV(data string) View {
	cfg, err := tableCfgFromCSV(data)
	if err != nil {
		return w.Table(tableCfgError(err.Error()))
	}
	return w.Table(cfg)
}

// TableCfgError creates a TableCfg with an error message.
func tableCfgError(message string) TableCfg {
	return TableCfg{Data: []TableRowCfg{TR([]TableCellCfg{tD(message)})}}
}

func copySelected(m map[int]bool) map[int]bool {
	if m == nil {
		return make(map[int]bool)
	}
	cp := make(map[int]bool, len(m))
	maps.Copy(cp, m)
	return cp
}
