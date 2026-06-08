package gui

import "testing"

func TestNewShapeDefaults(t *testing.T) {
	s := NewShape()
	if s == nil {
		t.Fatal("nil shape")
	}
	if s.UID == 0 {
		t.Error("UID should be nonzero")
	}
	if !f32AreClose(s.Opacity, 1.0) {
		t.Errorf("opacity: got %f, want 1.0", s.Opacity)
	}
}

func TestNewShapeUIDsUnique(t *testing.T) {
	a := NewShape()
	b := NewShape()
	if a.UID == b.UID {
		t.Error("UIDs should be unique")
	}
}

func TestPointInShapeInside(t *testing.T) {
	s := &Shape{shapeClip: drawClip{X: 10, Y: 20, Width: 50, Height: 30}}
	if !s.PointInShape(10, 20) {
		t.Error("top-left corner should be inside")
	}
	if !s.PointInShape(35, 35) {
		t.Error("center should be inside")
	}
}

func TestPointInShapeOutside(t *testing.T) {
	s := &Shape{shapeClip: drawClip{X: 10, Y: 20, Width: 50, Height: 30}}
	if s.PointInShape(9, 25) {
		t.Error("left of shape")
	}
	if s.PointInShape(60, 50) {
		t.Error("right edge exclusive")
	}
	if s.PointInShape(10, 50) {
		t.Error("bottom edge exclusive")
	}
}

func TestPointInShapeZeroSize(t *testing.T) {
	s := &Shape{shapeClip: drawClip{X: 0, Y: 0, Width: 0, Height: 0}}
	if s.PointInShape(0, 0) {
		t.Error("zero-size clip should not contain any point")
	}
}

func TestShapePaddingAccessors(t *testing.T) {
	s := &Shape{
		Padding:    Padding{Top: 2, Right: 3, Bottom: 4, Left: 5},
		SizeBorder: 1,
	}
	if !f32AreClose(s.PaddingLeft(), 6) {
		t.Errorf("PaddingLeft: got %f", s.PaddingLeft())
	}
	if !f32AreClose(s.PaddingTop(), 3) {
		t.Errorf("PaddingTop: got %f", s.PaddingTop())
	}
	if !f32AreClose(s.paddingWidth(), 10) {
		t.Errorf("paddingWidth: got %f", s.paddingWidth())
	}
	if !f32AreClose(s.paddingHeight(), 8) {
		t.Errorf("paddingHeight: got %f", s.paddingHeight())
	}
}

func TestShapeHasEvents(t *testing.T) {
	s := &Shape{}
	if s.hasEvents() {
		t.Error("should be false without events")
	}
	s.events = &eventHandlers{}
	if !s.hasEvents() {
		t.Error("should be true with events")
	}
}
