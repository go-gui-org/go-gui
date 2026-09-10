package gui

import "testing"

// The layout and render walks share the event walks' depth budget
// (maxEventDepth); see the depth-policy note in layout.go. These tests pin
// that: an ordinary-depth tree is measured normally, and a chain past the
// cap stops being descended instead of recursing until the stack gives out.

// deepRectChain builds a single-child chain of rectangles n levels deep.
// Only the leaf carries a size, so any pass that reaches the leaf shows up
// at the root and any pass that stops short leaves the root at zero.
func deepRectChain(n int) *Layout {
	root := &Layout{Shape: &Shape{
		shapeType: shapeRectangle,
		Axis:      axisTopToBottom,
	}}
	cur := root
	for range n {
		cur.Children = []Layout{{Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      axisTopToBottom,
		}}}
		cur = &cur.Children[0]
	}
	cur.Shape.Width = 50
	cur.Shape.Height = 20
	cur.Shape.ID = "deep"
	return root
}

// leafOf walks to the end of a chain built by deepRectChain.
func leafOf(root *Layout) *Layout {
	cur := root
	for len(cur.Children) > 0 {
		cur = &cur.Children[0]
	}
	return cur
}

func TestLayoutWidthsStopsAtDepthCap(t *testing.T) {
	shallow := deepRectChain(100)
	layoutWidths(shallow)
	if !f32AreClose(shallow.Shape.Width, 50) {
		t.Errorf("width at depth 100: got %f, want 50 (leaf must be reached)",
			shallow.Shape.Width)
	}

	deep := deepRectChain(maxEventDepth + 50)
	layoutWidths(deep)
	if !f32AreClose(deep.Shape.Width, 0) {
		t.Errorf("width past maxEventDepth (%d): got %f, want 0 (walk must "+
			"stop descending)", maxEventDepth, deep.Shape.Width)
	}
}

func TestLayoutHeightsStopsAtDepthCap(t *testing.T) {
	shallow := deepRectChain(100)
	layoutHeights(shallow)
	if !f32AreClose(shallow.Shape.Height, 20) {
		t.Errorf("height at depth 100: got %f, want 20", shallow.Shape.Height)
	}

	deep := deepRectChain(maxEventDepth + 50)
	layoutHeights(deep)
	if !f32AreClose(deep.Shape.Height, 0) {
		t.Errorf("height past maxEventDepth: got %f, want 0", deep.Shape.Height)
	}
}

func TestLayoutParentsStopsAtDepthCap(t *testing.T) {
	shallow := deepRectChain(100)
	layoutParents(shallow, nil)
	if leafOf(shallow).Parent == nil {
		t.Error("leaf at depth 100 has no parent")
	}

	deep := deepRectChain(maxEventDepth + 50)
	layoutParents(deep, nil)
	if leafOf(deep).Parent != nil {
		t.Error("layoutParents reached a leaf past maxEventDepth")
	}
}

func TestLayoutDisablesStopsAtDepthCap(t *testing.T) {
	shallow := deepRectChain(100)
	shallow.Shape.Disabled = true
	layoutDisables(shallow, false)
	if !leafOf(shallow).Shape.Disabled {
		t.Error("leaf at depth 100 not disabled by its ancestor")
	}

	deep := deepRectChain(maxEventDepth + 50)
	deep.Shape.Disabled = true
	layoutDisables(deep, false)
	if leafOf(deep).Shape.Disabled {
		t.Error("layoutDisables reached a leaf past maxEventDepth")
	}
}

func TestLayoutPositionsStopsAtDepthCap(t *testing.T) {
	w := &Window{}

	shallow := deepRectChain(100)
	layoutPositions(shallow, 5, 7, w)
	if !f32AreClose(leafOf(shallow).Shape.X, 5) {
		t.Errorf("leaf X at depth 100: got %f, want 5",
			leafOf(shallow).Shape.X)
	}

	deep := deepRectChain(maxEventDepth + 50)
	layoutPositions(deep, 5, 7, w)
	if !f32AreClose(leafOf(deep).Shape.X, 0) {
		t.Errorf("layoutPositions reached a leaf past maxEventDepth: X %f",
			leafOf(deep).Shape.X)
	}
}

// sizeChain gives every node in a chain a real box. layoutSetShapeClips
// intersects each node's rect with its parent's, so a chain of zero-width
// nodes clips to nothing and would hide whether the walk arrived at all.
func sizeChain(root *Layout) *Layout {
	for cur := root; ; {
		cur.Shape.Width = 100
		cur.Shape.Height = 100
		if len(cur.Children) == 0 {
			return root
		}
		cur = &cur.Children[0]
	}
}

