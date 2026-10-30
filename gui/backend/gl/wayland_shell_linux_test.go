//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/decor"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

func TestWlDecorEdges(t *testing.T) {
	seen := map[uint32]bool{}
	for e := gui.EdgeTopLeft; e <= gui.EdgeLeft; e++ {
		v := wlDecorEdges[e]
		if v == decor.EdgeNone || seen[v] {
			t.Errorf("edge %d maps to %d: none or a duplicate", e, v)
		}
		seen[v] = true
	}
	if wlDecorEdges[gui.EdgeBottomRight] != decor.EdgeBottomRight {
		t.Error("bottom-right is wrong")
	}
}

// TestWlMoveResizeGuards checks that moveResize sends nothing without a
// button held on the window, or for an unknown edge. The window has no
// toplevel, so a request that got through would crash.
func TestWlMoveResizeGuards(t *testing.T) {
	b := &Backend{}
	ww := &wlWindow{d: &wlDisplay{}, b: b}
	ww.moveResize(netMoveResizeMove) // no seat
	ww.d.seat = &wlSeat{}
	ww.moveResize(netMoveResizeMove) // pointer elsewhere
	ww.d.seat.ptrFocus = b
	ww.moveResize(netMoveResizeMove) // no button held
	ww.d.seat.buttons = x11MaskButton1
	ww.moveResize(netMoveResizeMove + 1) // unknown edge (0-7 are edges, 8 is move)
}

// TestWaylandDecorations checks which frame a window gets: the
// compositor's (sway), libdecor's (weston, mutter), or none when asked.
func TestWaylandDecorations(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{Title: "framed"})
	t.Cleanup(b.Destroy)
	ww := b.plat.wl
	d := ww.d
	switch {
	case d.decoMgr.Valid():
		if !ww.deco.Valid() || ww.frame != nil {
			t.Fatalf("server-side frame expected: deco %v, libdecor %v", ww.deco.Valid(), ww.frame != nil)
		}
	case decor.Load() == nil:
		if ww.frame == nil || ww.toplevel.Valid() {
			t.Fatal("libdecor is installed, but the window has no libdecor frame")
		}
	default:
		if ww.frame != nil || !ww.toplevel.Valid() {
			t.Fatal("no libdecor: want a plain toplevel")
		}
	}
	if !ww.configured {
		t.Fatal("not configured")
	}
	settleWindow(t, b)
	ww.setTitle("renamed")
	ww.flushTitle()
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}

	none := newWaylandTestBackend(t, gui.WindowCfg{Decorations: gui.DecorationNone})
	t.Cleanup(none.Destroy)
	nw := none.plat.wl
	if nw.frame != nil || !nw.toplevel.Valid() || nw.deco.Valid() != d.decoMgr.Valid() {
		t.Fatalf("DecorationNone: libdecor %v, toplevel %v, deco %v",
			nw.frame != nil, nw.toplevel.Valid(), nw.deco.Valid())
	}
	settleWindow(t, none)
}

// TestWaylandLibdecorFrame runs the libdecor path under sway, as on a
// compositor without xdg-decoration, so the virtual pointer can drive a
// move and resize through libdecor.
func TestWaylandLibdecorFrame(t *testing.T) {
	if err := decor.Load(); err != nil {
		t.Skip(err)
	}
	// first holds the display open while its xdg-decoration manager is
	// hidden from the second window, then goes, so sway tiles the second
	// over the whole output.
	first := newWaylandTestBackend(t, gui.WindowCfg{})
	d := first.plat.wl.d
	mgr := d.decoMgr
	d.decoMgr = wl.ZxdgDecorationManagerV1{}
	b := newWaylandTestBackend(t, gui.WindowCfg{Title: "libdecor"})
	d.decoMgr = mgr
	t.Cleanup(b.Destroy)
	// Destroy frees GL objects in the current context: first's own.
	first.plat.makeCurrent()
	first.Destroy()
	ww := b.plat.wl
	if ww.frame == nil {
		t.Fatal("no libdecor frame")
	}
	settleWindow(t, b)
	ww.setTitle("still libdecor")
	ww.flushTitle()

	if mgr.Valid() { // sway: a pointer to drive
		vp := virtualPointer(t, d)
		pointAtWindow(t, b, vp)
		vp.Button(0, btnLeft, wl.PointerButtonStatePressed)
		vp.Frame()
		waitFor(t, d, "button press", func() bool { return d.seat.buttons != 0 })
		b.plat.startMoveResize(netMoveResizeMove)
		b.plat.startMoveResize(uint32(gui.EdgeTop))
		vp.Button(0, btnLeft, wl.PointerButtonStateReleased)
		vp.Frame()
	}
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}

