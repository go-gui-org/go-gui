package gui

import (
	"math"
	"strings"
	"testing"
)

// Bounds consumers and the coordinate API under view transforms (#1019,
// phase 5).

func TestLocalToWindowTranslated(t *testing.T) {
	tr := Translate(200, 50)
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	b := mustShape(t, w, ScopeID("card", "b"))
	x, y, ok := w.LocalToWindow(ScopeID("card", "b"), 10, 5)
	if !ok || !xfNear(x, b.X+10+200) || !xfNear(y, b.Y+5+50) {
		t.Fatalf("LocalToWindow = (%v, %v, %v), want (%v, %v, true)",
			x, y, ok, b.X+210, b.Y+55)
	}
	lx, ly, ok := w.WindowToLocal(ScopeID("card", "b"), x, y)
	if !ok || !xfNear(lx, 10) || !xfNear(ly, 5) {
		t.Fatalf("WindowToLocal = (%v, %v, %v), want (10, 5, true)", lx, ly, ok)
	}
}

// A non-finite point has no place in the window or in a box, even with
// no transform on the path to map it through.
func TestLocalToWindowNonFiniteFails(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{Button(ButtonCfg{ID: "b", Width: 100, Height: 40})},
		})
	})
	id := "b"
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	if _, _, ok := w.LocalToWindow(id, nan, 0); ok {
		t.Error("LocalToWindow(NaN) ok, want false")
	}
	if _, _, ok := w.LocalToWindow(id, 0, inf); ok {
		t.Error("LocalToWindow(Inf) ok, want false")
	}
	if _, _, ok := w.WindowToLocal(id, nan, 0); ok {
		t.Error("WindowToLocal(NaN) ok, want false")
	}
	if _, _, ok := w.WindowToLocal(id, 0, inf); ok {
		t.Error("WindowToLocal(Inf) ok, want false")
	}
}

func TestLocalToWindowRotated(t *testing.T) {
	tr := Rotate(90)
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	card := mustShape(t, w, "card")
	b := mustShape(t, w, ScopeID("card", "b"))
	ccx, ccy := xfCenter(card)
	// The top-left corner of the button, turned 90° clockwise about the
	// center of the card: (dx, dy) -> (-dy, dx).
	dx, dy := b.X-ccx, b.Y-ccy
	x, y, ok := w.LocalToWindow(ScopeID("card", "b"), 0, 0)
	if !ok || !xfNear(x, ccx-dy) || !xfNear(y, ccy+dx) {
		t.Fatalf("LocalToWindow = (%v, %v, %v), want (%v, %v, true)",
			x, y, ok, ccx-dy, ccy+dx)
	}
	lx, ly, ok := w.WindowToLocal(ScopeID("card", "b"), x, y)
	if !ok || !xfNear(lx, 0) || !xfNear(ly, 0) {
		t.Fatalf("WindowToLocal = (%v, %v, %v), want (0, 0, true)", lx, ly, ok)
	}
}

func TestLocalToWindowNoTransform(t *testing.T) {
	var tr Transform
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	b := mustShape(t, w, ScopeID("card", "b"))
	x, y, ok := w.LocalToWindow(ScopeID("card", "b"), 3, 4)
	if !ok || x != b.X+3 || y != b.Y+4 {
		t.Fatalf("LocalToWindow = (%v, %v, %v), want (%v, %v, true)",
			x, y, ok, b.X+3, b.Y+4)
	}
}

func TestLocalToWindowMissAndHidden(t *testing.T) {
	tr := RotateY(180).BackfaceHidden()
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	if _, _, ok := w.LocalToWindow("nope", 0, 0); ok {
		t.Fatal("LocalToWindow of an unknown ID: ok, want false")
	}
	if _, _, ok := w.WindowToLocal("nope", 0, 0); ok {
		t.Fatal("WindowToLocal of an unknown ID: ok, want false")
	}
	if _, _, ok := w.LocalToWindow(ScopeID("card", "b"), 0, 0); ok {
		t.Fatal("LocalToWindow under a hidden back face: ok, want false")
	}
	if _, _, ok := w.WindowToLocal(ScopeID("card", "b"), 0, 0); ok {
		t.Fatal("WindowToLocal under a hidden back face: ok, want false")
	}
}

