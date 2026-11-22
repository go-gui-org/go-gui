package gui

import "testing"

// End scrolls the last row fully into view: the keyboard scroll uses
// the body's visible height, which leaves out the frozen header and the
// strip kept for the horizontal scrollbar (#1293).
func TestTableEndShowsLastRowWhole(t *testing.T) {
	for _, freeze := range []bool{false, true} {
		w := NewTestWindow(t, WindowCfg{Width: 600, Height: 400})
		// A measurer whose row height estimate matches the arranged
		// rows, so virtualization spacers and End agree with layout.
		w.textMeasurer = &tableTestMeasurer{}
		w.TestRender(func(*Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Content: []View{
				Table(TableCfg{ID: "t", Data: hscrollTableRows(3, 50),
					Sizing: FillFixed, Height: 200, Focusable: true,
					FreezeHeader: freeze}),
			}})
		})
		w.TestRender(nil)
		id := w.ResolveID("t")[0]
		if err := w.TestKey(id, KeyEnd, 0); err != nil {
			t.Fatal(err)
		}
		l := w.TestRender(nil)
		scrollID := id
		if freeze {
			scrollID = ScopeID(id, "scroll")
		}
		body, ok := l.FindByID(scrollID)
		if !ok {
			t.Fatal("scroll body not found")
		}
		viewBottom := body.Shape.Y + body.Shape.Height - body.Shape.Padding.Bottom
		var last *Layout
		for i := range body.Children {
			c := &body.Children[i]
			if len(hscrollCells(c)) > 0 {
				last = c
			}
		}
		if last == nil {
			t.Fatal("no row in the body")
		}
		if cells := hscrollCells(last); len(cells) == 0 || last.Shape.Y+last.Shape.Height > viewBottom+0.5 {
			t.Fatalf("freeze=%v: last row bottom %.1f is below the visible bottom %.1f",
				freeze, last.Shape.Y+last.Shape.Height, viewBottom)
		}
	}
}
