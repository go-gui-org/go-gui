package gui

import "testing"

// A new window starts visible: the zero hidden flag reads as shown.
func TestWindowStartsVisible(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(WindowCfg{})
	if !w.IsVisible() {
		t.Error("IsVisible = false for a new window, want true")
	}
}

// Hide flips visibility; Show restores it. Nil platform exercises
// the flag alone, the way headless tests run.
func TestWindowHideShowNilPlatform(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(WindowCfg{})
	w.Hide()
	if w.IsVisible() {
		t.Error("IsVisible = true after Hide, want false")
	}
	w.Show()
	if !w.IsVisible() {
		t.Error("IsVisible = false after Show, want true")
	}
}

// Hide and Show reach the native platform, and Show on an already
// visible window still raises (tray "Show Window" doubles as
// bring-to-front).
func TestWindowHideShowReachPlatform(t *testing.T) {
	t.Parallel()
	np := &recordingVisibilityPlatform{}
	w := NewTestWindow(WindowCfg{})
	w.SetNativePlatform(np)

	w.Hide()
	w.Show()
	w.Show()

	if np.hides != 1 {
		t.Errorf("hides = %d, want 1", np.hides)
	}
	if np.shows != 2 {
		t.Errorf("shows = %d, want 2", np.shows)
	}
	if !w.IsVisible() {
		t.Error("IsVisible = false after Show, want true")
	}
}

// Hiding is not closing: no close request fires and the window keeps
// its App registration, so ExitOnTrayRemoved stays alive.
func TestWindowHideIsNotClose(t *testing.T) {
	t.Parallel()
	app := NewApp()
	w := NewTestWindow(WindowCfg{})
	app.Register(1, w)

	w.Hide()

	if w.CloseRequested() {
		t.Error("CloseRequested = true after Hide, want false")
	}
	if app.Window(1) != w {
		t.Error("Hide unregistered the window, want it kept")
	}
}

// A destroyed window stays gone: Show after WindowCleanup is a no-op
// and never touches a dead platform handle (issue #779).
func TestWindowShowAfterDestroyNoops(t *testing.T) {
	t.Parallel()
	np := &recordingVisibilityPlatform{}
	w := NewTestWindow(WindowCfg{})
	w.SetNativePlatform(np)

	w.Hide()
	w.WindowCleanup()
	w.Hide()
	w.Show()

	if np.shows != 0 {
		t.Errorf("shows = %d after destroy, want 0", np.shows)
	}
	if np.hides != 1 {
		t.Errorf("hides = %d after destroy, want 1 (the pre-cleanup Hide)", np.hides)
	}
	if w.IsVisible() {
		t.Error("IsVisible = true after destroy, want false")
	}
}

// Nil windows are safe: tray callbacks outlive their window when the
// last window closed and the action runs off QueueCommand.
func TestWindowVisibilityNilSafe(t *testing.T) {
	t.Parallel()
	var w *Window
	w.Hide()
	w.Show()
	if w.IsVisible() {
		t.Error("IsVisible = true for nil window, want false")
	}
}

type recordingVisibilityPlatform struct {
	noopNativePlatform
	shows int
	hides int
}

func (p *recordingVisibilityPlatform) ShowWindow() { p.shows++ }

func (p *recordingVisibilityPlatform) HideWindow() { p.hides++ }
