package datagrid

import (
	gg "github.com/go-gui-org/go-gui/gui"
)

// Pinned columns (issue #1364).
//
// dataGridEffectiveColumns orders the columns left-pinned, unpinned,
// right-pinned. With any column pinned, the scroll body keeps its rows
// in place, and each row of cells (header, filter, frozen-top, data) is
// its pinned cells around a scroll window with the others. See
// docs/specs/scroll-window.md. A grid with no pinned column builds its
// rows as before.
//
// A row that is not made of cells is as wide as the viewport and stays
// in place with the rest of the body. A group header, a status line and
// the quick filter are meant to. A detail row is as wide as the columns
// and scrolls inside a window of its own. The column chooser's list
// scrolls on its own.

const (
	// dataGridPinEdgeWidth is the width of the line between the pinned
	// and the scrolling columns in a grid without cell borders.
	dataGridPinEdgeWidth = float32(1)

	// dataGridPinMinWindow is the room the scrolling columns keep in a
	// narrow grid: about one column.
	dataGridPinMinWindow = float32(80)

	// dataGridQuickFilterMinWidth is the narrowest the quick filter's
	// input gets. The theme's floor for a field would push the clear
	// button out of a narrow grid.
	dataGridQuickFilterMinWidth = float32(40)
)

// dataGridPins is what each row needs to keep its pinned cells in place.
// dataGridBuild computes it once per frame. The zero value pins nothing.
type dataGridPins struct {
	w *gg.Window
	// edge is the line between pinned and scrolling cells, or nil. One
	// view serves every row: generation only reads it.
	edge gg.View
	// tail fills the place of the row-delete button in a header or filter
	// row, so the cells pinned before it line up. Nil when there is none.
	tail     gg.View
	scrollID string
	// extent is the width of what the grid scrolls sideways.
	extent float32
	// rowMin is the narrowest a row can be.
	rowMin float32
	// windowWidth and windowMin are the window's widest and narrowest.
	windowWidth, windowMin float32
	// border is the cell border, which neighbouring cells share.
	border float32
	// left, center and right count the columns of each part.
	left, center, right int
	on                  bool
}

// newDataGridPins derives the pins for a frame. columns is in display
// order. Pinning every column leaves nothing to scroll past the pinned
// ones, so it is the same as pinning none: the grid scrolls as a whole.
func newDataGridPins(cfg *DataGridCfg, columns []GridColumnCfg, columnWidths map[string]float32,
	deleteButton bool, scrollID string, w *gg.Window) dataGridPins {
	var p dataGridPins
	for i := range columns {
		switch columns[i].Pin {
		case GridColumnPinLeft:
			p.left++
		case GridColumnPinRight:
			p.right++
		default:
			p.center++
		}
	}
	if p.center == 0 || p.left+p.right == 0 {
		return dataGridPins{}
	}
	p.on = true
	p.w = w
	p.scrollID = scrollID
	p.border = max(cfg.SizeBorder.Or(0), 0)
	rowWidth := dataGridRowWidth(cfg, columns, columnWidths, deleteButton)

	// The row-delete button ends the row. It scrolls with the columns
	// unless some are pinned before it.
	cells := p.center
	p.windowWidth = dataGridColumnsTotalWidth(columns[p.left:p.left+p.center], columnWidths)
	if deleteButton {
		if p.right == 0 {
			p.windowWidth += dataGridRowDeleteWidth
			cells++
		} else {
			p.tail = gg.Rectangle(gg.RectangleCfg{
				Color:  gg.ColorTransparent,
				Width:  dataGridRowDeleteWidth,
				Sizing: gg.FixedFill,
			})
		}
	}
	p.windowWidth -= float32(cells-1) * p.border

	if p.border == 0 {
		// Cell borders already mark the edge of a full grid. Without
		// them a cut cell would sit right next to the pinned one.
		p.edge = gg.Rectangle(gg.RectangleCfg{
			Color:  cfg.ColorsRow.Border,
			Width:  dataGridPinEdgeWidth,
			Sizing: gg.FixedFill,
		})
		if p.left > 0 {
			rowWidth += dataGridPinEdgeWidth
		}
		if p.right > 0 {
			rowWidth += dataGridPinEdgeWidth
		}
	}
	p.windowMin = min(p.windowWidth, dataGridPinMinWindow)
	p.rowMin = rowWidth - p.windowWidth + p.windowMin
	p.extent = rowWidth
	return p
}

