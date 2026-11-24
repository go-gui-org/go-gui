package datagrid

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	gg "github.com/go-gui-org/go-gui/gui"
)

// Pinned columns (#1364): they stay in place while the others scroll
// sideways, in every row of cells.

const pinTestCols = 10

// pinGrid builds a grid of pinTestCols columns c0.. and 30 rows 0..,
// with the columns in pins pinned.
func pinGrid(w *gg.Window, pins map[int]GridColumnPin, mut func(*DataGridCfg)) gg.View {
	return New(w, pinGridCfg(pins, mut))
}

func pinGridCfg(pins map[int]GridColumnPin, mut func(*DataGridCfg)) DataGridCfg {
	cols := make([]GridColumnCfg, 0, pinTestCols)
	for c := range pinTestCols {
		cols = append(cols, GridColumnCfg{
			ID: fmt.Sprint("c", c), Title: fmt.Sprint("C", c),
			Width: gg.SomeF(110), Pin: pins[c], Filterable: true,
		})
	}
	rows := make([]GridRow, 0, 30)
	for r := range 30 {
		cells := map[string]string{}
		for c := range pinTestCols {
			cells[fmt.Sprint("c", c)] = fmt.Sprint(r, "-", c)
		}
		rows = append(rows, GridRow{ID: strconv.Itoa(r), Cells: cells})
	}
	cfg := DataGridCfg{
		ID: "g", Columns: cols, Rows: rows,
		Sizing: gg.FillFixed, Height: 360, FreezeHeader: true,
	}
	if mut != nil {
		mut(&cfg)
	}
	return cfg
}

const pinScrollID = "g:scroll"

func pinWindow(t *testing.T, width int, pins map[int]GridColumnPin, mut func(*DataGridCfg)) *gg.Window {
	t.Helper()
	w := gg.NewTestWindow(t, gg.WindowCfg{Width: width, Height: 400})
	w.TestRender(func(w *gg.Window) gg.View { return pinGrid(w, pins, mut) })
	w.TestRender(nil)
	return w
}

func pinRTL(t *testing.T) {
	t.Helper()
	saved := gg.CurrentLocale()
	t.Cleanup(func() { gg.SetLocale(saved) })
	loc := saved
	loc.TextDir = gg.TextDirRTL
	gg.SetLocale(loc)
}

func pinFind(t *testing.T, l *gg.Layout, id string) *gg.Shape {
	t.Helper()
	ly, ok := l.FindByID(id)
	if !ok {
		t.Fatalf("%s not found", id)
	}
	return ly.Shape
}

func pinScroll(w *gg.Window, x float32) *gg.Layout {
	w.ScrollHorizontalTo(pinScrollID, x)
	return w.TestRender(nil)
}

func near(a, b, tol float32) bool { return a-b <= tol && b-a <= tol }

// pinCells are the IDs of column c's cell in each kind of cell row.
func pinCells(c int, kinds []string) []string {
	ids := make([]string, 0, len(kinds))
	for _, k := range kinds {
		ids = append(ids, gg.ScopeID("g", k, fmt.Sprint("c", c)))
	}
	return ids
}

