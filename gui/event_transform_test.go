package gui

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// Pointer input through view transforms (#1019, phase 3). The pointer is
// mapped into each transformed view's own frame, through the inverse of
// the matrix the view was drawn with.

// xfCardView is a window with one transformed card that holds a button.
// tr is read through TransformFn, so a test can change it between frames.
func xfCardView(tr *Transform, clicks *int) func(*Window) View {
	return func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID:          "card",
					Padding:     PaddingNone,
					SizeBorder:  NoBorder,
					TransformFn: func() Transform { return *tr },
					Content: []View{
						Button(ButtonCfg{
							ID: "b", Width: 100, Height: 40,
							OnClick: func(ctx EventCtx) {
								*clicks++
								ctx.Consume()
							},
						}),
					},
				}),
			},
		})
	}
}

// xfPress sends a left press and release at (x, y) and reports how many
// clicks the card's button took.
func xfPress(w *Window, clicks *int, x, y float32) int {
	before := *clicks
	down := Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y}
	w.EventFn(&down)
	w.settle()
	up := Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: x, MouseY: y}
	w.EventFn(&up)
	w.settle()
	return *clicks - before
}

// xfCenter returns the center of a shape's box, in its own frame.
func xfCenter(s *Shape) (float32, float32) {
	return s.X + s.Width/2, s.Y + s.Height/2
}

func TestTransformClickTranslatedButton(t *testing.T) {
	tr := Translate(200, 50)
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	cx, cy := xfCenter(mustShape(t, w, ScopeID("card", "b")))
	if got := xfPress(w, &clicks, cx+200, cy+50); got != 1 {
		t.Fatalf("click where the button is drawn: %d clicks, want 1", got)
	}
	if got := xfPress(w, &clicks, cx, cy); got != 0 {
		t.Fatalf("click where the button would be untransformed: %d clicks, want 0", got)
	}
}

func TestTransformClickRotatedButton(t *testing.T) {
	tr := Rotate(90)
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	b := mustShape(t, w, ScopeID("card", "b"))
	ccx, ccy := xfCenter(mustShape(t, w, "card"))
	// A point near the right end of the button, in its own frame.
	lx, ly := b.X+b.Width-5, b.Y+b.Height/2
	// A quarter turn clockwise about the card's center, as drawn.
	dx, dy := lx-ccx, ly-ccy
	if got := xfPress(w, &clicks, ccx-dy, ccy+dx); got != 1 {
		t.Fatalf("click on the turned end: %d clicks, want 1", got)
	}
	// The unturned end is uncovered once the button stands upright.
	if got := xfPress(w, &clicks, lx, ly); got != 0 {
		t.Fatalf("click on the uncovered end: %d clicks, want 0", got)
	}
}

func TestTransformClickPerspectiveButton(t *testing.T) {
	tr := Translate(150, 100).RotateY(50).Perspective(300)
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	b := mustShape(t, w, ScopeID("card", "b"))
	xf := mustShape(t, w, "card").xf
	// A point near a corner that is drawn outside the button's
	// untransformed box, so only the inverse map can find the button.
	var x, y float32
	found := false
	for _, p := range [][2]float32{
		{b.X + 3, b.Y + 3}, {b.X + b.Width - 3, b.Y + 3},
		{b.X + 3, b.Y + b.Height - 3}, {b.X + b.Width - 3, b.Y + b.Height - 3},
	} {
		px, py, ok := xf.m.apply(p[0], p[1])
		if ok && (px < b.X || px > b.X+b.Width || py < b.Y || py > b.Y+b.Height) {
			x, y, found = px, py, true
			break
		}
	}
	if !found {
		t.Fatal("no corner is drawn outside the untransformed box")
	}
	if got := xfPress(w, &clicks, x, y); got != 1 {
		t.Fatalf("click on the drawn corner: %d clicks, want 1", got)
	}
}

func TestTransformHiddenBackFaceTakesNoClick(t *testing.T) {
	tr := RotateY(180).Perspective(800).BackfaceHidden()
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	cx, cy := xfCenter(mustShape(t, w, ScopeID("card", "b")))
	if got := xfPress(w, &clicks, cx, cy); got != 0 {
		t.Fatalf("click on a hidden back face: %d clicks, want 0", got)
	}
}