func TestLayoutSetShapeClipsStopsAtDepthCap(t *testing.T) {
	clip := drawClip{Width: 800, Height: 600}

	shallow := sizeChain(deepRectChain(100))
	layoutSetShapeClips(shallow, clip)
	if leafOf(shallow).Shape.shapeClip.Width == 0 {
		t.Error("leaf at depth 100 got no clip")
	}

	deep := sizeChain(deepRectChain(maxEventDepth + 50))
	layoutSetShapeClips(deep, clip)
	if leafOf(deep).Shape.shapeClip.Width != 0 {
		t.Error("layoutSetShapeClips reached a leaf past maxEventDepth")
	}
}

// nodeAt walks n levels down a chain built by deepRectChain.
func nodeAt(root *Layout, n int) *Layout {
	cur := root
	for range n {
		cur = &cur.Children[0]
	}
	return cur
}

func TestLayoutRotationSwapStopsAtDepthCap(t *testing.T) {
	// A quarter-turned leaf swaps to 20x50; an unreached one keeps 50x20.
	swapChain := func(n int) *Layout {
		root := deepRectChain(n)
		leaf := leafOf(root)
		leaf.Shape.QuarterTurns = 1
		return root
	}

	shallow := swapChain(100)
	layoutRotationSwap(shallow)
	if !f32AreClose(leafOf(shallow).Shape.Width, 20) {
		t.Errorf("width at depth 100: got %f, want 20 (swapped)",
			leafOf(shallow).Shape.Width)
	}

	deep := swapChain(maxEventDepth + 50)
	layoutRotationSwap(deep)
	if !f32AreClose(leafOf(deep).Shape.Width, 50) {
		t.Errorf("width past maxEventDepth: got %f, want 50 (walk must "+
			"stop descending)", leafOf(deep).Shape.Width)
	}
}

func TestLayoutRemoveFloatingLayoutsStopsAtDepthCap(t *testing.T) {
	// The only Float in the chain is the leaf; reaching it extracts one
	// layout, stopping short extracts none.
	floatChain := func(n int) *Layout {
		root := deepRectChain(n)
		leafOf(root).Shape.Float = true
		return root
	}

	shallow := floatChain(100)
	var shallowFloats []*Layout
	layoutRemoveFloatingLayouts(shallow, nil, &shallowFloats)
	if len(shallowFloats) != 1 {
		t.Errorf("extracted %d floats at depth 100, want 1",
			len(shallowFloats))
	}

	deep := floatChain(maxEventDepth + 50)
	var deepFloats []*Layout
	layoutRemoveFloatingLayouts(deep, nil, &deepFloats)
	if len(deepFloats) != 0 {
		t.Errorf("extracted %d floats past maxEventDepth, want 0",
			len(deepFloats))
	}
}

func TestResolveFocusOwnersStopsAtDepthCap(t *testing.T) {
	// Root "a" with a "b" node scopes the leaf's owner reference to
	// "a:b". Past the cap the "b" node is never visited, so the leaf's
	// reference stays the bare "b".
	ownerChain := func(n, bAt int) *Layout {
		root := deepRectChain(n)
		root.Shape.ID = "a"
		nodeAt(root, bAt).Shape.ID = "b"
		leafOf(root).Shape.focusOwner = "b"
		return root
	}

	shallow := ownerChain(100, 60)
	resolveFocusOwners(shallow, &Window{})
	if got := leafOf(shallow).Shape.focusOwner; got != "a:b" {
		t.Errorf("focusOwner at depth 100: got %q, want %q", got, "a:b")
	}

	deep := ownerChain(maxEventDepth+50, maxEventDepth+40)
	resolveFocusOwners(deep, &Window{})
	if got := leafOf(deep).Shape.focusOwner; got != "b" {
		t.Errorf("focusOwner past maxEventDepth: got %q, want %q "+
			"(walk must stop descending)", got, "b")
	}
}

