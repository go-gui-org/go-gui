//go:build darwin && !ios

package metal

import (
	"math"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
)

// defaultTermSelTint is the fallback selection highlight when the
// caller leaves TermSelRange.Color unset.
var defaultTermSelTint = gui.RGBA(51, 153, 255, 100)

// drawTermGrid renders a terminal character grid from a shared cell
// buffer in a single pass: background fills, glyph runs (pinned to
// exact cell columns via DrawLayoutPlaced so the grid never drifts),
// then cursor and selection overlays. No per-cell Layout nodes exist.
//
// Attribute support (v1): reverse (swaps fg/bg) and underline are
// honored; bold/italic are reserved in TermAttr but not yet rendered.
func (b *windowState) drawTermGrid(r *gui.RenderCmd) {
	tg := r.TermGrid
	if b.textSys == nil || tg == nil || tg.Cols <= 0 || tg.Rows <= 0 ||
		tg.CellW <= 0 || math.IsNaN(float64(tg.CellW)) || math.IsInf(float64(tg.CellW), 0) ||
		tg.CellH <= 0 || math.IsNaN(float64(tg.CellH)) || math.IsInf(float64(tg.CellH), 0) ||
		len(tg.Cells) < tg.Cols*tg.Rows {
		return
	}
	gx, gy := r.X, r.Y
	cw, ch := tg.CellW, tg.CellH

	cfg := guiStyleToGlyphConfig(tg.Style)
	ascent := ch * 0.8
	if m, err := b.textSys.FontMetrics(cfg); err == nil && m.Ascender > 0 {
		ascent = m.Ascender
	}

	b.termGridBackgrounds(tg, gx, gy, cw, ch)
	b.termGridSelection(tg, gx, gy, cw, ch)
	b.termGridCursorUnder(tg, gx, gy, cw, ch)
	b.termGridGlyphs(tg, cfg, gx, gy, cw, ch, ascent)
	b.termGridDecorations(tg, gx, gy, cw, ch)
}

// termCellFB returns the effective foreground/background for a cell,
// applying the reverse attribute.
func termCellFB(c *gui.TermCell) (fg, bg gui.Color) {
	fg, bg = c.FG, c.BG
	if c.Attrs&gui.TermReverse != 0 {
		fg, bg = bg, fg
	}
	return fg, bg
}

// termGridBackgrounds fills batched runs of same-background cells.
func (b *windowState) termGridBackgrounds(
	tg *gui.TermGridData, gx, gy, cw, ch float32,
) {
	for row := 0; row < tg.Rows; row++ {
		col := 0
		for col < tg.Cols {
			c := &tg.Cells[row*tg.Cols+col]
			_, bg := termCellFB(c)
			if bg.A == 0 {
				col++
				continue
			}
			start := col
			for col < tg.Cols {
				_, nbg := termCellFB(&tg.Cells[row*tg.Cols+col])
				if nbg != bg {
					break
				}
				col++
			}
			b.fillTermRect(
				gx+float32(start)*cw, gy+float32(row)*ch,
				float32(col-start)*cw, ch, bg)
		}
	}
}

// termGridSelection tints the selected cell range per row.
func (b *windowState) termGridSelection(
	tg *gui.TermGridData, gx, gy, cw, ch float32,
) {
	sel := tg.Selection
	if sel.End <= sel.Start {
		return
	}
	tint := sel.Color
	if tint.A == 0 {
		tint = defaultTermSelTint
	}
	total := tg.Cols * tg.Rows
	beg := max(sel.Start, 0)
	end := min(sel.End, total)
	for row := 0; row < tg.Rows && beg < end; row++ {
		rowStart := row * tg.Cols
		rowEnd := rowStart + tg.Cols
		s := max(beg, rowStart)
		e := min(end, rowEnd)
		if s >= e {
			continue
		}
		c0 := s - rowStart
		c1 := e - rowStart
		b.fillTermRect(
			gx+float32(c0)*cw, gy+float32(row)*ch,
			float32(c1-c0)*cw, ch, tint)
	}
}

// termGridCursorUnder draws the block cursor beneath the glyphs so the
// character remains visible on top.
func (b *windowState) termGridCursorUnder(
	tg *gui.TermGridData, gx, gy, cw, ch float32,
) {
	cur := tg.Cursor
	if !cur.Visible || cur.Style != gui.TermCursorBlock ||
		cur.Col < 0 || cur.Col >= tg.Cols ||
		cur.Row < 0 || cur.Row >= tg.Rows {
		return
	}
	b.fillTermRect(
		gx+float32(cur.Col)*cw, gy+float32(cur.Row)*ch,
		cw, ch, cur.Color)
}

