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
	alignHead     *HorizontalAlign
	Selected      map[int]bool
	OnSelect      func(map[int]bool, int, EventCtx)
	ID            string `gui:"required"`
	A11YCfg
	columnAlignments []HorizontalAlign
	// RawData is a convenience field for CSV-style data. First row
	// is treated as the header. When set, RawData takes precedence
	// over Data.
	rawData [][]string
	Data    []TableRowCfg

	cellPadding        Padding
	columnWidthDefault float32
	columnWidthMin     float32
	SizeBorder         float32 // ergonomics-audit:opt-plain — 0 = no borders, applied as-is; public API kept plain
	SizeBorderHeader   float32 // ergonomics-audit:opt-plain — 0 = no header separator; public API kept plain

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
	ColorSelect Color
	ColorHover  Color

	// Sizing
	Sizing       Sizing
	BorderStyle  TableBorderStyle
	MultiSelect  bool
	Scrollbar    ScrollbarOverflow
	FreezeHeader bool
}

func applyTableDefaults(cfg *TableCfg) {
	s := &defaultTableStyle
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = s.ColorBorder
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = s.ColorSelect
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = s.ColorHover
	}
	if !cfg.cellPadding.IsSet() {
		cfg.cellPadding = s.cellPadding
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = s.TextStyle
	}
	if cfg.TextStyleHead == (TextStyle{}) {
		cfg.TextStyleHead = s.TextStyleHead
	}
	if cfg.alignHead == nil {
		cfg.alignHead = &s.alignHead
	}
	if cfg.columnWidthDefault == 0 {
		cfg.columnWidthDefault = s.columnWidthDefault
	}
	if cfg.columnWidthMin == 0 {
		cfg.columnWidthMin = s.columnWidthMin
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

func tableView(cfg TableCfg, w *Window) View {
	if len(cfg.rawData) > 0 {
		n := min(len(cfg.rawData), maxDataConvLen)
		cfg.Data = TableCfgFromData(cfg.rawData[:n]).Data
	}
	applyTableDefaults(&cfg)

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)

	if len(cfg.Data) == 0 {
		return Column(ContainerCfg{
			ID:      cfg.ID,
			Padding: NoPadding,
		})
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
	}

	capacity := last - first + 3
	if !virtualize {
		capacity = len(cfg.Data) * 2
	}
	rows := make([]View, 0, capacity)

	// Top spacer for virtualization.
	if virtualize && first > dataStart && rowHeight > 0 {
		rows = append(rows, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(first-dataStart) * rowHeight,
			Sizing: FillFixed,
		}))
	}

	for rowIdx := first; rowIdx <= last; rowIdx++ {
		if rowIdx < 0 || rowIdx > lastRowIdx {
			continue
		}
		rows = append(rows, tableBuildRow(
			&cfg, rowIdx, columnWidths, cellBorder,
			selected, multiSelect, colorHover, onSelect))

		// Horizontal separator.
		sepHeight := cfg.SizeBorder
		if rowIdx == 0 && cfg.SizeBorderHeader > 0 {
			sepHeight = cfg.SizeBorderHeader
		}

		needsSep := false
		switch cfg.BorderStyle {
		case TableBorderHorizontal:
			needsSep = rowIdx != lastRowIdx
		case TableBorderHeaderOnly:
			needsSep = rowIdx == 0
		}

		if needsSep {
			rows = append(rows, Rectangle(RectangleCfg{
				Color:  cfg.ColorBorder,
				Height: sepHeight,
				Sizing: FillFixed,
			}))
		}
	}

	// Bottom spacer for virtualization.
	if virtualize && last < lastRowIdx && rowHeight > 0 {
		remaining := lastRowIdx - last
		rows = append(rows, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(remaining) * rowHeight,
			Sizing: FillFixed,
		}))
	}

	if freeze {
		return tableFreezeLayout(&cfg, columnWidths, cellBorder,
			rowSpacing, selected, multiSelect, colorHover,
			onSelect, rows, scrollID)
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
