package gui

import (
	"encoding/csv"
	"hash/fnv"
	"log"
	"maps"
	"strings"
	"sync"

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
	OnClick func(*Layout, *Event, *Window)
	ID      string
	Cells   []TableCellCfg
}

// TableCellCfg configures a table cell.
type TableCellCfg struct {
	Content   View
	HAlign    *HorizontalAlign
	TextStyle *TextStyle
	RichText  *RichText
	OnClick   func(*Layout, *Event, *Window)
	ID        string
	Value     string
	HeadCell  bool
}

// TableCfg configures a table layout.
type TableCfg struct {
	TextStyle        TextStyle
	TextStyleHead    TextStyle
	ColorRowAlt      *Color
	AlignHead        *HorizontalAlign
	Selected         map[int]bool
	OnSelect         func(map[int]bool, int, *Event, *Window)
	ID               string `gui:"required"`
	A11YLabel        string
	A11YDescription  string
	ColumnAlignments []HorizontalAlign
	// RawData is a convenience field for CSV-style data. First row
	// is treated as the header. When set, RawData takes precedence
	// over Data.
	RawData [][]string
	Data    []TableRowCfg

	CellPadding        Opt[Padding]
	ColumnWidthDefault float32
	ColumnWidthMin     float32
	SizeBorder         float32
	SizeBorderHeader   float32

	// IDScroll enables scrolling. When set with Height or
	// MaxHeight, virtualization renders only visible rows.
	IDScroll    uint32
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
	s := &DefaultTableStyle
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = s.ColorBorder
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = s.ColorSelect
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = s.ColorHover
	}
	if !cfg.CellPadding.IsSet() {
		cfg.CellPadding = Some(s.CellPadding)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = s.TextStyle
	}
	if cfg.TextStyleHead == (TextStyle{}) {
		cfg.TextStyleHead = s.TextStyleHead
	}
	if cfg.AlignHead == nil {
		cfg.AlignHead = &s.AlignHead
	}
	if cfg.ColumnWidthDefault == 0 {
		cfg.ColumnWidthDefault = s.ColumnWidthDefault
	}
	if cfg.ColumnWidthMin == 0 {
		cfg.ColumnWidthMin = s.ColumnWidthMin
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
	return &deferredTableView{cfg: cfg}
}

type deferredTableView struct {
	cfg TableCfg
}

func (d *deferredTableView) Content() []View { return nil }

func (d *deferredTableView) GenerateLayout(w *Window) Layout {
	return generateViewLayout(tableView(d.cfg, w), w)
}

// Table generates a table with text measurement, column width
// caching, and optional virtualization.
func (w *Window) Table(cfg TableCfg) View {
	return tableView(cfg, w)
}