// The accessibility tree reports where a node is drawn: the platform
// reads the bounds in window coordinates.
func TestTransformA11yBoundsAtDrawnPosition(t *testing.T) {
	tr := Translate(200, 50)
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	b := mustShape(t, w, ScopeID("card", "b"))
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&w.layout, -1, &nodes, "", &live)
	for _, n := range nodes {
		if n.Role != AccessRoleButton {
			continue
		}
		if !xfNear(n.X, b.X+200) || !xfNear(n.Y, b.Y+50) ||
			!xfNear(n.W, b.Width) || !xfNear(n.H, b.Height) {
			t.Fatalf("button bounds = (%v, %v, %v, %v), want (%v, %v, %v, %v)",
				n.X, n.Y, n.W, n.H, b.X+200, b.Y+50, b.Width, b.Height)
		}
		return
	}
	t.Fatal("no button node in the accessibility tree")
}

// A quarter turn moves a child's box too.
func TestQuarterTurnA11yBoundsAtDrawnPosition(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				RotatedBox(RotatedBoxCfg{
					QuarterTurns: 1,
					Content: Button(ButtonCfg{
						ID: "b", Width: 100, Height: 40,
					}),
				}),
			},
		})
	})
	b := mustShape(t, w, "b")
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&w.layout, -1, &nodes, "", &live)
	for _, n := range nodes {
		if n.Role != AccessRoleButton {
			continue
		}
		// Turned a quarter, the 100x40 button is drawn 40 wide, 100 tall.
		if !xfNear(n.W, b.Height) || !xfNear(n.H, b.Width) {
			t.Fatalf("button bounds = %vx%v, want %vx%v",
				n.W, n.H, b.Height, b.Width)
		}
		return
	}
	t.Fatal("no button node in the accessibility tree")
}

// imeRectSpyPlatform records the last caret rect reported.
type imeRectSpyPlatform struct {
	noopNativePlatform
	x, y, w, h int32
}

func (p *imeRectSpyPlatform) IMESetRect(x, y, w, h int32) {
	p.x, p.y, p.w, p.h = x, y, w, h
}

// The caret rect reaches the platform where the caret is drawn: mapped
// out through the open transform brackets.
func TestTransformIMERectAtDrawnPosition(t *testing.T) {
	spy := &imeRectSpyPlatform{}
	w := newTestWindow()
	w.nativePlatform = spy
	w.xfStack = append(w.xfStack, xfFrame{acc: xfMat{1, 0, 200, 0, 1, 50, 0, 0, 1}})
	imeSetRectDrawn(drawClip{X: 10, Y: 20, Width: 2, Height: 16}, w)
	if spy.x != 210 || spy.y != 70 || spy.w != 2 || spy.h != 16 {
		t.Fatalf("IME rect = (%d, %d, %d, %d), want (210, 70, 2, 16)",
			spy.x, spy.y, spy.w, spy.h)
	}
}

// A float is placed against where its parent is drawn: a tooltip or a
// menu opens by the moved view, not by its untransformed box.
func TestTransformFloatAnchorAtDrawnPosition(t *testing.T) {
	for _, tc := range []struct {
		name string
		tr   Transform
	}{
		{"none", Transform{}},
		{"translate", Translate(120, 40)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
			w.TestRender(func(*Window) View {
				return Column(ContainerCfg{
					Sizing:  FillFill,
					Padding: PaddingNone,
					Content: []View{
						Column(ContainerCfg{
							ID:         "card",
							Width:      100,
							Height:     40,
							Sizing:     FixedFixed,
							Padding:    PaddingNone,
							SizeBorder: NoBorder,
							Transform:  tc.tr,
							Content: []View{
								Column(ContainerCfg{
									ID:          "pop",
									Float:       true,
									FloatAnchor: FloatBottomLeft,
									FloatTieOff: FloatTopLeft,
									Width:       30,
									Height:      20,
									Sizing:      FixedFixed,
								}),
							},
						}),
					},
				})
			})
			card := mustShape(t, w, "card")
			pop := mustShape(t, w, ScopeID("card", "pop"))
			m, _ := tc.tr.resolve(card.X, card.Y, card.Width, card.Height)
			wantX, wantY, _ := m.apply(card.X, card.Y+card.Height)
			if !xfNear(pop.X, wantX) || !xfNear(pop.Y, wantY) {
				t.Fatalf("float at (%v, %v), want (%v, %v)", pop.X, pop.Y, wantX, wantY)
			}
		})
	}
}

