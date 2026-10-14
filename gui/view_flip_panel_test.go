package gui

import (
	"math"
	"testing"
)

// flipTest holds a window with one FlipPanel ("card") whose faces each
// hold a focusable button: "front-btn" and "back-btn". OnFlip inverts
// flipped and counts the calls.
type flipTest struct {
	w       *Window
	flipped bool
	flips   int
	cfg     FlipPanelCfg
}

func newFlipTest(t *testing.T, cfg FlipPanelCfg) *flipTest {
	t.Helper()
	ft := &flipTest{cfg: cfg, flipped: cfg.Flipped}
	ft.w = NewTestWindow(t, WindowCfg{State: new(int)})
	ft.w.TestRender(ft.view)
	return ft
}

func flipTestFace(btnID string) View {
	return Column(ContainerCfg{
		Sizing:  FixedFixed,
		Width:   120,
		Height:  80,
		Padding: PadAll(10),
		Content: []View{
			Button(ButtonCfg{ID: btnID, Sizing: FixedFixed, Width: 40, Height: 20}),
		},
	})
}

func (ft *flipTest) view(*Window) View {
	c := ft.cfg
	c.ID = "card"
	c.Flipped = ft.flipped
	c.Front = flipTestFace("front-btn")
	c.Back = flipTestFace("back-btn")
	c.OnFlip = func(EventCtx) {
		ft.flipped = !ft.flipped
		ft.flips++
	}
	return Column(ContainerCfg{
		Sizing:  FillFill,
		Padding: PadAll(20),
		Content: []View{FlipPanel(c)},
	})
}

// render rebuilds the view so a changed flipped reaches the panel.
func (ft *flipTest) render() *Layout { return ft.w.TestRender(nil) }

// faces returns the panel's back and front face wrappers.
func (ft *flipTest) faces(t *testing.T) (back, front *Layout) {
	t.Helper()
	panel, ok := ft.w.layout.FindByID("card")
	if !ok || len(panel.Children) != 2 {
		t.Fatalf("panel not found or wrong child count")
	}
	return &panel.Children[0], &panel.Children[1]
}

func (ft *flipTest) state(t *testing.T) *flipPanelState {
	t.Helper()
	st, ok := StateMap[string, *flipPanelState](ft.w, nsFlipPanel, capMany).Get("card")
	if !ok {
		t.Fatal("no flip panel state")
	}
	return st
}

// flipSpring returns the registered spring of the panel, or nil.
func (ft *flipTest) flipSpring() *SpringAnimation {
	ft.w.animMu.Lock()
	defer ft.w.animMu.Unlock()
	sp, _ := ft.w.animations[nsFlipPanel+":card"].(*SpringAnimation)
	return sp
}

// The back face leaves the flow and fills the panel, so both faces
// cover the same box and the panel takes the front face's size.
func TestFlipPanelFacesShareBox(t *testing.T) {
	ft := newFlipTest(t, FlipPanelCfg{})
	panel, _ := ft.w.layout.FindByID("card")
	back, front := ft.faces(t)
	if !back.Shape.outOfFlow {
		t.Fatal("back face is in the flow")
	}
	for _, f := range []*Layout{back, front} {
		s := f.Shape
		if s.X != panel.Shape.X || s.Y != panel.Shape.Y ||
			s.Width != 120 || s.Height != 80 {
			t.Errorf("face box = (%v,%v %vx%v), want (%v,%v 120x80)",
				s.X, s.Y, s.Width, s.Height, panel.Shape.X, panel.Shape.Y)
		}
	}
	if panel.Shape.Width != 120 || panel.Shape.Height != 80 {
		t.Errorf("panel = %vx%v, want 120x80", panel.Shape.Width, panel.Shape.Height)
	}
}

// A fixed-size panel gives both faces its size.
func TestFlipPanelFixedSizingFillsBothFaces(t *testing.T) {
	ft := newFlipTest(t, FlipPanelCfg{Sizing: FixedFixed, Width: 200, Height: 150})
	back, front := ft.faces(t)
	for _, f := range []*Layout{back, front} {
		if f.Shape.Width != 200 || f.Shape.Height != 150 {
			t.Errorf("face = %vx%v, want 200x150", f.Shape.Width, f.Shape.Height)
		}
	}
}