func tableView(cfg TableCfg, w *Window) View {
	if len(cfg.RawData) > 0 {
		n := min(len(cfg.RawData), maxDataConvLen)
		cfg.Data = TableCfgFromData(cfg.RawData[:n]).Data
	}
	applyTableDefaults(&cfg)

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
	freeze := cfg.FreezeHeader && cfg.IDScroll > 0 && len(cfg.Data) > 1

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

	virtualize := cfg.IDScroll > 0 && listHeight > 0 &&
		dataCount > 0 && w != nil
	rowHeight := float32(0)
	first, last := dataStart, lastRowIdx
	if virtualize {
		rowHeight = tableEstimateRowHeight(&cfg, w)
		scrollY, _ := w.scrollY().Get(cfg.IDScroll)
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
			onSelect, rows)
	}

	outerCfg := ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleGrid,
		A11YLabel: cfg.A11YLabel,
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

	if cfg.IDScroll > 0 {
		outerCfg.IDScroll = cfg.IDScroll
		outerCfg.Padding = Some(Padding{Right: DefaultScrollbarStyle.Size + PadXSmall})
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

// tableBuildRow builds a single table row view.
func tableBuildRow(
	cfg *TableCfg, rowIdx int, columnWidths []float32,
	cellBorder float32, selected map[int]bool,
	multiSelect bool, colorHover Color,
	onSelect func(map[int]bool, int, *Event, *Window),
) View {
	r := cfg.Data[rowIdx]
	cells := make([]View, 0, len(r.Cells))

	for colIdx, cell := range r.Cells {
		cellTextStyle := cfg.TextStyle
		if cell.TextStyle != nil {
			cellTextStyle = *cell.TextStyle
		} else if cell.HeadCell {
			cellTextStyle = cfg.TextStyleHead
		}

		var colWidth float32
		if colIdx < len(columnWidths) {
			colWidth = columnWidths[colIdx]
		}

		hAlign := HAlignStart
		if cell.HAlign != nil {
			hAlign = *cell.HAlign
		} else if cell.HeadCell {
			hAlign = *cfg.AlignHead
		} else if colIdx < len(cfg.ColumnAlignments) {
			hAlign = cfg.ColumnAlignments[colIdx]
		}

		var cellContent []View
		if cell.RichText != nil {
			cellContent = []View{
				RTF(RtfCfg{RichText: *cell.RichText}),
			}
		} else if cell.Content != nil {
			cellContent = []View{cell.Content}
		} else {
			cellContent = []View{
				Text(TextCfg{
					Text:      cell.Value,
					TextStyle: cellTextStyle,
				}),
			}
		}

		cellOnClick := cell.OnClick
		ch := colorHover
		var cellOnHover func(*Layout, *Event, *Window)
		if cellOnClick != nil {
			cellOnHover = func(layout *Layout, _ *Event, w *Window) {
				w.SetMouseCursorPointingHand()
				layout.Shape.Color = ch
			}
		}

		cells = append(cells, Column(ContainerCfg{
			A11YRole:    AccessRoleGridCell,
			Color:       ColorTransparent,
			ColorBorder: cfg.ColorBorder,
			SizeBorder:  Some(cellBorder),
			Padding:     cfg.CellPadding,
			Radius:      SomeF(0),
			HAlign:      hAlign,
			Sizing:      FixedFill,
			Width:       colWidth,
			OnClick:     cellOnClick,
			OnHover:     cellOnHover,
			Content:     cellContent,
		}))
	}

	isSelected := selected[rowIdx]
	rowColor := ColorTransparent
	if isSelected {
		rowColor = cfg.ColorSelect
	} else if cfg.ColorRowAlt != nil && rowIdx%2 == 1 {
		rowColor = *cfg.ColorRowAlt
	}

	rowOnClick := r.OnClick
	ri := rowIdx

	return Row(ContainerCfg{
		Color:      rowColor,
		Spacing:    Some(-cellBorder),
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    cells,
		OnClick: func(layout *Layout, e *Event, w *Window) {
			if rowOnClick != nil {
				rowOnClick(layout, e, w)
			}
			if onSelect != nil {
				var newSel map[int]bool
				if multiSelect {
					newSel = copySelected(selected)
				} else {
					newSel = make(map[int]bool)
				}
				if newSel[ri] {
					delete(newSel, ri)
				} else {
					newSel[ri] = true
				}
				onSelect(newSel, ri, e, w)
			}
		},
		OnHover: func(layout *Layout, _ *Event, w *Window) {
			if onSelect != nil {
				w.SetMouseCursorPointingHand()
				if !isSelected {
					layout.Shape.Color = colorHover
				}
			}
		},
	})
}

// tableFreezeLayout builds the split layout: fixed header zone
// above a scrollable body zone.
func tableFreezeLayout(
	cfg *TableCfg, columnWidths []float32, cellBorder float32,
	rowSpacing float32, selected map[int]bool,
	multiSelect bool, colorHover Color,
	onSelect func(map[int]bool, int, *Event, *Window),
	bodyRows []View,
) View {
	// Header zone: row 0 + optional separator.
	headerViews := []View{
		tableBuildRow(cfg, 0, columnWidths, cellBorder,
			selected, multiSelect, colorHover, onSelect),
	}

	sepHeight := cfg.SizeBorder
	if cfg.SizeBorderHeader > 0 {
		sepHeight = cfg.SizeBorderHeader
	}
	needsSep := false
	switch cfg.BorderStyle {
	case TableBorderHorizontal, TableBorderHeaderOnly:
		needsSep = true
	}
	if needsSep {
		headerViews = append(headerViews, Rectangle(RectangleCfg{
			Color:  cfg.ColorBorder,
			Height: sepHeight,
			Sizing: FillFixed,
		}))
	}

	headerZone := Column(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NoPadding,
		Spacing:    Some(rowSpacing),
		SizeBorder: NoBorder,
		Content:    headerViews,
	})

	bodyCfg := ContainerCfg{
		Sizing:     FillFill,
		Padding:    Some(Padding{Right: DefaultScrollbarStyle.Size + PadXSmall}),
		Spacing:    Some(rowSpacing),
		SizeBorder: NoBorder,
		IDScroll:   cfg.IDScroll,
		Content:    bodyRows,
		ScrollbarCfgX: &ScrollbarCfg{
			Overflow: ScrollbarHidden,
		},
	}
	if cfg.Scrollbar != ScrollbarAuto {
		bodyCfg.ScrollbarCfgY = &ScrollbarCfg{
			Overflow: cfg.Scrollbar,
		}
	}
	bodyZone := Column(bodyCfg)

	return Column(ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleGrid,
		A11YLabel: cfg.A11YLabel,
		Color:     ColorTransparent,
		Padding:   NoPadding,
		Spacing:   SomeF(0),
		Radius:    SomeF(0),
		Sizing:    cfg.Sizing,
		Width:     cfg.Width,
		Height:    cfg.Height,
		MinWidth:  cfg.MinWidth,
		MaxWidth:  cfg.MaxWidth,
		MinHeight: cfg.MinHeight,
		MaxHeight: cfg.MaxHeight,
		Content: []View{
			headerZone,
			bodyZone,
		},
	})
}

