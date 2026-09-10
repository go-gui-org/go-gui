package gui

import (
	"testing"
)

// Generation caps how wide one container gets (maxChildViews) and
// nothing caps how deep a chain of single-child layouts runs. These
// tests pin the depth budget (maxEventDepth): walks stop descending past
// it instead of recursing until the stack gives out, and ordinary-depth
// trees are unaffected.

// deepChain builds a single-child chain n levels deep with a
// focusable leaf. Hand-built, so idKey falls back to the leaf ID.
func deepChain(n int) *Layout {
	root := &Layout{Shape: &Shape{}}
	cur := root
	for range n {
		cur.Children = []Layout{{Shape: &Shape{}}}
		cur = &cur.Children[0]
	}
	cur.Shape.Focusable = true
	cur.Shape.ID = "deep"
	return root
}

func TestFindByIDStopsAtDepthCap(t *testing.T) {
	if _, ok := deepChain(100).findByID("deep"); !ok {
		t.Error("findByID missed a leaf at depth 100")
	}
	if _, ok := deepChain(maxEventDepth + 50).findByID("deep"); ok {
		t.Errorf("findByID reached past maxEventDepth (%d)",
			maxEventDepth)
	}
}

func TestFindByIDDepthBoundary(t *testing.T) {
	// The budget is depth > maxEventDepth: a leaf sitting exactly
	// on the line is still reachable, one step past is not.
	if _, ok := deepChain(maxEventDepth).findByID("deep"); !ok {
		t.Errorf("findByID missed a leaf at depth %d", maxEventDepth)
	}
	if _, ok := deepChain(maxEventDepth + 1).findByID("deep"); ok {
		t.Errorf("findByID reached a leaf at depth %d",
			maxEventDepth+1)
	}
}

func TestFocusCandidatesStopAtDepthCap(t *testing.T) {
	var candidates []focusCandidate
	collectFocusCandidates(deepChain(maxEventDepth+50),
		&candidates, make(map[string]struct{}))
	if len(candidates) != 0 {
		t.Errorf("collected %d candidates past maxEventDepth, want 0",
			len(candidates))
	}
}

func TestCharHandlerDeepChainTerminates(t *testing.T) {
	root := deepChain(10000)
	w := newTestWindow()
	w.SetFocus("deep")
	e := &Event{Type: EventChar, CharCode: 'x'}
	charHandler(root, e, w)
	// Past the budget the leaf is never reached: the event travels
	// on unhandled instead of recursing without bound.
	if e.IsHandled {
		t.Error("charHandler reached past maxEventDepth")
	}
}

func TestMouseDownDeepChainTerminates(t *testing.T) {
	root := deepChain(10000)
	e := &Event{Type: EventMouseDown, MouseButton: MouseLeft}
	mouseDownHandler(root, false, e, &Window{})
	if e.IsHandled {
		t.Error("mouseDownHandler reached past maxEventDepth")
	}
}
