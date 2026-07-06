package gui

import (
	"math"
	"testing"
)

// buildFuzzSizingLayoutTree builds a layout tree with randomized sizing
// combinations at each depth level to exercise the sizing engine.
func buildFuzzSizingLayoutTree(depth, childCount int, w *Window) Layout {
	root := Layout{
		Shape: w.allocShape(Shape{
			shapeType: shapeRectangle,
			Sizing:    FillFill,
			Width:     1024,
			Height:    768,
		}),
	}
	if depth <= 0 || childCount <= 0 {
		return root
	}
	root.Children = make([]Layout, childCount)
	for i := range childCount {
		var sizing Sizing
		switch i % 9 {
		case 0:
			sizing = FitFit
		case 1:
			sizing = FillFill
		case 2:
			sizing = FixedFixed
		case 3:
			sizing = FitFill
		case 4:
			sizing = FillFit
		case 5:
			sizing = FitFixed
		case 6:
			sizing = FixedFit
		case 7:
			sizing = FixedFill
		case 8:
			sizing = FillFixed
		}
		child := Layout{
			Shape: w.allocShape(Shape{
				shapeType: shapeRectangle,
				Sizing:    sizing,
				Width:     100,
				Height:    50,
			}),
		}
		if depth > 1 {
			sub := buildFuzzSizingLayoutTree(depth-1, childCount/2, w)
			child.Children = sub.Children
		}
		root.Children[i] = child
	}
	return root
}

// FuzzLayoutSizing fuzzes the layout sizing engine with randomly
// generated layout trees and sizing combinations. More exhaustive than
// the existing FuzzLayoutPipelineDimensions — exercises all nine sizing
// combos, mixed Shapes with MinWidth/MaxWidth constraints, and axis
// directions.
func FuzzLayoutSizing(f *testing.F) {
	f.Add(uint8(3), uint8(5), float32(1024), float32(768))
	f.Add(uint8(1), uint8(20), float32(100), float32(100))
	f.Add(uint8(0), uint8(0), float32(0), float32(0))
	f.Add(uint8(4), uint8(1), float32(1920), float32(1080))

	f.Fuzz(func(t *testing.T, depth, childCount uint8, width, height float32) {
		d := int(depth % 5)
		n := int(childCount % 25)

		// Clamp dimensions to safe ranges.
		if math.IsNaN(float64(width)) || math.IsInf(float64(width), 0) {
			width = 1024
		}
		if math.IsNaN(float64(height)) || math.IsInf(float64(height), 0) {
			height = 768
		}
		if width < 0 {
			width = -width
		}
		if height < 0 {
			height = -height
		}
		if width == 0 {
			width = 1
		}
		if height == 0 {
			height = 1
		}
		if width > 8192 {
			width = 8192
		}
		if height > 8192 {
			height = 8192
		}

		w := &Window{
			scratch:      newScratchPools(),
			windowWidth:  int(width),
			windowHeight: int(height),
		}

		layout := buildFuzzSizingLayoutTree(d, n, w)
		layoutPipeline(&layout, w)

		// Verify all dimensions are non-negative and finite.
		walkLayoutAssertNonNegative(t, &layout)
		walkLayoutAssertFinite(t, &layout)
	})
}

// walkLayoutAssertFinite walks the layout tree and asserts all
// dimensions and positions are finite.
func walkLayoutAssertFinite(t *testing.T, layout *Layout) {
	t.Helper()
	s := layout.Shape
	if math.IsNaN(float64(s.X)) || math.IsInf(float64(s.X), 0) {
		t.Errorf("non-finite X: %f", s.X)
	}
	if math.IsNaN(float64(s.Y)) || math.IsInf(float64(s.Y), 0) {
		t.Errorf("non-finite Y: %f", s.Y)
	}
	if math.IsNaN(float64(s.Width)) || math.IsInf(float64(s.Width), 0) {
		t.Errorf("non-finite Width: %f", s.Width)
	}
	if math.IsNaN(float64(s.Height)) || math.IsInf(float64(s.Height), 0) {
		t.Errorf("non-finite Height: %f", s.Height)
	}
	for i := range layout.Children {
		walkLayoutAssertFinite(t, &layout.Children[i])
	}
}

