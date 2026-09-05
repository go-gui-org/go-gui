package gui

import (
	"math"
	"testing"
)

func TestOffsetMouseChangeX(t *testing.T) {
	w := &Window{}
	// Layout: 100 wide, content 400 wide (axis LTR so contentWidth sums children).
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 50}}
	layout := &Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "1",
			Width:      100,
			Height:     50,
			Axis:       axisLeftToRight,
		},
		Children: []Layout{child},
	}

	offset := offsetMouseChangeX(w.scrollX(), layout, 10, "1")
	// ratio = 400/100 = 4, newOffset = 10*4 = 40, offset = 0 - 40 = -40
	// clamped: min(0, max(-40, 100-400)) = min(0, max(-40, -300)) = min(0, -40) = -40
	if offset != -40 {
		t.Errorf("expected -40, got %v", offset)
	}
}

func TestOffsetMouseChangeY(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 500}}
	layout := &Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "2",
			Width:      50,
			Height:     100,
			Axis:       axisTopToBottom,
		},
		Children: []Layout{child},
	}

	offset := offsetMouseChangeY(w.scrollY(), layout, 5, "2")
	// ratio = 500/100 = 5, newOffset = 5*5 = 25, offset = 0 - 25 = -25
	// clamped: min(0, max(-25, 100-500)) = -25
	if offset != -25 {
		t.Errorf("expected -25, got %v", offset)
	}
}

func TestOffsetMouseChangeZeroViewport(t *testing.T) {
	// Degenerate zero-size viewport must not divide by zero: the
	// prior offset is returned unchanged and no NaN is produced.
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 400}}
	layout := &Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "z",
			Width:      0,
			Height:     0,
			Axis:       axisLeftToRight,
		},
		Children: []Layout{child},
	}
	w.scrollX().Set("z", -7)
	w.scrollY().Set("z", -9)

	gotX := offsetMouseChangeX(w.scrollX(), layout, 10, "z")
	if math.IsNaN(float64(gotX)) || gotX != -7 {
		t.Errorf("x offset = %v, want -7", gotX)
	}
	gotY := offsetMouseChangeY(w.scrollY(), layout, 10, "z")
	if math.IsNaN(float64(gotY)) || gotY != -9 {
		t.Errorf("y offset = %v, want -9", gotY)
	}
}

func TestOffsetFromMouseY(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "3",
			Width:      50,
			Height:     100,
			Axis:       axisTopToBottom,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseY=50 → percent=50/100=0.5 → offset = -0.5*(400-100) = -150
	offsetFromMouseY(root, 50, "3", w)
	sy := w.scrollY()
	v, _ := sy.Get("3")
	if v != -150 {
		t.Errorf("expected -150, got %v", v)
	}
}

func TestOffsetFromMouseX(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 300, Height: 50}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "4",
			Width:      100,
			Height:     50,
			Axis:       axisLeftToRight,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseX=100 → percent=100/100=1.0 → snap to 1
	// offset = -1*(300-100) = -200
	offsetFromMouseX(root, 100, "4", w)
	sx := w.scrollX()
	v, _ := sx.Get("4")
	if v != -200 {
		t.Errorf("expected -200, got %v", v)
	}
}

func TestOffsetFromMouseYSnap(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "5",
			Width:      50,
			Height:     100,
			Axis:       axisTopToBottom,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseY=2 → percent=0.02 → below snapMin(0.03) → snaps to 0
	offsetFromMouseY(root, 2, "5", w)
	sy := w.scrollY()
	v, _ := sy.Get("5")
	if v != 0 {
		t.Errorf("expected 0 (snap to start), got %v", v)
	}

	// mouseY=98 → percent=0.98 → above snapMax(0.97) → snaps to 1
	offsetFromMouseY(root, 98, "5", w)
	v, _ = sy.Get("5")
	// -1*(400-100) = -300
	if v != -300 {
		t.Errorf("expected -300 (snap to end), got %v", v)
	}
}

