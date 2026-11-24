package gui

import (
	"fmt"
	"runtime"
	"testing"
	"weak"
)

// Frozen leading columns (#1363): the row header and the first columns
// stay in place while the others scroll sideways.

const freezeTestCols = 12

func freezeTable(cfg TableCfg) View {
	cfg.ID = "t"
	if cfg.Data == nil {
		cfg.Data = hscrollTableRows(freezeTestCols, 40)
	}
	if !cfg.Sizing.IsSet() {
		cfg.Sizing = FillFill
	}
	if cfg.Height == 0 {
		cfg.Height = 300
	}
	return Table(cfg)
}

// freezeWindow renders cfg in a window of the given width.
func freezeWindow(t *testing.T, width int, cfg TableCfg) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: width, Height: 400})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Padding: NoPadding,
			SizeBorder: NoBorder, Content: []View{freezeTable(cfg)}})
	})
	w.TestRender(nil)
	return w
}

// freezeParts returns the scroll body and the column header row and
// first data row, wherever the header lives.
func freezeParts(t *testing.T, w *Window, l *Layout) (body, head, row *Layout) {
	t.Helper()
	id := w.ResolveID("t")
	if len(id) != 1 {
		t.Fatalf("ResolveID(t) = %v", id)
	}
	table, ok := l.FindByID(id[0])
	if !ok {
		t.Fatal("table not found")
	}
	if body, ok = l.FindByID(ScopeID(id[0], "scroll")); ok {
		return body, &table.Children[0].Children[0], freezeRows(body)[0]
	}
	rows := freezeRows(table)
	return table, rows[0], rows[1]
}

// freezeRows returns the children of body that hold grid cells.
func freezeRows(body *Layout) []*Layout {
	var rows []*Layout
	for i := range body.Children {
		if len(freezeCells(&body.Children[i])) > 0 {
			rows = append(rows, &body.Children[i])
		}
	}
	return rows
}

// freezeCells returns a row's grid cells in order, those in its window
// included.
func freezeCells(row *Layout) []*Layout {
	var cells []*Layout
	for i := range row.Children {
		c := &row.Children[i]
		if c.Shape.A11YRole == AccessRoleGridCell {
			cells = append(cells, c)
			continue
		}
		cells = append(cells, freezeCells(c)...)
	}
	return cells
}

// freezeView is the span of x the body shows its rows in.
func freezeView(body *Layout) (left, right float32) {
	s := body.Shape
	if effectiveTextDir(s) == TextDirRTL {
		left = s.X + s.Padding.Right + s.SizeBorder
	} else {
		left = s.X + s.PaddingLeft()
	}
	return left, left + s.Width - s.paddingWidth()
}

func freezeXs(cells []*Layout) []float32 {
	xs := make([]float32, len(cells))
	for i := range cells {
		xs[i] = cells[i].Shape.X
	}
	return xs
}

func freezeScroll(t *testing.T, w *Window, x float32) *Layout {
	t.Helper()
	body, _, _ := freezeParts(t, w, &w.layout)
	w.ScrollHorizontalTo(body.Shape.idKey(), x)
	return w.TestRender(nil)
}

// freezeRTL makes the locale right-to-left for one test.
func freezeRTL(t *testing.T) {
	t.Helper()
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	l := saved
	l.TextDir = TextDirRTL
	SetLocale(l)
}

// The frozen cells do not move, the others move by the offset, the
// header stays in line with the rows, and at the end of the range the
// last column ends at the viewport's edge.
func TestTableFreezeKeepsLeadingCells(t *testing.T) {
	for _, c := range []struct {
		name   string
		cfg    TableCfg
		frozen int // leading cells, the row header included
	}{
		{"row header", TableCfg{RowHeader: true, FreezeRowHeader: true}, 1},
		{"one column", TableCfg{FreezeColumns: 1}, 1},
		{"row header and two columns", TableCfg{RowHeader: true, FreezeColumns: 2}, 3},
	} {
		for _, header := range []bool{false, true} {
			for _, border := range []TableBorderStyle{TableBorderNone, TableBorderAll, TableBorderHorizontal, TableBorderHeaderOnly} {
				for _, rtl := range []bool{false, true} {
					name := fmt.Sprintf("%s/header=%v/border=%d/rtl=%v", c.name, header, border, rtl)
					t.Run(name, func(t *testing.T) {
						if rtl {
							freezeRTL(t)
						}
						cfg := c.cfg
						cfg.FreezeHeader = header
						cfg.BorderStyle = border
						cfg.SizeBorder = 1
						w := freezeWindow(t, 600, cfg)
						checkFreezeScroll(t, w, c.frozen, rtl)
					})
				}
			}
		}
	}
}