// A panel seen for the first time rests on the face Flipped names.
func TestFlipPanelFirstFrameRestsWithoutMotion(t *testing.T) {
	for _, flipped := range []bool{false, true} {
		ft := newFlipTest(t, FlipPanelCfg{Flipped: flipped})
		want := float32(0)
		if flipped {
			want = 180
		}
		if got := ft.state(t).angle; got != want {
			t.Errorf("flipped=%v: angle = %v, want %v", flipped, got, want)
		}
		if ft.flipSpring() != nil {
			t.Errorf("flipped=%v: a spring runs on the first frame", flipped)
		}
	}
}

// A click on the panel calls OnFlip, and the changed Flipped starts a
// render-only spring toward 180°.
func TestFlipPanelClickStartsSpring(t *testing.T) {
	ft := newFlipTest(t, FlipPanelCfg{})
	if err := ft.w.TestClick("card"); err != nil {
		t.Fatal(err)
	}
	ft.render()
	if ft.flips != 1 || !ft.flipped {
		t.Fatalf("flips = %d, flipped = %v; want 1, true", ft.flips, ft.flipped)
	}
	sp := ft.flipSpring()
	if sp == nil {
		t.Fatal("no spring after the click")
	}
	if sp.state.target != 180 || sp.Refresh != AnimationRefreshRenderOnly {
		t.Errorf("spring target %v refresh %v, want 180 render-only",
			sp.state.target, sp.Refresh)
	}
	if sp.Config != flipPanelSpring {
		t.Errorf("spring config = %+v, want the default", sp.Config)
	}
}

// Enter and Space on the focused panel call OnFlip.
func TestFlipPanelKeyboardFlips(t *testing.T) {
	for _, key := range []KeyCode{KeyEnter, KeySpace} {
		ft := newFlipTest(t, FlipPanelCfg{})
		if err := ft.w.TestKey("card", key, ModNone); err != nil {
			t.Fatal(err)
		}
		if ft.flips != 1 {
			t.Errorf("key %v: flips = %d, want 1", key, ft.flips)
		}
	}
}

// A second toggle during the turn keeps the angle and the velocity: the
// new spring starts where the old one was, moving as it moved.
func TestFlipPanelRetoggleKeepsVelocity(t *testing.T) {
	ft := newFlipTest(t, FlipPanelCfg{})
	ft.flipped = true
	ft.render()
	sp := ft.flipSpring()
	if sp == nil {
		t.Fatal("no spring after the first toggle")
	}
	// Step the spring by a fixed 100 ms (Update reads the wall clock),
	// and hand the value to the panel as the loop would.
	var ac AnimationCommands
	updateSpring(sp, 0.1, &ac)
	pos, vel := sp.state.position, sp.state.velocity
	if !(pos > 0 && pos < 180) || vel <= 0 {
		t.Fatalf("mid-turn position %v velocity %v", pos, vel)
	}
	sp.OnValue(pos, ft.w)
	if got := ft.state(t).angle; got != pos {
		t.Fatalf("panel angle %v, spring position %v", got, pos)
	}

	ft.flipped = false
	ft.render()
	next := ft.flipSpring()
	if next == nil || next == sp {
		t.Fatal("the second toggle did not add a new spring")
	}
	if next.state.target != 0 {
		t.Errorf("target = %v, want 0", next.state.target)
	}
	if next.state.position != pos || next.state.velocity != vel {
		t.Errorf("new spring at %v moving %v, want %v moving %v",
			next.state.position, next.state.velocity, pos, vel)
	}
}

// Only the face that shows has tab stops.
// TestFlipPanelPartialSpringSettles: a Spring with no stiffness has no
// force toward the target, and one with no damping never loses energy.
// Either would keep the panel animating forever, so both take the
// panel's own value.
func TestFlipPanelPartialSpringSettles(t *testing.T) {
	for _, spring := range []SpringCfg{
		{Damping: 20},
		{Stiffness: 200},
		{Stiffness: -5, Damping: float32(math.NaN())},
	} {
		ft := newFlipTest(t, FlipPanelCfg{Spring: spring})
		ft.flipped = true
		ft.render()
		sp := ft.flipSpring()
		if sp == nil {
			t.Fatalf("%+v: no spring after the toggle", spring)
		}
		var ac AnimationCommands
		for range 300 {
			updateSpring(sp, 1.0/60, &ac)
		}
		if !sp.state.atRest || sp.state.position != 180 {
			t.Errorf("%+v: after 5 s at %v, at rest %v; want 180 at rest",
				spring, sp.state.position, sp.state.atRest)
		}
	}
}

