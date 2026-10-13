package gui

import (
	"strings"
	"testing"
	"time"
)

func TestSidebarOpenWidth(t *testing.T) {
	w := &Window{}
	v := w.Sidebar(SidebarCfg{
		ID:    "sb",
		Open:  true,
		Width: 200,
		Content: []View{
			Text(TextCfg{Text: "nav"}),
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 200 {
		t.Errorf("open width = %f, want 200", layout.Shape.Width)
	}
}

func TestSidebarClosedWidth(t *testing.T) {
	w := &Window{}
	v := w.Sidebar(SidebarCfg{
		ID:    "sb",
		Open:  false,
		Width: 200,
		Content: []View{
			Text(TextCfg{Text: "nav"}),
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 0 {
		t.Errorf("closed width = %f, want 0", layout.Shape.Width)
	}
}

// TestSidebarClosedStaysClosedAfterFitPass guards against the
// Fixed-width-0 degrade-to-content-sizing rule (issue #94) reopening
// a fully closed sidebar: layoutWidths grows a Fixed box with Width 0
// to its content width, so the closed sidebar must render childless.
// The child here has a definite Fixed width because Text measures 0
// under the nil test TextMeasurer and would mask the regression.
func TestSidebarClosedStaysClosedAfterFitPass(t *testing.T) {
	w := &Window{}
	v := w.Sidebar(SidebarCfg{
		ID:    "sb",
		Open:  false,
		Width: 200,
		Content: []View{
			Column(ContainerCfg{
				Sizing: FixedFixed,
				Width:  120,
				Height: 40,
			}),
		},
	})
	layout := generateViewLayout(v, w)
	layoutWidths(&layout)
	if layout.Shape.Width != 0 {
		t.Errorf("closed width after fit pass = %f, want 0",
			layout.Shape.Width)
	}
	if len(layout.Children) != 0 {
		t.Errorf("closed sidebar children = %d, want 0",
			len(layout.Children))
	}
}

func TestSidebarInvisible(t *testing.T) {
	w := &Window{}
	v := w.Sidebar(SidebarCfg{
		ID:        "sb",
		Invisible: true,
		Content:   []View{Text(TextCfg{Text: "x"})},
	})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled || !layout.Shape.OverDraw {
		t.Error("invisible sidebar should be disabled+overdraw")
	}
}

func TestSidebarA11YRole(t *testing.T) {
	w := &Window{}
	v := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    true,
		Width:   100,
		Content: []View{Text(TextCfg{Text: "x"})},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleGroup {
		t.Errorf("role = %d, want Group", layout.Shape.A11YRole)
	}
}

func TestSidebarRuntimeStateInit(t *testing.T) {
	w := &Window{}
	v := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    true,
		Width:   100,
		Content: []View{Text(TextCfg{Text: "x"})},
	})
	// The state is written during generation, not by the factory call:
	// only generation knows the ID scope the key must carry.
	_ = generateViewLayout(v, w)
	sm := StateMap[string, sidebarRuntimeState](
		w, nsSidebar, capFew)
	rt, ok := sm.Get("sb")
	if !ok {
		t.Fatal("runtime state should exist")
	}
	if !rt.Initialized {
		t.Error("should be initialized")
	}
	if rt.animFrac != 1 {
		t.Errorf("animFrac = %f, want 1 (open)", rt.animFrac)
	}
}

func TestSidebarToggleToClosedPreservesState(t *testing.T) {
	w := &Window{}

	// First frame: sidebar is open.
	v := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    true,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 200 {
		t.Fatalf("initial open width = %f, want 200", layout.Shape.Width)
	}

	// Simulate animation completing to closed: manually set state.
	sm := StateMap[string, sidebarRuntimeState](w, nsSidebar, capFew)
	sm.Set("sb", sidebarRuntimeState{animFrac: 0, prevOpen: false, Initialized: true})

	// Next frame: sidebar should stay closed.
	v2 := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    false,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})
	layout2 := generateViewLayout(v2, w)
	if layout2.Shape.Width != 0 {
		t.Errorf("closed width = %f, want 0", layout2.Shape.Width)
	}

	// Verify state is still consistent.
	rt, _ := sm.Get("sb")
	if rt.animFrac != 0 {
		t.Errorf("AnimFrac = %f, want 0", rt.animFrac)
	}
	if rt.prevOpen != false {
		t.Errorf("PrevOpen = %v, want false", rt.prevOpen)
	}
}

func TestSidebarReinitAfterClearWithOpenFalse(t *testing.T) {
	w := &Window{}

	// First frame: sidebar is open, then cleared.
	_ = w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    true,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})

	// Clear the state map entirely (simulating clearViewState).
	w.viewState.registry.Clear()

	// Next frame with Open=false: re-init should produce closed sidebar.
	v := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    false,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 0 {
		t.Errorf("re-init closed width = %f, want 0", layout.Shape.Width)
	}

	sm := StateMap[string, sidebarRuntimeState](w, nsSidebar, capFew)
	rt, ok := sm.Get("sb")
	if !ok {
		t.Fatal("state should exist after re-init")
	}
	if rt.animFrac != 0 {
		t.Errorf("AnimFrac = %f, want 0", rt.animFrac)
	}
	if rt.prevOpen != false {
		t.Errorf("PrevOpen = %v, want false", rt.prevOpen)
	}
}

