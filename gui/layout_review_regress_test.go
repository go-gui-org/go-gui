package gui

import (
	"math"
	"testing"
)

// Regression tests for the layout review findings. Each test builds the
// smallest tree that shows one defect and fails on the code before its fix.

// fillPass runs the width fill pass with a live fill generation, so the
// contentW cache is read back instead of recomputed. A zero scratchPools
// leaves fillGen at 0, and contentWidth then hides a stale cache.
func fillPass(root *Layout) {
	var p scratchPools
	p.beginFillPass()
	layoutFillWidths(root, &p)
}

// A Fill child must shrink when a Fixed sibling is the largest child.
// The largest-size search used to include Fixed siblings, so no Fill child
// matched it and the row overflowed.
func TestFillShrinksPastLargerFixedSibling(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
			Sizing: FixedFixed, Width: 100, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed, Width: 80, Height: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFit, Width: 40, Height: 10}},
		},
	}
	layoutParents(root, nil)
	fillPass(root)
	if got := root.Children[1].Shape.Width; !f32AreClose(got, 20) {
		t.Errorf("fill child width: got %f, want 20", got)
	}
}

// A definite-width wrap that breaks into rows must cache the wrapped
// content width, not the one-row sum from the fill pass.
func TestWrapRecachesContentWidthForDefiniteWidth(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "wrap", Axis: axisLeftToRight,
			Wrap: true, Scrollable: true, Sizing: FixedFixed, Width: 100, Height: 200},
	}
	for range 5 {
		root.Children = append(root.Children, Layout{Shape: &Shape{
			shapeType: shapeRectangle, Sizing: FixedFixed, Width: 40, Height: 10}})
	}
	layoutParents(root, nil)
	fillPass(root)
	layoutWrapContainers(root, w)
	if got := contentWidth(root); !f32AreClose(got, 100) {
		t.Errorf("contentW after wrap: got %f, want 100", got)
	}
}

// overflowRow builds an Overflow row of the given width. The last width is
// the trigger.
func overflowRow(width float32, widths ...float32) *Layout {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, ID: "ov", Axis: axisLeftToRight,
			Overflow: true, Sizing: FixedFit, Width: width, Height: 10},
	}
	for _, cw := range widths {
		root.Children = append(root.Children, Layout{Shape: &Shape{
			shapeType: shapeRectangle, Sizing: FixedFit, Width: cw, Height: 10}})
	}
	layoutParents(root, nil)
	return root
}

// Hiding overflow children must refresh the content width that alignment
// reads, or a centered row is placed against the pre-hide width.
func TestOverflowRecachesContentWidthAfterHiding(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	root := overflowRow(100, 40, 40, 40, 20)
	root.Shape.HAlign = HAlignCenter
	fillPass(root)
	layoutOverflow(root, w)
	if root.Children[2].Shape.shapeType != shapeNone {
		t.Fatal("child 2 should be hidden")
	}
	if got := contentWidth(root); !f32AreClose(got, 100) {
		t.Errorf("contentW after hiding: got %f, want 100", got)
	}
	layoutPositions(root, 0, 0, w)
	if got := root.Children[0].Shape.X; !f32AreClose(got, 0) {
		t.Errorf("centered child 0 X: got %f, want 0", got)
	}
}

// When every item fits once the trigger is gone, no item is hidden and the
// trigger is hidden. Room for the trigger is only needed while an item
// after the current one still has to fit.
func TestOverflowLastItemFitsWithoutTrigger(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	root := overflowRow(80, 40, 40, 20)
	fillPass(root)
	layoutOverflow(root, w)
	for i := range 2 {
		if root.Children[i].Shape.shapeType == shapeNone {
			t.Errorf("item %d hidden, but both items fit", i)
		}
	}
	if root.Children[2].Shape.shapeType != shapeNone {
		t.Error("trigger should be hidden when every item fits")
	}
	if got, _ := w.overflow().Get("ov"); got != 2 {
		t.Errorf("visible count: got %d, want 2", got)
	}
}

