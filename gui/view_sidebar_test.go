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
