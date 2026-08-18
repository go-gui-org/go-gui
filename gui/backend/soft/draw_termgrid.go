package soft

import (
	"math"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
)

// defaultTermSelTint is the fallback selection highlight when the
// caller leaves TermSelRange.Color unset. It matches the Metal
// backend's fallback, so a headless capture of a terminal is the same
// picture the window shows.
var defaultTermSelTint = gui.RGBA(51, 153, 255, 100)

// drawTermGrid renders a terminal character grid from the shared cell
// buffer: background runs, selection tint, block cursor, glyph runs
// pinned to their columns, then underline and caret decorations.
//
// It is a port of gui/backend/metal/termgrid.go — the draw order and
// the run batching are what keep a grid from drifting a subpixel per
// column — over this package's rect fill and text system.
func (r *renderer) drawTermGrid(cmd *gui.RenderCmd) {
	tg := cmd.TermGrid
	if r.textSys == nil || tg == nil || !validTermGrid(tg) {
		return
	}
	gx, gy := cmd.X, cmd.Y
	cw, ch := tg.CellW, tg.CellH

	cfg := guiStyleToGlyphConfig(tg.Style)
	ascent := ch * 0.8
	if m, err := r.textSys.FontMetrics(cfg); err == nil && m.Ascender > 0 {
		ascent = m.Ascender
	}

	if r.warm {
		// The warming pass exists to rasterize glyphs into the atlas;
		// the fills would be wiped by the clear before pass 2 anyway.
		r.termGlyphs(tg, cfg, gx, gy, cw, ch, ascent)
		return
	}

	r.termBackgrounds(tg, gx, gy, cw, ch)
	r.termSelection(tg, gx, gy, cw, ch)
	r.termCursorUnder(tg, gx, gy, cw, ch)
	r.termGlyphs(tg, cfg, gx, gy, cw, ch, ascent)
	r.termDecorations(tg, gx, gy, cw, ch)
}

// validTermGrid rejects the degenerate geometry the render path also
// guards against, so a malformed grid draws nothing rather than
// looping on a NaN cell size. The cell-count check is written as a
// division so a hostile Cols*Rows cannot overflow the check and let a
// huge grid loop over an empty cell buffer.
func validTermGrid(tg *gui.TermGridData) bool {
	return tg.Cols > 0 && tg.Rows > 0 &&
		tg.CellW > 0 && !math.IsNaN(float64(tg.CellW)) &&
		!math.IsInf(float64(tg.CellW), 0) &&
		tg.CellH > 0 && !math.IsNaN(float64(tg.CellH)) &&
		!math.IsInf(float64(tg.CellH), 0) &&
		tg.Rows <= len(tg.Cells)/tg.Cols
}

// termCellFB returns a cell's effective colors, applying reverse video.
func termCellFB(c *gui.TermCell) (fg, bg gui.Color) {
	fg, bg = c.FG, c.BG
	if c.Attrs&gui.TermReverse != 0 {
		fg, bg = bg, fg
	}
	return fg, bg
}

// termBackgrounds fills batched runs of same-background cells.
func (r *renderer) termBackgrounds(tg *gui.TermGridData,
	gx, gy, cw, ch float32) {

	for row := range tg.Rows {
		col := 0
		for col < tg.Cols {
			_, bg := termCellFB(&tg.Cells[row*tg.Cols+col])
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
			r.fillTermRect(gx+float32(start)*cw, gy+float32(row)*ch,
				float32(col-start)*cw, ch, bg)
		}
	}
}

// termSelection tints the selected cell range, row by row.
func (r *renderer) termSelection(tg *gui.TermGridData,
	gx, gy, cw, ch float32) {

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
		s := max(beg, rowStart)
		e := min(end, rowStart+tg.Cols)
		if s >= e {
			continue
		}
		r.fillTermRect(gx+float32(s-rowStart)*cw, gy+float32(row)*ch,
			float32(e-s)*cw, ch, tint)
	}
}

// termCursorUnder draws the block cursor beneath the glyphs so the
// character stays visible on top of it.
func (r *renderer) termCursorUnder(tg *gui.TermGridData,
	gx, gy, cw, ch float32) {

	cur := tg.Cursor
	if !cur.Visible || cur.Style != gui.TermCursorBlock ||
		!inGrid(tg, cur.Col, cur.Row) {
		return
	}
	r.fillTermRect(gx+float32(cur.Col)*cw, gy+float32(cur.Row)*ch,
		cw, ch, cur.Color)
}