// An RTL row inside a quarter-turn container starts at the right edge of
// its unrotated frame. The centering correction was added for both
// directions, which pushed RTL children outside the container.
func TestRotatedRTLRowStartsAtInternalRightEdge(t *testing.T) {
	build := func(dir TextDirection) *Layout {
		root := &Layout{
			Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight,
				TextDir: dir, QuarterTurns: 1, Width: 100, Height: 40, HAlign: HAlignStart},
			Children: []Layout{
				{Shape: &Shape{shapeType: shapeRectangle, Width: 10, Height: 10}},
			},
		}
		layoutParents(root, nil)
		layoutPositions(root, 0, 0, &Window{})
		return root
	}
	// The unrotated frame is 40 wide, centered in the 100 wide display
	// box: x from 30 to 70.
	if got := build(TextDirLTR).Children[0].Shape.X; !f32AreClose(got, 30) {
		t.Errorf("LTR child X: got %f, want 30", got)
	}
	if got := build(TextDirRTL).Children[0].Shape.X; !f32AreClose(got, 60) {
		t.Errorf("RTL child X: got %f, want 60", got)
	}
}

// A float lifted out of a disabled container keeps the disabled state.
func TestFloatInheritsDisabledFromHost(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 400})
	layout := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 400,
			Sizing: FillFill, Opacity: 1},
		Children: []Layout{{
			Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 100, Height: 100, Disabled: true, Opacity: 1},
			Children: []Layout{{
				Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
					Width: 50, Height: 50, Float: true, Opacity: 1},
				Children: []Layout{{Shape: &Shape{shapeType: shapeRectangle,
					Sizing: FixedFixed, Width: 20, Height: 20, Opacity: 1}}},
			}},
		}},
	}
	layers := layoutArrange(&layout, w)
	if len(layers) < 2 {
		t.Fatalf("layers = %d, want 2", len(layers))
	}
	fl := layers[1]
	if !fl.Shape.Disabled {
		t.Error("float root should inherit Disabled from its host")
	}
	if !fl.Children[0].Shape.Disabled {
		t.Error("float descendant should inherit Disabled from its host")
	}
}

// Children of a rotated container stay inside the ancestor clip.
func TestRotatedChildClipIntersectsAncestorClip(t *testing.T) {
	root := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 100, Height: 100, Clip: true},
		Children: []Layout{{
			// Display box 20x60 at (40,70): center (50,100). The
			// unrotated frame is 60x20 at (20,90) and reaches y=110,
			// past the root's bottom edge.
			Shape: &Shape{X: 40, Y: 70, Width: 20, Height: 60, QuarterTurns: 1},
			Children: []Layout{
				{Shape: &Shape{X: 20, Y: 90, Width: 60, Height: 20}},
			},
		}},
	}
	layoutSetShapeClips(root, drawClip{Width: 500, Height: 500})
	c := root.Children[0].Children[0].Shape.shapeClip
	if c.Y+c.Height > 100+f32Tolerance {
		t.Errorf("rotated child clip reaches y=%f, want <= 100 (%+v)",
			c.Y+c.Height, c)
	}
}

// An OverflowPanel keys its overflow count and menu state by ID, so an
// empty ID must fail at construction.
func TestOverflowPanelRequiresID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("OverflowPanel without ID should panic")
		}
	}()
	OverflowPanel(&Window{}, OverflowPanelCfg{ // requiredid:ignore
		Items: []OverflowItem{{View: Text(TextCfg{Text: "a"})}},
	})
}

// A raw Overflow container keys the same state, so it needs an ID too.
func TestOverflowContainerRequiresID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Overflow container without ID should panic")
		}
	}()
	w := &Window{}
	generateViewLayout(Row(ContainerCfg{Overflow: true}), w)
}

