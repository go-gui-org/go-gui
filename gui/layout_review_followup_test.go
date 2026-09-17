package gui

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// A shape that stops being walked — disabled, or simply not generated —
// leaves its hover entry behind. Reading that entry as "was inside" fired a
// leave for a hover that had ended frames earlier, as soon as the shape came
// back with the pointer somewhere else.
func TestMouseLeaveIgnoresStaleHoverEntry(t *testing.T) {
	w := &Window{}
	left := 0
	shape := makeHoverShape("stale")
	shape.events = &eventHandlers{
		OnMouseLeave: func(ctx EventCtx) { left++ },
	}
	layout := Layout{Shape: shape}

	// Frame 1: the pointer is inside.
	w.frameCount = 1
	w.viewState.mousePosX, w.viewState.mousePosY = 15, 15
	layoutMouseLeave(&layout, w)

	// Frames 2 and 3: the shape is not walked at all. Only the frame
	// counter moves.
	w.frameCount = 4

	// Frame 4: it is back, and the pointer is elsewhere. The hover it
	// recorded ended three frames ago, so there is no leave to fire.
	w.viewState.mousePosX, w.viewState.mousePosY = 100, 100
	layoutMouseLeave(&layout, w)

	if left != 0 {
		t.Errorf("OnMouseLeave fired for a stale hover: got %d, want 0", left)
	}
}

// The frame stamp must not break the ordinary case: inside on one frame,
// outside on the next, fires exactly once.
func TestMouseLeaveFiresAcrossConsecutiveFrames(t *testing.T) {
	w := &Window{}
	left := 0
	shape := makeHoverShape("fresh")
	shape.events = &eventHandlers{
		OnMouseLeave: func(ctx EventCtx) { left++ },
	}
	layout := Layout{Shape: shape}

	w.frameCount = 7
	w.viewState.mousePosX, w.viewState.mousePosY = 15, 15
	layoutMouseLeave(&layout, w)

	w.frameCount = 8
	w.viewState.mousePosX, w.viewState.mousePosY = 100, 100
	layoutMouseLeave(&layout, w)

	if left != 1 {
		t.Errorf("OnMouseLeave fire count: got %d, want 1", left)
	}
}

// A zero-width in-flow child still holds a slot in the row, and
// layoutPositions still advances past it by width + spacing. Deciding the
// gap from the width accumulated so far lost that spacing, so the row kept
// an item it overflows.
func TestOverflowCountsGapAfterZeroWidthChild(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	root := &Layout{Shape: &Shape{shapeType: shapeRectangle, ID: "ov",
		Axis: axisLeftToRight, Overflow: true, Sizing: FixedFit,
		Width: 95, Height: 10, Spacing: 10}}
	for _, cw := range []float32{0, 40, 40, 20} {
		root.Children = append(root.Children, Layout{Shape: &Shape{
			shapeType: shapeRectangle, Sizing: FixedFit, Width: cw, Height: 10}})
	}
	layoutParents(root, nil)
	fillPass(root)
	layoutOverflow(root, w)

	// 0 + 10 + 40 + 10 + 40 is 100 in a row of 95: the second 40 cannot
	// stay, whatever its own width suggests in isolation.
	if root.Children[2].Shape.shapeType != shapeNone {
		t.Error("the item that overflows the row was kept")
	}
	if root.Children[1].Shape.shapeType == shapeNone {
		t.Error("the item that fits was hidden")
	}
}

// rtfTestShape builds a shape whose text config is ready for
// layoutWrapRTF, with the content every caller varies held constant.
func rtfTestShape(width float32) (*Shape, *shapeTextConfig) {
	rt := &RichText{Runs: []RichTextRun{
		{Text: "shared", Style: TextStyle{Size: 12}},
	}}
	tc := &shapeTextConfig{
		TextMode:     TextModeWrap,
		rTFRuns:      rt,
		rTFBaseStyle: glyph.TextStyle{Size: 12},
	}
	return &Shape{shapeType: shapeRTF, Width: width, TC: tc}, tc
}