func checkFreezeScroll(t *testing.T, w *Window, frozen int, rtl bool) {
	t.Helper()
	_, _, row := freezeParts(t, w, &w.layout)
	before := freezeXs(freezeCells(row))

	for _, ask := range []float32{-70, -100000} {
		l := freezeScroll(t, w, ask)
		body, head, row := freezeParts(t, w, l)
		off, _, err := w.TestScrollOffset(body.Shape.idKey())
		if err != nil {
			t.Fatal(err)
		}
		if off >= 0 {
			t.Fatalf("offset = %.1f after scrolling to %.0f, want < 0", off, ask)
		}
		dx := off
		if rtl {
			dx = -off
		}
		cells := freezeCells(row)
		headCells := freezeCells(head)
		if len(cells) != len(before) || len(headCells) != len(cells) {
			t.Fatalf("cells = %d, header cells = %d, want %d", len(cells), len(headCells), len(before))
		}
		for i, cell := range cells {
			want := before[i]
			if i >= frozen {
				want += dx
			}
			if f32Abs(cell.Shape.X-want) > 0.5 {
				t.Fatalf("offset %.0f: cell %d x = %.1f, want %.1f", off, i, cell.Shape.X, want)
			}
			if h := headCells[i].Shape; f32Abs(h.X-cell.Shape.X) > 0.5 ||
				f32Abs(h.Width-cell.Shape.Width) > 0.5 {
				t.Fatalf("offset %.0f: header cell %d x=%.1f w=%.1f, body x=%.1f w=%.1f",
					off, i, h.X, h.Width, cell.Shape.X, cell.Shape.Width)
			}
		}
		if ask > -1000 {
			continue
		}
		left, right := freezeView(body)
		last := cells[len(cells)-1].Shape
		got, want := last.X+last.Width, right
		if rtl {
			got, want = last.X, left
		}
		if f32Abs(got-want) > 0.5 {
			t.Fatalf("scrolled to the end: last column's edge = %.1f, want the viewport's %.1f", got, want)
		}
		// The row's fill covers the whole viewport.
		if s := row.Shape; s.X > left+0.5 || s.X+s.Width < right-0.5 {
			t.Fatalf("row spans %.1f..%.1f, want the viewport %.1f..%.1f",
				s.X, s.X+s.Width, left, right)
		}
	}
	if dup := w.TestDuplicateIDs(); len(dup) != 0 {
		t.Fatalf("duplicate IDs: %v", dup)
	}
	if found := w.TestFindings(DebugAll); len(found) != 0 {
		t.Fatalf("findings: %v", found)
	}
}

// Unscrolled, freezing moves no cell: only the edge line, where there is
// one, pushes the scrolling columns along by its width.
func TestTableFreezeUnscrolledGeometry(t *testing.T) {
	for _, c := range []struct {
		name   string
		border TableBorderStyle
		edge   float32
		cfg    TableCfg
		frozen int // leading cells, the row header included
	}{
		{"full grid", TableBorderAll, 0, TableCfg{RowHeader: true, FreezeColumns: 2}, 3},
		{"no borders", TableBorderNone, tableFreezeEdgeWidth,
			TableCfg{RowHeader: true, FreezeColumns: 2}, 3},
		{"full grid, row header only", TableBorderAll, 0,
			TableCfg{RowHeader: true, FreezeRowHeader: true}, 1},
		{"no borders, row header only", TableBorderNone, tableFreezeEdgeWidth,
			TableCfg{RowHeader: true, FreezeRowHeader: true}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			xs := func(cfg TableCfg) []float32 {
				cfg.BorderStyle, cfg.SizeBorder = c.border, 1
				w := freezeWindow(t, 600, cfg)
				_, _, row := freezeParts(t, w, &w.layout)
				return freezeXs(freezeCells(row))
			}
			plain, frozen := xs(TableCfg{RowHeader: true}), xs(c.cfg)
			for i := range plain {
				want := plain[i]
				if i >= c.frozen {
					want += c.edge
				}
				if f32Abs(frozen[i]-want) > 0.01 {
					t.Fatalf("cell %d x = %.2f, want %.2f", i, frozen[i], want)
				}
			}
		})
	}
}