// TestClick aims at the center of the widget's clip, in its own frame,
// so it must map that point out through the transform.
func TestTransformTestClickFindsTransformedWidget(t *testing.T) {
	tr := Translate(200, 50).Rotate(30)
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	if err := w.TestClick(ScopeID("card", "b")); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1", clicks)
	}
}

// A render-only frame can change the transform through TransformFn with
// no rebuild. The pointer must follow the matrix that frame drew with.
func TestTransformClickFollowsRenderOnlyFrame(t *testing.T) {
	tr := Transform{}
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	cx, cy := xfCenter(mustShape(t, w, ScopeID("card", "b")))
	tr = Translate(200, 0)
	w.markRenderOnlyRefresh(refreshAnimation)
	w.updateRenderOnly()
	// The press itself is dispatched before any settle rebuilds.
	down := Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: cx + 200, MouseY: cy}
	w.EventFn(&down)
	if clicks != 1 {
		t.Fatalf("click on the moved button: %d clicks, want 1", clicks)
	}
}

func TestTransformHoverAtDrawnPosition(t *testing.T) {
	var hovered bool
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	view := func(*Window) View {
		hovered = false
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					Padding:   PaddingNone,
					Transform: Translate(200, 0),
					Content: []View{
						Row(ContainerCfg{
							ID: "h", Width: 100, Height: 40, Sizing: FixedFixed,
							OnHover: func(ctx EventCtx) { hovered = true },
						}),
					},
				}),
			},
		})
	}
	w.TestRender(view)
	cx, cy := xfCenter(mustShape(t, w, "h"))
	w.EventFn(&Event{Type: EventMouseMove, MouseX: cx + 200, MouseY: cy})
	w.settle()
	if !hovered {
		t.Fatal("hover where the row is drawn did not fire OnHover")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: cx, MouseY: cy})
	w.settle()
	if hovered {
		t.Fatal("hover where the row would be untransformed fired OnHover")
	}
}

func TestTransformInteractionTarget(t *testing.T) {
	s := &Shape{
		ID: "t", effID: "t", shapeType: shapeRectangle,
		X: 0, Y: 0, Width: 100, Height: 40,
		xf: &shapeTransform{static: Translate(200, 0)},
	}
	s.xf.update(s, makeWindow())
	layoutSetShapeClips(&Layout{Shape: s}, makeClip(0, 0, 400, 300))
	ly := &Layout{Shape: s}
	if id, hit := interactionTargetDepth(ly, 250, 20, 0); !hit || id != "t" {
		t.Fatalf("at the drawn position: id=%q hit=%v, want t, true", id, hit)
	}
	if _, hit := interactionTargetDepth(ly, 50, 20, 0); hit {
		t.Fatal("hit at the untransformed position")
	}
}

func TestTransformGestureCentroid(t *testing.T) {
	var got int
	s := &Shape{
		shapeType: shapeRectangle, X: 0, Y: 0, Width: 100, Height: 40,
		xf: &shapeTransform{static: Translate(200, 0)},
		events: &eventHandlers{OnGesture: func(ctx EventCtx) {
			got++
			ctx.Consume()
		}},
	}
	s.xf.update(s, makeWindow())
	layoutSetShapeClips(&Layout{Shape: s}, makeClip(0, 0, 400, 300))
	w := makeWindow()
	e := &Event{GestureType: GestureTap, CentroidX: 250, CentroidY: 20}
	gestureHandler(&Layout{Shape: s}, e, w)
	if got != 1 {
		t.Fatalf("gesture at the drawn position: %d calls, want 1", got)
	}
	if e.CentroidX != 250 || e.CentroidY != 20 {
		t.Fatalf("centroid not restored: %v,%v", e.CentroidX, e.CentroidY)
	}
}