// A float anchored to a view that TransformFn moves in a render-only
// frame moves with it: the render-only pass re-places floats from the
// refreshed matrices, and the result is what a full frame lays out
// (#1019). Its hit clip moves too, so the pointer follows what is drawn.
func TestTransformFloatFollowsRenderOnlyFrame(t *testing.T) {
	tr := Translate(0, 0)
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID:          "card",
					Width:       100,
					Height:      40,
					Sizing:      FixedFixed,
					Padding:     PaddingNone,
					SizeBorder:  NoBorder,
					TransformFn: func() Transform { return tr },
					Content: []View{
						Column(ContainerCfg{
							ID:          "pop",
							Float:       true,
							FloatAnchor: FloatBottomLeft,
							FloatTieOff: FloatTopLeft,
							Width:       30,
							Height:      20,
							Sizing:      FixedFixed,
						}),
					},
				}),
			},
		})
	})
	popID := ScopeID("card", "pop")
	before := mustShape(t, w, popID)
	bx, by, bclip := before.X, before.Y, before.shapeClip

	tr = Translate(120, 40)
	w.markRenderOnlyRefresh(refreshAnimation)
	w.updateRenderOnly()

	moved := mustShape(t, w, popID)
	if !xfNear(moved.X, bx+120) || !xfNear(moved.Y, by+40) {
		t.Fatalf("float at (%v, %v), want (%v, %v)",
			moved.X, moved.Y, bx+120, by+40)
	}
	if !xfNear(moved.shapeClip.X, bclip.X+120) ||
		!xfNear(moved.shapeClip.Y, bclip.Y+40) {
		t.Fatalf("float clip at (%v, %v), want (%v, %v)",
			moved.shapeClip.X, moved.shapeClip.Y, bclip.X+120, bclip.Y+40)
	}

	// A full frame at the same value lays out the same position: the
	// render-only frame draws what the full frame would.
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	full := mustShape(t, w, popID)
	if !xfNear(moved.X, full.X) || !xfNear(moved.Y, full.Y) {
		t.Fatalf("render-only float at (%v, %v), full frame at (%v, %v)",
			moved.X, moved.Y, full.X, full.Y)
	}
}

// A float nested inside a float follows a render-only move of the host's
// host: the reattach pass repeats until no float moves, so the inner
// float reads its host's new position on the next pass (#1019).
func TestTransformFloatNestedFollowsRenderOnlyFrame(t *testing.T) {
	tr := Translate(0, 0)
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID:          "card",
					Width:       100,
					Height:      40,
					Sizing:      FixedFixed,
					Padding:     PaddingNone,
					SizeBorder:  NoBorder,
					TransformFn: func() Transform { return tr },
					Content: []View{
						Column(ContainerCfg{
							ID:          "pop",
							Float:       true,
							FloatAnchor: FloatBottomLeft,
							FloatTieOff: FloatTopLeft,
							Width:       60,
							Height:      40,
							Sizing:      FixedFixed,
							Padding:     PaddingNone,
							SizeBorder:  NoBorder,
							Content: []View{
								Column(ContainerCfg{
									ID:          "tip",
									Float:       true,
									FloatAnchor: FloatBottomLeft,
									FloatTieOff: FloatTopLeft,
									Width:       20,
									Height:      10,
									Sizing:      FixedFixed,
								}),
							},
						}),
					},
				}),
			},
		})
	})
	popID := ScopeID("card", "pop")
	tipID := ScopeID("card", "pop", "tip")
	popBefore := mustShape(t, w, popID)
	tipBefore := mustShape(t, w, tipID)
	px, py := popBefore.X, popBefore.Y
	tx, ty := tipBefore.X, tipBefore.Y

	tr = Translate(120, 40)
	w.markRenderOnlyRefresh(refreshAnimation)
	w.updateRenderOnly()

	movedPop := mustShape(t, w, popID)
	movedTip := mustShape(t, w, tipID)
	if !xfNear(movedPop.X, px+120) || !xfNear(movedPop.Y, py+40) {
		t.Fatalf("outer float at (%v, %v), want (%v, %v)",
			movedPop.X, movedPop.Y, px+120, py+40)
	}
	if !xfNear(movedTip.X, tx+120) || !xfNear(movedTip.Y, ty+40) {
		t.Fatalf("nested float at (%v, %v), want (%v, %v)",
			movedTip.X, movedTip.Y, tx+120, ty+40)
	}

	// A full frame at the same value lays out the same positions.
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	fullPop := mustShape(t, w, popID)
	fullTip := mustShape(t, w, tipID)
	if !xfNear(movedPop.X, fullPop.X) || !xfNear(movedPop.Y, fullPop.Y) {
		t.Fatalf("render-only outer float at (%v, %v), full frame at (%v, %v)",
			movedPop.X, movedPop.Y, fullPop.X, fullPop.Y)
	}
	if !xfNear(movedTip.X, fullTip.X) || !xfNear(movedTip.Y, fullTip.Y) {
		t.Fatalf("render-only nested float at (%v, %v), full frame at (%v, %v)",
			movedTip.X, movedTip.Y, fullTip.X, fullTip.Y)
	}
}