// In a table wider than its columns a row ends at the last column, as it
// does with nothing frozen, so its fill does not reach past it.
func TestTableFreezeRowEndsAtLastColumn(t *testing.T) {
	for _, n := range []int{0, 2} {
		w := freezeWindow(t, 2000, TableCfg{
			FreezeColumns: n, BorderStyle: TableBorderAll, SizeBorder: 1,
		})
		_, _, row := freezeParts(t, w, &w.layout)
		cells := freezeCells(row)
		last := cells[len(cells)-1].Shape
		if got, want := row.Shape.X+row.Shape.Width, last.X+last.Width; f32Abs(got-want) > 0.5 {
			t.Fatalf("freeze %d: row ends at %.1f, its last column at %.1f", n, got, want)
		}
	}
}

// A table too narrow for its frozen columns scrolls them away first, so
// the last column can still be brought into view, whole.
func TestTableFreezeNarrowReachesEveryColumn(t *testing.T) {
	for _, rtl := range []bool{false, true} {
		for _, header := range []bool{false, true} {
			t.Run(fmt.Sprintf("rtl=%v/header=%v", rtl, header), func(t *testing.T) {
				if rtl {
					freezeRTL(t)
				}
				cfg := TableCfg{RowHeader: true, FreezeColumns: 3, FreezeHeader: header}
				w := freezeWindow(t, 160, cfg)
				for i, ok := range freezeReachable(t, w) {
					if !ok {
						t.Errorf("column %d never shows whole", i)
					}
				}
				// The header stays in line while the lead moves the rows.
				for _, ask := range []float32{-20, -60, -100000} {
					l := freezeScroll(t, w, ask)
					_, head, row := freezeParts(t, w, l)
					hc, bc := freezeXs(freezeCells(head)), freezeXs(freezeCells(row))
					for i := range bc {
						if f32Abs(hc[i]-bc[i]) > 0.5 {
							t.Fatalf("offset %.0f: header cell %d x = %.1f, body %.1f", ask, i, hc[i], bc[i])
						}
					}
				}
			})
		}
	}
}

// freezeReachable scrolls through the whole range and reports for each
// cell of a row whether its start and its end both show at some offset.
func freezeReachable(t *testing.T, w *Window) []bool {
	t.Helper()
	var start, end []bool
	for ask := float32(0); ask > -1000; ask -= 10 {
		l := freezeScroll(t, w, ask)
		_, _, row := freezeParts(t, w, l)
		cells := freezeCells(row)
		if start == nil {
			start, end = make([]bool, len(cells)), make([]bool, len(cells))
		}
		for i, c := range cells {
			s, clip := c.Shape, c.Shape.shapeClip
			if clip.Width < 1 {
				continue
			}
			start[i] = start[i] || f32Abs(clip.X-s.X) < 0.5
			end[i] = end[i] || f32Abs(clip.X+clip.Width-s.X-s.Width) < 0.5
		}
	}
	for i := range start {
		start[i] = start[i] && end[i]
	}
	return start
}