func TestTransformAncestorClipTestedInParentFrame(t *testing.T) {
	// A view moved right by 100 inside a 200 wide clipping parent. The
	// part drawn at x 100..200 is inside the parent's clip, though its
	// own frame is x 0..100. The part drawn past x 200 is cut off.
	parent := &Shape{
		shapeType: shapeRectangle, Clip: true,
		X: 0, Y: 0, Width: 200, Height: 100,
	}
	child := xfShape(-100, 0, 200, 100, Translate(100, 0))
	child.xf.update(child, makeWindow())
	layoutSetShapeClips(&Layout{Shape: parent, Children: []Layout{{Shape: child}}},
		makeClip(0, 0, 400, 400))
	if x, y := xfCoordsInverse(child, 50, 50); !child.PointInShape(x, y) {
		t.Fatal("point drawn inside the clipping parent misses")
	}
	if x, y := xfCoordsInverse(child, 250, 50); child.PointInShape(x, y) {
		t.Fatal("point drawn outside the clipping parent hits")
	}
}

// Regression: the hit clips used to be cut through the matrix layout
// resolved. A render-only frame that turned the view then tested the
// pointer, mapped through the new matrix, against those stale clips.
// The ancestor clip is now tested in the parent's frame, which no
// matrix changes.
func TestTransformRenderOnlyTurnKeepsAncestorClip(t *testing.T) {
	parent := &Shape{
		shapeType: shapeRectangle, Opacity: 1, Clip: true,
		X: 0, Y: 0, Width: 100, Height: 100,
	}
	child := xfShape(0, 0, 100, 100, Transform{})
	tr := Transform{}
	child.xf.fn = func() Transform { return tr }
	layout := &Layout{Shape: parent, Children: []Layout{{Shape: child}}}
	w := makeWindow()
	child.xf.update(child, w)
	layoutSetShapeClips(layout, makeClip(0, 0, 400, 400))
	// A render-only frame moves the view right by 50.
	tr = Translate(50, 0)
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 400, 400), w)
	// Drawn at x 50..150, cut at 100 by the parent.
	if x, y := xfCoordsInverse(child, 75, 50); !child.PointInShape(x, y) {
		t.Fatal("point drawn inside the parent misses")
	}
	if x, y := xfCoordsInverse(child, 125, 50); child.PointInShape(x, y) {
		t.Fatal("point drawn past the parent's clip hits")
	}
}

// Regression: hover tests no ancestor on the way down, so a corner of
// a turned card that a clipping parent cuts off took the hover.
func TestTransformHoverCutByClippingParent(t *testing.T) {
	var hovered bool
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	view := func(*Window) View {
		hovered = false
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID: "clip", Clip: true, Width: 200, Height: 100,
					Sizing: FixedFixed, Padding: PaddingNone, SizeBorder: NoBorder,
					Content: []View{
						Row(ContainerCfg{
							ID: "h", Width: 100, Height: 40, Sizing: FixedFixed,
							Transform: Translate(150, 0),
							OnHover:   func(ctx EventCtx) { hovered = true },
						}),
					},
				}),
			},
		})
	}
	w.TestRender(view)
	clip := mustShape(t, w, "clip")
	// The row is drawn at x 150..250; the parent clips at x 200.
	y := clip.Y + 20
	w.EventFn(&Event{Type: EventMouseMove, MouseX: clip.X + 175, MouseY: y})
	w.settle()
	if !hovered {
		t.Fatal("hover on the visible part did not fire")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: clip.X + 225, MouseY: y})
	w.settle()
	if hovered {
		t.Fatal("hover on the part the parent cuts off fired")
	}
}