// Pinned cells do not move and the others move by the offset, in the
// header, filter, frozen-top and data rows, whose columns stay in line.
// At the end of the range the last scrolling column is whole.
func TestDataGridPinsStay(t *testing.T) {
	for _, pc := range []struct {
		name string
		pins map[int]GridColumnPin
	}{
		{"left", map[int]GridColumnPin{3: GridColumnPinLeft, 5: GridColumnPinLeft}},
		{"right", map[int]GridColumnPin{4: GridColumnPinRight}},
		{"both", map[int]GridColumnPin{3: GridColumnPinLeft, 4: GridColumnPinRight, 0: GridColumnPinRight}},
	} {
		for _, border := range []bool{true, false} {
			for _, rtl := range []bool{false, true} {
				for _, crud := range []bool{false, true} {
					for _, freeze := range []bool{false, true} {
						for _, detail := range []bool{false, true} {
							name := fmt.Sprintf("%s/border=%v/rtl=%v/crud=%v/freeze=%v/detail=%v",
								pc.name, border, rtl, crud, freeze, detail)
							t.Run(name, func(t *testing.T) {
								if rtl {
									pinRTL(t)
								}
								w := pinWindow(t, 520, pc.pins, func(c *DataGridCfg) {
									c.ShowFilterRow = true
									c.FrozenTopRowIDs = []string{"2"}
									c.ShowCRUDToolbar = crud
									c.FreezeHeader = freeze
									if detail {
										c.DetailExpandedRowIDs = map[string]bool{"1": true}
										c.DetailRowView = func(GridRow, *gg.Window) gg.View {
											return gg.Text(gg.TextCfg{Text: "detail"})
										}
									}
									if !border {
										c.SizeBorder = gg.NoBorder
									}
								})
								checkPins(t, w, pc.pins, border, rtl, crud)
							})
						}
					}
				}
			}
		}
	}
}

func checkPins(t *testing.T, w *gg.Window, pins map[int]GridColumnPin, border, rtl, crud bool) {
	t.Helper()
	kinds := []string{"header", "filter_cell", "cell:2", "cell:0", "cell:1"}
	ids := func(c int) []string {
		return pinCells(c, kinds)
	}
	l := w.TestRender(nil)
	before := map[string]float32{}
	for c := range pinTestCols {
		for _, id := range ids(c) {
			before[id] = pinFind(t, l, id).X
		}
	}
	for _, ask := range []float32{-90, -100000} {
		l = pinScroll(w, ask)
		off, _, err := w.TestScrollOffset(pinScrollID)
		if err != nil || off >= 0 {
			t.Fatalf("offset = %.1f (err %v) after scrolling to %.0f, want < 0", off, err, ask)
		}
		dx := off
		if rtl {
			dx = -off
		}
		for c := range pinTestCols {
			want := dx
			if pins[c] != GridColumnPinNone {
				want = 0
			}
			ref := pinFind(t, l, ids(c)[3]).X
			for _, id := range ids(c) {
				x := pinFind(t, l, id).X
				if !near(x-before[id], want, 0.5) {
					t.Fatalf("offset %.0f: %s moved by %.1f, want %.1f", off, id, x-before[id], want)
				}
				if !near(x, ref, 0.1) {
					t.Fatalf("offset %.0f: %s x = %.1f, data cell x = %.1f", off, id, x, ref)
				}
			}
		}
	}
	checkPinsAtEnd(t, l, pins, border, rtl, crud)
	if dup := w.TestDuplicateIDs(); len(dup) != 0 {
		t.Fatalf("duplicate IDs: %v", dup)
	}
	if found := w.TestFindings(gg.DebugAll); len(found) != 0 {
		t.Fatalf("findings: %v", found)
	}
}

