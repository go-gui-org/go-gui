//go:build windows && !js

package gl

import "testing"

// Issue #1270: a window the app hid before the backend started is
// created but not shown, and Window.Show brings it up. The window here
// is a real one, created hidden as New creates it.
func TestHiddenBeforeStartIsNotShown(t *testing.T) {
	b := newPlacedWindow(t)
	hwnd := b.plat.hwnd
	w := b.plat.w
	w.Hide() // before the platform attaches

	showAtStart(w, hwnd)
	if windowStyle(hwnd)&wsVisible != 0 {
		t.Fatal("a window hidden before the backend started was shown")
	}

	w.SetNativePlatform(&nativePlatform{b: b})
	w.Show()
	if windowStyle(hwnd)&wsVisible == 0 {
		t.Error("Show did not bring up a window that started hidden")
	}
	if !w.IsVisible() {
		t.Error("IsVisible = false after Show")
	}
}

// The control: a window the app did not hide is shown at start.
func TestVisibleAtStartIsShown(t *testing.T) {
	b := newPlacedWindow(t)
	hwnd := b.plat.hwnd
	if windowStyle(hwnd)&wsVisible != 0 {
		t.Fatal("the test window was created visible")
	}

	showAtStart(b.plat.w, hwnd)
	if windowStyle(hwnd)&wsVisible == 0 {
		t.Error("a window that was not hidden was not shown at start")
	}
}