// Regression: a press inside a transformed view reaches the widget in
// the view's frame, and a mouse lock taken there used to get its moves
// in window coordinates, so the drag jumped by the transform.
func TestTransformMouseLockMovesInViewFrame(t *testing.T) {
	var downX, moveX, upX float32
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	view := func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					Padding:   PaddingNone,
					Transform: Translate(200, 0),
					Content: []View{
						Row(ContainerCfg{
							ID: "d", Width: 100, Height: 40, Sizing: FixedFixed,
							OnClick: func(ctx EventCtx) {
								downX = ctx.Layout.Shape.X + ctx.Event.MouseX
								ctx.Window.MouseLock(MouseLockCfg{
									MouseMove: func(c EventCtx) { moveX = c.Event.MouseX },
									MouseUp: func(c EventCtx) {
										upX = c.Event.MouseX
										c.Window.MouseUnlock()
									},
								})
								ctx.Consume()
							},
						}),
					},
				}),
			},
		})
	}
	w.TestRender(view)
	d := mustShape(t, w, "d")
	cy := d.Y + 20
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: d.X + 210, MouseY: cy})
	w.EventFn(&Event{Type: EventMouseMove, MouseX: d.X + 230, MouseY: cy})
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: d.X + 240, MouseY: cy})
	if downX != d.X+10 {
		t.Fatalf("press at %v, want %v in the view frame", downX, d.X+10)
	}
	if moveX != d.X+30 || upX != d.X+40 {
		t.Fatalf("locked move at %v, up at %v; want %v, %v in the view frame",
			moveX, upX, d.X+30, d.X+40)
	}
	if w.viewState.mouseLockXfOn {
		t.Fatal("unlock left the lock frame on")
	}
}

// An identity transform emits no bracket. Its subtree must still take
// its clips from the render walk, not from layout clips cut through a
// transform it no longer has.
func TestRenderTransformIdentityKeepsParentClip(t *testing.T) {
	w := makeWindow()
	parent := &Shape{
		shapeType: shapeRectangle, Opacity: 1, Clip: true,
		X: 0, Y: 0, Width: 100, Height: 100,
	}
	parent.shapeClip = makeClip(0, 0, 100, 100)
	inner := &Shape{
		shapeType: shapeRectangle, Opacity: 1, Clip: true,
		X: 0, Y: 0, Width: 300, Height: 300,
	}
	// A layout clip that escapes the parent, as one cut through a stale
	// transform can.
	inner.shapeClip = makeClip(0, 0, 300, 300)
	mid := xfShape(0, 0, 300, 300, Transform{})
	mid.xf.fn = func() Transform { return Transform{} }
	layout := &Layout{Shape: parent, Children: []Layout{{
		Shape: mid, Children: []Layout{{Shape: inner}},
	}}}
	renderLayout(layout, ColorTransparent, makeClip(0, 0, 400, 400), w)
	var clips []RenderCmd
	for _, r := range w.renderers {
		if r.Kind == RenderTransformBegin || r.Kind == RenderTransformEnd {
			t.Fatalf("identity transform emitted %v", r.Kind)
		}
		if r.Kind == RenderClip {
			clips = append(clips, r)
		}
	}
	if len(clips) == 0 {
		t.Fatal("no clips emitted")
	}
	// The last clip restores the window clip after the parent.
	for _, r := range clips[:len(clips)-1] {
		if r.X+r.W > 100.01 || r.Y+r.H > 100.01 {
			t.Fatalf("clip %v,%v %vx%v escapes the parent", r.X, r.Y, r.W, r.H)
		}
	}
}

// A text input inside a turned card takes focus from a click where it is
// drawn, and then takes typed text.
func TestTransformClickFocusesInputThenTypes(t *testing.T) {
	text := ""
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	view := func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID:        "card",
					Padding:   PaddingNone,
					Transform: Translate(150, 100).Rotate(20),
					Content: []View{
						Input(InputCfg{
							ID: "in", Text: text, Width: 120,
							OnTextChanged: func(s string, _ EventCtx) { text = s },
						}),
					},
				}),
			},
		})
	}
	w.TestRender(view)
	id := ScopeID("card", "in")
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatal("no input")
	}
	s := ly.Shape
	x, y, ok := xfLocalToWindow(ly, s.X+s.Width/2, s.Y+s.Height/2)
	if !ok {
		t.Fatal("input center is behind the viewer")
	}
	down := Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y}
	w.EventFn(&down)
	w.settle()
	if !w.IsFocus(id) {
		t.Fatalf("click where the input is drawn did not focus it (focus %q)", w.FocusID())
	}
	w.EventFn(&Event{Type: EventChar, CharCode: 'h'})
	w.settle()
	w.EventFn(&Event{Type: EventChar, CharCode: 'i'})
	w.settle()
	if text != "hi" {
		t.Fatalf("text = %q, want hi", text)
	}
}