// checkPinsAtEnd checks a grid scrolled to the end of its range: the
// last scrolling column ends where the cells after it start, or at the
// viewport's edge, and what is pinned right ends at that edge.
func checkPinsAtEnd(t *testing.T, l *gg.Layout, pins map[int]GridColumnPin, border, rtl, crud bool) {
	t.Helper()
	gap := float32(1)
	if !border {
		gap = -dataGridPinEdgeWidth
	}
	body, _ := l.FindByID(pinScrollID)
	bs := body.Shape
	left, right := bs.X, bs.X+bs.Width
	if rtl {
		left += bs.Padding.Right
	} else {
		right -= bs.Padding.Right
	}
	const tol = float32(0.1)
	lastCenter, hasRight := -1, false
	for c := range pinTestCols {
		switch pins[c] {
		case GridColumnPinNone:
			lastCenter = c
		case GridColumnPinRight:
			hasRight = true
		}
	}
	last := pinFind(t, l, gg.ScopeID("g", "cell", "0", fmt.Sprint("c", lastCenter)))
	end := last.X + last.Width
	edge := right
	if rtl {
		end, edge = last.X, left
	}
	switch {
	case hasRight:
		// The first right-pinned column in display order.
		first := -1
		for c := range pinTestCols {
			if pins[c] == GridColumnPinRight {
				first = c
				break
			}
		}
		s := pinFind(t, l, gg.ScopeID("g", "cell", "0", fmt.Sprint("c", first)))
		// The two share a border, or have the edge line between them.
		edge = s.X + gap
		if rtl {
			edge = s.X + s.Width - gap
		}
		if !near(end, edge, tol) {
			t.Fatalf("last scrolling column ends at %.1f, the right pins start at %.1f", end, edge)
		}
	case crud:
		del := pinFind(t, l, gg.ScopeID("g", "row-delete", "0"))
		end = del.X + del.Width
		if rtl {
			end = del.X
		}
		fallthrough
	default:
		if !near(end, edge, tol) {
			t.Fatalf("row ends at %.1f, viewport at %.1f", end, edge)
		}
	}
	if hasRight && crud {
		del := pinFind(t, l, gg.ScopeID("g", "row-delete", "0"))
		end, edge = del.X+del.Width, right
		if rtl {
			end, edge = del.X, left
		}
		if !near(end, edge, tol) {
			t.Fatalf("delete button ends at %.1f, viewport at %.1f", end, edge)
		}
	}
}

// Unscrolled, a left pin moves no cell, and in a grid wider than its
// columns neither does a right pin, whatever the border: only the edge
// line adds its width.
func TestDataGridPinsUnscrolledGeometry(t *testing.T) {
	for _, border := range []bool{true, false} {
		t.Run(fmt.Sprint("border=", border), func(t *testing.T) {
			xs := func(pins map[int]GridColumnPin) []float32 {
				w := pinWindow(t, 1600, pins, func(c *DataGridCfg) {
					if !border {
						c.SizeBorder = gg.NoBorder
					}
				})
				l := w.TestRender(nil)
				out := make([]float32, 0, pinTestCols)
				for c := range pinTestCols {
					out = append(out, pinFind(t, l, gg.ScopeID("g", "cell", "0", fmt.Sprint("c", c))).X)
				}
				return out
			}
			plain := xs(nil)
			pinned := xs(map[int]GridColumnPin{
				0: GridColumnPinLeft, 1: GridColumnPinLeft, pinTestCols - 1: GridColumnPinRight,
			})
			for c := range plain {
				want := plain[c]
				if !border && c >= 2 {
					want += dataGridPinEdgeWidth
				}
				if !border && c == pinTestCols-1 {
					want += dataGridPinEdgeWidth
				}
				if !near(pinned[c], want, 0.01) {
					t.Fatalf("cell %d x = %.2f, want %.2f", c, pinned[c], want)
				}
			}
		})
	}
}

// A scrolling cell draws only between the pinned ones.
func TestDataGridPinsClip(t *testing.T) {
	w := pinWindow(t, 520, map[int]GridColumnPin{0: GridColumnPinLeft, 9: GridColumnPinRight}, nil)
	l := pinScroll(w, -60)
	c0 := pinFind(t, l, "g:cell:0:c0")
	c9 := pinFind(t, l, "g:cell:0:c9")
	var cut int
	for c := 1; c < pinTestCols-1; c++ {
		s := pinFind(t, l, gg.ScopeID("g", "cell", "0", fmt.Sprint("c", c)))
		if s.X < c0.X+c0.Width || s.X+s.Width > c9.X {
			cut++
		}
		// 0.5: the shared border belongs to the pinned cell.
		for _, x := range []float32{c0.X + c0.Width - 0.5, c9.X + 0.5} {
			if s.PointInShape(x, s.Y+s.Height/2) {
				t.Errorf("column %d takes the pointer at x=%.0f, over a pinned cell", c, x)
			}
		}
	}
	if cut == 0 {
		t.Fatal("no column is cut; the case needs one")
	}
}

