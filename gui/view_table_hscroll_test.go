package gui

import (
	"fmt"
	"testing"
)

// Frozen-header tables wider than their parent (#1293): the table fits
// the parent, the body scrolls sideways, and the header follows the
// body's X offset in the same frame.

func hscrollTableRows(cols, rows int) []TableRowCfg {
	data := make([]TableRowCfg, 0, rows+1)
	head := make([]TableCellCfg, 0, cols)
	for c := range cols {
		head = append(head, TH(fmt.Sprintf("Column header %02d", c)))
	}
	data = append(data, TR(head))
	for r := range rows {
		cells := make([]TableCellCfg, 0, cols)
		for c := range cols {
			cells = append(cells, TD(fmt.Sprintf("row %03d value %02d", r, c)))
		}
		data = append(data, TR(cells))
	}
	return data
}

// hscrollParts returns the table, its header row and the body's first
// data row from an arranged frame.
func hscrollParts(t *testing.T, w *Window, l *Layout) (table, headRow, body, bodyRow *Layout) {
	t.Helper()
	id := w.ResolveID("t")
	if len(id) != 1 {
		t.Fatalf("ResolveID(t) = %v", id)
	}
	table, ok := l.FindByID(id[0])
	if !ok {
		t.Fatal("table not found")
	}
	body, ok = l.FindByID(ScopeID(id[0], "scroll"))
	if !ok {
		t.Fatal("body not found")
	}
	if len(table.Children) == 0 || len(table.Children[0].Children) == 0 ||
		len(body.Children) == 0 {
		t.Fatal("table has no header row or no body row")
	}
	return table, &table.Children[0].Children[0], body, &body.Children[0]
}

// hscrollCells returns the grid cells of a row, in order.
func hscrollCells(row *Layout) []*Layout {
	var cells []*Layout
	for i := range row.Children {
		if row.Children[i].Shape.A11YRole == AccessRoleGridCell {
			cells = append(cells, &row.Children[i])
		}
	}
	return cells
}

// assertColumnsInLine fails when any header cell's left or right edge
// is off the matching body cell's.
func assertColumnsInLine(t *testing.T, headRow, bodyRow *Layout, when string) {
	t.Helper()
	hc, bc := hscrollCells(headRow), hscrollCells(bodyRow)
	if len(hc) != len(bc) || len(hc) == 0 {
		t.Fatalf("%s: header cells = %d, body cells = %d", when, len(hc), len(bc))
	}
	for i := range hc {
		h, b := hc[i].Shape, bc[i].Shape
		if f32Abs(h.X-b.X) > 0.5 || f32Abs(h.Width-b.Width) > 0.5 {
			t.Fatalf("%s: column %d header x=%.1f w=%.1f, body x=%.1f w=%.1f",
				when, i, h.X, h.Width, b.X, b.Width)
		}
	}
}

func hscrollTable(rh bool) View {
	return Table(TableCfg{
		ID: "t", Data: hscrollTableRows(12, 100), Sizing: FillFill,
		FreezeHeader: true, Height: 300, RowHeader: rh,
	})
}

func TestTableFreezeWideFitsParent(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 600, Height: 400})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{hscrollTable(false)}})
	})
	l := w.TestRender(nil)
	table, headRow, body, bodyRow := hscrollParts(t, w, l)

	if right := table.Shape.X + table.Shape.Width; right > 600 {
		t.Fatalf("table right edge = %.0f, want <= 600 (window width)", right)
	}
	if right := body.Shape.X + body.Shape.Width; right > 600 {
		t.Fatalf("body right edge (vertical scrollbar) = %.0f, want <= 600", right)
	}
	assertColumnsInLine(t, headRow, bodyRow, "not scrolled")

	w.ScrollHorizontalTo(body.Shape.idKey(), -100000)
	l = w.TestRender(nil)
	_, headRow, body, bodyRow = hscrollParts(t, w, l)
	x, _, err := w.TestScrollOffset(body.Shape.idKey())
	if err != nil {
		t.Fatal(err)
	}
	if x >= 0 {
		t.Fatalf("body X offset = %.0f after scrolling to the end, want < 0", x)
	}
	assertColumnsInLine(t, headRow, bodyRow, "scrolled to the end")
	if right := body.Shape.X + body.Shape.Width; right > 600 {
		t.Fatalf("scrolled: body right edge = %.0f, want <= 600", right)
	}
}