func TestSidebarReinitAfterClearWithOpenTrue(t *testing.T) {
	w := &Window{}

	// First frame: sidebar is open, then cleared.
	_ = w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    true,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})

	// Clear the state map entirely.
	w.viewState.registry.Clear()

	// Next frame with Open=true: re-init should produce open sidebar.
	v := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    true,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 200 {
		t.Errorf("re-init open width = %f, want 200", layout.Shape.Width)
	}

	sm := StateMap[string, sidebarRuntimeState](w, nsSidebar, capFew)
	rt, ok := sm.Get("sb")
	if !ok {
		t.Fatal("state should exist after re-init")
	}
	if rt.animFrac != 1 {
		t.Errorf("AnimFrac = %f, want 1", rt.animFrac)
	}
}

func TestSidebarNoDoubleAnimation(t *testing.T) {
	w := &Window{}

	// Open sidebar.
	v := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    true,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})
	_ = generateViewLayout(v, w)

	// Simulate closing animation halfway done.
	sm := StateMap[string, sidebarRuntimeState](w, nsSidebar, capFew)
	sm.Set("sb", sidebarRuntimeState{animFrac: 0.5, prevOpen: true, Initialized: true})

	// Now toggle closed — should start animation 0.5→0.
	v2 := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    false,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})
	layout2 := generateViewLayout(v2, w)
	// Still at 0.5 width (anim hasn't ticked yet).
	if layout2.Shape.Width != 100 {
		t.Errorf("width = %f, want 100 (half)", layout2.Shape.Width)
	}

	// PrevOpen should now be false.
	rt, _ := sm.Get("sb")
	if rt.prevOpen != false {
		t.Errorf("PrevOpen = %v, want false", rt.prevOpen)
	}

	// Same-frame second call (simulating double layout): should NOT restart animation.
	v3 := w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    false,
		Width:   200,
		Content: []View{Text(TextCfg{Text: "nav"})},
	})
	layout3 := generateViewLayout(v3, w)
	if layout3.Shape.Width != 100 {
		t.Errorf("second call width = %f, want 100", layout3.Shape.Width)
	}
	rt2, _ := sm.Get("sb")
	if rt2.prevOpen != false {
		t.Errorf("PrevOpen after second call = %v, want false", rt2.prevOpen)
	}
}

// TestSidebarStateKeyIsScoped pins the fix for the defect
// DebugUnresolvedKeys reports: the sidebar's animation state must be
// keyed by the effective ID, so two sidebars written with the same leaf
// under different panels animate independently.
//
// The factory body cannot resolve the key. It runs while the
// application builds its View tree, before generateViewLayout descends
// it, so the scope is still empty there. sidebarView.GenerateLayout
// resolves it in the phase that knows the scope.
func TestSidebarStateKeyIsScoped(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			ID:     "detail",
			Sizing: FillFill,
			Content: []View{
				w.Sidebar(SidebarCfg{
					ID:      "nav",
					Open:    true,
					Width:   100,
					Content: []View{Text(TextCfg{Text: "x"})},
				}),
			},
		})
	})

	sm := StateMap[string, sidebarRuntimeState](w, nsSidebar, capFew)
	if _, ok := sm.Get("detail:nav"); !ok {
		t.Fatalf("want state under the effective ID, keys are %q", sm.Keys())
	}
	if _, ok := sm.Get("nav"); ok {
		t.Error("state must not also live under the bare leaf")
	}
}

