//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"encoding/binary"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// The Wayland window tests need a compositor. They skip without one,
// unless GOGUI_REQUIRE_WAYLAND=1 (set by scripts/wayland/test.sh), which
// turns a missing compositor, or a silent fall back to X11, into a failure.
// Run them with: scripts/wayland/test.sh ./gui/backend/gl -test.run Wayland

func requireWayland(t *testing.T) {
	t.Helper()
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return
	}
	if os.Getenv("GOGUI_REQUIRE_WAYLAND") == "1" {
		t.Fatal("GOGUI_REQUIRE_WAYLAND=1 but WAYLAND_DISPLAY is unset")
	}
	t.Skip("no WAYLAND_DISPLAY")
}

func waylandTestWindow(cfg gui.WindowCfg) *gui.Window {
	cfg.State = new(int)
	w := gui.NewWindow(cfg)
	w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
	return w
}

// newWaylandBackend opens a window through New with GOGUI_WAYLAND=1 and
// fails if New fell back to X11.
func newWaylandTestBackend(t *testing.T, cfg gui.WindowCfg) *Backend {
	t.Helper()
	requireWayland(t)
	t.Setenv("GOGUI_WAYLAND", "1")
	b, err := New(waylandTestWindow(cfg))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if b.plat.wl == nil {
		b.Destroy()
		t.Fatal("New fell back to X11 with a compositor present")
	}
	return b
}

