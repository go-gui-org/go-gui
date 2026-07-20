package gui

import "testing"

// anchorPost is a fixed-height child of the test scrollable.
type anchorPost struct {
	id string
	h  float32
}

const anchorScrollID = "scroll"

// anchorLayoutTree builds a 100x100 scrollable column containing one
// fixed-height child per post.
func anchorLayoutTree(posts []anchorPost) Layout {
	children := make([]Layout, len(posts))
	for i, p := range posts {
		children[i] = Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				ID:        p.id,
				Width:     100,
				Height:    p.h,
			},
		}
	}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         anchorScrollID,
			Width:      100,
			Height:     100,
			Axis:       AxisTopToBottom,
		},
		Children: children,
	}
	return Layout{
		Shape:    &Shape{shapeType: shapeRectangle, Width: 100, Height: 100},
		Children: []Layout{scroll},
	}
}

// anchorLayoutPass mimics a frame's relevant pipeline steps for the
// given posts: clamp offsets, position, then apply pending anchors.
func anchorLayoutPass(w *Window, posts []anchorPost) {
	w.layout = anchorLayoutTree(posts)
	layoutAdjustScrollOffsets(&w.layout, w)
	layoutPositions(&w.layout, 0, 0, w)
	layoutApplyScrollAnchors(&w.layout, w)
}

// anchorWindow builds a window positioned at the given offset over
// the posts, ready for a ScrollAnchor request.
func anchorWindow(offset float32, posts []anchorPost) *Window {
	w := &Window{}
	w.scrollY().Set(anchorScrollID, offset)
	w.layout = anchorLayoutTree(posts)
	layoutAdjustScrollOffsets(&w.layout, w)
	layoutPositions(&w.layout, 0, 0, w)
	return w
}

func anchorChildY(t *testing.T, w *Window, id string) float32 {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("child %q not found", id)
	}
	return ly.Shape.Y
}

func TestScrollAnchorHoldsPositionAcrossPrepend(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)
	if got := anchorChildY(t, w, "A"); got != -30 {
		t.Fatalf("pre-anchor A.Y = %v, want -30", got)
	}

	w.ScrollAnchor(anchorScrollID, "A")
	anchorLayoutPass(w, append([]anchorPost{{"N", 40}}, posts...))

	if got := w.ScrollVerticalOffset(anchorScrollID); got != -70 {
		t.Errorf("offset = %v, want -70", got)
	}
	// The anchor keeps its viewport position; the prepended post
	// sits above it, off-screen.
	if got := anchorChildY(t, w, "A"); got != -30 {
		t.Errorf("A.Y = %v, want -30", got)
	}
	if got := anchorChildY(t, w, "N"); got != -70 {
		t.Errorf("N.Y = %v, want -70", got)
	}
	if len(w.scrollAnchors) != 0 {
		t.Errorf("anchor not consumed: %d pending", len(w.scrollAnchors))
	}
}

func TestScrollAnchorConsumedOnce(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)
	w.ScrollAnchor(anchorScrollID, "A")

	grown := append([]anchorPost{{"N", 40}}, posts...)
	anchorLayoutPass(w, grown)
	if got := w.ScrollVerticalOffset(anchorScrollID); got != -70 {
		t.Fatalf("offset = %v, want -70", got)
	}

	// A second pass over the same content must not re-apply.
	anchorLayoutPass(w, grown)
	if got := w.ScrollVerticalOffset(anchorScrollID); got != -70 {
		t.Errorf("offset after second pass = %v, want -70", got)
	}
}

func TestScrollAnchorBailsWhenContentFits(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)
	w.ScrollAnchor(anchorScrollID, "A")

	// Content shrinks to fit the viewport: nothing to anchor; the
	// clamp pass snaps the offset to 0 and the anchor stays out of it.
	anchorLayoutPass(w, []anchorPost{{"A", 50}, {"B", 30}})
	if got := w.ScrollVerticalOffset(anchorScrollID); got != 0 {
		t.Errorf("offset = %v, want 0", got)
	}
	if got := anchorChildY(t, w, "A"); got != 0 {
		t.Errorf("A.Y = %v, want 0 (no shift)", got)
	}
}

func TestScrollAnchorBailsWhenCorrectionLeavesRange(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)
	w.ScrollAnchor(anchorScrollID, "A")

	// New content 110 high: max offset -10, so the clamp pass moves
	// the reader to -10 and holding A would need -40. Out of range —
	// jump instead of anchoring.
	anchorLayoutPass(w, []anchorPost{{"N", 10}, {"A", 50}, {"B", 50}})
	if got := w.ScrollVerticalOffset(anchorScrollID); got != -10 {
		t.Errorf("offset = %v, want -10", got)
	}
}

