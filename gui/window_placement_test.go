package gui

import "testing"

// placementRecorder logs each native call in order.
type placementRecorder struct {
	noopNativePlatform
	calls []string
}

func (p *placementRecorder) MinimizeWindow() { p.calls = append(p.calls, "minimize") }
func (p *placementRecorder) MaximizeWindow() { p.calls = append(p.calls, "maximize") }
func (p *placementRecorder) RestoreWindow()  { p.calls = append(p.calls, "restore") }
func (p *placementRecorder) SetWindowFullscreen(on bool) {
	if on {
		p.calls = append(p.calls, "fullscreen")
		return
	}
	p.calls = append(p.calls, "windowed")
}
func (p *placementRecorder) ResizeWindow(_, _ int) { p.calls = append(p.calls, "resize") }
func (p *placementRecorder) MoveWindow(_, _ int)   { p.calls = append(p.calls, "move") }

func assertCalls(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("calls = %v, want %v", got, want)
		}
	}
}

// Each verb reaches its own native method.
func TestPlacementReachesPlatform(t *testing.T) {
	t.Parallel()
	np := &placementRecorder{}
	w := NewTestWindow(t, WindowCfg{})
	w.SetNativePlatform(np)

	w.Minimize()
	w.Maximize()
	w.Restore()
	w.SetFullscreen(true)
	w.SetFullscreen(false)
	w.SetWindowSize(300, 200)
	w.SetPosition(10, 20)

	assertCalls(t, np.calls, "minimize", "maximize", "restore",
		"fullscreen", "windowed", "resize", "move")
}

// A getter reports what the OS confirmed, not what was asked for.
func TestPlacementGettersWaitForDispatch(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	w.SetNativePlatform(&placementRecorder{})

	w.Maximize()
	w.SetFullscreen(true)
	if w.IsMaximized() || w.IsFullscreen() {
		t.Error("getter changed before the OS confirmed")
	}
	DispatchWindowState(w, true, false)
	if !w.IsMaximized() || w.IsFullscreen() {
		t.Error("getters do not match the dispatch (true, false)")
	}
	// Independent flags: fullscreen keeps a maximized window maximized.
	DispatchWindowState(w, true, true)
	if !w.IsMaximized() || !w.IsFullscreen() {
		t.Error("getters do not match the dispatch (true, true)")
	}
}

// A change marks one refresh and wakes the loop, so a view reading the
// getters redraws; a repeat of the same state does neither.
func TestDispatchWindowStateRefreshesOnChange(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	w.refresh.Store(0)
	DispatchWindowState(w, false, false)
	if w.refresh.Load() != 0 || wakes != 0 {
		t.Errorf("no change: refresh %#x, wakes %d", w.refresh.Load(), wakes)
	}
	DispatchWindowState(w, true, false)
	if refreshReason(w.refresh.Load())&refreshWindowState == 0 {
		t.Errorf("change marked refresh %#x", w.refresh.Load())
	}
	if wakes != 1 {
		t.Errorf("change woke main %d times, want 1", wakes)
	}
}

func TestPlacementNilSafe(t *testing.T) {
	t.Parallel()
	var w *Window
	w.Minimize()
	w.Maximize()
	w.Restore()
	w.SetFullscreen(true)
	w.SetWindowSize(10, 10)
	w.SetPosition(1, 1)
	DispatchWindowState(w, true, true)
	if w.IsMaximized() || w.IsFullscreen() {
		t.Error("nil window reports state")
	}
	if _, _, ok := w.Position(); ok {
		t.Error("nil window reports a position")
	}
}

