package gui

// BoundedStack is a stack with maximum size. When full, oldest
// entries are dropped (FIFO eviction).
// exportaudit:keep — reachable from an exported signature
type BoundedStack[T any] struct {
	buf     []T
	head    int
	size    int
	maxSize int
}

// NewBoundedStack creates a BoundedStack with the given max size.
func newBoundedStack[T any](maxSize int) *BoundedStack[T] {
	s := &BoundedStack[T]{maxSize: maxSize}
	if maxSize > 0 {
		s.buf = make([]T, maxSize)
	}
	return s
}

// Push adds element to stack. Drops oldest if at capacity.
func (s *BoundedStack[T]) Push(elem T) {
	if s.maxSize < 1 {
		return
	}
	if s.size < s.maxSize {
		idx := (s.head + s.size) % s.maxSize
		s.buf[idx] = elem
		s.size++
		return
	}
	// Full: overwrite oldest entry and advance head.
	s.buf[s.head] = elem
	s.head = (s.head + 1) % s.maxSize
}

// Pop removes and returns top element. Returns (zero, false) if empty.
func (s *BoundedStack[T]) Pop() (T, bool) {
	if s.size == 0 {
		var zero T
		return zero, false
	}
	idx := (s.head + s.size - 1) % s.maxSize
	elem := s.buf[idx]
	var zero T
	s.buf[idx] = zero
	s.size--
	if s.size == 0 {
		s.head = 0
	}
	return elem, true
}

// Len returns number of elements.
func (s *BoundedStack[T]) Len() int {
	return s.size
}

// IsEmpty returns true if stack has no elements.
func (s *BoundedStack[T]) isEmpty() bool {
	return s.size == 0
}

// Clear removes all elements.
func (s *BoundedStack[T]) Clear() {
	clear(s.buf)
	s.size = 0
	s.head = 0
}