// waitFor dispatches events until cond holds or the timeout passes.
func waitFor(t *testing.T, d *wlDisplay, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		if err := d.conn.Dispatch(50 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfigureSize(t *testing.T) {
	for _, c := range []struct{ pw, ph, cw, ch, ww, wh int32 }{
		{800, 600, 640, 480, 800, 600},                                 // compositor picks
		{0, 0, 640, 480, 640, 480},                                     // client picks: keep
		{0, 300, 640, 480, 640, 300},                                   // one axis each
		{-5, -5, 640, 480, 640, 480},                                   // broken compositor
		{0, 0, 0, 0, 1, 1},                                             // never a zero buffer
		{1 << 30, 1 << 30, 640, 480, wlMaxWindowSize, wlMaxWindowSize}, // huge
	} {
		w, h := configureSize(c.pw, c.ph, c.cw, c.ch)
		if w != c.ww || h != c.wh {
			t.Errorf("configureSize(%d,%d,%d,%d) = %d,%d, want %d,%d",
				c.pw, c.ph, c.cw, c.ch, w, h, c.ww, c.wh)
		}
	}
}

func TestWlInitialSize(t *testing.T) {
	for _, c := range []struct {
		w, h   int
		ww, wh int32
	}{
		{800, 600, 800, 600},
		{0, 0, 640, 480},    // unset
		{-5, 300, 640, 300}, // negative
		{1 << 40, 1 << 40, wlMaxWindowSize, wlMaxWindowSize},
		// Wraps to +640 as an int32: must still read as negative.
		{-(1 << 32) + 640, -(1 << 32) + 480, 640, 480},
	} {
		w, h := wlInitialSize(c.w, c.h)
		if w != c.ww || h != c.wh {
			t.Errorf("wlInitialSize(%d,%d) = %d,%d, want %d,%d", c.w, c.h, w, h, c.ww, c.wh)
		}
	}
}

func TestWlBoundSize(t *testing.T) {
	for _, c := range []struct{ w, h, s, ww, wh int32 }{
		{800, 600, 120, 800, 600},
		{16384, 16384, 120, 16384, 16384},
		{16384, 100, 240, 8192, 100},              // 2×: buffer 16384
		{16384, 16384, wlMaxScale120, 2048, 2048}, // 8×
		{16384, 16384, 0, 16384, 16384},           // broken scale: as 1×
	} {
		w, h := wlBoundSize(c.w, c.h, c.s)
		if w != c.ww || h != c.wh {
			t.Errorf("wlBoundSize(%d,%d,%d) = %d,%d, want %d,%d", c.w, c.h, c.s, w, h, c.ww, c.wh)
		}
		if wlScaled(w, max(c.s, 120)) > wlMaxWindowSize || wlScaled(h, max(c.s, 120)) > wlMaxWindowSize {
			t.Errorf("wlBoundSize(%d,%d,%d): buffer over the bound", c.w, c.h, c.s)
		}
	}
}

func TestStatesHave(t *testing.T) {
	states := make([]byte, 8)
	binary.LittleEndian.PutUint32(states, 1) // maximized
	binary.LittleEndian.PutUint32(states[4:], wl.XdgToplevelStateActivated)
	if !statesHave(states, wl.XdgToplevelStateActivated) {
		t.Error("activated not found")
	}
	if statesHave(states, 2) || statesHave(nil, 1) {
		t.Error("found a state that is not there")
	}
	// A truncated trailing entry is ignored, not read past the end.
	if statesHave(states[:7], wl.XdgToplevelStateActivated) {
		t.Error("read a truncated entry")
	}
}

// TestWaylandFallbackNoCompositor: with GOGUI_WAYLAND=1 and no compositor
// to reach, the Wayland branch declines and leaves nothing open.
func TestWaylandFallbackNoCompositor(t *testing.T) {
	t.Setenv("GOGUI_WAYLAND", "1")
	t.Setenv("WAYLAND_DISPLAY", "gogui-no-such-display")
	b, ok := tryWayland(waylandTestWindow(gui.WindowCfg{}))
	if ok || b != nil {
		t.Fatal("tryWayland succeeded with no compositor")
	}
	if wlShared != nil {
		t.Fatal("a failed connect left the shared display open")
	}
	if handled, err := runAppWayland(gui.NewApp(), nil); handled || err != nil {
		t.Fatalf("runAppWayland = %v, %v; want a fall back", handled, err)
	}
}

// TestWaylandWindow opens a window, draws into its EGL surface and checks
// the result before the swap, then closes it and the connection.
func TestWaylandWindow(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{Width: 320, Height: 200, Title: "wl test"})
	ww := b.plat.wl
	if !ww.configured || !ww.ready || ww.id() == 0 {
		t.Fatalf("window not set up: %+v", ww)
	}
	if b.physW != wlScaled(ww.logW, ww.scale120) || b.physH != wlScaled(ww.logH, ww.scale120) || b.physW <= 0 {
		t.Errorf("phys %dx%d, logical %dx%d at scale %d/120", b.physW, b.physH, ww.logW, ww.logH, ww.scale120)
	}

	b.plat.makeCurrent()
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, 0) // the window, not an MSAA target
	gogl.ClearColor(1, 0, 0, 1)
	gogl.Clear(gogl.COLOR_BUFFER_BIT)
	var px [4]byte
	gogl.ReadPixels(b.physW/2, b.physH/2, 1, 1, gogl.RGBA, gogl.UNSIGNED_BYTE, unsafe.Pointer(&px[0]))
	if px != [4]byte{255, 0, 0, 255} {
		t.Errorf("pixel after clear = %v, want red", px)
	}
	if e := gogl.GetError(); e != 0 {
		t.Errorf("GL error 0x%x", e)
	}
	b.plat.swap()

	b.Destroy()
	if wlShared != nil {
		t.Error("closing the last window left the connection open")
	}
}

