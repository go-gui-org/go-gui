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
			Axis:     AxisLeftToRight,
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
			Axis:     AxisTopToBottom,
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
			Axis:     AxisLeftToRight,
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
			Axis:       AxisLeftToRight,
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
			Axis:     AxisLeftToRight,
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
			Axis:     AxisLeftToRight,
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