func TestScrollbarMouseMoveVertical(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "6",
			Width:      50,
			Height:     100,
			Y:          10,
			Axis:       axisTopToBottom,
		},
		Children: []Layout{child},
	}
	root := Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	e := &Event{MouseY: 50, MouseDY: 5}
	scrollbarMouseMove(scrollbarVertical, "6", &root, e, w)
	sy := w.scrollY()
	v, _ := sy.Get("6")
	// ratio=400/100=4, newOffset=5*4=20, offset=0-20=-20
	if v != -20 {
		t.Errorf("expected -20, got %v", v)
	}
}

func TestScrollbarMouseMoveHorizontal(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 300, Height: 50}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "7",
			Width:      100,
			Height:     50,
			X:          0,
			Axis:       axisLeftToRight,
		},
		Children: []Layout{child},
	}
	root := Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	e := &Event{MouseX: 50, MouseDX: 10}
	scrollbarMouseMove(scrollbarHorizontal, "7", &root, e, w)
	sx := w.scrollX()
	v, _ := sx.Get("7")
	// ratio=300/100=3, newOffset=10*3=30, offset=0-30=-30
	if v != -30 {
		t.Errorf("expected -30, got %v", v)
	}
}

func TestThumbOnClickLocksAndUnlocks(t *testing.T) {
	w := &Window{}
	e := &Event{}
	handler := makeScrollbarOnMouseDown(ScrollbarCfg{
		Orientation: scrollbarVertical,
		ID:          "1",
		scrollID:    "1",
	})
	handler(EventCtx{nil, e, w})
	if !w.mouseIsLocked() {
		t.Error("expected mouse locked after thumb click")
	}
	if !e.IsHandled {
		t.Error("expected event handled")
	}

	// Simulate mouse up.
	w.viewState.mouseLock.MouseUp(EventCtx{nil, e, w})
	if w.mouseIsLocked() {
		t.Error("expected mouse unlocked after mouse up")
	}
}

func TestOffsetFromMouseXWithNonZeroOrigin(t *testing.T) {
	w := &Window{}
	child := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 300, Height: 50},
	}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "20",
			Width:      100,
			Height:     50,
			X:          50,
			Axis:       axisLeftToRight,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseX=100 is at 50% of the scrollbar (origin 50, width 100).
	// percent = (100-50)/100 = 0.5
	// offset = -0.5*(300-100) = -100
	offsetFromMouseX(root, 100, "20", w)
	sx := w.scrollX()
	v, _ := sx.Get("20")
	if v != -100 {
		t.Errorf("expected -100, got %v", v)
	}
}

func TestOffsetFromMouseYWithNonZeroOrigin(t *testing.T) {
	w := &Window{}
	child := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400},
	}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "21",
			Width:      50,
			Height:     100,
			Y:          200,
			Axis:       axisTopToBottom,
		},
		Children: []Layout{child},
	}
	root := &Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	// mouseY=250 is at 50% of the scrollbar (origin 200, height 100).
	// percent = (250-200)/100 = 0.5
	// offset = -0.5*(400-100) = -150
	offsetFromMouseY(root, 250, "21", w)
	sy := w.scrollY()
	v, _ := sy.Get("21")
	if v != -150 {
		t.Errorf("expected -150, got %v", v)
	}
}

func TestGutterClickSetsOffsetAndLocks(t *testing.T) {
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 400}}
	scroll := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "8",
			Width:      50,
			Height:     100,
			Axis:       axisTopToBottom,
		},
		Children: []Layout{child},
	}
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}

	e := &Event{MouseY: 50}
	handler := makeScrollbarGutterClick(ScrollbarCfg{
		Orientation: scrollbarVertical,
		ID:          "8",
		scrollID:    "8",
	})
	handler(EventCtx{nil, e, w})

	sy := w.scrollY()
	v, _ := sy.Get("8")
	// percent=50/100=0.5, offset = -0.5*(400-100) = -150
	if math.Abs(float64(v+150)) > 0.01 {
		t.Errorf("expected -150, got %v", v)
	}
	if !w.mouseIsLocked() {
		t.Error("expected mouse locked after gutter click")
	}
	if !e.IsHandled {
		t.Error("expected event handled")
	}
}