// The clamp in layoutAdjustScrollOffsets moves the offset when the
// table gets wider while scrolled to the end. The header reads the
// offset after that clamp, so it is in line on the very first frame.
func TestTableFreezeHeaderInLineOnResize(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 1200, Height: 400})
	width := float32(600)
	w.TestRender(func(*Window) View {
		return Row(ContainerCfg{Sizing: FitFill, Content: []View{
			Column(ContainerCfg{Sizing: FixedFill, Width: width,
				Content: []View{hscrollTable(false)}}),
		}})
	})
	l := w.TestRender(nil)
	_, _, body, _ := hscrollParts(t, w, l)
	w.ScrollHorizontalTo(body.Shape.idKey(), -100000)
	w.TestRender(nil)
	before, _, _ := w.TestScrollOffset(body.Shape.idKey())

	width = 700
	l = w.TestRender(nil)
	_, headRow, body, bodyRow := hscrollParts(t, w, l)
	after, _, _ := w.TestScrollOffset(body.Shape.idKey())
	if after == before {
		t.Fatalf("offset stayed %.0f after widening; the case needs a clamp", after)
	}
	assertColumnsInLine(t, headRow, bodyRow, "first frame after widening")
}

// A wheel over the header scrolls the body, sideways and down, exactly
// as far as the same wheel over the body does.
func TestTableFreezeHeaderWheelScrollsBody(t *testing.T) {
	wheel := func(overHeader bool) (x, y float32) {
		w := NewTestWindow(t, WindowCfg{Width: 600, Height: 400})
		w.TestRender(func(*Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Content: []View{hscrollTable(false)}})
		})
		l := w.TestRender(nil)
		table, _, body, _ := hscrollParts(t, w, l)
		at := body.Shape
		if overHeader {
			at = table.Children[0].Shape
		}
		e := Event{
			Type: EventMouseScroll, ScrollPrecise: true,
			MouseX: at.X + at.Width/2, MouseY: at.Y + at.Height/2,
			ScrollX: -2, ScrollY: -1,
		}
		w.EventFn(&e)
		if !e.IsHandled {
			t.Fatalf("wheel (over header %v) was not handled", overHeader)
		}
		l = w.TestRender(nil)
		x, y, _ = w.TestScrollOffset(body.Shape.idKey())
		_, headRow, _, bodyRow := hscrollParts(t, w, l)
		assertColumnsInLine(t, headRow, bodyRow, "after the wheel")
		return x, y
	}
	hx, hy := wheel(true)
	bx, by := wheel(false)
	if hx >= 0 || hy >= 0 {
		t.Fatalf("wheel over the header: body offset = (%.0f, %.0f), want both < 0", hx, hy)
	}
	if hx != bx || hy != by {
		t.Fatalf("wheel over the header moved the body to (%.0f, %.0f), over the body to (%.0f, %.0f)",
			hx, hy, bx, by)
	}
}

// Under an RTL locale the body moves its scrollbar gutter to the left;
// the header zone mirrors the same gutter, so the columns stay in line.
// The unscrolled table shows its first column at the right edge, and
// scrolling to the end brings the last one to the left edge (#1345).
func TestTableFreezeWideRTLInLine(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	l := saved
	l.TextDir = TextDirRTL
	SetLocale(l)

	w := NewTestWindow(t, WindowCfg{Width: 600, Height: 400})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{hscrollTable(false)}})
	})
	ly := w.TestRender(nil)
	_, headRow, body, bodyRow := hscrollParts(t, w, ly)
	assertColumnsInLine(t, headRow, bodyRow, "RTL not scrolled")
	cells := hscrollCells(bodyRow)
	if len(cells) == 0 {
		t.Fatal("body row has no cells")
	}
	left, right := contentBoxX(body.Shape)
	if first := cells[0].Shape; first.X+first.Width < left || first.X+first.Width > right {
		t.Fatalf("RTL not scrolled: first column ends at x=%.0f, outside the viewport %.0f..%.0f",
			first.X+first.Width, left, right)
	}

	w.ScrollHorizontalTo(body.Shape.idKey(), -100000)
	ly = w.TestRender(nil)
	_, headRow, body, bodyRow = hscrollParts(t, w, ly)
	assertColumnsInLine(t, headRow, bodyRow, "RTL scrolled to the end")
	cells = hscrollCells(bodyRow)
	if len(cells) == 0 {
		t.Fatal("body row has no cells after scrolling to the end")
	}
	left, right = contentBoxX(body.Shape)
	if last := cells[len(cells)-1].Shape; last.X < left || last.X > right {
		t.Fatalf("RTL scrolled to the end: last column starts at x=%.0f, outside the viewport %.0f..%.0f",
			last.X, left, right)
	}
}