// xfClippedCardView is a 200x100 clipping box that holds a row moved
// right by dx.
func xfClippedCardView(dx float32, clicks *int) func(*Window) View {
	return func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID: "clip", Clip: true, Width: 200, Height: 100,
					Sizing: FixedFixed, Padding: PaddingNone, SizeBorder: NoBorder,
					Content: []View{
						Row(ContainerCfg{
							ID: "r", Width: 100, Height: 40, Sizing: FixedFixed,
							Transform: Translate(dx, 0),
							OnClick: func(ctx EventCtx) {
								*clicks++
								ctx.Consume()
							},
						}),
					},
				}),
			},
		})
	}
}

// Regression: TestClick aimed at the clip center in the widget's frame.
// Inside a transform the clip is not cut by the ancestors, so a widget
// moved fully out of its clipping parent was reported clickable, and one
// moved partly out was aimed at a point the parent cuts off.
func TestTransformTestClickRespectsAncestorClip(t *testing.T) {
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfClippedCardView(150, &clicks))
	id := ScopeID("clip", "r")
	if err := w.TestClick(id); err != nil {
		t.Fatalf("partly visible widget: %v", err)
	}
	if clicks != 1 {
		t.Fatalf("partly visible widget: %d clicks, want 1", clicks)
	}
	w.TestRender(xfClippedCardView(300, &clicks))
	if err := w.TestClick(id); !errors.Is(err, errTestNotVisible) {
		t.Fatalf("widget moved out of its parent: err = %v, want not visible", err)
	}
}

// Regression: the unconsumed-event sweep aimed its probes at the clip
// center in the widget's own frame, so it never reached a transformed
// widget.
func TestTransformSweepReachesTransformedWidget(t *testing.T) {
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.SetView(xfClippedCardView(80, &clicks))
	w.TestUnconsumedEvents()
	if clicks == 0 {
		t.Fatal("the sweep's press did not reach the transformed widget")
	}
}

// Regression: the mouse lock frame held only the view transforms on the
// press path, not the quarter turns. A lock taken under both then got
// moves in another frame than the press: a move at the press point must
// arrive where the press did.
func TestTransformMouseLockUnderQuarterTurn(t *testing.T) {
	var pressX, pressY, moveX, moveY float32
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 400})
	view := func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				RotatedBox(RotatedBoxCfg{
					QuarterTurns: 1,
					Content: Column(ContainerCfg{
						Padding:   PaddingNone,
						Transform: Translate(30, 10),
						Content: []View{
							Row(ContainerCfg{
								ID: "d", Width: 120, Height: 60, Sizing: FixedFixed,
								OnClick: func(ctx EventCtx) {
									pressX = ctx.Layout.Shape.X + ctx.Event.MouseX
									pressY = ctx.Layout.Shape.Y + ctx.Event.MouseY
									ctx.Window.MouseLock(MouseLockCfg{
										MouseMove: func(c EventCtx) {
											moveX, moveY = c.Event.MouseX, c.Event.MouseY
										},
										MouseUp: func(c EventCtx) { c.Window.MouseUnlock() },
									})
									ctx.Consume()
								},
							}),
						},
					}),
				}),
			},
		})
	}
	w.TestRender(view)
	ly, ok := w.layout.FindByID("d")
	if !ok {
		t.Fatal("no d")
	}
	s := ly.Shape
	x, y, ok := xfLocalToWindow(ly, s.X+s.Width/2, s.Y+s.Height/2)
	if !ok {
		t.Fatal("center not on screen")
	}
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y})
	if pressX == 0 && pressY == 0 {
		t.Fatal("press did not reach the widget")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	if math.Abs(float64(moveX-pressX)) > 0.01 || math.Abs(float64(moveY-pressY)) > 0.01 {
		t.Fatalf("locked move at %v,%v; press at %v,%v", moveX, moveY, pressX, pressY)
	}
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: x, MouseY: y})
}

