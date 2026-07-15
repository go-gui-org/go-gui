package gui

import (
	"math"
	"testing"
)

func TestOffsetMouseChangeX(t *testing.T) {
	w := &Window{}
	// Layout: 100 wide, content 400 wide (axis LTR so contentWidth sums children).
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 50}}
	layout := &Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "1",
			Width:      100,
			Height:     50,
			Axis:       AxisLeftToRight,
		},
		Children: []Layout{child},
	}

	offset := offsetMouseChangeX(w.scrollX(), layout, 10, "1")
	// ratio = 400/100 = 4, newOffset = 10*4 = 40, offset = 0 - 40 = -40
	// clamped: min(0, max(-40, 100-400)) = min(0, max(-40, -300)) = min(0, -40) = -40
	if offset != -40 {
		t.Errorf("expected -40, got %v", offset)
	}
}

func TestOffsetMouseChangeY(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 500}}
	layout := &Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "2",
			Width:      50,
			Height:     100,
			Axis:       AxisTopToBottom,
		},
		Children: []Layout{child},
	}

	offset := offsetMouseChangeY(w.scrollY(), layout, 5, "2")
	// ratio = 500/100 = 5, newOffset = 5*5 = 25, offset = 0 - 25 = -25
	// clamped: min(0, max(-25, 100-500)) = -25
	if offset != -25 {
		t.Errorf("expected -25, got %v", offset)
	}
}

func TestOffsetFromMouseY(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "3",
			Width:      50,
			Height:     100,
			Axis:       AxisTopToBottom,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseY=50 → percent=50/100=0.5 → offset = -0.5*(400-100) = -150
	offsetFromMouseY(root, 50, "3", w)
	sy := w.scrollY()
	v, _ := sy.Get("3")
	if v != -150 {
		t.Errorf("expected -150, got %v", v)
	}
}

func TestOffsetFromMouseX(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 300, Height: 50}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "4",
			Width:      100,
			Height:     50,
			Axis:       AxisLeftToRight,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseX=100 → percent=100/100=1.0 → snap to 1
	// offset = -1*(300-100) = -200
	offsetFromMouseX(root, 100, "4", w)
	sx := w.scrollX()
	v, _ := sx.Get("4")
	if v != -200 {
		t.Errorf("expected -200, got %v", v)
	}
}

func TestOffsetFromMouseYSnap(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "5",
			Width:      50,
			Height:     100,
			Axis:       AxisTopToBottom,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseY=2 → percent=0.02 → below snapMin(0.03) → snaps to 0
	offsetFromMouseY(root, 2, "5", w)
	sy := w.scrollY()
	v, _ := sy.Get("5")
	if v != 0 {
		t.Errorf("expected 0 (snap to start), got %v", v)
	}

	// mouseY=98 → percent=0.98 → above snapMax(0.97) → snaps to 1
	offsetFromMouseY(root, 98, "5", w)
	v, _ = sy.Get("5")
	// -1*(400-100) = -300
	if v != -300 {
		t.Errorf("expected -300 (snap to end), got %v", v)
	}
}

func TestScrollbarMouseMoveVertical(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "6",
			Width:      50,
			Height:     100,
			Y:          10,
			Axis:       AxisTopToBottom,
		},
		Children: []Layout{child},
	}
	root := Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	e := &Event{MouseY: 50, MouseDY: 5}
	scrollbarMouseMove(ScrollbarVertical, "6", &root, e, w)
	sy := w.scrollY()
	v, _ := sy.Get("6")
	// ratio=400/100=4, newOffset=5*4=20, offset=0-20=-20
	if v != -20 {
		t.Errorf("expected -20, got %v", v)
	}
}

func TestScrollbarMouseMoveHorizontal(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 300, Height: 50}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "7",
			Width:      100,
			Height:     50,
			X:          0,
			Axis:       AxisLeftToRight,
		},
		Children: []Layout{child},
	}
	root := Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	e := &Event{MouseX: 50, MouseDX: 10}
	scrollbarMouseMove(ScrollbarHorizontal, "7", &root, e, w)
	sx := w.scrollX()
	v, _ := sx.Get("7")
	// ratio=300/100=3, newOffset=10*3=30, offset=0-30=-30
	if v != -30 {
		t.Errorf("expected -30, got %v", v)
	}
}

func TestThumbOnClickLocksAndUnlocks(t *testing.T) {
	w := &Window{}
	e := &Event{}
	handler := makeScrollbarOnMouseDown(ScrollbarCfg{
		Orientation: ScrollbarVertical,
		ID:          "1",
		ScrollID:    "1",
	})
	handler(nil, e, w)
	if !w.MouseIsLocked() {
		t.Error("expected mouse locked after thumb click")
	}
	if !e.IsHandled {
		t.Error("expected event handled")
	}

	// Simulate mouse up.
	w.viewState.mouseLock.MouseUp(nil, e, w)
	if w.MouseIsLocked() {
		t.Error("expected mouse unlocked after mouse up")
	}
}

func TestOffsetFromMouseXWithNonZeroOrigin(t *testing.T) {
	w := &Window{}
	child := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 300, Height: 50},
	}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "20",
			Width:      100,
			Height:     50,
			X:          50,
			Axis:       AxisLeftToRight,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseX=100 is at 50% of the scrollbar (origin 50, width 100).
	// percent = (100-50)/100 = 0.5
	// offset = -0.5*(300-100) = -100
	offsetFromMouseX(root, 100, "20", w)
	sx := w.scrollX()
	v, _ := sx.Get("20")
	if v != -100 {
		t.Errorf("expected -100, got %v", v)
	}
}

func TestOffsetFromMouseYWithNonZeroOrigin(t *testing.T) {
	w := &Window{}
	child := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400},
	}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "21",
			Width:      50,
			Height:     100,
			Y:          200,
			Axis:       AxisTopToBottom,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseY=250 is at 50% of the scrollbar (origin 200, height 100).
	// percent = (250-200)/100 = 0.5
	// offset = -0.5*(400-100) = -150
	offsetFromMouseY(root, 250, "21", w)
	sy := w.scrollY()
	v, _ := sy.Get("21")
	if v != -150 {
		t.Errorf("expected -150, got %v", v)
	}
}

func TestGutterClickSetsOffsetAndLocks(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "8",
			Width:      50,
			Height:     100,
			Axis:       AxisTopToBottom,
		},
		Children: []Layout{child},
	}
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	e := &Event{MouseY: 50}
	handler := makeScrollbarGutterClick(ScrollbarCfg{
		Orientation: ScrollbarVertical,
		ID:          "8",
		ScrollID:    "8",
	})
	handler(nil, e, w)

	sy := w.scrollY()
	v, _ := sy.Get("8")
	// percent=50/100=0.5, offset = -0.5*(400-100) = -150
	if math.Abs(float64(v+150)) > 0.01 {
		t.Errorf("expected -150, got %v", v)
	}
	if !w.MouseIsLocked() {
		t.Error("expected mouse locked after gutter click")
	}
	if !e.IsHandled {
		t.Error("expected event handled")
	}
}
