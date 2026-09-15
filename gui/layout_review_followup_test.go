package gui

import (
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
