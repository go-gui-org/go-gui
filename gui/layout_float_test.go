package gui

import (
	"slices"
	"testing"
)

func TestFloatAutoFlipVertical(t *testing.T) {
	// Parent near bottom edge; tooltip below overflows → flip above.
	parent := Layout{
		Shape: &Shape{X: 100, Y: 550, Width: 100, Height: 30},
		Children: []Layout{{Shape: &Shape{
			Float: true, FloatAutoFlip: true,
			Width: 80, Height: 100,
			FloatAnchor: FloatBottomCenter,
			FloatTieOff: FloatTopCenter,
		}}},
	}
	parent.Children[0].Parent = &parent
	win := drawClip{Width: 800, Height: 600}

	x, y := floatAttachLayout(&parent.Children[0], win)
	if !f32AreClose(x, 110) {
		t.Errorf("X: got %f, want 110", x)
	}
	if !f32AreClose(y, 450) {
		t.Errorf("Y: got %f, want 450", y)
	}
}

func TestFloatAutoFlipHorizontal(t *testing.T) {
	// Parent near right edge; tooltip right overflows → flip left.
	parent := Layout{
		Shape: &Shape{X: 750, Y: 200, Width: 40, Height: 30},
		Children: []Layout{{Shape: &Shape{
			Float: true, FloatAutoFlip: true,
			Width: 100, Height: 50,
			FloatAnchor: FloatMiddleRight,
			FloatTieOff: FloatMiddleLeft,
		}}},
	}
	parent.Children[0].Parent = &parent
	win := drawClip{Width: 800, Height: 600}

	x, y := floatAttachLayout(&parent.Children[0], win)
	if !f32AreClose(x, 650) {
		t.Errorf("X: got %f, want 650", x)
	}
	if !f32AreClose(y, 190) {
		t.Errorf("Y: got %f, want 190", y)
	}
}

func TestFloatAutoFlipNoChange(t *testing.T) {
	// Parent centered; tooltip fits → no flip.
	parent := Layout{
		Shape: &Shape{X: 350, Y: 250, Width: 100, Height: 30},
		Children: []Layout{{Shape: &Shape{
			Float: true, FloatAutoFlip: true,
			Width: 80, Height: 40,
			FloatAnchor: FloatBottomCenter,
			FloatTieOff: FloatTopCenter,
		}}},
	}
	parent.Children[0].Parent = &parent
	win := drawClip{Width: 800, Height: 600}

	x, y := floatAttachLayout(&parent.Children[0], win)
	if !f32AreClose(x, 360) {
		t.Errorf("X: got %f, want 360", x)
	}
	if !f32AreClose(y, 280) {
		t.Errorf("Y: got %f, want 280", y)
	}
}

func TestFloatAutoFlipDisabled(t *testing.T) {
	// Same overflow but FloatAutoFlip=false → no flip.
	parent := Layout{
		Shape: &Shape{X: 100, Y: 550, Width: 100, Height: 30},
		Children: []Layout{{Shape: &Shape{
			Float:       true,
			Width:       80,
			Height:      100,
			FloatAnchor: FloatBottomCenter,
			FloatTieOff: FloatTopCenter,
		}}},
	}
	parent.Children[0].Parent = &parent
	win := drawClip{Width: 800, Height: 600}

	x, y := floatAttachLayout(&parent.Children[0], win)
	if !f32AreClose(x, 110) {
		t.Errorf("X: got %f, want 110", x)
	}
	if !f32AreClose(y, 580) {
		t.Errorf("Y: got %f, want 580", y)
	}
}

func TestFloatZIndexSorting(t *testing.T) {
	// Floats with z-index 2,0,1 → sorted 0,1,2.
	floats := []*Layout{
		{Shape: &Shape{FloatZIndex: 2, ID: "a"}},
		{Shape: &Shape{FloatZIndex: 0, ID: "b"}},
		{Shape: &Shape{FloatZIndex: 1, ID: "c"}},
	}
	slices.SortStableFunc(floats, func(a, b *Layout) int {
		return a.Shape.FloatZIndex - b.Shape.FloatZIndex
	})
	if floats[0].Shape.ID != "b" || floats[1].Shape.ID != "c" || floats[2].Shape.ID != "a" {
		t.Errorf("got %s,%s,%s; want b,c,a",
			floats[0].Shape.ID, floats[1].Shape.ID, floats[2].Shape.ID)
	}
}

func TestFloatZIndexStableOrder(t *testing.T) {
	// Same z-index preserves extraction order.
	floats := []*Layout{
		{Shape: &Shape{FloatZIndex: 0, ID: "first"}},
		{Shape: &Shape{FloatZIndex: 0, ID: "second"}},
		{Shape: &Shape{FloatZIndex: 0, ID: "third"}},
	}
	slices.SortStableFunc(floats, func(a, b *Layout) int {
		return a.Shape.FloatZIndex - b.Shape.FloatZIndex
	})
	if floats[0].Shape.ID != "first" || floats[1].Shape.ID != "second" || floats[2].Shape.ID != "third" {
		t.Errorf("got %s,%s,%s; want first,second,third",
			floats[0].Shape.ID, floats[1].Shape.ID, floats[2].Shape.ID)
	}
}

func TestFloatAutoFlipClamp(t *testing.T) {
	// Tooltip taller than window → flip reduces overflow, clamp
	// pins to top edge.
	parent := Layout{
		Shape: &Shape{X: 50, Y: 100, Width: 40, Height: 30},
		Children: []Layout{{Shape: &Shape{
			Float: true, FloatAutoFlip: true,
			Width: 80, Height: 200,
			FloatAnchor: FloatBottomCenter,
			FloatTieOff: FloatTopCenter,
		}}},
	}
	parent.Children[0].Parent = &parent
	win := drawClip{Width: 200, Height: 150}

	x, y := floatAttachLayout(&parent.Children[0], win)
	if !f32AreClose(x, 30) {
		t.Errorf("X: got %f, want 30", x)
	}
	if !f32AreClose(y, 0) {
		t.Errorf("Y: got %f, want 0", y)
	}
}
