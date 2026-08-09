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

// FindLayoutByFocusID recursively searches for a layout with matching focus ID.
//
// id is an effective ID (see gui/id_resolve.go): the full path a widget
// resolves to under its ID-bearing ancestors, which is what the focus
// store holds.
func FindLayoutByFocusID(layout *Layout, id string) (*Layout, bool) {
	if id != "" && layout.Shape.Focusable && layout.Shape.idKey() == id {
		return layout, true
	}
	for i := range layout.Children {
		if ly, ok := FindLayoutByFocusID(&layout.Children[i], id); ok {
			return ly, true
		}
	}
	return nil, false
}

// FindLayoutByScrollID recursively searches for a Scrollable layout
// with matching scroll ID. An empty id never matches. id is an
// effective ID, as in [FindLayoutByFocusID].
func FindLayoutByScrollID(layout *Layout, id string) (*Layout, bool) {
	if id != "" && layout.Shape.Scrollable && layout.Shape.idKey() == id {
		return layout, true
	}
	for i := range layout.Children {
		if ly, ok := FindLayoutByScrollID(&layout.Children[i], id); ok {
			return ly, true
		}
	}
	return nil, false
}

// FindByID searches the layout tree for a layout with the given ID.
// An empty id never matches — a widget without an ID cannot be
// addressed — matching the id != "" guards in FindLayoutByScrollID and
// FindLayoutByFocusID.
//
// id is the widget's effective ID: a leaf under an ID-bearing ancestor
// is addressed by its full path ("settings:name"), not by the leaf its
// Cfg was written with. See gui/id_resolve.go.
// A nil Shape means the layout tree has not been built yet (no frame
// has been laid out), so there is nothing to find. Guarding here rather
// than in each caller matches findScrollLayout, which already treats a
// nil root Shape as "not found".
func (layout *Layout) FindByID(id string) (*Layout, bool) {
	if id == "" {
		return nil, false
	}
	if layout.Shape == nil {
		return nil, false
	}
	if layout.Shape.idKey() == id {
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
	id    string
}

func collectFocusCandidates(layout *Layout, candidates *[]focusCandidate, seen map[string]struct{}) {
	s := layout.Shape
	if s.Focusable && !s.FocusSkip && !s.Disabled && s.ID != "" {
		// Candidates carry the effective ID: it is what the focus store
		// holds, so it is what focusFindNext/Previous compare against.
		//
		// A duplicate ID collapses to a single tab stop; the extra
		// widget is skipped. Reported by the debug gate's duplicate-ID
		// check (debug.go), which sees every ID, not only focusable ones.
		key := s.idKey()
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			*candidates = append(*candidates, focusCandidate{
				id:    key,
				shape: s,
			})
		}
	}
	for i := range layout.Children {
		collectFocusCandidates(&layout.Children[i], candidates, seen)
	}
}

// focusFindNext returns the candidate after the one whose id equals
// focusID, in DFS (tab) order, wrapping to the first. When focusID
// is not among the candidates, returns the first.
func focusFindNext(candidates []focusCandidate, focusID string) (*Shape, bool) {
	if len(candidates) == 0 {
		return nil, false
	}
	for i, c := range candidates {
		if c.id == focusID {
			return candidates[(i+1)%len(candidates)].shape, true
		}
	}
	return candidates[0].shape, true
}

// focusFindPrevious returns the candidate before the one whose id
// equals focusID, in DFS (tab) order, wrapping to the last. When
// focusID is not among the candidates, returns the last.
func focusFindPrevious(candidates []focusCandidate, focusID string) (*Shape, bool) {
	if len(candidates) == 0 {
		return nil, false
	}
	for i, c := range candidates {
		if c.id == focusID {
			return candidates[(i-1+len(candidates))%len(candidates)].shape, true
		}
	}
	return candidates[len(candidates)-1].shape, true
}

type focusFinder func([]focusCandidate, string) (*Shape, bool)

func (layout *Layout) findFocusable(w *Window, find focusFinder) (*Shape, bool) {
	var candidates []focusCandidate
	var seen map[string]struct{}
	var focusID string
	if w != nil {
		candidates = w.scratch.focusCandidates.take(0)
		defer func() { w.scratch.focusCandidates.put(candidates) }()
		seen = w.scratch.focusSeen.take(len(candidates))
		defer func() { w.scratch.focusSeen.put(seen) }()
		focusID = w.viewState.focusID
	} else {
		seen = make(map[string]struct{})
	}
	collectFocusCandidates(layout, &candidates, seen)
	if len(candidates) == 0 {
		return nil, false
	}
	return find(candidates, focusID)
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
// Returns (drawClip, false) if no intersection.
func rectIntersection(a, b drawClip) (drawClip, bool) {
	x1 := f32Max(a.X, b.X)
	y1 := f32Max(a.Y, b.Y)
	x2 := f32Min(a.X+a.Width, b.X+b.Width)
	y2 := f32Min(a.Y+a.Height, b.Y+b.Height)

	if x2 > x1 && y2 > y1 {
		return drawClip{
			X:      x1,
			Y:      y1,
			Width:  x2 - x1,
			Height: y2 - y1,
		}, true
	}
	return drawClip{}, false
}

// PointInRectangle returns true if point is within bounds of rectangle.
func PointInRectangle(x, y float32, rect drawClip) bool {
	return x >= rect.X && y >= rect.Y &&
		x < (rect.X+rect.Width) && y < (rect.Y+rect.Height)
}
