// Termgrid demonstrates the TermGrid primitive: a fixed-pitch
// character grid drawn straight from a cell buffer in one render
// command (no per-cell Layout nodes). It shows per-cell foreground /
// background colors, the reverse and underline attributes, a block
// cursor, and a selection range.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	cols  = 48
	rows  = 14
	cellW = 9
	cellH = 20
)

type App struct {
	cells []gui.TermCell
}

func main() {
	gui.SetTheme(gui.ThemeDark)

	app := &App{cells: buildCells()}

	w := gui.NewWindow(gui.WindowCfg{
		State:  app,
		Title:  "termgrid",
		Width:  cols*cellW + 40,
		Height: rows*cellH + 40,
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func view(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Color:  gui.RGB(16, 16, 20),
		Content: []gui.View{
			gui.TermGrid(gui.TermGridCfg{
				Cols:      cols,
				Rows:      rows,
				CellW:     cellW,
				CellH:     cellH,
				Cells:     app.cells,
				TextStyle: monoStyle(),
				Cursor: gui.TermCursor{
					Col: 10, Row: 6, Visible: true,
					Style: gui.TermCursorBlock,
					Color: gui.RGB(120, 220, 120),
				},
				Selection: gui.TermSelRange{
					Start: 8*cols + 2,
					End:   8*cols + 22,
				},
			}),
		},
	})
}

func monoStyle() gui.TextStyle {
	s := gui.CurrentTheme().M3
	s.Size = cellH * 0.75
	return s
}

// buildCells lays out demo content over a blank grid.
func buildCells() []gui.TermCell {
	cells := make([]gui.TermCell, cols*rows)
	bg := gui.RGB(16, 16, 20)
	fg := gui.RGB(220, 220, 220)
	for i := range cells {
		cells[i] = gui.TermCell{Ch: ' ', FG: fg, BG: bg, Width: 1}
	}

	put := func(row, col int, s string, cf, cb gui.Color, attr gui.TermAttr) {
		for i, ch := range s {
			c := col + i
			if row < 0 || row >= rows || c < 0 || c >= cols {
				continue
			}
			cells[row*cols+c] = gui.TermCell{
				Ch: ch, FG: cf, BG: cb, Attrs: attr, Width: 1,
			}
		}
	}

	cyan := gui.RGB(80, 220, 220)
	green := gui.RGB(120, 220, 120)
	yellow := gui.RGB(230, 210, 90)
	red := gui.RGB(230, 110, 110)
	blue := gui.RGB(110, 160, 240)

	put(0, 1, "go-gui TermGrid — single render command", cyan, bg, 0)
	put(2, 1, "$ ls -la", green, bg, 0)
	put(3, 1, "drwxr-xr-x  6 mike  staff   192 main.go", fg, bg, 0)
	put(4, 1, "-rw-r--r--  1 mike  staff  1024 README.md", fg, bg, 0)
	put(6, 1, "cursor -> ", yellow, bg, 0)
	put(8, 1, "selection spans these cells on this row", fg, bg, 0)
	put(10, 1, "reverse video", gui.RGB(16, 16, 20), yellow, gui.TermReverse)
	put(10, 18, "underlined text", blue, bg, gui.TermUnderline)
	put(12, 1, "colors:", fg, bg, 0)
	put(12, 9, "red", red, bg, 0)
	put(12, 13, "green", green, bg, 0)
	put(12, 19, "blue", blue, bg, 0)
	put(12, 24, "on-bg", gui.RGB(20, 20, 24), red, 0)

	return cells
}