// tableColumnWidths computes column widths. When w is non-nil,
// measures text and caches results in StateMap.
func tableColumnWidths(cfg *TableCfg, w *Window) []float32 {
	numCols := 0
	for _, r := range cfg.Data {
		if len(r.Cells) > numCols {
			numCols = len(r.Cells)
		}
	}

	if w == nil || w.textMeasurer == nil {
		widths := make([]float32, numCols)
		cw := cfg.ColumnWidthDefault +
			cfg.CellPadding.Get(Padding{}).Width()
		for i := range widths {
			widths[i] = cw
		}
		return widths
	}

	hash := tableColumnWidthHash(cfg)

	if cfg.ID != "" {
		cache := StateMap[string, tableColWidthCache](
			w, nsTableColWidths, capModerate)
		if cached, ok := cache.Get(cfg.ID); ok &&
			cached.hash == hash {
			return cached.widths
		}
	} else if len(cfg.Data) > 20 {
		tableWarnNoID()
	}

	widths := tableMeasureWidths(cfg, w.textMeasurer)

	if cfg.ID != "" {
		cache := StateMap[string, tableColWidthCache](
			w, nsTableColWidths, capModerate)
		cache.Set(cfg.ID, tableColWidthCache{
			hash: hash, widths: widths,
		})
	}

	return widths
}

// tableMeasureWidths measures all columns using TextMeasurer.
func tableMeasureWidths(
	cfg *TableCfg, tm TextMeasurer,
) []float32 {
	numCols := 0
	for _, r := range cfg.Data {
		if len(r.Cells) > numCols {
			numCols = len(r.Cells)
		}
	}
	widths := make([]float32, numCols)
	pad := cfg.CellPadding.Get(Padding{}).Width()

	for _, r := range cfg.Data {
		for ci, cell := range r.Cells {
			var tw float32
			if cell.RichText != nil {
				tw = tableRichTextWidth(cell.RichText, tm)
			} else {
				style := cfg.TextStyle
				if cell.TextStyle != nil {
					style = *cell.TextStyle
				} else if cell.HeadCell {
					style = cfg.TextStyleHead
				}
				tw = tm.TextWidth(cell.Value, style)
			}
			tw += pad
			if tw > widths[ci] {
				widths[ci] = tw
			}
		}
	}

	for i := range widths {
		if widths[i] < cfg.ColumnWidthMin {
			widths[i] = cfg.ColumnWidthMin
		}
	}
	return widths
}