// Rows that are not made of cells: a group header and the quick filter
// stay, so the label, the search text and the clear button show; a detail
// row scrolls with the columns; the column chooser stays and its list
// scrolls on its own.
func TestDataGridPinsOtherRows(t *testing.T) {
	for _, rtl := range []bool{false, true} {
		t.Run(fmt.Sprint("rtl=", rtl), func(t *testing.T) {
			if rtl {
				pinRTL(t)
			}
			w := pinWindow(t, 520, map[int]GridColumnPin{0: GridColumnPinLeft}, func(c *DataGridCfg) {
				c.Rows = c.Rows[:4]
				// Too long for the chooser's list to fit the grid.
				for i := range c.Columns {
					c.Columns[i].Title = fmt.Sprint("A long column title ", i)
				}
				c.GroupBy = []string{"c9"}
				c.ShowQuickFilter = true
				c.ShowColumnChooser = true
				c.OnHiddenColumnsChange = func(map[string]bool, gg.EventCtx) {}
				c.DetailExpandedRowIDs = map[string]bool{"0": true}
				c.DetailRowView = func(GridRow, *gg.Window) gg.View {
					return gg.Rectangle(gg.RectangleCfg{
						ID: "g:det", Width: 30, Height: 10, Sizing: gg.FixedFixed,
					})
				}
			})
			gg.StateMap[string, bool](w, nsDgChooserOpen, capModerate).Set("g", true)
			l := w.TestRender(nil)
			ids := []string{"g:det", "g:quick_filter", "g:filter_clear", "g:column_chooser", "g:col-chooser:c0"}
			before := map[string]float32{}
			for _, id := range ids {
				before[id] = pinFind(t, l, id).X
			}
			group := pinGroupRow(t, l)
			groupX := group.X

			l = pinScroll(w, -100)
			off, _, _ := w.TestScrollOffset(pinScrollID)
			dx := off
			if rtl {
				dx = -off
			}
			for id, want := range map[string]float32{
				"g:det": dx, "g:quick_filter": 0, "g:filter_clear": 0, "g:column_chooser": 0, "g:col-chooser:c0": 0,
			} {
				if got := pinFind(t, l, id).X - before[id]; !near(got, want, 0.5) {
					t.Errorf("%s moved by %.1f, want %.1f", id, got, want)
				}
			}
			for _, id := range []string{"g:quick_filter", "g:filter_clear"} {
				s := pinFind(t, l, id)
				for _, x := range []float32{s.X + 1, s.X + s.Width - 1} {
					if !s.PointInShape(x, s.Y+s.Height/2) {
						t.Errorf("%s is cut off at x=%.0f", id, x)
					}
				}
			}
			if got := pinGroupRow(t, l).X; got != groupX {
				t.Errorf("group header moved from %.1f to %.1f", groupX, got)
			}

			if ls, last := pinFind(t, l, gg.ScopeID("g", "col-chooser-list")),
				pinFind(t, l, "g:col-chooser:c9"); !rtl && last.X < ls.X+ls.Width {
				t.Fatal("the chooser's list fits; the case needs it too wide")
			}
			// The chooser list's own range reaches its last toggle.
			list := gg.ScopeID("g", "col-chooser-list")
			w.ScrollHorizontalTo(list, -100000)
			l = w.TestRender(nil)
			ls, last := pinFind(t, l, list), pinFind(t, l, "g:col-chooser:c9")
			if last.X < ls.X-0.5 || last.X+last.Width > ls.X+ls.Width+0.5 {
				t.Errorf("last chooser toggle spans %.0f..%.0f, the list %.0f..%.0f",
					last.X, last.X+last.Width, ls.X, ls.X+ls.Width)
			}
		})
	}
}