// A sidebar always clips its content (issue #641): it animates its
// width from 0, so unclipped content sticks out for the whole slide,
// not only at rest. The inner container clips regardless of config.
func TestSidebarAlwaysClips(t *testing.T) {
	w := &Window{}
	v := w.Sidebar(SidebarCfg{
		ID:    "sb",
		Open:  true,
		Width: 200,
		Content: []View{
			Text(TextCfg{Text: "nav"}),
		},
	})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Clip {
		t.Error("open sidebar should clip its content")
	}
}

// Content wider than the sidebar must not escape it (issue #641):
// the clip exempts the sidebar from the containment invariant, so
// the audit that found the defect stays silent on the widget.
func TestSidebarWideContentStaysInside(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	defer w.Close()
	w.TestRender(func(w *Window) View {
		return w.Sidebar(SidebarCfg{
			ID:    "sb",
			Open:  true,
			Width: 100,
			Content: []View{
				Column(ContainerCfg{
					Sizing: FixedFixed,
					Width:  150,
					Height: 20,
				}),
			},
		})
	})

	if found := w.TestFindings(DebugLayoutInvariants); len(found) != 0 {
		t.Fatalf("wide sidebar content must not escape, got %q", found)
	}
}

// The audit that found this defect must now be silent on the widget.
func TestSidebarNoUnresolvedKeyFinding(t *testing.T) {
	buf := captureDebugMask(t, DebugAll|DebugUnresolvedKeys)
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			ID:     "detail",
			Sizing: FillFill,
			Content: []View{
				w.Sidebar(SidebarCfg{
					ID:      "nav",
					Open:    true,
					Width:   100,
					Content: []View{Text(TextCfg{Text: "x"})},
				}),
			},
		})
	})

	if got := buf.String(); strings.Contains(got, nsSidebar) {
		t.Fatalf("want no sidebar finding, got %q", got)
	}
}

// Toggling a sidebar during its slide keeps the spring's motion (#1015):
// the width does not jump, and the velocity at the toggle is the velocity
// of the slide, so the reversal is smooth instead of a stop and restart.
func TestSidebarToggleMidSlideKeepsVelocity(t *testing.T) {
	w := &Window{}
	sidebar := func(open bool) {
		v := w.Sidebar(SidebarCfg{
			ID:    "sb",
			Open:  open,
			Width: 200,
			// An easing with no duration selects spring mode; the
			// zero Cfg defaults to a 300ms tween.
			TweenEasing: EaseInOutCubic,
			Content:     []View{Text(TextCfg{Text: "nav"})},
		})
		_ = generateViewLayout(v, w)
	}
	animID := ScopeID("sidebar", "sb")
	spring := func() *SpringAnimation {
		sp, ok := w.animations[animID].(*SpringAnimation)
		if !ok {
			t.Fatalf("no spring under %q", animID)
		}
		return sp
	}
	// tick runs the spring's queued OnValue against w, so the sidebar's
	// state sees the frames the animation loop would deliver.
	tick := func(sp *SpringAnimation, n int) {
		for range n {
			deferred := make([]queuedCommand, 0, 4)
			ac := newAnimationCommands(&deferred)
			updateSpring(sp, 0.016, &ac)
			for i := range deferred {
				if deferred[i].kind == queuedCommandValueFn {
					deferred[i].valueFn(deferred[i].value, w)
				}
			}
		}
	}

	sidebar(false)
	sidebar(true) // start opening
	opening := spring()
	tick(opening, 5)
	vel := opening.state.velocity
	if vel <= 0 {
		t.Fatalf("setup: opening velocity = %f, want > 0", vel)
	}
	sm := StateMap[string, sidebarRuntimeState](w, nsSidebar, capFew)
	rt, _ := sm.Get("sb")
	frac := rt.animFrac

	sidebar(false) // toggle mid-slide
	closing := spring()
	if closing.state.position != frac {
		t.Errorf("position = %f, want %f (no jump)", closing.state.position, frac)
	}
	if closing.state.velocity != vel {
		t.Errorf("velocity = %f, want %f (kept)", closing.state.velocity, vel)
	}
	// The opening momentum brakes the first closing step. A spring that
	// restarts from rest at the same point (the bug) jumps further toward
	// closed in one frame: that jerk is what the carried velocity removes.
	fromRest := &SpringAnimation{AnimID: "ref", Config: SpringStiff,
		OnValue: func(float32, *Window) {}}
	fromRest.SpringTo(frac, 0)
	tick(fromRest, 1)
	tick(closing, 1)
	rt, _ = sm.Get("sb")
	gotStep := frac - rt.animFrac
	restStep := frac - fromRest.state.position
	if gotStep >= restStep {
		t.Errorf("first closing step = %f, want < %f (from-rest step)",
			gotStep, restStep)
	}
}