// Two shapes with identical layout inputs hit one cache entry. The entry
// holds the layout by pointer, so the hit hands out what is already there
// instead of moving a copy to the heap for every RTF shape, every frame.
func TestRTFLayoutCacheHitSharesOneLayout(t *testing.T) {
	m := &rtfStubTextMeasurer{layout: glyph.Layout{Width: 80, Height: 24}}
	w := &Window{windowBackend: windowBackend{textMeasurer: m}}

	s1, tc1 := rtfTestShape(80)
	layoutWrapRTF(s1, tc1, w)
	if tc1.rTFLayout == nil {
		t.Fatal("first pass stored no layout")
	}

	s2, tc2 := rtfTestShape(80)
	layoutWrapRTF(s2, tc2, w)
	if tc2.rTFLayout == nil {
		t.Fatal("second pass stored no layout")
	}

	if tc1.rTFLayout != tc2.rTFLayout {
		t.Error("a cache hit copied the layout instead of sharing it")
	}
	if s2.Height != 24 {
		t.Errorf("height from cache: got %v, want 24", s2.Height)
	}
}

// FindLayout answers the outermost match. It walks pre-order, self before
// children, the same order findByID uses. A post-order walk answered the
// innermost match, so one predicate that matched an ancestor and a
// descendant named a different widget than FindByID did.
func TestFindLayoutReturnsOutermostMatch(t *testing.T) {
	leaf := Layout{Shape: &Shape{shapeType: shapeRectangle, ID: "dup"}}
	mid := Layout{Shape: &Shape{shapeType: shapeRectangle, ID: "dup"},
		Children: []Layout{leaf}}
	root := Layout{Shape: &Shape{shapeType: shapeRectangle},
		Children: []Layout{mid}}
	got, ok := root.FindLayout(func(l Layout) bool { return l.Shape.ID == "dup" })
	if !ok {
		t.Fatal("FindLayout found no match")
	}
	if got != &root.Children[0] {
		t.Error("FindLayout answered the inner match, want the outer one")
	}
}

// A nil Shape never matches, because every predicate reads the Shape,
// but the walk still descends past it. A hand-built mid-tree node with
// no Shape then cannot hide a valid match below it, and a nil receiver
// matches nothing. Each case panicked before the guard.
func TestFindLayoutSkipsNilShape(t *testing.T) {
	matchDeep := func(l Layout) bool { return l.Shape.ID == "deep" }
	deep := Layout{Shape: &Shape{shapeType: shapeRectangle, ID: "deep"}}
	mid := Layout{Children: []Layout{deep}}
	root := Layout{Shape: &Shape{shapeType: shapeRectangle},
		Children: []Layout{mid}}
	got, ok := root.FindLayout(matchDeep)
	if !ok {
		t.Fatal("FindLayout found no match past a nil Shape")
	}
	if got != &root.Children[0].Children[0] {
		t.Error("FindLayout stopped at a nil Shape instead of descending")
	}
	var nilRoot *Layout
	if _, ok := nilRoot.FindLayout(matchDeep); ok {
		t.Error("FindLayout on a nil root matched")
	}
}

// A hand-built child with no Shape must not panic the hover walk. The
// mouse-leave walk already tolerated one. Hover read Shape with no
// guard, so the same tree panicked here.
func TestLayoutHoverSkipsNilShape(t *testing.T) {
	w := &Window{}
	root := Layout{
		Shape:    &Shape{shapeType: shapeRectangle, Width: 100, Height: 100},
		Children: []Layout{{}},
	}
	if layoutHover(&root, w) {
		t.Error("layoutHover with no handlers reported a handled hover")
	}
	if layoutHover(nil, w) {
		t.Error("layoutHover on a nil root reported a handled hover")
	}
}

// A NaN size sailed through clampSize, because each NaN comparison is
// false, and then poisoned each later f32Max that took it as the second
// argument, plus the scroll range from it. The clamp now contains NaN
// to the effective Min, or to 0 when no Min applies.
func TestClampSizeContainsNaN(t *testing.T) {
	nan := float32(math.NaN())
	if got := clampSize(nan, 0, 0); got != 0 {
		t.Errorf("clampSize(NaN, 0, 0) = %v, want 0", got)
	}
	if got := clampSize(nan, 5, 10); got != 5 {
		t.Errorf("clampSize(NaN, 5, 10) = %v, want 5", got)
	}
	if got := clampSize(float32(math.Inf(1)), 0, 10); got != 10 {
		t.Errorf("clampSize(+Inf, 0, 10) = %v, want 10", got)
	}
	if got := clampSize(7, 0, 10); got != 7 {
		t.Errorf("clampSize(7, 0, 10) = %v, want 7", got)
	}
}
