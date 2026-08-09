package gui

import "testing"

func TestCollectFocusCandidatesDedupes(t *testing.T) {
	s1 := &Shape{Focusable: true, ID: "f9"}
	s2 := &Shape{Focusable: true, ID: "f9"}
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: s1},
			{Shape: s2},
		},
	}
	var candidates []focusCandidate
	seen := make(map[string]struct{})
	collectFocusCandidates(root, &candidates, seen)
	if len(candidates) != 1 {
		t.Fatalf("candidates: got %d, want 1", len(candidates))
	}
	if candidates[0].id != "f9" {
		t.Errorf("id: got %q, want f9", candidates[0].id)
	}
}

func TestFocusFindNextByID(t *testing.T) {
	s1 := &Shape{Focusable: true, ID: "f30"}
	s2 := &Shape{Focusable: true, ID: "f10"}
	s3 := &Shape{Focusable: true, ID: "f40"}
	candidates := []focusCandidate{
		{id: "f30", shape: s1},
		{id: "f10", shape: s2},
		{id: "f40", shape: s3},
	}
	// Positional: the entry after f30 in DFS order is f10.
	next, ok := focusFindNext(candidates, "f30")
	if !ok {
		t.Fatal("missing next focus")
	}
	if next.ID != "f10" {
		t.Errorf("next: got %q, want f10", next.ID)
	}
	// Unknown focus falls back to the first candidate.
	fallback, ok := focusFindNext(candidates, "nope")
	if !ok {
		t.Fatal("missing fallback")
	}
	if fallback.ID != "f30" {
		t.Errorf("fallback: got %q, want f30", fallback.ID)
	}
}

func TestNextFocusable(t *testing.T) {
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{Focusable: true, ID: "f10"}},
			{Shape: &Shape{Focusable: true, ID: "f20"}},
			{Shape: &Shape{Focusable: true, ID: "f30"}},
		},
	}
	w := &Window{}
	w.viewState.focusID = "f10"

	s, ok := root.nextFocusable(w)
	if !ok || s.ID != "f20" {
		t.Errorf("next from 10: got %v, want 20", s)
	}

	w.viewState.focusID = "f30"
	s, ok = root.nextFocusable(w)
	if !ok || s.ID != "f10" {
		t.Errorf("wrap from 30: got %v, want 10", s)
	}
}

func TestPreviousFocusable(t *testing.T) {
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{Focusable: true, ID: "f10"}},
			{Shape: &Shape{Focusable: true, ID: "f20"}},
			{Shape: &Shape{Focusable: true, ID: "f30"}},
		},
	}
	w := &Window{}
	w.viewState.focusID = "f20"

	s, ok := root.previousFocusable(w)
	if !ok || s.ID != "f10" {
		t.Errorf("prev from 20: got %v, want 10", s)
	}

	w.viewState.focusID = "f10"
	s, ok = root.previousFocusable(w)
	if !ok || s.ID != "f30" {
		t.Errorf("wrap from 10: got %v, want 30", s)
	}
}

func TestFocusableSkipsDisabledAndFocusSkip(t *testing.T) {
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{Focusable: true, ID: "f10"}},
			{Shape: &Shape{Focusable: true, ID: "f20", Disabled: true}},
			{Shape: &Shape{Focusable: true, ID: "f30", FocusSkip: true}},
			{Shape: &Shape{Focusable: true, ID: "f40"}},
		},
	}
	w := &Window{}
	w.viewState.focusID = "f10"

	s, ok := root.nextFocusable(w)
	if !ok || s.ID != "f40" {
		t.Errorf("next from 10 skipping disabled/focusskip: got %v, want 40", s)
	}
}

func TestFocusableEmpty(t *testing.T) {
	root := &Layout{Shape: &Shape{}}
	w := &Window{}

	_, ok := root.nextFocusable(w)
	if ok {
		t.Error("empty should return false")
	}
	_, ok = root.previousFocusable(w)
	if ok {
		t.Error("empty should return false")
	}
}

func TestFocusFindPreviousByID(t *testing.T) {
	s1 := &Shape{Focusable: true, ID: "f30"}
	s2 := &Shape{Focusable: true, ID: "f10"}
	s3 := &Shape{Focusable: true, ID: "f40"}
	candidates := []focusCandidate{
		{id: "f30", shape: s1},
		{id: "f10", shape: s2},
		{id: "f40", shape: s3},
	}
	// Positional: the entry before f10 in DFS order is f30.
	prev, ok := focusFindPrevious(candidates, "f10")
	if !ok {
		t.Fatal("missing previous")
	}
	if prev.ID != "f30" {
		t.Errorf("prev: got %q, want f30", prev.ID)
	}
	// Unknown focus falls back to the last candidate.
	fallback, ok := focusFindPrevious(candidates, "nope")
	if !ok {
		t.Fatal("missing fallback")
	}
	if fallback.ID != "f40" {
		t.Errorf("fallback: got %q, want f40", fallback.ID)
	}
}

