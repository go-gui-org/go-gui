package gui

import (
	"hash/fnv"
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