// scrollbarTrackX arranges a scrolling column with the given
// scrollbar override and returns the resolved track X.
func scrollbarTrackX(t *testing.T, override *ScrollbarCfg) float32 {
	t.Helper()

	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 200})
	w.viewGenerator = func(*Window) View {
		rows := make([]View, 40)
		for i := range rows {
			rows[i] = Column(ContainerCfg{Sizing: FillFixed, Height: 20})
		}
		return Column(ContainerCfg{
			ID:            "scroller",
			Sizing:        FillFill,
			Scrollable:    true,
			ScrollMode:    ScrollVerticalOnly,
			ScrollbarCfgY: override,
			Content:       rows,
		})
	}
	w.refreshLayout = true
	w.FrameFn()

	sc, ok := w.layout.FindByID("scroller")
	if !ok {
		t.Fatal("scroller not found")
	}
	// The bar is appended after the content, so it is the last child.
	return sc.Children[len(sc.Children)-1].Shape.X
}

// TestScrollbarGapEdgeExplicitZero is why ScrollbarCfg.GapEdge is an
// Opt: an override asking for zero must sit flush against the edge,
// not fall back to the theme's inset.
func TestScrollbarGapEdgeExplicitZero(t *testing.T) {
	unset := scrollbarTrackX(t, nil)
	zero := scrollbarTrackX(t, &ScrollbarCfg{GapEdge: SomeF(0)})
	wide := scrollbarTrackX(t, &ScrollbarCfg{GapEdge: SomeF(20)})

	if zero <= unset {
		t.Errorf("explicit zero gap X = %v, want right of the themed %v", zero, unset)
	}
	if got, want := zero-unset, DefaultScrollbarStyle.GapEdge; got != want {
		t.Errorf("themed inset = %v, want %v", got, want)
	}
	if got, want := zero-wide, float32(20); got != want {
		t.Errorf("20px gap moved the bar %v, want %v", got, want)
	}
}

// scrollbarTrackH arranges a horizontally scrolling row and returns
// the horizontal bar's track layout together with the scroller it
// rides in, so a test can state the inset against the container
// rather than against a hard-coded pixel.
func scrollbarTrackH(t *testing.T, override *ScrollbarCfg) (track, scroller *Layout) {
	t.Helper()

	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 200})
	w.viewGenerator = func(*Window) View {
		// Fixed-width cells wider than the 300px window, so the bar
		// has something to scroll and stays visible.
		cells := make([]View, 40)
		for i := range cells {
			cells[i] = Column(ContainerCfg{Sizing: FixedFill, Width: 40})
		}
		return Row(ContainerCfg{
			ID:            "scroller",
			Sizing:        FillFill,
			Scrollable:    true,
			ScrollMode:    ScrollHorizontalOnly,
			ScrollbarCfgX: override,
			Content:       cells,
		})
	}
	w.refreshLayout = true
	w.FrameFn()

	sc, ok := w.layout.FindByID("scroller")
	if !ok {
		t.Fatal("scroller not found")
	}
	// container() appends the horizontal bar first, then the
	// vertical one, so the horizontal track is the second to last
	// child.
	return &sc.Children[len(sc.Children)-2], sc
}

// TestScrollbarGapEndHorizontal pins the direction GapEnd is applied
// in. The horizontal branch used to subtract it, putting the track
// outside its container instead of inset at both ends (issue #497).
func TestScrollbarGapEndHorizontal(t *testing.T) {
	track, sc := scrollbarTrackH(t, nil)
	gapEnd := DefaultScrollbarStyle.GapEnd

	gutterX := sc.Shape.X + sc.Shape.Padding.Left
	gutterWidth := sc.Shape.Width - sc.Shape.Padding.Width()

	if got, want := track.Shape.X, gutterX+gapEnd; got != want {
		t.Errorf("track X = %v, want %v (inset by GapEnd, not outset)", got, want)
	}
	if got, want := track.Shape.Width, gutterWidth-gapEnd-gapEnd; got != want {
		t.Errorf("track width = %v, want %v", got, want)
	}
	// The two end insets must match: that is what "centred in the
	// gutter" means, and it is the property the sign bug broke.
	leadIn := track.Shape.X - gutterX
	trailIn := (gutterX + gutterWidth) - (track.Shape.X + track.Shape.Width)
	if leadIn != trailIn {
		t.Errorf("end insets = %v and %v, want equal", leadIn, trailIn)
	}
}
