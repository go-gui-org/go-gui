package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestIsLightGroundPolarity(t *testing.T) {
	if isLightGround(gui.ThemeDark) {
		t.Error("ThemeDark classified as light ground")
	}
	if !isLightGround(gui.ThemeLight) {
		t.Error("ThemeLight classified as dark ground")
	}
}

func TestSkeletonCustomColorsDarkGround(t *testing.T) {
	base, hl := skeletonCustomColors(gui.ThemeDark)
	if base != gui.RGB(60, 60, 80) {
		t.Errorf("dark base = %v, want RGB(60, 60, 80)", base)
	}
	if hl != gui.RGB(100, 100, 140) {
		t.Errorf("dark highlight = %v, want RGB(100, 100, 140)", hl)
	}
}

func TestSkeletonCustomColorsLightGround(t *testing.T) {
	base, hl := skeletonCustomColors(gui.ThemeLight)
	if base != gui.RGB(200, 205, 218) {
		t.Errorf("light base = %v, want RGB(200, 205, 218)", base)
	}
	if hl != gui.RGB(240, 245, 255) {
		t.Errorf("light highlight = %v, want RGB(240, 245, 255)", hl)
	}
	// The light pair must read as a lifted slate, not a
	// near-black slab: its channels sit well above the dark pair.
	sum := uint(base.R) + uint(base.G) + uint(base.B)
	if sum < 450 {
		t.Errorf("light base channel sum = %d, want >= 450", sum)
	}
	if hl == base {
		t.Errorf("highlight = base %v: no visible shimmer", base)
	}
}

// TestToastAnchorDemoRoundTrip checks that each radio value in the
// toast demo builds a theme with that anchor, and that the demo reads
// the same value back from the theme.
func TestToastAnchorDemoRoundTrip(t *testing.T) {
	for _, o := range toastAnchorOptions {
		th := withToastAnchor(gui.ThemeLight, o.anchor)
		if got := toastAnchorValue(th); got != o.value {
			t.Errorf("%s: read back %q", o.value, got)
		}
		// The rest of the theme stays: only the anchor changes.
		if th.ColorBackground != gui.ThemeLight.ColorBackground {
			t.Errorf("%s: background changed", o.value)
		}
	}
}

// TestToastAnchorDemoDefault checks that a theme with no anchor set
// selects bottom right, the anchor gui uses when none is set.
func TestToastAnchorDemoDefault(t *testing.T) {
	if got := toastAnchorValue(gui.ThemeDark); got != "bottom-right" {
		t.Errorf("toastAnchorValue(ThemeDark) = %q, want bottom-right", got)
	}
}

// toastPageWindow renders the showcase at the Toast page.
func toastPageWindow(t *testing.T) *gui.Window {
	t.Helper()
	app := newShowcaseApp()
	app.SelectedGroup = groupMessages
	app.SelectedComponent = "toast"
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 1200})
	w.TestRender(mainView)
	w.TestRender(nil)
	return w
}

// TestToastPageAnchorClick clicks each anchor radio on the Toast page.
// The window theme must take that anchor. Where the toast lands for an
// anchor is pinned in gui (TestGoldenToastTopLeft).
func TestToastPageAnchorClick(t *testing.T) {
	w := toastPageWindow(t)
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
	for i, o := range toastAnchorOptions {
		id := gui.ScopeID("detail", gui.ScopeIDN("toast-anchor", "opt", i))
		if err := w.TestClick(id); err != nil {
			t.Fatalf("%s: TestClick: %v", o.value, err)
		}
		w.TestRender(nil)
		if got := toastAnchorValue(w.Theme()); got != o.value {
			t.Errorf("click %s: window anchor = %s", o.value, got)
		}
	}
	w.ToastDismissAll()
}