func TestFlipPanelHiddenFaceOutOfTabOrder(t *testing.T) {
	ft := newFlipTest(t, FlipPanelCfg{})
	has := func(id string) bool {
		var cands []focusCandidate
		collectFocusCandidates(&ft.w.layout, &cands, map[string]struct{}{})
		for _, c := range cands {
			if c.id == id {
				return true
			}
		}
		return false
	}
	if !has("card:front-btn") || has("card:back-btn") {
		t.Errorf("front showing: front %v back %v, want true false",
			has("card:front-btn"), has("card:back-btn"))
	}
	if !has("card") {
		t.Error("the panel itself is not a tab stop")
	}
	ft.flipped = true
	ft.render()
	if has("card:front-btn") || !has("card:back-btn") {
		t.Errorf("back showing: front %v back %v, want false true",
			has("card:front-btn"), has("card:back-btn"))
	}
}

// FocusDisabled takes the panel out of the tab order.
func TestFlipPanelFocusDisabled(t *testing.T) {
	ft := newFlipTest(t, FlipPanelCfg{FocusDisabled: true})
	panel, _ := ft.w.layout.FindByID("card")
	if panel.Shape.Focusable {
		t.Error("panel is focusable with FocusDisabled")
	}
}

// Only the face that shows is in the accessibility tree.
func TestFlipPanelHiddenFaceOutOfA11yTree(t *testing.T) {
	ft := newFlipTest(t, FlipPanelCfg{
		Front: nil, Back: nil,
	})
	labels := func() map[string]bool {
		var nodes []A11yNode
		var live []liveNode
		a11yCollect(&ft.w.layout, -1, &nodes, "", &live)
		out := map[string]bool{}
		for i := range nodes {
			out[nodes[i].Label] = true
		}
		return out
	}
	ft.cfg.Front = nil
	// The test faces' buttons carry no text; label them through the
	// a11y config so the nodes can be told apart.
	ft.w.TestRender(func(*Window) View {
		face := func(id, label string) View {
			return Column(ContainerCfg{
				Sizing: FixedFixed, Width: 120, Height: 80,
				Content: []View{Button(ButtonCfg{
					ID: id, A11YCfg: A11YCfg{A11YLabel: label},
				})},
			})
		}
		return FlipPanel(FlipPanelCfg{
			ID: "card", Flipped: ft.flipped,
			Front: face("f", "front face"), Back: face("b", "back face"),
		})
	})
	got := labels()
	if !got["front face"] || got["back face"] {
		t.Errorf("front showing: labels %v", got)
	}
	ft.flipped = true
	ft.render()
	got = labels()
	if got["front face"] || !got["back face"] {
		t.Errorf("back showing: labels %v", got)
	}
	// The find walk skips the same subtrees, so every collected index
	// finds its own layout.
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&ft.w.layout, -1, &nodes, "", &live)
	for i := range nodes {
		ly := a11yFindLayout(&ft.w.layout, i)
		if ly == nil {
			t.Fatalf("node %d (%q) has no layout", i, nodes[i].Label)
		}
		if nodes[i].Label == "back face" && ly.Shape.ID != "b" {
			t.Errorf("node %d found shape %q, want b", i, ly.Shape.ID)
		}
	}
}

// When the back shows, the front face takes no pointer: a click where
// its button is reaches the back face's button on the same spot. With
// the turn, the front face's hidden back face already refuses the
// pointer; under reduced motion nothing turns, the front face is drawn
// on top, and only its inert flag sends the click through.
func TestFlipPanelHiddenFaceTakesNoPointer(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		pressed := ""
		w := NewTestWindow(t, WindowCfg{State: new(int)})
		if reduced {
			w.nativePlatform = flipReducedMotionPlatform{}
		}
		face := func(id string) View {
			return Column(ContainerCfg{
				Sizing: FixedFixed, Width: 120, Height: 80,
				Content: []View{Button(ButtonCfg{
					ID: id, Sizing: FixedFixed, Width: 120, Height: 80,
					OnClick: func(ctx EventCtx) {
						pressed = id
						ctx.Consume()
					},
				})},
			})
		}
		w.TestRender(func(*Window) View {
			return Column(ContainerCfg{
				Sizing: FillFill, Padding: PadAll(20),
				Content: []View{FlipPanel(FlipPanelCfg{
					ID: "card", Flipped: true,
					Front: face("f"), Back: face("b"),
				})},
			})
		})
		// Render once more so the transforms are resolved for input.
		w.TestRender(nil)
		x, y := float32(80), float32(60)
		down := Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y}
		w.EventFn(&down)
		up := Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: x, MouseY: y}
		w.EventFn(&up)
		if pressed != "b" {
			t.Errorf("reduced=%v: pressed %q, want b", reduced, pressed)
		}
	}
}