// TestWaylandTitlebarDark switches a libdecor frame between light and
// dark (#1192): the GTK setting flips, the frame is committed one pixel
// taller and then at its size (the GTK plugin redraws on nothing less),
// and the window commits to show the repaint. Only the GTK plugin can
// switch, so sway (whose frames are its own) skips.
func TestWaylandTitlebarDark(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{Title: "variant"})
	t.Cleanup(b.Destroy)
	ww := b.plat.wl
	if ww.frame == nil {
		t.Skip("the compositor draws the frame")
	}
	start, ok := ww.d.decor.DarkVariant()
	if !ok {
		// A skip must not hide a broken plugin check: look for the
		// plugin the plain way.
		if maps, err := os.ReadFile("/proc/self/maps"); err == nil &&
			strings.Contains(string(maps), "/libdecor-gtk.so") {
			t.Fatal("libdecor's GTK plugin is loaded, but the context does not see it")
		}
		t.Skip("no GTK plugin draws the frame")
	}
	settleWindow(t, b)
	w, h := ww.frame.ContentSize()
	var commits [][2]int32
	decor.CommitFunc = func(cw, ch int32) { commits = append(commits, [2]int32{cw, ch}) }
	t.Cleanup(func() { decor.CommitFunc = nil })
	repaint := [][2]int32{{w, h + 1}, {w, h}}
	np := &nativePlatform{b: b}
	for _, dark := range []bool{!start, start} {
		ww.dirty, commits = false, nil
		np.TitlebarDark(dark)
		if got, _ := ww.d.decor.DarkVariant(); got != dark {
			t.Fatalf("TitlebarDark(%v): GTK setting %v", dark, got)
		}
		if !slices.Equal(commits, repaint) {
			t.Fatalf("TitlebarDark(%v): frame commits %v, want %v", dark, commits, repaint)
		}
		if !ww.dirty {
			t.Fatalf("TitlebarDark(%v): the window does not commit the repaint", dark)
		}
		if gw, gh := ww.frame.ContentSize(); gw != w || gh != h {
			t.Fatalf("TitlebarDark(%v): content %dx%d after the repaint, want %dx%d", dark, gw, gh, w, h)
		}
		settleWindow(t, b)
	}

	ww.dirty, commits = false, nil
	np.TitlebarDark(start)
	if ww.dirty || len(commits) != 0 {
		t.Fatalf("TitlebarDark repainted for a variant already set: commits %v", commits)
	}

	// From another goroutine the call waits for the loop's thread.
	done := make(chan struct{})
	go func() {
		np.TitlebarDark(!start)
		close(done)
	}()
	<-done
	if got, _ := ww.d.decor.DarkVariant(); got != start || len(commits) != 0 {
		t.Fatal("TitlebarDark ran GTK off the loop's thread")
	}
	ww.d.runPosted()
	if got, _ := ww.d.decor.DarkVariant(); got != !start {
		t.Fatal("the posted TitlebarDark did not run")
	}
	if !slices.Equal(commits, repaint) {
		t.Fatalf("posted TitlebarDark: frame commits %v, want %v", commits, repaint)
	}
	np.TitlebarDark(start)
	settleWindow(t, b)
}