// TestWaylandFrameThrottle: a rendered frame asks for a frame callback and
// the compositor answers it, which is what paces the run loop.
func TestWaylandFrameThrottle(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	defer b.Destroy()
	ww := b.plat.wl
	now := time.Now()
	ww.requestFrame(now)
	b.renderFrame(b.plat.w)
	if pending, left := ww.framePending(now); !pending || left != wlFrameTimeout {
		t.Fatalf("framePending = %v, %v after a frame", pending, left)
	}
	waitFor(t, ww.d, "frame callback", func() bool { return !ww.frameCb.Valid() })

	// Past the timeout a window renders again, without a second callback.
	ww.requestFrame(now)
	cb := ww.frameCb
	later := now.Add(wlFrameTimeout + time.Millisecond)
	if pending, _ := ww.framePending(later); pending {
		t.Error("still pending after the timeout")
	}
	ww.requestFrame(later)
	if ww.frameCb != cb || !ww.frameAt.Equal(later) {
		t.Error("a timed-out callback was replaced instead of kept")
	}
}

// TestWaylandVSyncOff: with VSyncOff no frame callback paces the window.
func TestWaylandVSyncOff(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{VSyncOff: true})
	defer b.Destroy()
	ww := b.plat.wl
	ww.requestFrame(time.Now())
	if ww.frameCb.Valid() {
		t.Error("VSyncOff window asked for a frame callback")
	}
}

// TestWaylandResize drives a size and scale change as a configure or a
// preferred_buffer_scale event would, and checks gui hears about it.
func TestWaylandResize(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{Width: 300, Height: 200})
	defer b.Destroy()
	ww := b.plat.wl
	ww.resize(400, 250, 240) // 2× in 120ths
	if b.physW != 800 || b.physH != 500 || b.dpiScale != 2 {
		t.Errorf("after resize: phys %dx%d scale %v", b.physW, b.physH, b.dpiScale)
	}
	if w, h := b.plat.w.WindowSize(); w != 400 || h != 250 {
		t.Errorf("gui window size %dx%d, want 400x250", w, h)
	}
	// A frame at the new size commits the matching buffer, so the scale
	// change is valid; a protocol error would show on the round trip.
	b.renderFrame(b.plat.w)
	if err := ww.d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}

// TestWaylandRunCloses runs the real loop until the window is closed from
// another goroutine, which also proves Wake ends the idle wait.
func TestWaylandRunCloses(t *testing.T) {
	requireWayland(t)
	t.Setenv("GOGUI_WAYLAND", "1")
	frames := 0
	cfg := gui.WindowCfg{OnInit: func(w *gui.Window) {
		d := wlShared
		go func() {
			time.Sleep(300 * time.Millisecond)
			w.Close()
			d.conn.Wake()
		}()
	}}
	w := waylandTestWindow(cfg)
	w.SetView(func(_ *gui.Window) gui.View {
		frames++
		return gui.Column(gui.ContainerCfg{})
	})
	done := make(chan error, 1)
	go func() { done <- runE(w) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after Close")
	}
	if frames == 0 {
		t.Error("the loop never built a frame")
	}
	if wlShared != nil {
		t.Error("connection left open after Run")
	}
}

// TestWaylandRunAppTwoWindows opens a second window through
// App.OpenWindow while the loop runs, then closes the main one, which
// ends the app and must take the second window with it.
func TestWaylandRunAppTwoWindows(t *testing.T) {
	requireWayland(t)
	t.Setenv("GOGUI_WAYLAND", "1")
	app := gui.NewApp()
	second := make(chan *gui.Window, 1)
	// The display is read on the loop's goroutine and handed over, so
	// the test can Wake the loop without racing on wlShared.
	disp := make(chan *wlDisplay, 1)
	first := waylandTestWindow(gui.WindowCfg{OnInit: func(*gui.Window) {
		disp <- wlShared
		app.OpenWindow(gui.WindowCfg{State: new(int), Title: "second",
			OnInit: func(w *gui.Window) { second <- w }})
	}})
	done := make(chan error, 1)
	go func() { done <- runAppE(app, first) }()

	select {
	case <-second:
	case <-time.After(10 * time.Second):
		t.Fatal("second window never opened")
	}
	if n := len(app.Windows()); n != 2 {
		t.Errorf("app has %d windows, want 2", n)
	}
	// Only the main window: with the default ExitOnMainClose the app
	// exits, and the loop must still destroy the second window.
	first.Close()
	(<-disp).conn.Wake() // Close alone does not wake an idle loop
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunApp did not return after the main window closed")
	}
	// The second window holds a display reference, so a window left
	// alive also leaves the connection open.
	if wlShared != nil {
		t.Error("RunApp returned with the second window and the connection still open")
	}
}