// Under reduced motion the faces do not turn: a toggle cross-fades them
// with a layout tween, and neither face carries a turn.
func TestFlipPanelReducedMotionFades(t *testing.T) {
	ft := &flipTest{}
	ft.w = NewTestWindow(t, WindowCfg{State: new(int)})
	ft.w.nativePlatform = flipReducedMotionPlatform{}
	ft.w.TestRender(ft.view)
	back, front := ft.faces(t)
	for _, f := range []*Layout{back, front} {
		if f.Shape.xf != nil && f.Shape.xf.fn != nil {
			t.Fatal("a face turns under reduced motion")
		}
	}
	if back.Shape.Opacity != 0 || front.Shape.Opacity != 1 {
		t.Errorf("rest opacity back %v front %v, want 0 1",
			back.Shape.Opacity, front.Shape.Opacity)
	}

	ft.flipped = true
	ft.render()
	ft.w.animMu.Lock()
	tw, ok := ft.w.animations[nsFlipPanel+":card"].(*TweenAnimation)
	ft.w.animMu.Unlock()
	if !ok {
		t.Fatal("no fade tween after the toggle")
	}
	if tw.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("fade refresh = %v, want layout", tw.RefreshKind())
	}
	if got := ft.state(t).angle; got != 180 {
		t.Errorf("angle = %v, want 180 (no turn)", got)
	}

	// Halfway through the fade both faces show at half strength, and
	// the face's children fade with it.
	ft.state(t).fade = 0.5
	ft.w.TestRender(nil)
	back, front = ft.faces(t)
	if back.Shape.Opacity != 0.5 || front.Shape.Opacity != 0.5 {
		t.Errorf("mid-fade opacity back %v front %v, want 0.5 0.5",
			back.Shape.Opacity, front.Shape.Opacity)
	}
	if c := front.Children[0].Children[0].Shape; c.Opacity != 0.5 {
		t.Errorf("front button opacity %v, want 0.5", c.Opacity)
	}
}

// The X axis turns the faces with RotateX.
// TestFlipPanelPerspectiveFallback: a Perspective that is not a
// positive finite distance takes 800, so a bad value never reaches the
// face transforms.
func TestFlipPanelPerspectiveFallback(t *testing.T) {
	inf := float32(math.Inf(1))
	for _, p := range []float32{0, -10, float32(math.NaN()), inf} {
		ft := newFlipTest(t, FlipPanelCfg{Perspective: p})
		ft.render()
		_, front := ft.faces(t)
		fn := front.Shape.xf.fn
		if fn == nil {
			t.Fatalf("perspective %v: front face has no TransformFn", p)
		}
		if got, want := fn(), flipPanelTurn(FlipAxisY, flipPanelPerspective, 0); got != want {
			t.Errorf("perspective %v: transform %+v, want %+v", p, got, want)
		}
	}
}

func TestFlipPanelTurnAxis(t *testing.T) {
	y := flipPanelTurn(FlipAxisY, 800, 30)
	x := flipPanelTurn(FlipAxisX, 800, 30)
	if y != Perspective(800).RotateY(30).BackfaceHidden() {
		t.Errorf("Y turn = %+v", y)
	}
	if x != Perspective(800).RotateX(30).BackfaceHidden() {
		t.Errorf("X turn = %+v", x)
	}
}

// flipReducedMotionPlatform is the no-op platform reporting reduced
// motion.
type flipReducedMotionPlatform struct{ noopNativePlatform }

func (flipReducedMotionPlatform) PrefersReducedMotion() bool { return true }