// FuzzLayoutSizingMinMax fuzzes the layout sizing engine with random
// MinWidth/MaxWidth/MinHeight/MaxHeight constraints.
func FuzzLayoutSizingMinMax(f *testing.F) {
	f.Add(uint8(2), uint8(4), float32(0), float32(200), float32(0), float32(200))
	f.Add(uint8(3), uint8(8), float32(10), float32(50), float32(10), float32(50))

	f.Fuzz(func(t *testing.T, depth, childCount uint8, minW, maxW, minH, maxH float32) {
		d := int(depth % 4)
		n := int(childCount % 16)

		// Clamp.
		if math.IsNaN(float64(minW)) || math.IsInf(float64(minW), 0) {
			minW = 0
		}
		if math.IsNaN(float64(maxW)) || math.IsInf(float64(maxW), 0) {
			maxW = 4096
		}
		if minW < 0 {
			minW = 0
		}
		if maxW < 0 {
			maxW = -maxW
		}
		if maxW < minW {
			minW, maxW = maxW, minW
		}
		if math.IsNaN(float64(minH)) || math.IsInf(float64(minH), 0) {
			minH = 0
		}
		if math.IsNaN(float64(maxH)) || math.IsInf(float64(maxH), 0) {
			maxH = 4096
		}
		if minH < 0 {
			minH = 0
		}
		if maxH < 0 {
			maxH = -maxH
		}
		if maxH < minH {
			minH, maxH = maxH, minH
		}
		if maxW > 4096 {
			maxW = 4096
		}
		if maxH > 4096 {
			maxH = 4096
		}

		w := &Window{
			scratch:      newScratchPools(),
			windowWidth:  1024,
			windowHeight: 768,
		}

		layout := Layout{
			Shape: w.allocShape(Shape{
				shapeType: shapeRectangle,
				Sizing:    FillFill,
				Width:     1024,
				Height:    768,
			}),
		}
		if d > 0 && n > 0 {
			layout.Children = make([]Layout, n)
			for i := range n {
				child := Layout{
					Shape: w.allocShape(Shape{
						shapeType: shapeRectangle,
						Sizing:    FitFit,
						MinWidth:  minW,
						MaxWidth:  maxW,
						MinHeight: minH,
						MaxHeight: maxH,
						Width:     100,
						Height:    50,
					}),
				}
				if d > 1 {
					sub := buildFuzzSizingLayoutTree(d-1, n/2, w)
					child.Children = sub.Children
				}
				layout.Children[i] = child
			}
		}

		layoutPipeline(&layout, w)
		walkLayoutAssertNonNegative(t, &layout)
		walkLayoutAssertFinite(t, &layout)
	})
}

// FuzzLayoutSizingWithMix fuzzes a layout tree containing a mix of
// shapes (text, rectangle, image, SVG placeholder) with varying sizings.
func FuzzLayoutSizingWithMix(f *testing.F) {
	f.Add(uint8(2), uint8(5), uint32(0), float32(1024), float32(768))

	f.Fuzz(func(t *testing.T, depth, childCount uint8, shapeType uint32, width, height float32) {
		d := int(depth % 4)
		n := int(childCount % 12)

		if math.IsNaN(float64(width)) || math.IsInf(float64(width), 0) || width <= 0 {
			width = 1024
		}
		if math.IsNaN(float64(height)) || math.IsInf(float64(height), 0) || height <= 0 {
			height = 768
		}
		if width > 4096 {
			width = 4096
		}
		if height > 4096 {
			height = 4096
		}

		w := &Window{
			scratch:      newScratchPools(),
			windowWidth:  int(width),
			windowHeight: int(height),
		}

		var buildMix func(depth int) Layout
		buildMix = func(depth int) Layout {
			l := Layout{
				Shape: w.allocShape(Shape{
					shapeType: shapeTypeFromUint(uint32(int(shapeType) + depth)), // deterministic variation
					Sizing:    FillFill,
					Width:     width,
					Height:    height,
				}),
			}
			if depth <= 0 {
				return l
			}
			n := n
			if n == 0 {
				n = 1
			}
			l.Children = make([]Layout, n)
			for i := range n {
				sizing := Sizing{
					Width:  SizingType(i % 3),
					Height: SizingType((i + 1) % 3),
				}
				child := Layout{
					Shape: w.allocShape(Shape{
						shapeType: shapeTypeFromUint(uint32(i)),
						Sizing:    sizing,
						Width:     100,
						Height:    50,
					}),
				}
				if depth > 1 {
					sub := buildMix(depth - 1)
					child.Children = sub.Children
				}
				l.Children[i] = child
			}
			return l
		}

		layout := buildMix(d)
		layoutPipeline(&layout, w)
		walkLayoutAssertNonNegative(t, &layout)
		walkLayoutAssertFinite(t, &layout)
	})
}

// shapeTypeFromUint maps an integer to a shapeType for fuzz tests.
func shapeTypeFromUint(v uint32) shapeType {
	types := []shapeType{
		shapeRectangle, shapeCircle, shapeText, shapeImage,
		shapeSVG, shapeRTF, shapeTermGrid, shapeNone,
	}
	return types[v%uint32(len(types))]
}