// The refusals live in gui so every backend agrees.
func TestPlacementRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  WindowCfg
		arm  func(*Window)
		call func(*Window)
	}{
		{"maximize fixed", WindowCfg{FixedSize: true}, nil,
			func(w *Window) { w.Maximize() }},
		{"resize fixed", WindowCfg{FixedSize: true}, nil,
			func(w *Window) { w.SetWindowSize(20, 20) }},
		{"resize zero", WindowCfg{}, nil,
			func(w *Window) { w.SetWindowSize(0, 20) }},
		{"resize maximized", WindowCfg{},
			func(w *Window) { DispatchWindowState(w, true, false) },
			func(w *Window) { w.SetWindowSize(20, 20) }},
		{"resize fullscreen", WindowCfg{},
			func(w *Window) { DispatchWindowState(w, false, true) },
			func(w *Window) { w.SetWindowSize(20, 20) }},
		{"move maximized", WindowCfg{},
			func(w *Window) { DispatchWindowState(w, true, false) },
			func(w *Window) { w.SetPosition(1, 1) }},
		{"move fullscreen", WindowCfg{},
			func(w *Window) { DispatchWindowState(w, false, true) },
			func(w *Window) { w.SetPosition(1, 1) }},
		{"minimize hidden", WindowCfg{},
			func(w *Window) { w.Hide() }, func(w *Window) { w.Minimize() }},
		{"maximize hidden", WindowCfg{},
			func(w *Window) { w.Hide() }, func(w *Window) { w.Maximize() }},
		{"restore hidden", WindowCfg{},
			func(w *Window) { w.Hide() }, func(w *Window) { w.Restore() }},
		{"fullscreen hidden", WindowCfg{},
			func(w *Window) { w.Hide() },
			func(w *Window) { w.SetFullscreen(true) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			np := &placementRecorder{}
			w := NewTestWindow(t, c.cfg)
			w.SetNativePlatform(np)
			if c.arm != nil {
				c.arm(w)
			}
			c.call(w)
			assertCalls(t, np.calls)
		})
	}
}

// resizeRecorder keeps the size the backend was handed.
type resizeRecorder struct {
	noopNativePlatform
	width, height int
}

func (p *resizeRecorder) ResizeWindow(width, height int) {
	p.width, p.height = width, height
}

func TestSetWindowSizeClampsToLimits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		cfg          WindowCfg
		inW, inH     int
		wantW, wantH int
	}{
		{"unset", WindowCfg{}, 500, 400, 500, 400},
		{"floor", WindowCfg{MinWidth: 300, MinHeight: 200}, 100, 100, 300, 200},
		{"ceiling", WindowCfg{MaxWidth: 300, MaxHeight: 200}, 900, 900, 300, 200},
		{"inside", WindowCfg{MinWidth: 100, MaxWidth: 900}, 500, 400, 500, 400},
		{"huge", WindowCfg{}, 1 << 20, 1 << 20, maxWindowExtent, maxWindowExtent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			np := &resizeRecorder{}
			w := NewTestWindow(t, c.cfg)
			w.SetNativePlatform(np)
			w.SetWindowSize(c.inW, c.inH)
			if np.width != c.wantW || np.height != c.wantH {
				t.Errorf("size = %d×%d, want %d×%d",
					np.width, np.height, c.wantW, c.wantH)
			}
		})
	}
}

// Calls before the backend attaches wait for it, normal rect first, so
// maximize and fullscreen come back to the size the app asked for.
func TestPlacementReplaysInOrder(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	w.SetFullscreen(true)
	w.Maximize()
	w.Minimize()
	w.SetPosition(5, 6)
	w.SetWindowSize(300, 200)

	np := &placementRecorder{}
	w.SetNativePlatform(np)
	assertCalls(t, np.calls,
		"resize", "move", "maximize", "fullscreen", "minimize")

	// Once only: a second attach has nothing to replay.
	np2 := &placementRecorder{}
	w.SetNativePlatform(np2)
	assertCalls(t, np2.calls)
}

// The last request of each kind wins.
func TestPlacementReplayLastWins(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	w.Maximize()
	w.Minimize()
	w.Restore()
	w.SetFullscreen(true)
	w.SetFullscreen(false)

	np := &placementRecorder{}
	w.SetNativePlatform(np)
	assertCalls(t, np.calls)
}

// No backend, no position, and no crash in a headless test window.
func TestPositionWithoutPlatform(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	if _, _, ok := w.Position(); ok {
		t.Error("Position ok = true with no platform")
	}
}

// Being resized is not input: a window the user is not typing in still
// has to learn its size. macOS drops key status during a fullscreen
// transition, so the final size of the animation arrived unfocused.
func TestResizeReachesUnfocusedWindow(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{Width: 300, Height: 200})
	w.EventFn(&Event{Type: EventUnfocused})
	if w.focused {
		t.Fatal("window still focused after EventUnfocused")
	}
	w.EventFn(&Event{Type: EventResized, WindowWidth: 640, WindowHeight: 480})
	if ww, wh := w.WindowSize(); ww != 640 || wh != 480 {
		t.Errorf("size = %d×%d, want 640×480", ww, wh)
	}
}