// A render-only frame that moves nothing leaves floats where layout put
// them, and repeating it does not drift them (#1019).
func TestTransformFloatRenderOnlyWithoutMoveIsStable(t *testing.T) {
	tr := Translate(30, 10)
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					ID:          "card",
					Width:       100,
					Height:      40,
					Sizing:      FixedFixed,
					Padding:     PaddingNone,
					SizeBorder:  NoBorder,
					TransformFn: func() Transform { return tr },
					Content: []View{
						Column(ContainerCfg{
							ID:          "pop",
							Float:       true,
							FloatAnchor: FloatBottomLeft,
							FloatTieOff: FloatTopLeft,
							Width:       30,
							Height:      20,
							Sizing:      FixedFixed,
						}),
					},
				}),
			},
		})
	})
	popID := ScopeID("card", "pop")
	before := mustShape(t, w, popID)
	bx, by := before.X, before.Y

	w.markRenderOnlyRefresh(refreshAnimation)
	w.updateRenderOnly()
	w.updateRenderOnly()

	after := mustShape(t, w, popID)
	if !xfNear(after.X, bx) || !xfNear(after.Y, by) {
		t.Fatalf("float at (%v, %v), want stable (%v, %v)",
			after.X, after.Y, bx, by)
	}
}

// A drag that reorders a list inside a moved view: the press that arms
// the drag and the locked moves that follow are in one frame, so the
// row lands where the pointer drops it.
func TestTransformDragReorderInMovedList(t *testing.T) {
	for _, tc := range []struct {
		name string
		tr   Transform
	}{
		{"none", Transform{}},
		{"translate", Translate(120, 60)},
		{"rotate", Rotate(90)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var moved, before string
			w := NewTestWindow(t, WindowCfg{Width: 600, Height: 600})
			w.TestRender(func(*Window) View {
				return Column(ContainerCfg{
					Sizing:  FillFill,
					Padding: PaddingNone,
					Content: []View{
						Column(ContainerCfg{
							ID:         "card",
							Padding:    PaddingNone,
							SizeBorder: NoBorder,
							Transform:  tc.tr,
							Content: []View{
								ListBox(ListBoxCfg{
									ID:          "lb",
									Data:        listBoxTestData(4),
									Reorderable: true,
									OnReorder: func(m, b string, _ EventCtx) {
										moved, before = m, b
									},
								}),
							},
						}),
					},
				})
			})
			var rows []*Layout
			xfCollectRows(&w.layout, &rows)
			if len(rows) < 4 {
				t.Fatalf("found %d rows, want 4", len(rows))
			}
			// Press on row 0, drag to the lower half of row 2, release.
			sx, sy, ok := shapeHitPoint(rows[0])
			if !ok {
				t.Fatal("row 0 has no hit point")
			}
			r2 := rows[2].Shape
			ex, ey, ok := w.LocalToWindow(r2.idKey(), r2.Width/2, r2.Height*0.75)
			if !ok {
				t.Fatal("row 2 does not map")
			}
			w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: sx, MouseY: sy})
			w.settle()
			const steps = 8
			for i := 1; i <= steps; i++ {
				f := float32(i) / steps
				w.EventFn(&Event{Type: EventMouseMove,
					MouseX: sx + (ex-sx)*f, MouseY: sy + (ey-sy)*f})
				w.settle()
			}
			w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: ex, MouseY: ey})
			w.settle()
			if moved != "id-0" || before != "id-3" {
				t.Fatalf("OnReorder(%q, %q), want (\"id-0\", \"id-3\")", moved, before)
			}
		})
	}
}