// pinGroupRow returns the first group header row: the body child whose
// text reads "<column title>: <value>".
func pinGroupRow(t *testing.T, l *gg.Layout) *gg.Shape {
	t.Helper()
	body, _ := l.FindByID(pinScrollID)
	for i := range body.Children {
		if strings.Contains(pinFirstText(&body.Children[i]), ": ") {
			return body.Children[i].Shape
		}
	}
	t.Fatal("group header not found")
	return nil
}

// A detail row ends where the data rows do, and its end can be reached.
func TestDataGridPinsDetailEndReachable(t *testing.T) {
	w := pinWindow(t, 520, map[int]GridColumnPin{0: GridColumnPinLeft}, func(c *DataGridCfg) {
		c.DetailExpandedRowIDs = map[string]bool{"0": true}
		c.DetailRowView = func(GridRow, *gg.Window) gg.View {
			return gg.Row(gg.ContainerCfg{ID: "g:det", Sizing: gg.FillFill})
		}
	})
	l := pinScroll(w, -100000)
	body, _ := l.FindByID(pinScrollID)
	det := pinFind(t, l, "g:det")
	right := body.Shape.X + body.Shape.Width - body.Shape.Padding.Right
	pad := pinFind(t, l, "g:detail:0").Padding.Right
	if end := det.X + det.Width; !near(end, right-pad, 0.1) {
		t.Fatalf("detail content ends at %.1f, want %.1f", end, right-pad)
	}
}

// A grid too narrow for its pinned columns scrolls them away first, so
// both ends of every column can be brought into view.
func TestDataGridPinsNarrowReachesEveryColumn(t *testing.T) {
	for _, rtl := range []bool{false, true} {
		t.Run(fmt.Sprint("rtl=", rtl), func(t *testing.T) {
			if rtl {
				pinRTL(t)
			}
			w := pinWindow(t, 200, map[int]GridColumnPin{
				0: GridColumnPinLeft, 1: GridColumnPinLeft, 9: GridColumnPinRight,
			}, func(c *DataGridCfg) { c.ShowQuickFilter = true })
			var start, end [pinTestCols]bool
			for ask := float32(0); ask > -1100; ask -= 10 {
				l := pinScroll(w, ask)
				for c := range pinTestCols {
					s := pinFind(t, l, gg.ScopeID("g", "cell", "0", fmt.Sprint("c", c)))
					y := s.Y + s.Height/2
					// Inside the border, which a pinned neighbour draws.
					start[c] = start[c] || s.PointInShape(s.X+2, y)
					end[c] = end[c] || s.PointInShape(s.X+s.Width-2, y)
					if h := pinFind(t, l, dataGridHeaderCellID("g", fmt.Sprint("c", c))); !near(h.X, s.X, 0.1) {
						t.Fatalf("offset %.0f: header cell %d x = %.1f, body %.1f", ask, c, h.X, s.X)
					}
				}
			}
			for c := range pinTestCols {
				if !start[c] || !end[c] {
					t.Errorf("column %d: start shown %v, end shown %v", c, start[c], end[c])
				}
			}
		})
	}
}

// In that narrow grid the rows that are not made of cells span the
// viewport at the end of the range too, where the rows have moved, and
// both ends of the quick filter can be reached.
func TestDataGridPinsNarrowOtherRows(t *testing.T) {
	w := pinWindow(t, 200, map[int]GridColumnPin{0: GridColumnPinLeft, 9: GridColumnPinRight},
		func(c *DataGridCfg) {
			c.ShowQuickFilter = true
			c.ShowColumnChooser = true
			c.GroupBy = []string{"c8"}
			c.DetailExpandedRowIDs = map[string]bool{"0": true}
			c.DetailRowView = func(GridRow, *gg.Window) gg.View {
				return gg.Text(gg.TextCfg{Text: "detail"})
			}
		})
	l := w.TestRender(nil)
	if quick := pinFind(t, l, "g:quick_filter"); !quick.PointInShape(quick.X+1, quick.Y+quick.Height/2) {
		t.Errorf("the quick filter's start at x=%.0f cannot be reached", quick.X)
	}
	l = pinScroll(w, -100000)
	body, _ := l.FindByID(pinScrollID)
	right := body.Shape.X + body.Shape.Width - body.Shape.Padding.Right
	spans := map[string]*gg.Shape{
		"group header": pinGroupRow(t, l),
		"detail row":   pinFind(t, l, "g:detail:0"),
		"chooser":      body.Children[0].Shape,
	}
	for name, s := range spans {
		if s.X > body.Shape.X+1 || s.X+s.Width < right-1 {
			t.Errorf("%s spans %.0f..%.0f, viewport %.0f..%.0f", name, s.X, s.X+s.Width, body.Shape.X, right)
		}
	}
	clear := pinFind(t, l, "g:filter_clear")
	if !clear.PointInShape(clear.X+clear.Width/2, clear.Y+clear.Height/2) {
		t.Errorf("the quick filter's clear button at x=%.0f cannot be reached", clear.X)
	}
}

