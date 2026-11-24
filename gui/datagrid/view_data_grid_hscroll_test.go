package datagrid

import (
	"fmt"
	"strconv"
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// A grid wider than its parent scrolls sideways; its frozen header
// follows the rows through ContainerCfg.ScrollLinkX (#1293).

func hscrollGridCfg(w *gg.Window) gg.View { return hscrollGrid(w, false) }

// hscrollGrid builds the test grid; crud adds the CRUD toolbar, which
// ends every row with a delete button.
func hscrollGrid(w *gg.Window, crud bool) gg.View {
	cols := make([]GridColumnCfg, 0, 8)
	for c := range 8 {
		cols = append(cols, GridColumnCfg{
			ID: fmt.Sprint("c", c), Title: fmt.Sprint("C", c), Width: gg.SomeF(110),
		})
	}
	rows := make([]GridRow, 0, 30)
	for r := range 30 {
		cells := map[string]string{}
		for c := range 8 {
			cells[fmt.Sprint("c", c)] = fmt.Sprint(r, "-", c)
		}
		rows = append(rows, GridRow{ID: strconv.Itoa(r), Cells: cells})
	}
	return New(w, DataGridCfg{
		ID: "g", Columns: cols, Rows: rows,
		Sizing: gg.FillFixed, Height: 300, FreezeHeader: true,
		ShowCRUDToolbar: crud,
	})
}

// hscrollHeaderAndCell returns column col's header cell and the
// matching cell of the first body row.
func hscrollHeaderAndCell(t *testing.T, l *gg.Layout, col int) (head, cell *gg.Layout) {
	t.Helper()
	head, ok := l.FindByID(dataGridHeaderCellID("g", fmt.Sprint("c", col)))
	if !ok {
		t.Fatalf("header cell c%d not found", col)
	}
	body, ok := l.FindByID(gg.ScopeID("g", "scroll"))
	if !ok || len(body.Children) == 0 {
		t.Fatal("body or its first row not found")
	}
	row := &body.Children[0]
	if len(row.Children) < 8 {
		t.Fatalf("first body row has %d cells, want at least 8", len(row.Children))
	}
	return head, &row.Children[col]
}

func assertGridColumnInLine(t *testing.T, l *gg.Layout, col int, when string) {
	t.Helper()
	head, cell := hscrollHeaderAndCell(t, l, col)
	if d := head.Shape.X - cell.Shape.X; d > 0.1 || d < -0.1 {
		t.Fatalf("%s: column c%d header x=%.1f, body cell x=%.1f",
			when, col, head.Shape.X, cell.Shape.X)
	}
}

func TestDataGridFrozenHeaderInLineOnResize(t *testing.T) {
	width := float32(500)
	w := gg.NewTestWindow(t, gg.WindowCfg{Width: 1200, Height: 400})
	w.TestRender(func(w *gg.Window) gg.View {
		return gg.Row(gg.ContainerCfg{Sizing: gg.FitFill, Content: []gg.View{
			gg.Column(gg.ContainerCfg{Sizing: gg.FixedFill, Width: width,
				Content: []gg.View{hscrollGridCfg(w)}}),
		}})
	})
	w.TestRender(nil)
	scroll := gg.ScopeID("g", "scroll")
	w.ScrollHorizontalTo(scroll, -100000)
	l := w.TestRender(nil)
	before, _, _ := w.TestScrollOffset(scroll)
	if before >= 0 {
		t.Fatalf("offset = %.0f after scrolling to the end, want < 0", before)
	}
	assertGridColumnInLine(t, l, 7, "scrolled to the end")

	width = 600
	l = w.TestRender(nil)
	after, _, _ := w.TestScrollOffset(scroll)
	if after == before {
		t.Fatalf("offset stayed %.0f after widening; the case needs a clamp", after)
	}
	assertGridColumnInLine(t, l, 7, "first frame after widening")
}

// Under RTL the header band and the body are both Columns with the same
// padding, so they place the columns at the same x. The band used to be
// a Row, which started its content at the right edge while the body
// pinned it at the left, so the header showed other columns than the
// rows below it.
func TestDataGridFrozenHeaderInLineRTL(t *testing.T) {
	for _, crud := range []bool{false, true} {
		t.Run(fmt.Sprint("crud=", crud), func(t *testing.T) {
			gridFrozenHeaderInLineRTL(t, crud)
		})
	}
}

// gridFrozenHeaderInLineRTL checks one grid under RTL. With crud, each
// row ends with a delete button, which the header band must count in
// its width: the band places its cells from its right edge, as a row
// does.
func gridFrozenHeaderInLineRTL(t *testing.T, crud bool) {
	saved := gg.CurrentLocale()
	t.Cleanup(func() { gg.SetLocale(saved) })
	loc := saved
	loc.TextDir = gg.TextDirRTL
	gg.SetLocale(loc)

	w := gg.NewTestWindow(t, gg.WindowCfg{Width: 500, Height: 400})
	w.TestRender(func(w *gg.Window) gg.View {
		return gg.Column(gg.ContainerCfg{Sizing: gg.FillFill,
			Content: []gg.View{hscrollGrid(w, crud)}})
	})
	l := w.TestRender(nil)
	for col := range 8 {
		assertGridColumnInLine(t, l, col, "RTL not scrolled")
	}
	// The unscrolled grid shows its first column, at the right edge
	// (#1345); it used to open at the last one.
	assertGridColumnVisible(t, l, 0, "RTL not scrolled")
	scroll := gg.ScopeID("g", "scroll")
	w.ScrollHorizontalTo(scroll, -100000)
	l = w.TestRender(nil)
	for col := range 8 {
		assertGridColumnInLine(t, l, col, "RTL scrolled to the end")
	}
	assertGridColumnVisible(t, l, 7, "RTL scrolled to the end")
}

// assertGridColumnVisible fails unless column col's first body cell lies
// within the body's width.
func assertGridColumnVisible(t *testing.T, l *gg.Layout, col int, when string) {
	t.Helper()
	_, cell := hscrollHeaderAndCell(t, l, col)
	scroll := gg.ScopeID("g", "scroll")
	body, ok := l.FindByID(scroll)
	if !ok {
		t.Fatalf("%s: body %q not found", when, scroll)
	}
	c, b := cell.Shape, body.Shape
	if c.X < b.X || c.X+c.Width > b.X+b.Width {
		t.Fatalf("%s: column c%d spans x=%.0f..%.0f, outside the body %.0f..%.0f",
			when, col, c.X, c.X+c.Width, b.X, b.X+b.Width)
	}
}

// A wheel over the frozen header scrolls the rows sideways.
func TestDataGridFrozenHeaderWheelScrollsRows(t *testing.T) {
	w := gg.NewTestWindow(t, gg.WindowCfg{Width: 500, Height: 400})
	w.TestRender(func(w *gg.Window) gg.View {
		return gg.Column(gg.ContainerCfg{Sizing: gg.FillFill,
			Content: []gg.View{hscrollGridCfg(w)}})
	})
	l := w.TestRender(nil)
	head, _ := hscrollHeaderAndCell(t, l, 0)
	e := gg.Event{
		Type: gg.EventMouseScroll, ScrollPrecise: true,
		MouseX:  head.Shape.X + head.Shape.Width/2,
		MouseY:  head.Shape.Y + head.Shape.Height/2,
		ScrollX: -2,
	}
	w.EventFn(&e)
	l = w.TestRender(nil)
	x, _, _ := w.TestScrollOffset(gg.ScopeID("g", "scroll"))
	if x >= 0 {
		t.Fatalf("offset = %.0f after a wheel over the header, want < 0", x)
	}
	assertGridColumnInLine(t, l, 0, "after the wheel")
}

// The horizontal scrollbar has a strip of its own under the rows. Scrolled
// to the end, by the wheel range or with the End key, the last row is
// whole above it, with and without a frozen header, pager and pins.
func TestDataGridLastRowAboveHorizontalScrollbar(t *testing.T) {
	for _, c := range []struct {
		name string
		mut  func(*DataGridCfg)
	}{
		{"frozen header", nil},
		{"header in the body", func(c *DataGridCfg) { c.FreezeHeader = false }},
		{"pager", func(c *DataGridCfg) { c.PageSize = 25 }},
		{"pins", func(c *DataGridCfg) { c.Columns[0].Pin = GridColumnPinLeft }},
		{"not virtualized", func(c *DataGridCfg) { c.Height, c.Sizing = 0, gg.FillFill }},
	} {
		for _, key := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/key=%v", c.name, key), func(t *testing.T) {
				var selected GridSelection
				w := gg.NewTestWindow(t, gg.WindowCfg{Width: 500, Height: 300})
				var lastRow string
				w.TestRender(func(w *gg.Window) gg.View {
					return pinGrid(w, nil, func(cfg *DataGridCfg) {
						cfg.Height = 280
						cfg.Selection = selected
						cfg.OnSelectionChange = func(s GridSelection, _ gg.EventCtx) { selected = s }
						if c.mut != nil {
							c.mut(cfg)
						}
						lastRow = "29"
						if cfg.PageSize > 0 {
							lastRow = strconv.Itoa(cfg.PageSize - 1)
						}
					})
				})
				w.TestRender(nil)
				if key && c.name == "not virtualized" {
					t.Skip("a grid with no height does not scroll to the selected row")
				}
				if key {
					if err := w.TestKey("g", gg.KeyEnd, gg.ModNone); err != nil {
						t.Fatal(err)
					}
				} else {
					w.ScrollVerticalTo(pinScrollID, -100000)
				}
				w.TestRender(nil)
				l := w.TestRender(nil)
				body, _ := l.FindByID(pinScrollID)
				row := pinFind(t, l, gg.ScopeID("g", "row", lastRow))
				bar := float32(0)
				for i := range body.Children {
					if s := body.Children[i].Shape; s.OverDraw && s.Width > s.Height {
						bar = s.Y
					}
				}
				if bar == 0 {
					t.Fatal("no horizontal scrollbar; the case needs one")
				}
				if end := row.Y + row.Height; end > bar {
					t.Fatalf("last row ends at %.1f, the scrollbar starts at %.1f", end, bar)
				}
			})
		}
	}
}