func TestApplyTransitionStopsAtDepthCap(t *testing.T) {
	// The leaf lerps from X 100 toward snapshot X 0 at progress 0.5, so
	// a reached leaf reads 50 and an unreached one keeps 100.
	transitionChain := func(n int) (*Layout, *layoutTransition) {
		root := deepRectChain(n)
		leafOf(root).Shape.X = 100
		lt := &layoutTransition{
			transitionBase: transitionBase{progress: 0.5},
			snapshots: map[string]posSnapshot{
				"deep": {x: 0, y: 0, width: 50, height: 20},
			},
		}
		return root, lt
	}

	shallow, shallowLT := transitionChain(100)
	applyTransitionRecursive(shallow, shallowLT, 0, 0, 0)
	if !f32AreClose(leafOf(shallow).Shape.X, 50) {
		t.Errorf("leaf X at depth 100: got %f, want 50",
			leafOf(shallow).Shape.X)
	}

	deep, deepLT := transitionChain(maxEventDepth + 50)
	applyTransitionRecursive(deep, deepLT, 0, 0, 0)
	if !f32AreClose(leafOf(deep).Shape.X, 100) {
		t.Errorf("leaf X past maxEventDepth: got %f, want 100 (walk must "+
			"stop descending)", leafOf(deep).Shape.X)
	}
}

func TestApplyHeroStopsAtDepthCap(t *testing.T) {
	// A hero leaf with no outgoing entry only fades: at progress 0.75 a
	// reached leaf drops to opacity 0.5, an unreached one keeps 1.
	heroChain := func(n int) *Layout {
		root := deepRectChain(n)
		leaf := leafOf(root)
		leaf.Shape.Hero = true
		leaf.Shape.Opacity = 1
		return root
	}
	empty := map[string]posSnapshot{}

	shallow := heroChain(100)
	applyHeroRecursive(shallow, 0.75, empty, empty, 0, 0)
	if !f32AreClose(leafOf(shallow).Shape.Opacity, 0.5) {
		t.Errorf("leaf opacity at depth 100: got %f, want 0.5",
			leafOf(shallow).Shape.Opacity)
	}

	deep := heroChain(maxEventDepth + 50)
	applyHeroRecursive(deep, 0.75, empty, empty, 0, 0)
	if !f32AreClose(leafOf(deep).Shape.Opacity, 1) {
		t.Errorf("leaf opacity past maxEventDepth: got %f, want 1 (walk "+
			"must stop descending)", leafOf(deep).Shape.Opacity)
	}
}

func TestScrollAnchorShiftYStopsAtDepthCap(t *testing.T) {
	shallow := deepRectChain(100)
	scrollAnchorShiftY(shallow, 5)
	if !f32AreClose(leafOf(shallow).Shape.Y, 5) {
		t.Errorf("leaf Y at depth 100: got %f, want 5",
			leafOf(shallow).Shape.Y)
	}

	deep := deepRectChain(maxEventDepth + 50)
	scrollAnchorShiftY(deep, 5)
	if !f32AreClose(leafOf(deep).Shape.Y, 0) {
		t.Errorf("leaf Y past maxEventDepth: got %f, want 0 (walk must "+
			"stop descending)", leafOf(deep).Shape.Y)
	}
}

// nestView builds a single-child container chain n levels deep for the
// generation-cap test. Only the leaf carries a size and an ID, so a
// generation pass that reaches the leaf shows up in its identity and a
// pass that stops short leaves a placeholder with neither.
type nestView struct{ n int }

func (v nestView) GenerateLayout(w *Window) Layout {
	l := Layout{Shape: &Shape{
		shapeType: shapeRectangle,
		Axis:      axisTopToBottom,
	}}
	if v.n == 0 {
		l.Shape.Width = 50
		l.Shape.Height = 20
		l.Shape.ID = "deep"
		return l
	}
	appendChildViews(w, &l, []View{nestView{v.n - 1}})
	return l
}

func TestGenerateViewLayoutStopsAtDepthCap(t *testing.T) {
	w := &Window{}

	shallow := generateViewLayout(nestView{100}, w)
	if got := leafOf(&shallow).Shape.ID; got != "deep" {
		t.Errorf("leaf ID at depth 100: got %q, want %q (view must be "+
			"generated)", got, "deep")
	}

	deep := generateViewLayout(nestView{maxEventDepth + 50}, w)
	leaf := leafOf(&deep)
	if leaf.Shape.ID == "deep" {
		t.Error("leaf past maxEventDepth was generated (generation must " +
			"stop descending)")
	}
	if leaf.Shape.shapeType != shapeNone {
		t.Error("truncated subtree is not a placeholder (want shapeNone)")
	}
	if got := w.viewState.genDepth; got != 0 {
		t.Errorf("genDepth after generation: got %d, want 0 (balanced)",
			got)
	}
}