func TestFindShapeFound(t *testing.T) {
	root := &Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{Shape: &Shape{ID: "child1"}},
			{Shape: &Shape{ID: "child2"}},
		},
	}
	s, ok := root.findShape(func(l Layout) bool {
		return l.Shape.ID == "child2"
	})
	if !ok {
		t.Fatal("should find child2")
	}
	if s.ID != "child2" {
		t.Errorf("ID = %q, want child2", s.ID)
	}
}

func TestFindShapeNotFound(t *testing.T) {
	root := &Layout{
		Shape: &Shape{ID: "root"},
	}
	_, ok := root.findShape(func(l Layout) bool {
		return l.Shape.ID == "missing"
	})
	if ok {
		t.Error("should not find missing shape")
	}
}

func TestPointInRectangle(t *testing.T) {
	rect := drawClip{X: 10, Y: 10, Width: 100, Height: 50}
	tests := []struct {
		x, y float32
		want bool
	}{
		{50, 30, true},   // inside
		{10, 10, true},   // top-left corner
		{109, 59, true},  // just inside bottom-right
		{110, 60, false}, // at edge (exclusive)
		{5, 30, false},   // left of rect
		{50, 5, false},   // above rect
	}
	for _, tt := range tests {
		got := pointInRectangle(tt.x, tt.y, rect)
		if got != tt.want {
			t.Errorf("PointInRectangle(%f,%f) = %v, want %v",
				tt.x, tt.y, got, tt.want)
		}
	}
}

func TestNextPreviousFocusableNilWindow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{Focusable: true, ID: "f10"}},
			{Shape: &Shape{Focusable: true, ID: "f20"}},
			{Shape: &Shape{Focusable: true, ID: "f30"}},
		},
	}

	next, ok := root.nextFocusable(nil)
	if !ok || next.ID != "f10" {
		t.Fatalf("next nil window: got %v, want ID f10", next)
	}

	prev, ok := root.previousFocusable(nil)
	if !ok || prev.ID != "f30" {
		t.Fatalf("prev nil window: got %v, want ID f30", prev)
	}
}

func TestFindByIDFound(t *testing.T) {
	root := &Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{Shape: &Shape{ID: "child1"}},
			{Shape: &Shape{ID: "child2"}},
		},
	}
	ly, ok := root.FindByID("child2")
	if !ok {
		t.Fatal("should find child2")
	}
	if ly.Shape.ID != "child2" {
		t.Errorf("ID = %q, want child2", ly.Shape.ID)
	}
}

// A Layout with a nil Shape is what Window.layout holds before the
// first frame is laid out. FindByID must report "not found" rather
// than dereferencing it.
func TestFindByIDNilShape(t *testing.T) {
	root := &Layout{}
	if _, ok := root.FindByID("anything"); ok {
		t.Error("nil Shape should not match")
	}
}

// Children can also be unbuilt while the parent is not; the recursive
// step must survive that too.
func TestFindByIDNilChildShape(t *testing.T) {
	root := &Layout{
		Shape:    &Shape{ID: "root"},
		Children: []Layout{{}},
	}
	if _, ok := root.FindByID("missing"); ok {
		t.Error("should not find missing id")
	}
}

// An unbuilt child must be skipped, not abort the search: siblings
// after it must still be reachable.
func TestFindByIDSkipsNilChildContinuesSearch(t *testing.T) {
	root := &Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{}, // unbuilt subtree
			{Shape: &Shape{ID: "after"}},
		},
	}
	ly, ok := root.FindByID("after")
	if !ok {
		t.Fatal("should find sibling after unbuilt child")
	}
	if ly.Shape.ID != "after" {
		t.Errorf("ID = %q, want after", ly.Shape.ID)
	}
}

// An empty id cannot address a widget, so it must never match — not
// even a shape whose ID is also empty. Mirrors the id != "" guards in
// FindLayoutByScrollID and FindLayoutByFocusID.
func TestFindByIDEmptyIDNeverMatches(t *testing.T) {
	root := &Layout{
		Shape:    &Shape{ID: ""},
		Children: []Layout{{Shape: &Shape{ID: "child"}}},
	}
	if _, ok := root.FindByID(""); ok {
		t.Error(`FindByID("") must not match an empty-ID shape`)
	}
}

// ScrollToView is the caller that can reach FindByID before any layout
// pass — a scroll request issued while the overlay owning the target is
// still being built. It must be a no-op, not a panic.
func TestScrollToViewBeforeLayout(t *testing.T) {
	var w Window
	w.scrollToView("some-id")
}
