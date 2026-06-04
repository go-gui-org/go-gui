package gui

// FindShape walks the layout depth-first until predicate is satisfied.
func (layout *Layout) FindShape(predicate func(Layout) bool) (*Shape, bool) {
	for i := range layout.Children {
		if s, ok := layout.Children[i].FindShape(predicate); ok {
			return s, true
		}
	}
	if predicate(*layout) {
		return layout.Shape, true
	}
	return nil, false
}

// FindLayout walks the layout depth-first until predicate is satisfied.
func (layout *Layout) FindLayout(predicate func(Layout) bool) (*Layout, bool) {
	for i := range layout.Children {
		if l, ok := layout.Children[i].FindLayout(predicate); ok {
			return l, true
		}
	}
	if predicate(*layout) {
		return layout, true
	}
	return nil, false
}

// FindLayoutByIDFocus recursively searches for a layout with matching IDFocus.
func FindLayoutByIDFocus(layout *Layout, idFocus uint32) (*Layout, bool) {
	if layout.Shape.IDFocus == idFocus {
		return layout, true
	}
	for i := range layout.Children {
		if ly, ok := FindLayoutByIDFocus(&layout.Children[i], idFocus); ok {
			return ly, true
		}
	}
	return nil, false
}

// FindLayoutByIDScroll recursively searches for a layout with matching IDScroll.
func FindLayoutByIDScroll(layout *Layout, idScroll uint32) (*Layout, bool) {
	if layout.Shape.IDScroll == idScroll {
		return layout, true
	}
	for i := range layout.Children {
		if ly, ok := FindLayoutByIDScroll(&layout.Children[i], idScroll); ok {
			return ly, true
		}
	}
	return nil, false
}

// FindByID searches the layout tree for a layout with the given ID.
func (layout *Layout) FindByID(id string) (*Layout, bool) {
	if layout.Shape.ID == id {
		return layout, true
	}
	for i := range layout.Children {
		if res, ok := layout.Children[i].FindByID(id); ok {
			return res, true
		}
	}
	return nil, false
}

type focusCandidate struct {
	shape *Shape
	id    uint32
}

func collectFocusCandidates(layout *Layout, candidates *[]focusCandidate, seen map[uint32]struct{}) {
	if layout.Shape.IDFocus > 0 && !layout.Shape.FocusSkip {
		if !layout.Shape.Disabled {
			if _, ok := seen[layout.Shape.IDFocus]; !ok {
				seen[layout.Shape.IDFocus] = struct{}{}
				*candidates = append(*candidates, focusCandidate{
					id:    layout.Shape.IDFocus,
					shape: layout.Shape,
				})
			}
		}
	}
	for i := range layout.Children {
		collectFocusCandidates(&layout.Children[i], candidates, seen)
	}
}

func focusFindNext(candidates []focusCandidate, idFocus uint32) (*Shape, bool) {
	var minID uint32 = 0xffffffff
	var minShape *Shape
	var nextID uint32 = 0xffffffff
	var nextShape *Shape
	for _, c := range candidates {
		if c.id < minID {
			minID = c.id
			minShape = c.shape
		}
		if idFocus > 0 && c.id > idFocus && c.id < nextID {
			nextID = c.id
			nextShape = c.shape
		}
	}
	if nextShape != nil {
		return nextShape, true
	}
	if minShape != nil {
		return minShape, true
	}
	return nil, false
}

func focusFindPrevious(candidates []focusCandidate, idFocus uint32) (*Shape, bool) {
	var maxID uint32
	var maxShape *Shape
	var prevID uint32
	var prevShape *Shape
	for _, c := range candidates {
		if maxShape == nil || c.id > maxID {
			maxID = c.id
			maxShape = c.shape
		}
		if idFocus > 0 && c.id < idFocus &&
			(prevShape == nil || c.id > prevID) {
			prevID = c.id
			prevShape = c.shape
		}
	}
	if prevShape != nil {
		return prevShape, true
	}
	if maxShape != nil {
		return maxShape, true
	}
	return nil, false
}

type focusFinder func([]focusCandidate, uint32) (*Shape, bool)

func (layout *Layout) findFocusable(w *Window, find focusFinder) (*Shape, bool) {
	var candidates []focusCandidate
	var seen map[uint32]struct{}
	var idFocus uint32
	if w != nil {
		candidates = w.scratch.focusCandidates.take(0)
		defer func() { w.scratch.focusCandidates.put(candidates) }()
		seen = w.scratch.focusSeen.take(len(candidates))
		defer func() { w.scratch.focusSeen.put(seen) }()
		idFocus = w.viewState.idFocus
	} else {
		seen = make(map[uint32]struct{})
	}
	collectFocusCandidates(layout, &candidates, seen)
	if len(candidates) == 0 {
		return nil, false
	}
	return find(candidates, idFocus)
}

// NextFocusable returns the next focusable shape after the
// current focus. Wraps to first if at end.
func (layout *Layout) NextFocusable(w *Window) (*Shape, bool) {
	return layout.findFocusable(w, focusFindNext)
}

// PreviousFocusable returns the previous focusable shape before
// the current focus. Wraps to last if at beginning.
func (layout *Layout) PreviousFocusable(w *Window) (*Shape, bool) {
	return layout.findFocusable(w, focusFindPrevious)
}

// rectIntersection returns the intersection of two rectangles.
// Returns (DrawClip, false) if no intersection.
func rectIntersection(a, b DrawClip) (DrawClip, bool) {
	x1 := f32Max(a.X, b.X)
	y1 := f32Max(a.Y, b.Y)
	x2 := f32Min(a.X+a.Width, b.X+b.Width)
	y2 := f32Min(a.Y+a.Height, b.Y+b.Height)

	if x2 > x1 && y2 > y1 {
		return DrawClip{
			X:      x1,
			Y:      y1,
			Width:  x2 - x1,
			Height: y2 - y1,
		}, true
	}
	return DrawClip{}, false
}

// PointInRectangle returns true if point is within bounds of rectangle.
func PointInRectangle(x, y float32, rect DrawClip) bool {
	return x >= rect.X && y >= rect.Y &&
		x < (rect.X+rect.Width) && y < (rect.Y+rect.Height)
}