// The header reads the offset after this frame's clamp, so it is in line
// on the first frame after the table gets wider at the end of its range.
func TestTableFreezeInLineOnResize(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 1200, Height: 400})
	width := float32(600)
	w.TestRender(func(*Window) View {
		return Row(ContainerCfg{Sizing: FitFill, Content: []View{
			Column(ContainerCfg{Sizing: FixedFill, Width: width, Content: []View{
				freezeTable(TableCfg{FreezeColumns: 2, FreezeHeader: true}),
			}}),
		}})
	})
	w.TestRender(nil)
	freezeScroll(t, w, -100000)
	width = 700
	l := w.TestRender(nil)
	body, head, row := freezeParts(t, w, l)
	hc, bc := freezeXs(freezeCells(head)), freezeXs(freezeCells(row))
	for i := range bc {
		if f32Abs(hc[i]-bc[i]) > 0.5 {
			t.Fatalf("header cell %d x = %.1f, body %.1f", i, hc[i], bc[i])
		}
	}
	_, right := freezeView(body)
	last := freezeCells(row)[len(bc)-1].Shape
	if f32Abs(last.X+last.Width-right) > 0.5 {
		t.Fatalf("last column ends at %.1f, want %.1f", last.X+last.Width, right)
	}
}

// Counts that freeze nothing leave the table as it was: every cell
// scrolls, and the row holds its cells directly.
func TestTableFreezeNothing(t *testing.T) {
	for _, c := range []struct {
		name string
		cfg  TableCfg
	}{
		{"negative", TableCfg{FreezeColumns: -3}},
		{"every column", TableCfg{FreezeColumns: freezeTestCols}},
		{"more than every column", TableCfg{FreezeColumns: freezeTestCols + 5}},
		{"row header frozen without one", TableCfg{FreezeRowHeader: true}},
		{"row header not frozen", TableCfg{RowHeader: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := freezeWindow(t, 600, c.cfg)
			_, _, row := freezeParts(t, w, &w.layout)
			before := freezeXs(freezeCells(row))
			if len(hscrollCells(row)) != len(before) {
				t.Fatal("the row is split though nothing is frozen")
			}
			l := freezeScroll(t, w, -70)
			_, _, row = freezeParts(t, w, l)
			for i, x := range freezeXs(freezeCells(row)) {
				if f32Abs(x-before[i]+70) > 0.5 {
					t.Fatalf("cell %d moved by %.1f, want -70", i, x-before[i])
				}
			}
		})
	}
}

// Rows shorter than the others, down to no cells at all, do not disturb
// the rest.
func TestTableFreezeRaggedRows(t *testing.T) {
	data := hscrollTableRows(freezeTestCols, 6)
	data[2].Cells = data[2].Cells[:5]
	data[3].Cells = data[3].Cells[:2] // exactly the frozen ones
	data[4].Cells = data[4].Cells[:1]
	data[5].Cells = nil
	w := freezeWindow(t, 600, TableCfg{Data: data, FreezeColumns: 2, RowHeader: true})
	l := freezeScroll(t, w, -50)
	body, _, _ := freezeParts(t, w, l)
	rows := freezeRows(body)
	if len(rows) != len(data) {
		t.Fatalf("rows = %d, want %d", len(rows), len(data))
	}
	full := freezeXs(freezeCells(rows[1]))
	for r, row := range rows {
		cells := freezeCells(row)
		if want := len(data[r].Cells) + 1; len(cells) != want {
			t.Fatalf("row %d: cells = %d, want %d", r, len(cells), want)
		}
		for i, x := range freezeXs(cells) {
			if f32Abs(x-full[i]) > 0.5 {
				t.Fatalf("row %d cell %d x = %.1f, want %.1f", r, i, x, full[i])
			}
		}
	}
}

// A row is as high as its highest cell on either side of the edge, and
// every cell fills it, so the two sides cannot drift apart.
func TestTableFreezeRowHeights(t *testing.T) {
	for _, tall := range []int{0, 5} {
		t.Run(fmt.Sprintf("tall cell %d", tall), func(t *testing.T) {
			data := hscrollTableRows(freezeTestCols, 3)
			data[1].Cells[tall] = TableCellCfg{Content: Rectangle(RectangleCfg{
				Width: 20, Height: 90, Sizing: FixedFixed,
			})}
			w := freezeWindow(t, 600, TableCfg{Data: data, FreezeColumns: 2})
			_, _, row := freezeParts(t, w, &w.layout)
			if row.Shape.Height < 90 {
				t.Fatalf("row height = %.1f, want >= 90", row.Shape.Height)
			}
			for i, c := range freezeCells(row) {
				if f32Abs(c.Shape.Height-row.Shape.Height) > 0.5 || f32Abs(c.Shape.Y-row.Shape.Y) > 0.5 {
					t.Fatalf("cell %d y=%.1f h=%.1f, row y=%.1f h=%.1f", i,
						c.Shape.Y, c.Shape.Height, row.Shape.Y, row.Shape.Height)
				}
			}
		})
	}
}

