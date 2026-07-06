package gui

import "math"

// renderTermGrid emits a single RenderTermGrid command for a terminal
// character grid. The whole grid is drawn by the backend from the
// shared TermGridData buffer in one pass — no per-cell Layout nodes
// and no per-cell RenderText commands.
func renderTermGrid(shape *Shape, clip drawClip, w *Window) {
	tg := shape.tg
	if tg == nil || tg.Cols <= 0 || tg.Rows <= 0 ||
		tg.CellW <= 0 || math.IsNaN(float64(tg.CellW)) || math.IsInf(float64(tg.CellW), 0) ||
		tg.CellH <= 0 || math.IsNaN(float64(tg.CellH)) || math.IsInf(float64(tg.CellH), 0) {
		return
	}
	if !rectsOverlap(shapeBounds(shape), clip) {
		return
	}
	emitRenderer(RenderCmd{
		Kind:     RenderTermGrid,
		X:        shape.X,
		Y:        shape.Y,
		W:        shape.Width,
		H:        shape.Height,
		TermGrid: tg,
	}, w)
}