// The default sidebar slides with a tween. Toggling it during the tween
// hands the motion to a spring that starts at the tween's value and
// velocity (#1015), so the width does not jump and the slide does not stop
// dead. A toggle from rest still uses the tween.
func TestSidebarDefaultToggleMidTweenKeepsVelocity(t *testing.T) {
	w := &Window{}
	sidebar := func(open bool) {
		v := w.Sidebar(SidebarCfg{
			ID:      "sb",
			Open:    open,
			Width:   200,
			Content: []View{Text(TextCfg{Text: "nav"})},
		})
		_ = generateViewLayout(v, w)
	}
	animID := ScopeID("sidebar", "sb")

	sidebar(false)
	sidebar(true) // start opening from rest: a tween
	tw, ok := w.animations[animID].(*TweenAnimation)
	if !ok {
		t.Fatalf("toggle from rest: got %T, want *TweenAnimation",
			w.animations[animID])
	}
	// Halfway through EaseInOutCubic: the fastest point of the slide. A
	// long duration makes the wall time between this sample and the
	// hand-off too small to move the value, so the check below is exact
	// to float precision even on a loaded runner.
	tw.Duration = time.Hour
	tw.start = time.Now().Add(-tw.Duration / 2)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateTween(tw, &ac)
	for i := range deferred {
		if deferred[i].kind == queuedCommandValueFn {
			deferred[i].valueFn(deferred[i].value, w)
		}
	}
	sm := StateMap[string, sidebarRuntimeState](w, nsSidebar, capFew)
	rt, _ := sm.Get("sb")
	frac := rt.animFrac

	sidebar(false) // toggle mid-tween
	sp, ok := w.animations[animID].(*SpringAnimation)
	if !ok {
		t.Fatalf("toggle mid-tween: got %T, want *SpringAnimation",
			w.animations[animID])
	}
	if f32Abs(sp.state.position-frac) > 1e-4 {
		t.Errorf("position = %f, want ~%f (no jump)", sp.state.position, frac)
	}
	// Still moving open: the spring inherits the tween's direction.
	if sp.state.velocity <= 0 {
		t.Errorf("velocity = %f, want > 0 (kept)", sp.state.velocity)
	}
	if sp.state.target != 0 {
		t.Errorf("target = %f, want 0", sp.state.target)
	}
}

// A tween that has run its course but is still registered (the loop
// retires it on its next tick) has no motion to hand over. A toggle then
// gets the configured tween, not a spring at rest.
func TestSidebarToggleAfterTweenEndsUsesTween(t *testing.T) {
	w := &Window{}
	sidebar := func(open bool) {
		v := w.Sidebar(SidebarCfg{
			ID:      "sb",
			Open:    open,
			Width:   200,
			Content: []View{Text(TextCfg{Text: "nav"})},
		})
		_ = generateViewLayout(v, w)
	}
	animID := ScopeID("sidebar", "sb")

	sidebar(false)
	sidebar(true)
	tw, ok := w.animations[animID].(*TweenAnimation)
	if !ok {
		t.Fatalf("setup: got %T, want *TweenAnimation", w.animations[animID])
	}
	tw.start = time.Now().Add(-2 * tw.Duration) // elapsed, not yet retired

	sidebar(false)
	if _, ok = w.animations[animID].(*TweenAnimation); !ok {
		t.Errorf("toggle after tween end: got %T, want *TweenAnimation",
			w.animations[animID])
	}
}