// TestWaylandRunAppFallbackStaysX11: once RunApp has declined Wayland, the
// X11 loop opens its windows without trying Wayland again. A retry that
// succeeded would hand the X11 event pump a Wayland window with no X
// connection. No X server is needed: the X11 open may fail, as long as it
// does not try Wayland first.
func TestWaylandRunAppFallbackStaysX11(t *testing.T) {
	t.Setenv("GOGUI_WAYLAND", "1")
	t.Setenv("WAYLAND_DISPLAY", "gogui-no-such-display")
	t.Setenv("DISPLAY", "")
	tries := 0
	old := newWaylandBackendFunc
	newWaylandBackendFunc = func(w *gui.Window) (*Backend, error) {
		tries++
		return old(w)
	}
	defer func() { newWaylandBackendFunc = old }()

	_ = runAppE(gui.NewApp(), waylandTestWindow(gui.WindowCfg{}))
	if tries != 0 {
		t.Errorf("the X11 RunApp loop tried Wayland %d times after declining it", tries)
	}
}

// TestWaylandFrameAllocs pins the cost of one paced frame: ask for a frame
// callback, present a buffer, and dispatch until the callback is done. What
// remains is purego's own allocation (see wl.TestDispatchIdleAllocs); the
// callback's decoder is built once per window, not once per frame.
//
// Each pass presents a new buffer (swap), not a bare commit: a compositor
// that repaints only on damage (wlroots) sends no callback for a commit that
// changes nothing.
func TestWaylandFrameAllocs(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	defer b.Destroy()
	ww := b.plat.wl
	b.plat.makeCurrent()
	frame := func() {
		ww.requestFrame(time.Now())
		b.plat.swap()
		deadline := time.Now().Add(3 * time.Second)
		for ww.frameCb.Valid() {
			if time.Now().After(deadline) {
				t.Fatal("no frame callback within 3s")
			}
			if err := ww.d.conn.Dispatch(100 * time.Millisecond); err != nil {
				t.Fatal(err)
			}
		}
	}
	// AllocsPerRun counts every goroutine's mallocs, and New starts
	// go-glyph's background fallback-coverage warm, which cmap-parses the
	// system fonts. A desktop with hundreds of fonts (Linux Mint: 647) is
	// still warming when the first rounds run and measured over 1,300
	// allocs per frame; the font-poor harness image finishes first. That
	// work is transient and a frame's own cost is steady, so the best of a
	// few rounds is the frame's cost.
	allocs := testing.AllocsPerRun(20, frame)
	for round := 1; round < wlFrameAllocRounds && allocs > wlFrameAllocs; round++ {
		allocs = min(allocs, testing.AllocsPerRun(20, frame))
	}
	t.Logf("allocs per paced frame: %.1f", allocs)
	if allocs > wlFrameAllocs {
		t.Errorf("a paced frame allocates %.1f times, want at most %d", allocs, wlFrameAllocs)
	}
}

// wlFrameAllocs is the measured cost of one paced frame under sway, weston
// and mutter (linux/arm64, purego v0.11.1), most of it inside
// eglSwapBuffers' purego wrapper. A decoder closure built per frame cost 2
// more (measured with a bare commit in place of the swap).
const wlFrameAllocs = 19

// wlFrameAllocRounds bounds the rounds TestWaylandFrameAllocs measures while
// background work settles. One round of 20 paced frames takes about 0.35 s
// on a 60 Hz output, so this waits at most about 3.5 s.
const wlFrameAllocRounds = 10