// termGridGlyphs shapes each row into foreground-color runs and draws
// them with glyphs pinned to their cell columns.
func (b *windowState) termGridGlyphs(
	tg *gui.TermGridData, cfg glyph.TextConfig,
	gx, gy, cw, ch, ascent float32,
) {
	b.useGlyphPipeline()
	for row := 0; row < tg.Rows; row++ {
		baseline := gy + float32(row)*ch + ascent
		col := 0
		for col < tg.Cols {
			c := &tg.Cells[row*tg.Cols+col]
			if c.Width == 0 { // continuation of a wide cell
				col++
				continue
			}
			fg, _ := termCellFB(c)
			// Extend a run of glyphs sharing this foreground color.
			b.termGridRunReset()
			runFG := fg
			for col < tg.Cols {
				cell := &tg.Cells[row*tg.Cols+col]
				if cell.Width == 0 {
					col++
					continue
				}
				fg2, _ := termCellFB(cell)
				blank := cell.Ch == 0 || cell.Ch == ' '
				if !blank && fg2 != runFG {
					break
				}
				if blank {
					b.termGridRunAppend(' ', col)
				} else {
					b.termGridRunAppend(cell.Ch, col)
				}
				col++
			}
			b.termGridDrawRun(cfg, runFG, gx, baseline, cw)
		}
	}
}

// termGridDecorations draws underline attributes and the bar /
// underline cursor styles on top of the glyphs.
func (b *windowState) termGridDecorations(
	tg *gui.TermGridData, gx, gy, cw, ch float32,
) {
	thick := max(ch*0.08, 1)
	for row := 0; row < tg.Rows; row++ {
		for col := 0; col < tg.Cols; col++ {
			c := &tg.Cells[row*tg.Cols+col]
			if c.Attrs&gui.TermUnderline == 0 {
				continue
			}
			fg, _ := termCellFB(c)
			b.fillTermRect(
				gx+float32(col)*cw, gy+float32(row+1)*ch-thick,
				cw, thick, fg)
		}
	}

	cur := tg.Cursor
	if !cur.Visible || cur.Col < 0 || cur.Col >= tg.Cols ||
		cur.Row < 0 || cur.Row >= tg.Rows {
		return
	}
	x := gx + float32(cur.Col)*cw
	y := gy + float32(cur.Row)*ch
	switch cur.Style {
	case gui.TermCursorBar:
		b.fillTermRect(x, y, max(cw*0.15, 1), ch, cur.Color)
	case gui.TermCursorUnderline:
		b.fillTermRect(x, y+ch-thick, cw, thick, cur.Color)
	case gui.TermCursorBlock:
		// Drawn under the glyph in termGridCursorUnder.
	}
}

// --- run assembly helpers (reuse scratch buffers) ---

func (b *windowState) termGridRunReset() {
	b.termRunText = b.termRunText[:0]
	b.termRunCols = b.termRunCols[:0]
}

func (b *windowState) termGridRunAppend(ch rune, col int) {
	b.termRunText = append(b.termRunText, ch)
	b.termRunCols = append(b.termRunCols, col)
}

// termGridDrawRun shapes the accumulated run and draws its glyphs at
// fixed cell columns. Falls back to natural flow if the shaper does
// not produce one glyph per cell (e.g. clustered non-ASCII runs).
func (b *windowState) termGridDrawRun(
	cfg glyph.TextConfig, fg gui.Color, gx, baseline, cw float32,
) {
	if len(b.termRunCols) == 0 {
		return
	}
	cfg.Style.Color = glyph.Color{R: fg.R, G: fg.G, B: fg.B, A: fg.A}
	layout, err := b.textSys.LayoutText(string(b.termRunText), cfg)
	if err != nil {
		return
	}
	if len(layout.Glyphs) != len(b.termRunCols) {
		// Cluster/ligature mismatch — draw at the run's first column.
		b.textSys.DrawLayout(layout, gx+float32(b.termRunCols[0])*cw, baseline)
		return
	}
	b.termPlace = b.termPlace[:0]
	for _, col := range b.termRunCols {
		b.termPlace = append(b.termPlace, glyph.GlyphPlacement{
			X: gx + float32(col)*cw,
			Y: baseline,
		})
	}
	b.textSys.DrawLayoutPlaced(layout, b.termPlace)
}

// fillTermRect draws a solid filled rectangle in logical coordinates.
func (b *windowState) fillTermRect(x, y, w, h float32, c gui.Color) {
	if w <= 0 || h <= 0 || c.A == 0 {
		return
	}
	cmd := gui.RenderCmd{
		Kind: gui.RenderRect, X: x, Y: y, W: w, H: h,
		Color: c, Fill: true,
	}
	b.drawRect(&cmd)
}
