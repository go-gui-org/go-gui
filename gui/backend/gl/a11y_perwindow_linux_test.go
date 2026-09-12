//go:build linux && !android

package gl

import "testing"

// Each window must own its AT-SPI2 bridge: one shared bridge would
// deliver a second window's actions and tree into the first window.
func TestNativePlatformA11yBridgePerWindow(t *testing.T) {
	n1 := &nativePlatform{}
	n2 := &nativePlatform{}
	n1.A11yInit(func(_, _ int) {})
	n2.A11yInit(func(_, _ int) {})
	if n1.a11y == nil || n2.a11y == nil {
		t.Fatal("each window must own a bridge after A11yInit")
	}
	if n1.a11y == n2.a11y {
		t.Error("windows share one bridge — actions hijack across windows")
	}
	// Destroying one window's bridge must not disturb the other's.
	n1.A11yDestroy()
	if n2.a11y == nil {
		t.Error("destroying window 1 cleared window 2's bridge")
	}
}

// Re-initializing must replace the bridge, not stack a second live
// bus connection onto the first.
func TestNativePlatformA11yReinitReplacesBridge(t *testing.T) {
	n := &nativePlatform{}
	n.A11yInit(func(_, _ int) {})
	first := n.a11y
	n.A11yInit(func(_, _ int) {})
	if n.a11y == nil {
		t.Fatal("re-init must leave a live bridge")
	}
	if n.a11y == first {
		t.Error("re-init must replace the bridge, not reuse it")
	}
	// An over-count sync clamps instead of panicking.
	n.A11ySync(nil, 5, 0)
	n.A11yDestroy()
}