// A drag ghost for a list inside a moved view tracks the pointer: the
// float offset is in the list's frame, so it is mapped through the
// transform with the anchor. Under a translation the ghost moves with
// the pointer; under a quarter turn it moves across it (#1019).
// The translate case also passes before the fix (a translation leaves
// offsets alone); it pins the common path. The rotate case fails
// before it: the ghost moved with the pointer instead of across it.
func TestTransformDragGhostFollowsPointer(t *testing.T) {
	for _, tc := range []struct {
		name string
		tr   Transform
		// dx, dy is the window pointer delta per phase.
		dx, dy float32
	}{
		{"translate", Translate(120, 60), 0, 6},
		{"rotate", Rotate(90), 6, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{Width: 600, Height: 600})
			w.TestRender(func(*Window) View {
				return Column(ContainerCfg{
					Sizing:  FillFill,
					Padding: PaddingNone,
					Content: []View{
						Column(ContainerCfg{
							ID:         "card",
							Padding:    PaddingNone,
							SizeBorder: NoBorder,
							Transform:  tc.tr,
							Content: []View{
								ListBox(ListBoxCfg{
									ID:          "lb",
									Data:        listBoxTestData(4),
									Reorderable: true,
									OnReorder: func(_, _ string, _ EventCtx) {
									},
								}),
							},
						}),
					},
				})
			})
			var rows []*Layout
			xfCollectRows(&w.layout, &rows)
			if len(rows) < 4 {
				t.Fatalf("found %d rows, want 4", len(rows))
			}
			// The middle row: room to move either way without
			// leaving it, which would start a FLIP animation and
			// freeze the ghost mid-flight.
			sx, sy, ok := shapeHitPoint(rows[1])
			if !ok {
				t.Fatal("row 1 has no hit point")
			}
			ghostPos := func() (float32, float32, bool) {
				ly, found := w.layout.findLayoutDepth(func(l Layout) bool {
					return l.Shape != nil && l.Shape.ID == "drag_reorder_ghost"
				}, 0)
				if !found {
					return 0, 0, false
				}
				return ly.Shape.X, ly.Shape.Y, true
			}
			// Interpolated steps, as a real pointer moves: a single
			// jump can outrun the drop-target tracking.
			dragTo := func(x, y float32) {
				const steps = 8
				for i := 1; i <= steps; i++ {
					f := float32(i) / steps
					w.EventFn(&Event{Type: EventMouseMove,
						MouseX: sx + (x-sx)*f, MouseY: sy + (y-sy)*f})
					w.settle()
				}
			}
			w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: sx, MouseY: sy})
			w.settle()
			// Past the drag threshold: the ghost is drawn from here on.
			// Both reads stay in the row, so no FLIP starts.
			dragTo(sx+tc.dx, sy+tc.dy)
			_, _, ok = ghostPos()
			if !ok {
				t.Fatal("no drag ghost after the threshold move")
			}
			dragTo(sx+2*tc.dx, sy+2*tc.dy)
			gx, gy, ok := ghostPos()
			if !ok {
				t.Fatal("drag ghost lost mid-drag")
			}
			// The ghost sits a grab offset behind the pointer. The
			// grab is in the list's frame, so it is turned with the
			// linear part of the card's matrix; the translation
			// cancels out.
			card := mustShape(t, w, "card")
			m, _ := tc.tr.resolve(card.X, card.Y, card.Width, card.Height)
			st := dragReorderGet(w, ScopeID("card", "lb"))
			grabX := st.startMouseX - rows[1].Shape.X
			grabY := st.startMouseY - rows[1].Shape.Y
			wantX := (sx + 2*tc.dx) - (m[0]*grabX + m[1]*grabY)
			wantY := (sy + 2*tc.dy) - (m[3]*grabX + m[4]*grabY)
			if !xfNear(gx, wantX) || !xfNear(gy, wantY) {
				t.Fatalf("ghost at (%v, %v), want (%v, %v)", gx, gy, wantX, wantY)
			}
		})
	}
}

// xfCollectRows appends, in order, the list rows: the layouts whose
// identity names a list item.
func xfCollectRows(ly *Layout, out *[]*Layout) {
	if s := ly.Shape; s != nil && s.TC != nil && strings.HasPrefix(s.TC.Text, "Name ") {
		*out = append(*out, ly.Parent)
	}
	for i := range ly.Children {
		xfCollectRows(&ly.Children[i], out)
	}
}

// A node under a hidden back face is not drawn, so it reports an empty
// box rather than its layout box, where other content is drawn.
func TestTransformA11yBoundsHiddenBackFaceEmpty(t *testing.T) {
	tr := RotateY(180).BackfaceHidden()
	var clicks int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(xfCardView(&tr, &clicks))
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&w.layout, -1, &nodes, "", &live)
	for _, n := range nodes {
		if n.Role != AccessRoleButton {
			continue
		}
		if n.W != 0 || n.H != 0 {
			t.Fatalf("hidden button bounds = (%v, %v, %v, %v), want an empty box",
				n.X, n.Y, n.W, n.H)
		}
		return
	}
	t.Fatal("no button node in the accessibility tree")
}
