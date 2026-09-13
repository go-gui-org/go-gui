package gui

import (
	"log"
	"sync"
)

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
			cfg.CellPadding.Or(PaddingNone).Width()
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
	pad := cfg.CellPadding.Or(PaddingNone).Width()

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

// tableColumnWidthHash computes an FNV-1a hash over the inputs
// column widths derive from: row count, cell padding, minimum
// width, the table and header text styles, and sampled cell
// values. It samples first, middle, and last rows so wide tables
// stay cheap; unit separators keep adjacent cells apart so one
// row of ("ab", "c") never keys like ("a", "bc").
func tableColumnWidthHash(cfg *TableCfg) uint64 {
	h := Fnv64Offset
	if cfg == nil {
		return h
	}
	h = fnvU64(h, uint64(len(cfg.Data)))
	h = fnvU64(h, uint64(normFloat32Bits(
		cfg.CellPadding.Or(PaddingNone).Width())))
	h = fnvU64(h, uint64(normFloat32Bits(cfg.ColumnWidthMin)))
	h = fnvTextStyle(h, cfg.TextStyle)
	h = fnvTextStyle(h, cfg.TextStyleHead)
	n := len(cfg.Data)
	indices := [3]int{0, n / 2, n - 1}
	last := -1
	for _, i := range indices {
		if i < 0 || i >= n || i == last {
			continue
		}
		last = i
		h = Fnv64Byte(h, fnvRecordSep)
		for _, cell := range cfg.Data[i].Cells {
			h = Fnv64Byte(h, fnvUnitSep)
			h = Fnv64Str(h, cell.Value)
			if cell.HeadCell {
				h = Fnv64Byte(h, 1)
			} else {
				h = Fnv64Byte(h, 0)
			}
			if cell.TextStyle != nil {
				h = fnvTextStyle(h, *cell.TextStyle)
			}
			h = Fnv64Byte(h, fnvUnitSep)
			if cell.RichText != nil {
				for _, run := range cell.RichText.Runs {
					h = Fnv64Str(h, run.Text)
					h = Fnv64Byte(h, fnvUnitSep)
					h = fnvTextStyle(h, run.Style)
				}
			}
		}
	}
	return h
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
	return height + cfg.CellPadding.Or(PaddingNone).Height()
}

// ClearTableCache removes cached column widths for the given
// table ID.
func (w *Window) clearTableCache(id string) {
	cache := StateMapRead[string, tableColWidthCache](
		w, nsTableColWidths)
	if cache != nil {
		cache.Delete(id)
	}
}

// ClearAllTableCaches removes all cached table column widths.
func (w *Window) clearAllTableCaches() {
	cache := StateMapRead[string, tableColWidthCache](
		w, nsTableColWidths)
	if cache != nil {
		cache.Clear()
	}
}