// Regression: a tap on a drag-scroll container inside a transform was
// replayed from the root with the press position in the view's frame,
// and the root mapped it through the transform a second time.
func TestTransformDragScrollTapReplay(t *testing.T) {
	clicks := 0
	w := dragScrollTestWindow(t, scrollBoth, OnClickChild{onClick: &clicks})
	v := w.layout.Children[0].Shape
	v.xf = &shapeTransform{static: Translate(200, 0)}
	v.xf.update(v, w)
	v.xf.outer = drawClip{Width: 800, Height: 600}
	// The node child is at (20,20)-(80,40) in the view's frame, drawn
	// 200 to the right.
	dragScrollDown(w, 240, 30)
	if w.viewState.dragPan.scrollID != "view" {
		t.Fatal("press inside the drawn viewport was not claimed")
	}
	dragScrollUp(w, 240, 30)
	if clicks != 1 {
		t.Fatalf("tap replay: %d clicks, want 1", clicks)
	}
}

// Regression: the nested claim walk tested a nested transformed
// drag-scroll container at its untransformed box, so the outer one took
// the press.
func TestTransformDragScrollNestedClaim(t *testing.T) {
	w := &Window{}
	inner := Layout{Shape: &Shape{
		shapeType: shapeRectangle, Scrollable: true, DragScroll: true,
		ID: "inner", X: 20, Y: 20, Width: 60, Height: 60, ScrollMode: scrollBoth,
		shapeClip: drawClip{X: 20, Y: 20, Width: 60, Height: 60},
		xf:        &shapeTransform{static: Translate(30, 0)},
	}}
	is := inner.Shape
	is.xf.update(is, w)
	is.xf.outer = drawClip{X: 10, Y: 10, Width: 100, Height: 100}
	w.layout = Layout{
		Shape: &Shape{shapeType: shapeRectangle, shapeClip: drawClip{Width: 800, Height: 600}},
		Children: []Layout{{Shape: &Shape{
			shapeType: shapeRectangle, Scrollable: true, DragScroll: true,
			ID: "outer", X: 10, Y: 10, Width: 100, Height: 100, ScrollMode: scrollBoth,
			shapeClip: drawClip{X: 10, Y: 10, Width: 100, Height: 100},
		}, Children: []Layout{inner}}},
	}
	// Drawn inside the inner viewport (x 50..110), outside its
	// untransformed box (x 20..80).
	dragScrollDown(w, 105, 50)
	if got := w.viewState.dragPan.scrollID; got != "inner" {
		t.Fatalf("claim = %q, want inner", got)
	}
	dragScrollUp(w, 105, 50)
}

// Regression: the focused-scroll path handed the callback the pointer
// from window coordinates, the fallback walk from the view's frame.
func TestTransformFocusedScrollSameFrame(t *testing.T) {
	var gotX float32
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID: "card", Padding: PaddingNone, Transform: Translate(200, 0),
					Content: []View{
						Row(ContainerCfg{
							ID: "s", Width: 100, Height: 40, Sizing: FixedFixed,
							Focusable: true,
							OnMouseScroll: func(ctx EventCtx) {
								gotX = ctx.Event.MouseX
								ctx.Consume()
							},
						}),
					},
				}),
			},
		})
	})
	id := ScopeID("card", "s")
	s := mustShape(t, w, id)
	x, y := s.X+210, s.Y+20
	mouseScrollHandler(&w.layout, &Event{MouseX: x, MouseY: y, ScrollY: 1}, w)
	unfocused := gotX
	w.SetFocus(id)
	mouseScrollHandler(&w.layout, &Event{MouseX: x, MouseY: y, ScrollY: 1}, w)
	if gotX != unfocused || unfocused != 10 {
		t.Fatalf("focused MouseX = %v, unfocused %v, want 10 both", gotX, unfocused)
	}
}