// tableRichTextWidth sums the width of each run.
func tableRichTextWidth(rt *RichText, tm TextMeasurer) float32 {
	var w float32
	for _, run := range rt.Runs {
		w += tm.TextWidth(run.Text, run.Style)
	}
	return w
}

// tableColumnWidthHash computes FNV-1a hash over sampled cell
// values. Samples first, middle, and last rows.
func tableColumnWidthHash(cfg *TableCfg) uint64 {
	h := fnv.New64a()
	n := len(cfg.Data)
	_, _ = h.Write([]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)})
	indices := make([]int, 0, 3)
	if n > 0 {
		indices = append(indices, 0)
	}
	if n > 2 {
		indices = append(indices, n/2)
	}
	if n > 1 {
		indices = append(indices, n-1)
	}
	for _, i := range indices {
		for _, cell := range cfg.Data[i].Cells {
			_, _ = h.Write([]byte(cell.Value))
		}
	}
	return h.Sum64()
}

var tableWarnNoID = sync.OnceFunc(func() {
	log.Printf("gui.Table: table with >20 rows has no ID; " +
		"column width caching disabled")
})

// tableEstimateRowHeight estimates row height from TextStyle,
// cell padding, and border.
func tableEstimateRowHeight(cfg *TableCfg, w *Window) float32 {
	style := cfg.TextStyle
	height := style.Size
	if w != nil && w.textMeasurer != nil {
		height = w.textMeasurer.FontHeight(style)
	}
	return height + cfg.CellPadding.Get(Padding{}).Height()
}

// ClearTableCache removes cached column widths for the given
// table ID.
func (w *Window) ClearTableCache(id string) {
	cache := StateMapRead[string, tableColWidthCache](
		w, nsTableColWidths)
	if cache != nil {
		cache.Delete(id)
	}
}

// ClearAllTableCaches removes all cached table column widths.
func (w *Window) ClearAllTableCaches() {
	cache := StateMapRead[string, tableColWidthCache](
		w, nsTableColWidths)
	if cache != nil {
		cache.Clear()
	}
}

// TR creates a table row from the given cells.
func TR(cols []TableCellCfg) TableRowCfg {
	return TableRowCfg{Cells: cols}
}

// TH creates a header cell with bold text.
func TH(value string) TableCellCfg {
	ts := DefaultTextStyle
	ts.Typeface = glyph.TypefaceBold
	return TableCellCfg{Value: value, HeadCell: true, TextStyle: &ts}
}

// TD creates a data cell.
func TD(value string) TableCellCfg {
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
				cells = append(cells, TH(cell))
			} else {
				cells = append(cells, TD(cell))
			}
		}
		rows = append(rows, TableRowCfg{Cells: cells})
	}
	return TableCfg{Data: rows}
}

// TableCfgFromCSV parses CSV data into a TableCfg. First row
// is treated as a header row.
func TableCfgFromCSV(data string) (TableCfg, error) {
	reader := csv.NewReader(strings.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return TableCfg{}, err
	}
	return TableCfgFromData(records), nil
}

// TableFromCSV parses CSV data and returns a table view.
// On parse error, returns an error table.
func (w *Window) TableFromCSV(data string) View {
	cfg, err := TableCfgFromCSV(data)
	if err != nil {
		return w.Table(TableCfgError(err.Error()))
	}
	return w.Table(cfg)
}

// TableCfgError creates a TableCfg with an error message.
func TableCfgError(message string) TableCfg {
	return TableCfg{Data: []TableRowCfg{TR([]TableCellCfg{TD(message)})}}
}

func copySelected(m map[int]bool) map[int]bool {
	if m == nil {
		return make(map[int]bool)
	}
	cp := make(map[int]bool, len(m))
	maps.Copy(cp, m)
	return cp
}