// A Fit table still grows to its columns and has nothing to scroll.
func TestTableFreezeFitWidth(t *testing.T) {
	width := func(n int) float32 {
		w := freezeWindow(t, 4000, TableCfg{
			FreezeColumns: n, Sizing: FitFill, BorderStyle: TableBorderAll, SizeBorder: 1,
		})
		body, _, _ := freezeParts(t, w, &w.layout)
		if scrollMaxOffsetX(body) < 0 {
			t.Fatalf("freeze %d: max offset = %.1f, want 0", n, scrollMaxOffsetX(body))
		}
		return body.Shape.Width
	}
	if plain, frozen := width(0), width(2); f32Abs(plain-frozen) > 0.5 {
		t.Fatalf("table width = %.1f with frozen columns, want %.1f", frozen, plain)
	}
}

// A click reaches the cell that is drawn there: a scrolled cell that is
// partly cut off at the edge takes clicks only where it shows.
func TestTableFreezeClicks(t *testing.T) {
	clicked := -1
	data := hscrollTableRows(freezeTestCols, 6)
	for c := range data[1].Cells {
		data[1].Cells[c].OnClick = func(ctx EventCtx) { clicked = c; ctx.Consume() }
	}
	w := freezeWindow(t, 600, TableCfg{Data: data, FreezeColumns: 2})
	l := freezeScroll(t, w, -20)
	_, _, row := freezeParts(t, w, l)
	cells := freezeCells(row)
	y := row.Shape.Y + row.Shape.Height/2
	edge := cells[1].Shape.X + cells[1].Shape.Width
	if cells[2].Shape.X >= edge {
		t.Fatalf("cell 2 x = %.1f is not under the frozen edge %.1f", cells[2].Shape.X, edge)
	}
	press := func(x float32) int {
		clicked = -1
		for _, typ := range []EventType{EventMouseDown, EventMouseUp} {
			w.EventFn(&Event{Type: typ, MouseX: x, MouseY: y, MouseButton: MouseLeft})
		}
		return clicked
	}
	if got := press(edge - 3); got != 1 {
		t.Errorf("click left of the edge reached column %d, want 1", got)
	}
	if got := press(edge + 5); got != 2 {
		t.Errorf("click right of the edge reached column %d, want 2", got)
	}
}

// Selection works in the frozen part as it does in the rest: a row
// header click selects, Shift extends, and a click on a scrolled cell
// selects the same row.
func TestTableFreezeSelection(t *testing.T) {
	selected := map[int]bool{}
	w := NewTestWindow(t, WindowCfg{Width: 600, Height: 400})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Padding: NoPadding, SizeBorder: NoBorder,
			Content: []View{freezeTable(TableCfg{
				RowHeader: true, FreezeColumns: 1, FreezeHeader: true, MultiSelect: true,
				Selected: selected,
				OnSelect: func(s map[int]bool, _ int, _ EventCtx) { selected = s },
			})}})
	})
	w.TestRender(nil)
	l := freezeScroll(t, w, -100)
	click := func(row, cell int, mods Modifier) {
		body, _, _ := freezeParts(t, w, l)
		c := freezeCells(freezeRows(body)[row])[cell].Shape
		for _, typ := range []EventType{EventMouseDown, EventMouseUp} {
			w.EventFn(&Event{Type: typ, Modifiers: mods, MouseButton: MouseLeft,
				MouseX: c.X + c.Width/2, MouseY: c.Y + c.Height/2})
		}
		l = w.TestRender(nil)
	}
	click(0, 0, ModNone)
	click(2, 0, ModShift)
	if len(selected) != 3 || !selected[1] || !selected[2] || !selected[3] {
		t.Fatalf("selected = %v after Shift on the row header, want rows 1..3", selected)
	}
	click(3, 4, ModNone)
	if !selected[4] {
		t.Fatalf("selected = %v after a click on a scrolled cell, want row 4 in it", selected)
	}
	click(4, 1, ModNone)
	if !selected[5] {
		t.Fatalf("selected = %v after a click on a frozen cell, want row 5 in it", selected)
	}
}