// Pinning every column, or none, leaves the grid as it was: every cell
// scrolls.
func TestDataGridPinsNothingToScroll(t *testing.T) {
	all := map[int]GridColumnPin{}
	for c := range pinTestCols {
		all[c] = GridColumnPinLeft
	}
	for name, pins := range map[string]map[int]GridColumnPin{"none": nil, "all": all} {
		t.Run(name, func(t *testing.T) {
			w := pinWindow(t, 520, pins, nil)
			x := pinFind(t, w.TestRender(nil), "g:cell:0:c2").X
			l := pinScroll(w, -80)
			if got := pinFind(t, l, "g:cell:0:c2").X; !near(got, x-80, 0.5) {
				t.Fatalf("cell moved by %.1f, want -80", got-x)
			}
		})
	}
}

// The header stays in line on the first frame after the grid gets wider
// at the end of its range.
func TestDataGridPinsInLineOnResize(t *testing.T) {
	width := float32(500)
	w := gg.NewTestWindow(t, gg.WindowCfg{Width: 1200, Height: 400})
	w.TestRender(func(w *gg.Window) gg.View {
		return gg.Row(gg.ContainerCfg{Sizing: gg.FitFill, Content: []gg.View{
			gg.Column(gg.ContainerCfg{Sizing: gg.FixedFill, Width: width, Content: []gg.View{
				pinGrid(w, map[int]GridColumnPin{0: GridColumnPinLeft, 9: GridColumnPinRight}, nil),
			}}),
		}})
	})
	w.TestRender(nil)
	pinScroll(w, -100000)
	before, _, _ := w.TestScrollOffset(pinScrollID)
	width = 620
	l := w.TestRender(nil)
	if after, _, _ := w.TestScrollOffset(pinScrollID); after == before {
		t.Fatalf("offset stayed %.0f after widening; the case needs a clamp", after)
	}
	for c := range pinTestCols {
		h := pinFind(t, l, dataGridHeaderCellID("g", fmt.Sprint("c", c)))
		s := pinFind(t, l, gg.ScopeID("g", "cell", "0", fmt.Sprint("c", c)))
		if !near(h.X, s.X, 0.1) {
			t.Fatalf("header cell %d x = %.1f, body %.1f", c, h.X, s.X)
		}
	}
}

// A wheel scrolls the grid by the same amount over pinned and scrolling
// cells, in the header and in the rows.
func TestDataGridPinsWheel(t *testing.T) {
	wheel := func(id string) (x, y float32) {
		w := pinWindow(t, 520, map[int]GridColumnPin{0: GridColumnPinLeft, 9: GridColumnPinRight}, nil)
		s := pinFind(t, w.TestRender(nil), id)
		w.EventFn(&gg.Event{
			Type: gg.EventMouseScroll, ScrollPrecise: true, ScrollX: -2, ScrollY: -1,
			MouseX: s.X + s.Width/2, MouseY: s.Y + s.Height/2,
		})
		w.TestRender(nil)
		x, y, err := w.TestScrollOffset(pinScrollID)
		if err != nil {
			t.Fatal(err)
		}
		return x, y
	}
	wx, wy := wheel("g:cell:1:c2")
	if wx >= 0 || wy >= 0 {
		t.Fatalf("wheel over a scrolling cell: offset (%.1f, %.1f), want both < 0", wx, wy)
	}
	for _, id := range []string{"g:cell:1:c0", "g:cell:1:c9", "g:header:c0", "g:header:c2", "g:header:c9"} {
		if x, y := wheel(id); x != wx || y != wy {
			t.Errorf("wheel over %s: offset (%.1f, %.1f), want (%.1f, %.1f)", id, x, y, wx, wy)
		}
	}
}

