package gui

import "testing"

func TestHideOverflowChild(t *testing.T) {
	child := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     50,
			Clip:      false,
		},
	}
	hideOverflowChild(&child)
	if child.Shape.shapeType != shapeNone {
		t.Error("shapeType should be shapeNone")
	}
	if child.Shape.Width != 0 {
		t.Error("Width should be 0")
	}
	if !child.Shape.Clip {
		t.Error("Clip should be true")
	}
}

func TestLayoutOverflowSkipsNonOverflow(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	layout := &Layout{
		Shape: &Shape{
			Overflow: false,
			Axis:     axisLeftToRight,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
		},
	}
	layoutOverflow(layout, w)
	// No children should be hidden.
	for i, c := range layout.Children {
		if c.Shape.shapeType == shapeNone {
			t.Errorf("child %d should not be hidden", i)
		}
	}
}

func TestLayoutOverflowSkipsNonLTR(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	layout := &Layout{
		Shape: &Shape{
			Overflow: true,
			Axis:     axisTopToBottom,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
		},
	}
	layoutOverflow(layout, w)
	for i, c := range layout.Children {
		if c.Shape.shapeType == shapeNone {
			t.Errorf("child %d should not be hidden for non-LTR axis", i)
		}
	}
}

func TestLayoutOverflowSkipsTooFewChildren(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	layout := &Layout{
		Shape: &Shape{
			Overflow: true,
			Axis:     axisLeftToRight,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
		},
	}
	layoutOverflow(layout, w)
	if layout.Children[0].Shape.shapeType == shapeNone {
		t.Error("single child should not be hidden")
	}
}

func TestLayoutOverflowSkipsScrollContainer(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	layout := &Layout{
		Shape: &Shape{
			Overflow:   true,
			Axis:       axisLeftToRight,
			Scrollable: true,
			ID:         "1",
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
		},
	}
	layoutOverflow(layout, w)
	for i, c := range layout.Children {
		if c.Shape.shapeType == shapeNone {
			t.Errorf("child %d should not be hidden for scroll container", i)
		}
	}
}

// TestLayoutOverflowSkipsWrapContainer pins issue #380: Wrap and Overflow
// are contradictory strategies for the same condition and wrap wins, so
// layoutOverflow must not hide a child of a Wrap container — not even in
// the single-row case where the axis stayed LTR and the trigger-room check
// would hide one despite the slack.
func TestLayoutOverflowSkipsWrapContainer(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	layout := &Layout{
		Shape: &Shape{
			Overflow: true,
			Wrap:     true,
			Axis:     axisLeftToRight,
			Width:    100,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
		},
	}
	layoutOverflow(layout, w)
	for i, c := range layout.Children {
		if c.Shape.shapeType == shapeNone {
			t.Errorf("child %d should not be hidden on a Wrap container", i)
		}
	}
}

func TestLayoutOverflowHidesExcessChildren(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()

	// Container width=100, 3 visible children + 1 trigger button.
	// Child widths: 40, 40, 40; trigger: 20.
	// Available = 100, spacing = 0.
	// child0(40) + trigger(20) = 60 <= 100 -> visible
	// child0(40) + child1(40) + trigger(20) = 100 <= 100 -> visible
	// child0(40) + child1(40) + child2(40) + trigger(20) = 140 > 100 -> child2 hidden
	layout := &Layout{
		Shape: &Shape{
			ID:       "overflow-test",
			Overflow: true,
			Axis:     axisLeftToRight,
			Width:    100,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 20}}, // trigger
		},
	}
	layoutOverflow(layout, w)

	if layout.Children[0].Shape.shapeType == shapeNone {
		t.Error("child 0 should be visible")
	}
	if layout.Children[1].Shape.shapeType == shapeNone {
		t.Error("child 1 should be visible")
	}
	if layout.Children[2].Shape.shapeType != shapeNone {
		t.Error("child 2 should be hidden")
	}
	// Trigger (last child) should remain visible.
	if layout.Children[3].Shape.shapeType == shapeNone {
		t.Error("trigger child should remain visible")
	}
}

func TestLayoutOverflowAllFit(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()

	// All children fit: trigger should be hidden.
	layout := &Layout{
		Shape: &Shape{
			ID:       "overflow-allfit",
			Overflow: true,
			Axis:     axisLeftToRight,
			Width:    200,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 20}}, // trigger
		},
	}
	layoutOverflow(layout, w)

	if layout.Children[0].Shape.shapeType == shapeNone {
		t.Error("child 0 should be visible")
	}
	if layout.Children[1].Shape.shapeType == shapeNone {
		t.Error("child 1 should be visible")
	}
	// When all fit, trigger gets hidden.
	if layout.Children[2].Shape.shapeType != shapeNone {
		t.Error("trigger should be hidden when all children fit")
	}
}

// TestWrapOverflowSingleRowKeepsChildren is issue #380 end to end: a
// FillFit wrap+overflow container whose content fits one row used to keep
// its LTR axis, after which layoutOverflow hid a child anyway (it reserves
// room for the trigger button). With wrap winning, every child stays
// visible and the overflow map never records the container.
func TestWrapOverflowSingleRowKeepsChildren(t *testing.T) {
	const (
		parentWidth = 700
		childWidth  = 100
		spacing     = 10
		childCount  = 6 // 6*100 + 5*10 = 650 < 700, comfortably one row
	)

	children := make([]Layout, childCount)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 20, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: parentWidth, Height: 200,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				ID: "wo", Overflow: true, Wrap: true,
				Axis: axisLeftToRight, Sizing: FillFit,
				Spacing: spacing, shapeType: shapeRectangle,
			},
			Children: children,
		}},
	}

	w := NewWindow(WindowCfg{})
	defer w.Close()

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, w)
	layoutOverflow(root, w)

	wo := &root.Children[0]
	if wo.Shape.Axis != axisLeftToRight {
		t.Error("single-row wrap should keep its LTR axis")
	}
	for i := range wo.Children {
		if wo.Children[i].Shape.shapeType == shapeNone {
			t.Errorf("child %d should not be hidden", i)
		}
	}
	if _, ok := w.overflow().Get("wo"); ok {
		t.Error("overflow map must not record a wrap container")
	}
}

// TestOverflowFitWidthConstrained is the issue #379 Overflow half: a
// Fit-width overflow container used to resolve its width to the full content
// sum, so the hide test never fired. With the constraint it sizes against
// its parent and hides the children that do not fit.
func TestOverflowFitWidthConstrained(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: 100, Height: 100,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				ID:        "overflow-fit",
				Overflow:  true,
				Axis:      axisLeftToRight,
				Sizing:    FitFit,
				shapeType: shapeRectangle,
			},
			Children: []Layout{
				{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
				{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
				{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
				{Shape: &Shape{shapeType: shapeRectangle, Width: 20}}, // trigger
			},
		}},
	}

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutOverflow(root, w)

	of := &root.Children[0]
	if !f32AreClose(of.Shape.Width, 100) {
		t.Errorf("overflow width: got %f, want 100", of.Shape.Width)
	}
	// Available = 100: child0 (40) + child1 (40) + trigger (20) fits;
	// adding child2 (40) would reach 140 > 100, so child2 is hidden.
	if of.Children[0].Shape.shapeType == shapeNone {
		t.Error("child 0 should be visible")
	}
	if of.Children[1].Shape.shapeType == shapeNone {
		t.Error("child 1 should be visible")
	}
	if of.Children[2].Shape.shapeType != shapeNone {
		t.Error("child 2 should be hidden")
	}
	if of.Children[3].Shape.shapeType == shapeNone {
		t.Error("trigger child should remain visible")
	}
}