// Hovering one side tints the whole row, which is still one shape.
func TestTableFreezeHoverTintsWholeRow(t *testing.T) {
	w := freezeWindow(t, 600, TableCfg{
		FreezeColumns: 1, OnSelect: func(map[int]bool, int, EventCtx) {},
	})
	_, _, row := freezeParts(t, w, &w.layout)
	rest := row.Shape.Color
	for _, cell := range []int{0, 2} {
		c := freezeCells(row)[cell].Shape
		w.EventFn(&Event{Type: EventMouseMove, MouseX: c.X + c.Width/2, MouseY: c.Y + c.Height/2})
		l := w.TestRender(nil)
		_, _, row = freezeParts(t, w, l)
		if row.Shape.Color == rest {
			t.Fatalf("hover over cell %d did not tint the row", cell)
		}
		w.EventFn(&Event{Type: EventMouseMove, MouseX: 599, MouseY: 399})
		l = w.TestRender(nil)
		_, _, row = freezeParts(t, w, l)
	}
}

// A wheel scrolls the body by the same amount wherever it is turned.
func TestTableFreezeWheel(t *testing.T) {
	wheel := func(at func(body, head, row *Layout) *Shape) (x, y float32) {
		w := freezeWindow(t, 600, TableCfg{FreezeColumns: 2, FreezeHeader: true})
		body, head, row := freezeParts(t, w, &w.layout)
		s := at(body, head, row)
		w.EventFn(&Event{
			Type: EventMouseScroll, ScrollPrecise: true, ScrollX: -2, ScrollY: -1,
			MouseX: s.X + s.Width/2, MouseY: s.Y + s.Height/2,
		})
		w.TestRender(nil)
		x, y, err := w.TestScrollOffset(body.Shape.idKey())
		if err != nil {
			t.Fatal(err)
		}
		return x, y
	}
	wx, wy := wheel(func(_, _, row *Layout) *Shape { return freezeCells(row)[3].Shape })
	if wx >= 0 || wy >= 0 {
		t.Fatalf("wheel over a scrolling cell: offset = (%.1f, %.1f), want both < 0", wx, wy)
	}
	for name, at := range map[string]func(body, head, row *Layout) *Shape{
		"frozen cell":           func(_, _, row *Layout) *Shape { return freezeCells(row)[0].Shape },
		"frozen header cell":    func(_, head, _ *Layout) *Shape { return freezeCells(head)[0].Shape },
		"scrolling header cell": func(_, head, _ *Layout) *Shape { return freezeCells(head)[3].Shape },
	} {
		if x, y := wheel(at); x != wx || y != wy {
			t.Errorf("wheel over a %s: offset = (%.1f, %.1f), want (%.1f, %.1f)", name, x, y, wx, wy)
		}
	}
}

// Virtualization builds the same rows with and without frozen columns.
func TestTableFreezeVirtualization(t *testing.T) {
	rows := func(n int) int {
		w := freezeWindow(t, 600, TableCfg{
			Data: hscrollTableRows(freezeTestCols, 400), FreezeColumns: n, FreezeHeader: true,
		})
		body, _, _ := freezeParts(t, w, &w.layout)
		return len(freezeRows(body))
	}
	if plain, frozen := rows(0), rows(2); plain != frozen || plain >= 400 {
		t.Fatalf("rows = %d plain, %d with frozen columns", plain, frozen)
	}
}

