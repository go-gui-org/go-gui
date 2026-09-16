package gui

import (
	"strconv"
	"testing"
)

func benchScrollLayout(scrollRegions, childrenPer int) Layout {
	root := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      axisTopToBottom,
			Sizing:    FillFill,
			Width:     1200,
			Height:    900,
		},
		Children: make([]Layout, scrollRegions),
	}
	for i := range scrollRegions {
		container := Layout{
			Shape: &Shape{
				shapeType:  shapeRectangle,
				Axis:       axisTopToBottom,
				Scrollable: true,
				ID:         "test-scroll-" + strconv.Itoa(i+1),
				Width:      1200,
				Height:     200,
			},
			Children: make([]Layout, childrenPer),
		}
		for j := range childrenPer {
			container.Children[j] = Layout{
				Shape: &Shape{
					shapeType: shapeRectangle,
					Width:     1200,
					Height:    40,
					Focusable: true,
					ID:        "f" + strconv.Itoa(i*childrenPer+j+1),
				},
			}
		}
		root.Children[i] = container
	}
	return root
}

func BenchmarkLayoutAdjustScrollOffsets(b *testing.B) {
	w := &Window{scratch: newScratchPools()}
	w.windowWidth = 1200
	w.windowHeight = 900
	layout := benchScrollLayout(10, 20)
	b.ReportAllocs()
	for b.Loop() {
		layoutAdjustScrollOffsets(&layout, w)
	}
}

func benchOverflowLayout(totalChildren int) Layout {
	root := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      axisLeftToRight,
			Overflow:  true,
			Width:     600,
			Height:    40,
			Spacing:   4,
		},
		Children: make([]Layout, totalChildren),
	}
	for i := range totalChildren - 1 {
		root.Children[i] = Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				Width:     80,
				Height:    40,
			},
		}
	}
	// Last child is the overflow trigger button.
	root.Children[totalChildren-1] = Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     32,
			Height:    40,
		},
	}
	return root
}

func BenchmarkLayoutOverflow(b *testing.B) {
	w, layout, widths := newOverflowBench(20)
	b.ReportAllocs()
	for b.Loop() {
		resetOverflowLayout(layout, widths)
		layoutOverflow(layout, w)
	}
}

// newOverflowBench builds the fixture on the heap, as a real frame does, and
// keeps the start width of each child so every iteration can restore it.
//
// It is a separate function on purpose: returning the pointers makes them
// escape. Built inside the benchmark, escape analysis kept the Window, the
// root Shape and the Children slice on the goroutine stack, and ns/op then
// depended on where that stack landed. Some processes ran ~8x slower (~800 vs
// ~95 ns/op on Apple Silicon) for the same work and the same result, so
// benchstat reported +600% between two runs of one commit.
//
//go:noinline
func newOverflowBench(totalChildren int) (*Window, *Layout, []float32) {
	w := &Window{scratch: newScratchPools()}
	layout := benchOverflowLayout(totalChildren)
	widths := make([]float32, len(layout.Children))
	for i := range layout.Children {
		widths[i] = layout.Children[i].Shape.Width
	}
	return w, &layout, widths
}

// resetOverflowLayout undoes everything hideOverflowChild wrote. Resetting
// only shapeType left hidden children at Width 0, so from the second
// iteration on every item fit and the benchmark measured the all-fit case,
// not the overflow case it is named for.
func resetOverflowLayout(layout *Layout, widths []float32) {
	for i := range layout.Children {
		s := layout.Children[i].Shape
		s.shapeType = shapeRectangle
		s.Width = widths[i]
		s.Clip = false
	}
}

// The benchmark must measure the overflow case on every iteration, not only
// the first. Resetting only shapeType left hidden children at Width 0, so the
// second pass found that everything fit and hid just the trigger.
func TestResetOverflowLayoutKeepsOverflowCase(t *testing.T) {
	w, layout, widths := newOverflowBench(20)
	for pass := range 3 {
		resetOverflowLayout(layout, widths)
		layoutOverflow(layout, w)
		hidden := 0
		for i := range layout.Children {
			if layout.Children[i].Shape.shapeType == shapeNone {
				hidden++
			}
		}
		// 600 wide, items 80 + 4 spacing, trigger 32: 6 items fit, 13 hide.
		if hidden != 13 {
			t.Fatalf("pass %d: hidden = %d, want 13", pass, hidden)
		}
		if layout.Children[19].Shape.shapeType == shapeNone {
			t.Fatalf("pass %d: trigger hidden, want visible", pass)
		}
	}
}