// detailWidth is the width of a detail row's content. Without pinned
// columns it is as wide as the columns, and the row reaches past the
// data rows by its padding. With them it ends where the data rows do: a
// longer scroll range would open a gap between the last scrolling column
// and what follows it.
func (p *dataGridPins) detailWidth(columnsWidth float32, pad gg.Padding) float32 {
	if !p.on {
		return columnsWidth
	}
	return max(p.extent-pad.Width(), 0)
}

// dataGridRowCells collects the cells of one row, in column order, and
// puts the scrolling ones in a window when columns are pinned. Both
// lists are parts of one slice.
type dataGridRowCells struct {
	pins *dataGridPins
	// row holds the row's own children: every cell when nothing is
	// pinned, else the pinned ones, the edges and the window.
	row []gg.View
	// window holds the scrolling cells.
	window []gg.View
	slot   int // index of the window in row
	next   int // index of the next column
}

func newDataGridRowCells(cfg *DataGridCfg, columns int) dataGridRowCells {
	p := &cfg.pins
	if !p.on {
		// The columns and the row-delete button.
		return dataGridRowCells{pins: p, row: make([]gg.View, 0, columns+1)}
	}
	// The pinned cells, two edges, the window and the row-delete button;
	// then the scrolling cells with room for that button.
	rowCap := p.left + p.right + 4
	buf := make([]gg.View, rowCap+p.center+1)
	return dataGridRowCells{
		pins:   p,
		row:    buf[0:0:rowCap],
		window: buf[rowCap:rowCap],
	}
}

// add appends the cell of the next column.
func (c *dataGridRowCells) add(cell gg.View) {
	p := c.pins
	i := c.next
	c.next++
	if !p.on {
		c.row = append(c.row, cell)
		return
	}
	if i == p.left {
		if p.edge != nil && p.left > 0 {
			c.row = append(c.row, p.edge)
		}
		c.slot = len(c.row)
		c.row = append(c.row, nil)
	}
	if i >= p.left && i < p.left+p.center {
		c.window = append(c.window, cell)
		return
	}
	if p.edge != nil && i == p.left+p.center {
		c.row = append(c.row, p.edge)
	}
	c.row = append(c.row, cell)
}

// addDelete appends the row-delete button after every column.
func (c *dataGridRowCells) addDelete(button gg.View) {
	if c.pins.on && c.pins.right == 0 {
		c.window = append(c.window, button)
		return
	}
	c.row = append(c.row, button)
}

// addTail holds the place of the row-delete button in a row that has
// none.
func (c *dataGridRowCells) addTail() {
	if c.pins.tail != nil {
		c.row = append(c.row, c.pins.tail)
	}
}

// views returns the row's children.
func (c *dataGridRowCells) views() []gg.View {
	p := c.pins
	if !p.on || c.next <= p.left {
		return c.row
	}
	c.row[c.slot] = p.w.ScrollWindow(gg.ScrollWindowCfg{
		ScrollID: p.scrollID,
		Content:  c.window,
		MinWidth: p.windowMin,
		MaxWidth: p.windowWidth,
		Border:   p.border,
		ClipEnd:  p.right > 0,
	})
	return c.row
}

// scrollWhole wraps the content of a row that is not made of cells and
// is width wide, so that it scrolls sideways with the columns. It
// returns content itself when nothing is pinned: then the row scrolls
// with the body.
func (p *dataGridPins) scrollWhole(content gg.View, width float32) gg.View {
	if !p.on {
		return content
	}
	return p.w.ScrollWindow(gg.ScrollWindowCfg{
		ScrollID: p.scrollID,
		MaxWidth: width,
		Content: []gg.View{
			// Content that fills would take the window's width, and
			// nothing would be left of it once it scrolled.
			gg.Column(gg.ContainerCfg{
				Width:      width,
				Sizing:     gg.FixedFill,
				Padding:    gg.NoPadding,
				Spacing:    gg.NoSpacing,
				SizeBorder: gg.NoBorder,
				Color:      gg.ColorTransparent,
				Content:    []gg.View{content},
			}),
		},
	})
}
