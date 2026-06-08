package gui

import "testing"

func TestCollectFocusCandidatesDedupes(t *testing.T) {
	s1 := &Shape{IDFocus: 9}
	s2 := &Shape{IDFocus: 9}
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: s1},
			{Shape: s2},
		},
	}
	var candidates []focusCandidate
	seen := make(map[uint32]struct{})
	collectFocusCandidates(root, &candidates, seen)
	if len(candidates) != 1 {
		t.Fatalf("candidates: got %d, want 1", len(candidates))
	}
	if candidates[0].id != 9 {
		t.Errorf("id: got %d, want 9", candidates[0].id)
	}
}

func TestFocusFindNextByID(t *testing.T) {
	s1 := &Shape{IDFocus: 30}
	s2 := &Shape{IDFocus: 10}
	s3 := &Shape{IDFocus: 40}
	candidates := []focusCandidate{
		{id: 30, shape: s1},
		{id: 10, shape: s2},
		{id: 40, shape: s3},
	}
	next, ok := focusFindNext(candidates, 20)
	if !ok {
		t.Fatal("missing next focus")
	}
	if next.IDFocus != 30 {
		t.Errorf("next: got %d, want 30", next.IDFocus)
	}
	fallback, ok := focusFindNext(candidates, 99)
	if !ok {
		t.Fatal("missing fallback")
	}
	if fallback.IDFocus != 10 {
		t.Errorf("fallback: got %d, want 10", fallback.IDFocus)
	}
}

func TestNextFocusable(t *testing.T) {
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{IDFocus: 10}},
			{Shape: &Shape{IDFocus: 20}},
			{Shape: &Shape{IDFocus: 30}},
		},
	}
	w := &Window{}
	w.viewState.idFocus = 10

	s, ok := root.NextFocusable(w)
	if !ok || s.IDFocus != 20 {
		t.Errorf("next from 10: got %v, want 20", s)
	}

	w.viewState.idFocus = 30
	s, ok = root.NextFocusable(w)
	if !ok || s.IDFocus != 10 {
		t.Errorf("wrap from 30: got %v, want 10", s)
	}
}

func TestPreviousFocusable(t *testing.T) {
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{IDFocus: 10}},
			{Shape: &Shape{IDFocus: 20}},
			{Shape: &Shape{IDFocus: 30}},
		},
	}
	w := &Window{}
	w.viewState.idFocus = 20

	s, ok := root.PreviousFocusable(w)
	if !ok || s.IDFocus != 10 {
		t.Errorf("prev from 20: got %v, want 10", s)
	}

	w.viewState.idFocus = 10
	s, ok = root.PreviousFocusable(w)
	if !ok || s.IDFocus != 30 {
		t.Errorf("wrap from 10: got %v, want 30", s)
	}
}

func TestFocusableSkipsDisabledAndFocusSkip(t *testing.T) {
	root := &Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{IDFocus: 10}},
			{Shape: &Shape{IDFocus: 20, Disabled: true}},
			{Shape: &Shape{IDFocus: 30, FocusSkip: true}},
			{Shape: &Shape{IDFocus: 40}},
		},
	}
	w := &Window{}
	w.viewState.idFocus = 10

	s, ok := root.NextFocusable(w)
	if !ok || s.IDFocus != 40 {
		t.Errorf("next from 10 skipping disabled/focusskip: got %v, want 40", s)
	}
}

func TestFocusableEmpty(t *testing.T) {
	root := &Layout{Shape: &Shape{}}
	w := &Window{}

	_, ok := root.NextFocusable(w)
	if ok {
		t.Error("empty should return false")
	}
	_, ok = root.PreviousFocusable(w)
	if ok {
		t.Error("empty should return false")
	}
}

func TestFocusFindPreviousByID(t *testing.T) {
	s1 := &Shape{IDFocus: 30}
	s2 := &Shape{IDFocus: 10}
	s3 := &Shape{IDFocus: 40}
	candidates := []focusCandidate{
		{id: 30, shape: s1},
		{id: 10, shape: s2},
		{id: 40, shape: s3},
	}
	prev, ok := focusFindPrevious(candidates, 35)
	if !ok {
		t.Fatal("missing previous")
	}
	if prev.IDFocus != 30 {
		t.Errorf("prev: got %d, want 30", prev.IDFocus)
	}
	fallback, ok := focusFindPrevious(candidates, 1)
	if !ok {
		t.Fatal("missing fallback")
	}
	if fallback.IDFocus != 40 {
		t.Errorf("fallback: got %d, want 40", fallback.IDFocus)
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
	s, ok := root.FindShape(func(l Layout) bool {
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
	_, ok := root.FindShape(func(l Layout) bool {
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
		got := PointInRectangle(tt.x, tt.y, rect)
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
			{Shape: &Shape{IDFocus: 10}},
			{Shape: &Shape{IDFocus: 20}},
			{Shape: &Shape{IDFocus: 30}},
		},
	}

	next, ok := root.NextFocusable(nil)
	if !ok || next.IDFocus != 10 {
		t.Fatalf("next nil window: got %v, want IDFocus 10", next)
	}

	prev, ok := root.PreviousFocusable(nil)
	if !ok || prev.IDFocus != 30 {
		t.Fatalf("prev nil window: got %v, want IDFocus 30", prev)
	}
}