// termGlyphs shapes each row into foreground-color runs and draws them
// with every glyph pinned to its cell column.
func (r *renderer) termGlyphs(tg *gui.TermGridData, cfg glyph.TextConfig,
	gx, gy, cw, ch, ascent float32) {

	for row := range tg.Rows {
		baseline := gy + float32(row)*ch + ascent
		col := 0
		for col < tg.Cols {
			c := &tg.Cells[row*tg.Cols+col]
			if c.Width == 0 { // continuation of a wide cell
				col++
				continue
			}
			runFG, _ := termCellFB(c)
			r.termRunes = r.termRunes[:0]
			r.termCols = r.termCols[:0]
			for col < tg.Cols {
				cell := &tg.Cells[row*tg.Cols+col]
				if cell.Width == 0 {
					col++
					continue
				}
				fg, _ := termCellFB(cell)
				blank := cell.Ch == 0 || cell.Ch == ' '
				if !blank && fg != runFG {
					break
				}
				glyphRune := cell.Ch
				if blank {
					glyphRune = ' '
				}
				r.termRunes = append(r.termRunes, glyphRune)
				r.termCols = append(r.termCols, col)
				col++
			}
			r.termDrawRun(cfg, runFG, gx, baseline, cw)
		}
	}
}

// termDrawRun shapes the accumulated run and draws its glyphs at fixed
// cell columns, falling back to natural flow when the shaper does not
// produce one glyph per cell (clustered or ligated runs).
func (r *renderer) termDrawRun(cfg glyph.TextConfig, fg gui.Color,
	gx, baseline, cw float32) {

	if len(r.termCols) == 0 {
		return
	}
	cfg.Style.Color = glyph.Color{R: fg.R, G: fg.G, B: fg.B, A: fg.A}
	layout, err := r.textSys.LayoutText(string(r.termRunes), cfg)
	if err != nil {
		return
	}
	if len(layout.Glyphs) != len(r.termCols) {
		r.textSys.DrawLayout(layout,
			gx+float32(r.termCols[0])*cw, baseline)
		return
	}
	r.placements = r.placements[:0]
	for _, col := range r.termCols {
		r.placements = append(r.placements, glyph.GlyphPlacement{
			X: gx + float32(col)*cw,
			Y: baseline,
		})
	}
	r.textSys.DrawLayoutPlaced(layout, r.placements)
}

// termDecorations draws underline attributes and the bar / underline
// cursor styles on top of the glyphs.
func (r *renderer) termDecorations(tg *gui.TermGridData,
	gx, gy, cw, ch float32) {

	thick := max(ch*0.08, 1)
	for row := range tg.Rows {
		for col := range tg.Cols {
			c := &tg.Cells[row*tg.Cols+col]
			if c.Attrs&gui.TermUnderline == 0 {
				continue
			}
			fg, _ := termCellFB(c)
			r.fillTermRect(gx+float32(col)*cw,
				gy+float32(row+1)*ch-thick, cw, thick, fg)
		}
	}

	cur := tg.Cursor
	if !cur.Visible || !inGrid(tg, cur.Col, cur.Row) {
		return
	}
	x := gx + float32(cur.Col)*cw
	y := gy + float32(cur.Row)*ch
	switch cur.Style {
	case gui.TermCursorBar:
		r.fillTermRect(x, y, max(cw*0.15, 1), ch, cur.Color)
	case gui.TermCursorUnderline:
		r.fillTermRect(x, y+ch-thick, cw, thick, cur.Color)
	case gui.TermCursorBlock:
		// Drawn under the glyph in termCursorUnder.
	}
}

// inGrid reports whether a cell coordinate is on the grid.
func inGrid(tg *gui.TermGridData, col, row int) bool {
	return col >= 0 && col < tg.Cols && row >= 0 && row < tg.Rows
}

// fillTermRect fills one solid rect given in logical coordinates.
func (r *renderer) fillTermRect(x, y, w, h float32, c gui.Color) {
	if w <= 0 || h <= 0 || c.A == 0 {
		return
	}
	cmd := gui.RenderCmd{
		Kind: gui.RenderRect, X: x, Y: y, W: w, H: h,
		Color: c, Fill: true,
	}
	r.drawRect(&cmd)
}