// TestWaylandNoLibdecor checks the frameless fallback: libdecor missing on
// a compositor that draws no frames.
func TestWaylandNoLibdecor(t *testing.T) {
	first := newWaylandTestBackend(t, gui.WindowCfg{})
	t.Cleanup(first.Destroy)
	d := first.plat.wl.d
	mgr, ctx, tried := d.decoMgr, d.decor, d.decorTried
	d.decoMgr, d.decor, d.decorTried = wl.ZxdgDecorationManagerV1{}, nil, true
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	d.decoMgr, d.decor, d.decorTried = mgr, ctx, tried
	t.Cleanup(b.Destroy)
	ww := b.plat.wl
	if ww.frame != nil || !ww.toplevel.Valid() || ww.deco.Valid() {
		t.Fatalf("libdecor %v, toplevel %v, deco %v", ww.frame != nil, ww.toplevel.Valid(), ww.deco.Valid())
	}
	settleWindow(t, b)
}

// TestWlNoFrameFallback: a window that would get no frame falls back to X11
// only when it is the first window, Wayland was not forced, and an X server
// is there to fall back to (#1121).
func TestWlNoFrameFallback(t *testing.T) {
	cases := []struct {
		first, forced bool
		display       string
		want          bool
	}{
		{true, false, ":0", true},
		{true, true, ":0", false},   // GOGUI_WAYLAND=1 keeps Wayland
		{true, false, "", false},    // no X server to fall back to
		{false, false, ":0", false}, // a later window cannot move to X11
	}
	for _, c := range cases {
		if got := wlNoFrameFallback(c.first, c.forced, c.display); got != c.want {
			t.Errorf("wlNoFrameFallback(%v, %v, %q) = %v, want %v",
				c.first, c.forced, c.display, got, c.want)
		}
	}
}

// TestWaylandNoFrameFallsBack: on a compositor with no server-side frames
// and no libdecor, the first window fails with errWaylandNoFrame, so New
// and RunApp use X11 (XWayland) and the window keeps a frame (#1121).
// GOGUI_WAYLAND=1 still opens the frameless window.
func TestWaylandNoFrameFallsBack(t *testing.T) {
	requireWayland(t)
	// Hold the display open, and hide its frame providers from the windows.
	d, err := acquireWaylandDisplay()
	if err != nil {
		t.Fatal(err)
	}
	mgr, ctx, tried := d.decoMgr, d.decor, d.decorTried
	d.decoMgr, d.decor, d.decorTried = wl.ZxdgDecorationManagerV1{}, nil, true
	defer func() {
		d.decoMgr, d.decor, d.decorTried = mgr, ctx, tried
		d.release()
	}()
	t.Setenv("DISPLAY", ":99")
	t.Setenv("GOGUI_WAYLAND", "")
	reachable := x11Reachable
	defer func() { x11Reachable = reachable }()

	// DISPLAY set but no X server answers: falling back would start
	// nothing, so the window opens with no frame.
	x11Reachable = func() bool { return false }
	stale, err := newWaylandBackend(waylandTestWindow(gui.WindowCfg{}))
	if err != nil {
		t.Fatalf("stale DISPLAY: %v, want a frameless window", err)
	}
	stale.Destroy()

	x11Reachable = func() bool { return true }
	if b, err := newWaylandBackend(waylandTestWindow(gui.WindowCfg{})); !errors.Is(err, errWaylandNoFrame) {
		if b != nil {
			b.Destroy()
		}
		t.Fatalf("frameless first window: err %v, want errWaylandNoFrame", err)
	}
	if len(d.wins) != 0 {
		t.Fatalf("the failed window stayed registered: %d windows", len(d.wins))
	}

	t.Setenv("GOGUI_WAYLAND", "1")
	b, err := newWaylandBackend(waylandTestWindow(gui.WindowCfg{}))
	if err != nil {
		t.Fatalf("GOGUI_WAYLAND=1: %v", err)
	}
	defer b.Destroy()
	if ww := b.plat.wl; ww.frame != nil || !ww.toplevel.Valid() {
		t.Fatalf("GOGUI_WAYLAND=1: libdecor %v, toplevel %v", ww.frame != nil, ww.toplevel.Valid())
	}
}