// A click on either side of the edge selects the same row.
func TestDataGridPinsSelection(t *testing.T) {
	var selected GridSelection
	w := pinWindow(t, 520, map[int]GridColumnPin{0: GridColumnPinLeft, 9: GridColumnPinRight},
		func(c *DataGridCfg) {
			c.OnSelectionChange = func(s GridSelection, _ gg.EventCtx) { selected = s }
		})
	l := pinScroll(w, -70)
	for _, id := range []string{"g:cell:3:c0", "g:cell:4:c3", "g:cell:5:c9"} {
		s := pinFind(t, l, id)
		for _, typ := range []gg.EventType{gg.EventMouseDown, gg.EventMouseUp} {
			w.EventFn(&gg.Event{Type: typ, MouseButton: gg.MouseLeft,
				MouseX: s.X + s.Width/2, MouseY: s.Y + s.Height/2})
		}
		row := id[len("g:cell:") : len("g:cell:")+1]
		if !selected.SelectedRowIDs[row] {
			t.Errorf("click on %s: selected %v, want row %s", id, selected.SelectedRowIDs, row)
		}
	}
}

// A grid with a data source keeps its pinned columns too, while it loads
// and once the rows are there.
func TestDataGridPinsDataSource(t *testing.T) {
	pins := map[int]GridColumnPin{0: GridColumnPinLeft, 9: GridColumnPinRight}
	cfg := pinGridCfg(pins, func(c *DataGridCfg) {
		c.DataSource = NewInMemoryDataSource(c.Rows)
		c.Rows = nil
		c.PageLimit = 20
		c.ShowFilterRow = true
		c.ShowCRUDToolbar = true
	})
	w := gg.NewTestWindow(t, gg.WindowCfg{Width: 520, Height: 400})
	w.TestRender(func(w *gg.Window) gg.View { return New(w, cfg) })
	l := w.TestRender(nil)
	x0, x9 := pinFind(t, l, "g:header:c0").X, pinFind(t, l, "g:header:c9").X
	l = pinScroll(w, -80)
	if pinFind(t, l, "g:header:c0").X != x0 || pinFind(t, l, "g:header:c9").X != x9 {
		t.Fatal("a pinned header cell moved while the source loads")
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		w.FrameFn() // takes the source's rows off the command queue
		l = w.TestRender(nil)
		if _, ok := l.FindByID("g:cell:0:c0"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the source did not load")
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, ask := range []float32{-80, -100000} {
		l = pinScroll(w, ask)
		for c := range pinTestCols {
			id := fmt.Sprint("c", c)
			head, cell := pinFind(t, l, "g:header:"+id), pinFind(t, l, "g:cell:0:"+id)
			if !near(head.X, cell.X, 0.1) {
				t.Fatalf("offset %.0f: %s header x = %.1f, cell %.1f", ask, id, head.X, cell.X)
			}
		}
		if pinFind(t, l, "g:cell:0:c0").X != x0 || pinFind(t, l, "g:cell:0:c9").X != x9 {
			t.Fatalf("offset %.0f: a pinned cell moved", ask)
		}
	}
}

// A grid that fits its columns is as wide with pins as without, and has
// nothing to scroll.
func TestDataGridPinsFitWidth(t *testing.T) {
	width := func(pins map[int]GridColumnPin, quick bool) float32 {
		w := gg.NewTestWindow(t, gg.WindowCfg{Width: 3000, Height: 400})
		w.TestRender(func(w *gg.Window) gg.View {
			return pinGrid(w, pins, func(c *DataGridCfg) {
				c.Sizing, c.Height, c.MaxHeight = gg.FitFit, 0, 300
				c.ShowQuickFilter = quick
			})
		})
		l := pinScroll(w, -100000)
		if off, _, err := w.TestScrollOffset(pinScrollID); err != nil || off != 0 {
			t.Errorf("pins %v: offset = %.1f (err %v), want 0", pins, off, err)
		}
		return pinFind(t, l, "g").Width
	}
	pins := map[int]GridColumnPin{0: GridColumnPinLeft, 9: GridColumnPinRight}
	for _, quick := range []bool{false, true} {
		if plain, pinned := width(nil, quick), width(pins, quick); !near(plain, pinned, 0.1) {
			t.Errorf("quick filter %v: grid width = %.1f with pins, %.1f without", quick, pinned, plain)
		}
	}
}

// Pinning costs a few heap allocations per frame and none per row: the
// same few for 40 rows as for 80. The grid is configured once, outside
// the frame. Building it with fmt in the view made the counts vary under
// the race detector, which makes sync.Pool drop items at random.
func TestDataGridPinsAllocs(t *testing.T) {
	frame := func(pins map[int]GridColumnPin, rows int) float64 {
		cfg := pinGridCfg(pins, func(c *DataGridCfg) {
			for len(c.Rows) < rows {
				row := c.Rows[0]
				row.ID = strconv.Itoa(len(c.Rows))
				c.Rows = append(c.Rows, row)
			}
			c.Height = 4000 // every row
		})
		w := gg.NewTestWindow(t, gg.WindowCfg{Width: 520, Height: 400})
		w.TestRender(func(w *gg.Window) gg.View { return New(w, cfg) })
		w.TestRender(nil)
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
		plain := frame(nil, rows)
		pinned := frame(map[int]GridColumnPin{0: GridColumnPinLeft, 9: GridColumnPinRight}, rows)
		t.Logf("%d rows: %.0f allocs plain, %.0f pinned", rows, plain, pinned)
		return pinned - plain
	}
	few, many := extra(40), extra(80)
	if few > 4 || many > 4 {
		t.Errorf("pins cost %.0f allocs per frame with 40 rows and %.0f with 80, want at most 4", few, many)
	}
}

// The pin control's arrow points at the side the column is pinned to.
func TestDataGridPinControlArrowRTL(t *testing.T) {
	label := func(pin GridColumnPin) string {
		cfg := &DataGridCfg{ID: "g", OnColumnPinChange: func(string, GridColumnPin, gg.EventCtx) {}}
		w := gg.NewTestWindow(t, gg.WindowCfg{Width: 100, Height: 100})
		l := w.TestRender(func(*gg.Window) gg.View {
			return dataGridPinControl(cfg, GridColumnCfg{ID: "a", Pin: pin})
		})
		return pinFirstText(l)
	}
	if l, r := label(GridColumnPinLeft), label(GridColumnPinRight); l != "↤" || r != "↦" {
		t.Fatalf("LTR arrows = %q, %q, want ↤ and ↦", l, r)
	}
	pinRTL(t)
	if l, r := label(GridColumnPinLeft), label(GridColumnPinRight); l != "↦" || r != "↤" {
		t.Fatalf("RTL arrows = %q, %q, want ↦ and ↤", l, r)
	}
}

func pinFirstText(l *gg.Layout) string {
	if l.Shape != nil && l.Shape.TC != nil && l.Shape.TC.Text != "" {
		return l.Shape.TC.Text
	}
	for i := range l.Children {
		if s := pinFirstText(&l.Children[i]); s != "" {
			return s
		}
	}
	return ""
}
