package gui

import "testing"

// WithTooltip only adds a hover test: the wrapped view keeps the place
// and the size it has without the wrapper.
func TestWithTooltipAddsNoRoom(t *testing.T) {
	box := func() View {
		return Row(ContainerCfg{ID: "box", Sizing: FixedFixed, Width: 40, Height: 20, Padding: PaddingNone})
	}
	row := func(wrap bool) func(*Window) View {
		return func(w *Window) View {
			v := box()
			if wrap {
				v = WithTooltip(w, WithTooltipCfg{ID: "tip", Text: "tip", Content: []View{v}})
			}
			return Row(ContainerCfg{ID: "row", Sizing: FitFit, Padding: PaddingNone, Spacing: NoSpacing,
				SizeBorder: NoBorder, Content: []View{v}})
		}
	}
	shapes := func(wrap bool) (*Shape, *Shape) {
		w := newTipWindow(t, row(wrap))
		r, b := findShapeByID(&w.layout, "row"), findShapeByID(&w.layout, "box")
		if r == nil || b == nil {
			t.Fatalf("wrap %v: shapes not found", wrap)
		}
		return r.Shape, b.Shape
	}
	plainRow, plainBox := shapes(false)
	tipRow, tipBox := shapes(true)
	if tipBox.X-tipRow.X != plainBox.X-plainRow.X || tipBox.Y-tipRow.Y != plainBox.Y-plainRow.Y {
		t.Errorf("box moved inside the row: at (%v,%v) wrapped, (%v,%v) plain",
			tipBox.X-tipRow.X, tipBox.Y-tipRow.Y, plainBox.X-plainRow.X, plainBox.Y-plainRow.Y)
	}
	if tipRow.Width != plainRow.Width || tipRow.Height != plainRow.Height {
		t.Errorf("row is %vx%v wrapped, %vx%v plain", tipRow.Width, tipRow.Height, plainRow.Width, plainRow.Height)
	}
}
