package gui

import "testing"

func TestBoundedStackPushPop(t *testing.T) {
	s := newBoundedStack[int](3)
	s.Push(1)
	s.Push(2)
	s.Push(3)

	if s.Len() != 3 {
		t.Errorf("len: got %d", s.Len())
	}
	if v, ok := s.Pop(); !ok || v != 3 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 2 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 1 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if !s.isEmpty() {
		t.Error("should be empty")
	}
}

func TestBoundedStackOverflowEvictsOldest(t *testing.T) {
	s := newBoundedStack[int](3)
	s.Push(1)
	s.Push(2)
	s.Push(3)
	s.Push(4) // evicts 1

	if s.Len() != 3 {
		t.Errorf("len: got %d", s.Len())
	}
	if v, ok := s.Pop(); !ok || v != 4 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 3 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 2 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if _, ok := s.Pop(); ok {
		t.Error("should be empty")
	}
}

func TestBoundedStackPopEmpty(t *testing.T) {
	s := newBoundedStack[int](3)
	if _, ok := s.Pop(); ok {
		t.Error("pop on empty should return false")
	}
	if !s.isEmpty() {
		t.Error("should be empty")
	}
}

func TestBoundedStackClear(t *testing.T) {
	s := newBoundedStack[int](3)
	s.Push(1)
	s.Push(2)
	s.Clear()
	if !s.isEmpty() {
		t.Error("should be empty after clear")
	}
	if s.Len() != 0 {
		t.Errorf("len: got %d", s.Len())
	}
}

func TestBoundedStackDefaultSize(t *testing.T) {
	s := &BoundedStack[int]{maxSize: 50}
	if s.maxSize != 50 {
		t.Errorf("maxSize: got %d", s.maxSize)
	}
}

func TestBoundedStackManyPushes(t *testing.T) {
	s := newBoundedStack[int](5)
	for i := range 100 {
		s.Push(i)
	}
	if s.Len() != 5 {
		t.Errorf("len: got %d", s.Len())
	}
	// should have 95,96,97,98,99
	if v, ok := s.Pop(); !ok || v != 99 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 98 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 97 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 96 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 95 {
		t.Errorf("pop: got %d, %v", v, ok)
	}
}

func TestBoundedStackMaxSizeZeroNoPanicNoStore(t *testing.T) {
	s := newBoundedStack[int](0)
	s.Push(1)
	s.Push(2)
	if s.Len() != 0 {
		t.Errorf("len: got %d", s.Len())
	}
	if _, ok := s.Pop(); ok {
		t.Error("pop should return false")
	}
}

func TestBoundedStackMaxSizeNegativeNoPanicNoStore(t *testing.T) {
	s := newBoundedStack[int](-5)
	s.Push(1)
	if s.Len() != 0 {
		t.Errorf("len: got %d", s.Len())
	}
	if _, ok := s.Pop(); ok {
		t.Error("pop should return false")
	}
}

func TestBoundedStackOverflowMaintainsOrderRing(t *testing.T) {
	s := newBoundedStack[int](3)
	for i := 1; i <= 8; i++ {
		s.Push(i)
	}
	want := []int{8, 7, 6}
	for i, exp := range want {
		v, ok := s.Pop()
		if !ok || v != exp {
			t.Fatalf("pop %d: got %d, %v want %d, true", i, v, ok, exp)
		}
	}
	if _, ok := s.Pop(); ok {
		t.Error("stack should be empty")
	}
}

func TestBoundedStackClearThenReuse(t *testing.T) {
	s := newBoundedStack[int](3)
	s.Push(1)
	s.Push(2)
	s.Clear()
	s.Push(10)
	s.Push(11)
	if s.Len() != 2 {
		t.Fatalf("len: got %d, want 2", s.Len())
	}
	if v, ok := s.Pop(); !ok || v != 11 {
		t.Fatalf("pop: got %d, %v want 11, true", v, ok)
	}
	if v, ok := s.Pop(); !ok || v != 10 {
		t.Fatalf("pop: got %d, %v want 10, true", v, ok)
	}
}