// A table whose columns fit keeps its width and has nothing to scroll.
func TestTableFreezeNarrowUnchanged(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 600, Height: 400})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FitFit, Content: []View{
			Table(TableCfg{ID: "t", Data: hscrollTableRows(2, 5), FreezeHeader: true}),
		}})
	})
	l := w.TestRender(nil)
	_, headRow, body, bodyRow := hscrollParts(t, w, l)
	assertColumnsInLine(t, headRow, bodyRow, "narrow")
	w.ScrollHorizontalTo(body.Shape.idKey(), -100000)
	w.TestRender(nil)
	if x, _, _ := w.TestScrollOffset(body.Shape.idKey()); x != 0 {
		t.Fatalf("narrow table X offset = %.0f, want 0", x)
	}
}

// A table without FreezeHeader scrolls as one container: it fits the
// parent too, and scrolls both sideways and down.
func TestTablePlainWideScrollsBothWays(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 600, Height: 400})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Table(TableCfg{ID: "t", Data: hscrollTableRows(12, 100),
				Sizing: FillFill, Height: 300}),
		}})
	})
	l := w.TestRender(nil)
	id := w.ResolveID("t")[0]
	table, ok := l.FindByID(id)
	if !ok {
		t.Fatal("table not found")
	}
	if right := table.Shape.X + table.Shape.Width; right > 600 {
		t.Fatalf("table right edge = %.0f, want <= 600", right)
	}
	w.ScrollHorizontalTo(id, -100000)
	w.ScrollVerticalTo(id, -100000)
	w.TestRender(nil)
	x, y, err := w.TestScrollOffset(id)
	if err != nil {
		t.Fatal(err)
	}
	if x >= 0 || y >= 0 {
		t.Fatalf("offset = (%.0f, %.0f) after scrolling to the end, want both < 0", x, y)
	}
}

// The lines between rows (Horizontal and HeaderOnly borders) reach the
// last column when the table is scrolled sideways, in the header zone
// and in the body.
func TestTableSeparatorsSpanScrolledColumns(t *testing.T) {
	for _, style := range []TableBorderStyle{TableBorderHorizontal, TableBorderHeaderOnly} {
		w := NewTestWindow(t, WindowCfg{Width: 600, Height: 400})
		w.TestRender(func(*Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Content: []View{
				Table(TableCfg{ID: "t", Data: hscrollTableRows(12, 20), Sizing: FillFill,
					FreezeHeader: true, Height: 300, BorderStyle: style, SizeBorder: 1}),
			}})
		})
		l := w.TestRender(nil)
		_, _, body, _ := hscrollParts(t, w, l)
		w.ScrollHorizontalTo(body.Shape.idKey(), -100000)
		l = w.TestRender(nil)
		table, headRow, body, _ := hscrollParts(t, w, l)
		cells := hscrollCells(headRow)
		lastRight := cells[len(cells)-1].Shape.X + cells[len(cells)-1].Shape.Width

		var seps []*Layout
		zone := &table.Children[0]
		for i := 1; i < len(zone.Children); i++ {
			seps = append(seps, &zone.Children[i])
		}
		for i := range body.Children {
			c := &body.Children[i]
			if len(c.Children) == 0 && c.Shape.Color == table.Children[0].Children[len(zone.Children)-1].Shape.Color &&
				c.Shape.Height <= 2 && c.Shape.Height > 0 {
				seps = append(seps, c)
			}
		}
		if len(seps) == 0 {
			t.Fatalf("style %d: no separators found", style)
		}
		for _, s := range seps {
			if right := s.Shape.X + s.Shape.Width; right < lastRight-0.5 {
				t.Fatalf("style %d: separator ends at x=%.0f, last column ends at %.0f",
					style, right, lastRight)
			}
		}
	}
}