// The rotation re-fit of a Fit axisNone container encloses children at
// their own X, like fitAxisNoneWidth.
func TestRotationRefitHonorsAxisNoneChildOffset(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisNone, Sizing: FitFit},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				X: 200, Width: 50, Height: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 10, Height: 30, QuarterTurns: 1}},
		},
	}
	layoutParents(root, nil)
	layoutWidths(root)
	layoutHeights(root)
	layoutRotationSwap(root)
	if got := root.Shape.Width; got < 250-f32Tolerance {
		t.Errorf("width after rotation re-fit: got %f, want >= 250", got)
	}
}

// An auto-flipped float keeps its gap from the anchor: the offset changes
// sign with the side it moved to.
func TestFloatAutoFlipNegatesOffset(t *testing.T) {
	win := drawClip{Width: 800, Height: 600}

	vp := Layout{
		Shape: &Shape{X: 100, Y: 550, Width: 100, Height: 30},
		Children: []Layout{{Shape: &Shape{
			Float: true, floatAutoFlip: true, Width: 80, Height: 100,
			FloatAnchor: FloatBottomLeft, FloatTieOff: FloatTopLeft,
			FloatOffsetY: 4,
		}}},
	}
	vp.Children[0].Parent = &vp
	if _, y := floatAttachLayout(&vp.Children[0], win); !f32AreClose(y, 446) {
		t.Errorf("vertical flip Y: got %f, want 446 (4px gap above)", y)
	}

	hp := Layout{
		Shape: &Shape{X: 750, Y: 200, Width: 40, Height: 30},
		Children: []Layout{{Shape: &Shape{
			Float: true, floatAutoFlip: true, Width: 100, Height: 50,
			FloatAnchor: FloatMiddleRight, FloatTieOff: floatMiddleLeft,
			FloatOffsetX: 4,
		}}},
	}
	hp.Children[0].Parent = &hp
	if x, _ := floatAttachLayout(&hp.Children[0], win); !f32AreClose(x, 646) {
		t.Errorf("horizontal flip X: got %f, want 646 (4px gap left)", x)
	}
}

// A Clip container does not take its children's minimum height, on either
// axis, the same way layoutWidths already skips child minimum widths.
func TestClipContainerDropsChildMinHeights(t *testing.T) {
	for _, axis := range []Axis{axisTopToBottom, axisLeftToRight} {
		root := &Layout{
			Shape: &Shape{shapeType: shapeRectangle, Axis: axis, Clip: true},
			Children: []Layout{
				{Shape: &Shape{shapeType: shapeRectangle, Height: 50, MinHeight: 50}},
			},
		}
		layoutHeights(root)
		if got := root.Shape.MinHeight; got != 0 {
			t.Errorf("axis %v: MinHeight got %f, want 0", axis, got)
		}
	}
}

// Float Z order sorts without integer overflow.
func TestFloatZIndexSortExtremes(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 400})
	layout := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 400,
			Sizing: FillFill, Opacity: 1},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 10, Height: 10, Float: true, FloatZIndex: math.MaxInt, Opacity: 1}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 10, Height: 10, Float: true, FloatZIndex: -5, Opacity: 1}},
		},
	}
	layers := layoutArrange(&layout, w)
	if len(layers) != 3 {
		t.Fatalf("layers = %d, want 3", len(layers))
	}
	if layers[1].Shape.FloatZIndex != -5 || layers[2].Shape.FloatZIndex != math.MaxInt {
		t.Errorf("layer Z order: got %d, %d; want -5, MaxInt",
			layers[1].Shape.FloatZIndex, layers[2].Shape.FloatZIndex)
	}
}