// A frozen table inside a cell of a frozen table follows its own body.
func TestTableFreezeNested(t *testing.T) {
	inner := hscrollTableRows(6, 3)
	data := hscrollTableRows(freezeTestCols, 3)
	data[1].Cells[0] = TableCellCfg{Content: Table(TableCfg{
		ID: "inner", Data: inner, FreezeColumns: 1, Sizing: FixedFixed, Width: 200, Height: 80,
	})}
	w := freezeWindow(t, 600, TableCfg{Data: data, FreezeColumns: 1})
	ids := w.ResolveID("inner")
	if len(ids) != 1 {
		t.Fatalf("ResolveID(inner) = %v", ids)
	}
	in0, _ := w.layout.FindByID(ids[0])
	before := freezeXs(freezeCells(freezeRows(in0)[1]))
	w.ScrollHorizontalTo(ids[0], -30)
	l := w.TestRender(nil)
	outerBody, _, _ := freezeParts(t, w, l)
	if off, _, err := w.TestScrollOffset(outerBody.Shape.idKey()); err != nil || off != 0 {
		t.Fatalf("outer offset = %.1f (err %v), want 0", off, err)
	}
	if off, _, err := w.TestScrollOffset(ids[0]); err != nil || off != -30 {
		t.Fatalf("inner offset = %.1f (err %v), want -30", off, err)
	}
	in, _ := l.FindByID(ids[0])
	cells := freezeXs(freezeCells(freezeRows(in)[1]))
	if cells[0] != before[0] || cells[1] != before[1]-30 {
		t.Fatalf("inner cells moved from %v to %v, want the first in place and the second 30 left",
			before[:2], cells[:2])
	}
}

// Freezing costs a few heap allocations per frame and none per row: the
// windows come from the frame's pool and share the row's slice. So it
// costs the same few for 40 rows as for 80.
func TestTableFreezeAllocs(t *testing.T) {
	frame := func(n, rows int) float64 {
		w := freezeWindow(t, 600, TableCfg{
			Data: hscrollTableRows(freezeTestCols, rows), FreezeColumns: n, FreezeHeader: true,
			Height: 4000, // every row
		})
		best := float64(1 << 30)
		for range 3 {
			best = min(best, testing.AllocsPerRun(20, func() {
				w.InvalidateLayout()
				w.TestRender(nil)
			}))
		}
		return best
	}
	extra := func(rows int) float64 {
		plain, frozen := frame(0, rows), frame(2, rows)
		t.Logf("%d rows: %.0f allocs plain, %.0f frozen", rows, plain, frozen)
		return frozen - plain
	}
	few, many := extra(40), extra(80)
	if few > 4 || many > 4 {
		t.Errorf("frozen columns cost %.0f allocs per frame with 40 rows and %.0f with 80, want at most 4", few, many)
	}
}

// A table that is gone leaves none of its cell views behind: the pools
// its rows came from drop them at the next frame. The wide table grows
// the view arena past its cap, so the arena is replaced and only the
// windows could hold on to the old one.
func TestTableFreezeReleasesViews(t *testing.T) {
	for _, n := range []int{0, 1} {
		for _, cols := range []int{6, viewArenaRetainMax} {
			t.Run(fmt.Sprintf("freeze=%d/cols=%d", n, cols), func(t *testing.T) {
				w := NewTestWindow(t, WindowCfg{Width: 300, Height: 300})
				show := true
				var ref weak.Pointer[[1 << 16]byte]
				w.TestRender(func(*Window) View {
					if !show {
						return Column(ContainerCfg{Sizing: FillFill})
					}
					payload := new([1 << 16]byte)
					ref = weak.Make(payload)
					data := hscrollTableRows(cols, 1)
					data[1].Cells[2] = TableCellCfg{Content: ViewFunc(func(*Window) View {
						payload[0]++
						return Text(TextCfg{Text: "v"})
					})}
					return freezeTable(TableCfg{Data: data, FreezeColumns: n})
				})
				w.TestRender(nil)
				show = false
				w.TestRender(nil)
				w.TestRender(nil)
				runtime.GC()
				runtime.GC()
				if ref.Value() != nil {
					t.Error("a cell's view is still reachable after its table is gone")
				}
				runtime.KeepAlive(w)
			})
		}
	}
}