// Regression: the blur check tested the press in window coordinates
// against the focused widget's box in its own frame. A press on empty
// space over the untransformed box kept focus.
func TestTransformPressOutsideDrawnInputBlurs(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID: "card", Padding: PaddingNone, Transform: Translate(200, 100),
					Content: []View{Input(InputCfg{ID: "in", Width: 120})},
				}),
			},
		})
	})
	id := ScopeID("card", "in")
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	if !w.IsFocus(id) {
		t.Fatal("click did not focus the input")
	}
	cx, cy := xfCenter(mustShape(t, w, id))
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: cx, MouseY: cy})
	w.settle()
	if w.IsFocus(id) {
		t.Fatal("press on empty space over the untransformed box kept focus")
	}
}

// Regression: the tooltip hook compared the window pointer with the
// shape's box in its own frame.
func TestTransformTooltipHoverAtDrawnPosition(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	view := func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					Padding: PaddingNone, Transform: Translate(200, 0),
					Content: []View{
						WithTooltip(w, WithTooltipCfg{
							ID: "tip", Text: "hi",
							Content: []View{Row(ContainerCfg{ID: "box", Width: 100, Height: 40, Sizing: FixedFixed})},
						}),
					},
				}),
			},
		})
	}
	w.TestRender(view)
	cx, cy := xfCenter(mustShape(t, w, "box"))
	w.EventFn(&Event{Type: EventMouseMove, MouseX: cx, MouseY: cy})
	w.settle()
	if w.viewState.tooltip.hoverID != "" {
		t.Fatal("hover over the untransformed box started the tooltip")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: cx + 200, MouseY: cy})
	w.settle()
	if w.viewState.tooltip.hoverID != "tip" {
		t.Fatalf("hover where it is drawn: hoverID = %q, want tip", w.viewState.tooltip.hoverID)
	}
}

// Regression: the unconsumed-event sweep tested ancestors with the
// pointer in the transformed child's frame, so an ancestor that would
// get the click went unreported.
func TestTransformSweepAncestorInItsOwnFrame(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.SetView(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone,
			Content: []View{
				Row(ContainerCfg{
					ID: "outer", Width: 100, Height: 100, Sizing: FixedFixed,
					Padding: PaddingNone, SizeBorder: NoBorder,
					OnClick: func(ctx EventCtx) { ctx.Consume() },
					Content: []View{
						Row(ContainerCfg{
							ID: "card", Width: 200, Height: 40, Sizing: FixedFixed,
							Transform: Translate(-100, 0),
							OnClick:   func(EventCtx) {},
						}),
					},
				}),
			},
		})
	})
	found := w.TestUnconsumedEvents()
	for _, f := range found {
		if strings.Contains(f, "outer") {
			return
		}
	}
	t.Fatalf("no finding names the outer handler: %q", found)
}

// Regression: a MouseLock from inside a locked callback dropped the
// drag's frame, so the drag switched to window coordinates mid way.
func TestTransformRelockKeepsFrame(t *testing.T) {
	var moveX float32
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	second := MouseLockCfg{
		MouseMove: func(c EventCtx) { moveX = c.Event.MouseX },
		MouseUp:   func(c EventCtx) { c.Window.MouseUnlock() },
	}
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					Padding: PaddingNone, Transform: Translate(200, 0),
					Content: []View{
						Row(ContainerCfg{
							ID: "d", Width: 100, Height: 40, Sizing: FixedFixed,
							OnClick: func(ctx EventCtx) {
								ctx.Window.MouseLock(MouseLockCfg{
									MouseMove: func(c EventCtx) { c.Window.MouseLock(second) },
								})
								ctx.Consume()
							},
						}),
					},
				}),
			},
		})
	})
	d := mustShape(t, w, "d")
	y := d.Y + 20
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: d.X + 210, MouseY: y})
	w.EventFn(&Event{Type: EventMouseMove, MouseX: d.X + 220, MouseY: y})
	w.EventFn(&Event{Type: EventMouseMove, MouseX: d.X + 230, MouseY: y})
	if moveX != d.X+30 {
		t.Fatalf("move after relock at %v, want %v in the view frame", moveX, d.X+30)
	}
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: d.X + 230, MouseY: y})
}
