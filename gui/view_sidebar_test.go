package gui

import "testing"

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
	_ = w.Sidebar(SidebarCfg{
		ID:      "sb",
		Open:    true,
		Width:   100,
		Content: []View{Text(TextCfg{Text: "x"})},
	})
	sm := StateMap[string, SidebarRuntimeState](
		w, nsSidebar, capFew)
	rt, ok := sm.Get("sb")
	if !ok {
		t.Fatal("runtime state should exist")
	}
	if !rt.Initialized {
		t.Error("should be initialized")
	}
	if rt.AnimFrac != 1 {
		t.Errorf("animFrac = %f, want 1 (open)", rt.AnimFrac)
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
	sm := StateMap[string, SidebarRuntimeState](w, nsSidebar, capFew)
	sm.Set("sb", SidebarRuntimeState{AnimFrac: 0, PrevOpen: false, Initialized: true})

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
	if rt.AnimFrac != 0 {
		t.Errorf("AnimFrac = %f, want 0", rt.AnimFrac)
	}
	if rt.PrevOpen != false {
		t.Errorf("PrevOpen = %v, want false", rt.PrevOpen)
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

	sm := StateMap[string, SidebarRuntimeState](w, nsSidebar, capFew)
	rt, ok := sm.Get("sb")
	if !ok {
		t.Fatal("state should exist after re-init")
	}
	if rt.AnimFrac != 0 {
		t.Errorf("AnimFrac = %f, want 0", rt.AnimFrac)
	}
	if rt.PrevOpen != false {
		t.Errorf("PrevOpen = %v, want false", rt.PrevOpen)
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

	sm := StateMap[string, SidebarRuntimeState](w, nsSidebar, capFew)
	rt, ok := sm.Get("sb")
	if !ok {
		t.Fatal("state should exist after re-init")
	}
	if rt.AnimFrac != 1 {
		t.Errorf("AnimFrac = %f, want 1", rt.AnimFrac)
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
	sm := StateMap[string, SidebarRuntimeState](w, nsSidebar, capFew)
	sm.Set("sb", SidebarRuntimeState{AnimFrac: 0.5, PrevOpen: true, Initialized: true})

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
	if rt.PrevOpen != false {
		t.Errorf("PrevOpen = %v, want false", rt.PrevOpen)
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
	if rt2.PrevOpen != false {
		t.Errorf("PrevOpen after second call = %v, want false", rt2.PrevOpen)
	}
}