func TestScrollAnchorBailsWhenAnchorGone(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)
	w.ScrollAnchor(anchorScrollID, "A")

	anchorLayoutPass(w, []anchorPost{{"N", 40}, {"B", 50}, {"C", 50}})
	if got := w.ScrollVerticalOffset(anchorScrollID); got != -30 {
		t.Errorf("offset = %v, want -30 (unchanged)", got)
	}
	if len(w.scrollAnchors) != 0 {
		t.Errorf("anchor not consumed: %d pending", len(w.scrollAnchors))
	}
}

func TestScrollAnchorRevealArmsEaseFromTop(t *testing.T) {
	// The reader is at the very top — the case where arming the ease
	// before the correction would no-op (target 0 == displayed 0).
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(0, posts)

	w.ScrollAnchorReveal(anchorScrollID, "A")
	anchorLayoutPass(w, append([]anchorPost{{"N", 40}}, posts...))

	if got := w.ScrollVerticalOffset(anchorScrollID); got != -40 {
		t.Fatalf("offset = %v, want -40", got)
	}
	e := w.scrollSmooth.findEntry(anchorScrollID, scrollAxisY)
	if e == nil || !e.active {
		t.Fatal("expected an active ease entry")
	}
	if e.target != 0 {
		t.Errorf("ease target = %v, want 0", e.target)
	}
	if e.current != -40 {
		t.Errorf("ease current = %v, want -40", e.current)
	}

	// Drive the ease: the new post glides fully into view.
	driveScrollSmooth(w, 200)
	if got := w.ScrollVerticalOffset(anchorScrollID); got != 0 {
		t.Errorf("settled offset = %v, want 0", got)
	}
}

func TestScrollAnchorShiftsInFlightEase(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)

	// An ease toward -10 is mid-flight when the anchor applies.
	sc, ok := FindLayoutByScrollID(&w.layout, anchorScrollID)
	if !ok {
		t.Fatal("scrollable not found")
	}
	if !scrollSmoothTo(w, sc, scrollAxisY, -10) {
		t.Fatal("expected ease to arm")
	}

	w.ScrollAnchor(anchorScrollID, "A")
	anchorLayoutPass(w, append([]anchorPost{{"N", 40}}, posts...))

	// The correction moved the displayed offset by -40; the ease
	// continues from there instead of snapping back to pre-anchor
	// values on its next tick.
	e := w.scrollSmooth.findEntry(anchorScrollID, scrollAxisY)
	if e == nil || !e.active {
		t.Fatal("expected the ease to stay active")
	}
	if e.current != -70 {
		t.Errorf("ease current = %v, want -70", e.current)
	}
	if e.target != -10 {
		t.Errorf("ease target = %v, want -10", e.target)
	}
}

func TestScrollAnchorStaleRequestDropped(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)
	w.ScrollAnchor(anchorScrollID, "A")

	// Passes over trees without the scrollable (e.g. floating
	// subtrees) keep the request pending...
	other := Layout{Shape: &Shape{shapeType: shapeRectangle}}
	layoutApplyScrollAnchors(&other, w)
	if len(w.scrollAnchors) != 1 {
		t.Fatalf("pending anchors = %d, want 1", len(w.scrollAnchors))
	}

	// ...but only for scrollAnchorMaxAge frames.
	w.frameCount += scrollAnchorMaxAge + 1
	layoutApplyScrollAnchors(&other, w)
	if len(w.scrollAnchors) != 0 {
		t.Errorf("stale anchor kept: %d pending", len(w.scrollAnchors))
	}
}

func TestScrollAnchorLastWriteWinsPerScrollable(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)
	w.ScrollAnchor(anchorScrollID, "A")
	w.ScrollAnchorReveal(anchorScrollID, "B")

	if len(w.scrollAnchors) != 1 {
		t.Fatalf("pending anchors = %d, want 1", len(w.scrollAnchors))
	}
	a := w.scrollAnchors[0]
	if a.anchorID != "B" || !a.reveal {
		t.Errorf("pending anchor = %+v, want anchorID B with reveal", a)
	}
	// B sat at -30+50 = 20 relative to the viewport top.
	if a.relY != 20 {
		t.Errorf("relY = %v, want 20", a.relY)
	}
}

func TestScrollAnchorUnknownIDsNoOp(t *testing.T) {
	posts := []anchorPost{{"A", 50}, {"B", 50}, {"C", 50}}
	w := anchorWindow(-30, posts)

	w.ScrollAnchor("", "A")
	w.ScrollAnchor(anchorScrollID, "")
	w.ScrollAnchor("nope", "A")
	w.ScrollAnchor(anchorScrollID, "nope")
	if len(w.scrollAnchors) != 0 {
		t.Errorf("pending anchors = %d, want 0", len(w.scrollAnchors))
	}
}

func TestScrollVerticalOffset(t *testing.T) {
	w := &Window{}
	if got := w.ScrollVerticalOffset("missing"); got != 0 {
		t.Errorf("missing id offset = %v, want 0", got)
	}
	w.scrollY().Set("s", -42)
	if got := w.ScrollVerticalOffset("s"); got != -42 {
		t.Errorf("offset = %v, want -42", got)
	}
}