// A nested float with a lower Z than its host is positioned after the host,
// so it anchors to where the host landed.
func TestNestedFloatLowerZAnchorsToPositionedHost(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 400})
	layout := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 400,
			Sizing: FillFill, Opacity: 1},
		Children: []Layout{{
			Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 50, Height: 50, Float: true, FloatZIndex: 5, Opacity: 1,
				FloatAnchor: FloatTopLeft, FloatTieOff: FloatTopLeft,
				FloatOffsetX: 100, FloatOffsetY: 100},
			Children: []Layout{{Shape: &Shape{shapeType: shapeRectangle,
				Sizing: FixedFixed, Width: 10, Height: 10, Float: true,
				FloatZIndex: 1, Opacity: 1,
				FloatAnchor: FloatTopLeft, FloatTieOff: FloatTopLeft}}},
		}},
	}
	layers := layoutArrange(&layout, w)
	if len(layers) != 3 {
		t.Fatalf("layers = %d, want 3", len(layers))
	}
	// layers[1] is Z 1 (nested), layers[2] is Z 5 (host).
	nested, host := layers[1].Shape, layers[2].Shape
	if !f32AreClose(host.X, 100) {
		t.Fatalf("host X: got %f, want 100", host.X)
	}
	if !f32AreClose(nested.X, host.X) || !f32AreClose(nested.Y, host.Y) {
		t.Errorf("nested float at (%f,%f), want host origin (%f,%f)",
			nested.X, nested.Y, host.X, host.Y)
	}
}

// FindLayout and findShape follow the depth policy of every other walk.
func TestFindLayoutStopsAtDepthCap(t *testing.T) {
	isDeep := func(l Layout) bool { return l.Shape.ID == "deep" }
	if _, ok := deepRectChain(100).FindLayout(isDeep); !ok {
		t.Error("FindLayout at depth 100 should reach the leaf")
	}
	if _, ok := deepRectChain(100).findShape(isDeep); !ok {
		t.Error("findShape at depth 100 should reach the leaf")
	}
	deep := deepRectChain(maxEventDepth + 50)
	if _, ok := deep.FindLayout(isDeep); ok {
		t.Error("FindLayout past maxEventDepth should stop descending")
	}
	if _, ok := deep.findShape(isDeep); ok {
		t.Error("findShape past maxEventDepth should stop descending")
	}
}

// The last-item check skips out-of-flow children between the last item and
// the trigger. Counting the float as the last item would reserve trigger
// room for the real last item and hide it although everything fits.
func TestOverflowLastItemSkipsOutOfFlowChild(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	root := overflowRow(70, 40, 30, 10, 20)
	root.Children[2].Shape.Float = true
	fillPass(root)
	layoutOverflow(root, w)
	if root.Children[1].Shape.shapeType == shapeNone {
		t.Error("last item hidden, but it fits once the trigger is gone")
	}
	if root.Children[3].Shape.shapeType != shapeNone {
		t.Error("trigger should be hidden when every item fits")
	}
}

// A rotated container entirely outside the ancestor clip gives its children
// an empty clip, not the unrotated frame.
func TestRotatedChildClipEmptyOutsideAncestorClip(t *testing.T) {
	root := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 100, Height: 100, Clip: true},
		Children: []Layout{{
			Shape: &Shape{X: 300, Y: 300, Width: 20, Height: 60, QuarterTurns: 1},
			Children: []Layout{
				{Shape: &Shape{X: 280, Y: 320, Width: 60, Height: 20}},
			},
		}},
	}
	layoutSetShapeClips(root, drawClip{Width: 500, Height: 500})
	if c := root.Children[0].Children[0].Shape.shapeClip; c.Width != 0 || c.Height != 0 {
		t.Errorf("clip outside ancestor: got %+v, want empty", c)
	}
}

// A NaN child X is skipped by the axisNone rotation re-fit, as
// fitAxisNoneWidth skips it, so the container width stays finite.
func TestRotationRefitAxisNoneSkipsNaNChildX(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisNone, Sizing: FitFit},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				X: float32(math.NaN()), Width: 50, Height: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FixedFixed,
				Width: 10, Height: 30, QuarterTurns: 1}},
		},
	}
	layoutParents(root, nil)
	layoutWidths(root)
	layoutHeights(root)
	layoutRotationSwap(root)
	if got := root.Shape.Width; !f32IsFinite(got) || got < 30-f32Tolerance {
		t.Errorf("width after re-fit: got %f, want finite >= 30", got)
	}
}