// A control on a face that turns away loses focus, as under the HTML
// inert attribute: focus moves to the panel, so Enter flips the panel
// and does not press the hidden button.
func TestFlipPanelHiddenFaceDropsFocus(t *testing.T) {
	for _, focusDisabled := range []bool{false, true} {
		frontPresses := 0
		flipped := false
		flips := 0
		w := NewTestWindow(t, WindowCfg{State: new(int)})
		face := func(id string, onClick func(EventCtx)) View {
			return Column(ContainerCfg{
				Sizing: FixedFixed, Width: 120, Height: 80,
				Content: []View{Button(ButtonCfg{ID: id, OnClick: onClick})},
			})
		}
		w.TestRender(func(*Window) View {
			return FlipPanel(FlipPanelCfg{
				ID: "card", Flipped: flipped, FocusDisabled: focusDisabled,
				Front: face("front-btn", func(ctx EventCtx) {
					frontPresses++
					ctx.Consume()
				}),
				Back:   face("back-btn", nil),
				OnFlip: func(EventCtx) { flips++ },
			})
		})
		w.SetFocus("card:front-btn")
		// The app turns the panel itself, not through OnFlip.
		flipped = true
		w.TestRender(nil)
		w.settle()

		want := "card"
		if focusDisabled {
			want = ""
		}
		if got := w.FocusID(); got != want {
			t.Fatalf("focusDisabled=%v: focus %q, want %q", focusDisabled, got, want)
		}
		down := Event{Type: EventKeyDown, KeyCode: KeyEnter}
		w.EventFn(&down)
		w.settle()
		if frontPresses != 0 {
			t.Errorf("focusDisabled=%v: Enter pressed the hidden front button", focusDisabled)
		}
		wantFlips := 1
		if focusDisabled {
			wantFlips = 0
		}
		if flips != wantFlips {
			t.Errorf("focusDisabled=%v: flips = %d, want %d", focusDisabled, flips, wantFlips)
		}
	}
}

// The turned-away face takes no pointer on the paths that map a window
// point by hand, not through the hit walks: a tooltip there does not
// open, layoutHitsWindowPoint misses it, and TestClick reports it as
// not visible. Under reduced motion the inert front face is drawn on
// top with no turn, so only its inert flag stops these.
func TestFlipPanelHiddenFaceNoTooltipNoHit(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		for _, flipped := range []bool{false, true} {
			w := NewTestWindow(t, WindowCfg{State: new(int)})
			if reduced {
				w.nativePlatform = flipReducedMotionPlatform{}
			}
			w.viewState.mousePosX, w.viewState.mousePosY = 60, 40
			w.TestRender(func(w *Window) View {
				return FlipPanel(FlipPanelCfg{
					ID: "card", Flipped: flipped,
					Front: Column(ContainerCfg{
						Sizing: FixedFixed, Width: 120, Height: 80,
						Content: []View{
							WithTooltip(w, WithTooltipCfg{
								ID: "tip", Text: "front tip",
								Content: []View{Button(ButtonCfg{
									ID: "front-btn", Sizing: FixedFixed,
									Width: 120, Height: 80,
									OnClick: func(EventCtx) {},
								})},
							}),
						},
					}),
					Back: Column(ContainerCfg{Sizing: FixedFixed, Width: 120, Height: 80}),
				})
			})
			w.TestRender(nil)

			// flipped=false is the control: the front face shows, so
			// its tooltip opens and its button is hit.
			wantHit := !flipped
			if got := w.viewState.tooltip.hoverID != ""; got != wantHit {
				t.Errorf("reduced=%v flipped=%v: tooltip hovering %v, want %v",
					reduced, flipped, got, wantHit)
			}
			btn, ok := w.layout.FindByID("card:front-btn")
			if !ok {
				t.Fatal("front button not found")
			}
			if got := layoutHitsWindowPoint(btn, 60, 40); got != wantHit {
				t.Errorf("reduced=%v flipped=%v: layoutHitsWindowPoint %v, want %v",
					reduced, flipped, got, wantHit)
			}
			err := w.TestClick("card:front-btn")
			if wantHit && err != nil {
				t.Errorf("reduced=%v: TestClick on the shown face: %v", reduced, err)
			}
			if !wantHit && err == nil {
				t.Errorf("reduced=%v: TestClick on the hidden face reported no error", reduced)
			}
		}
	}
}
