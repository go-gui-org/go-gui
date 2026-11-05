//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"encoding/binary"
	"os"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/decor"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

func xdgStates(states ...uint32) []byte {
	var b []byte
	for _, s := range states {
		b = binary.LittleEndian.AppendUint32(b, s)
	}
	return b
}

func TestToplevelConfigureReadsPlacement(t *testing.T) {
	t.Parallel()
	ww := &wlWindow{}
	ww.toplevelConfigure(0, 0, xdgStates(
		wl.XdgToplevelStateMaximized, wl.XdgToplevelStateFullscreen))
	if !ww.place.pendMaximized || !ww.place.pendFullscreen {
		t.Errorf("pending = %+v, want both set", ww.place)
	}
	ww.toplevelConfigure(0, 0, nil)
	if ww.place.pendMaximized || ww.place.pendFullscreen {
		t.Errorf("pending = %+v, want both clear", ww.place)
	}
}

func TestDecorStatesReadPlacement(t *testing.T) {
	t.Parallel()
	ww := &wlWindow{}
	ww.decorStates(decor.StateMaximized | decor.StateFullscreen)
	if !ww.place.pendMaximized || !ww.place.pendFullscreen {
		t.Errorf("pending = %+v, want both set", ww.place)
	}
	ww.decorStates(decor.StateActive)
	if ww.place.pendMaximized || ww.place.pendFullscreen {
		t.Errorf("pending = %+v, want both clear", ww.place)
	}
}

// A configure leaving maximized or fullscreen with no size must return
// to the floating size, not keep the maximized one.
func TestConfiguredSize(t *testing.T) {
	t.Parallel()
	maxed := wlPlacement{maximized: true, floatW: 300, floatH: 200}
	full := wlPlacement{fullscreen: true, floatW: 300, floatH: 200}
	stay := maxed
	stay.pendMaximized = true
	cases := []struct {
		name         string
		place        wlPlacement
		pendW, pendH int32
		wantW, wantH int32
	}{
		{"leave maximized, client picks", maxed, 0, 0, 300, 200},
		{"leave fullscreen, client picks", full, 0, 0, 300, 200},
		{"leave maximized, compositor picks", maxed, 640, 480, 640, 480},
		{"stay maximized", stay, 0, 0, 1280, 800},
		{"floating", wlPlacement{floatW: 300, floatH: 200}, 0, 0, 1280, 800},
		{"nothing to return to", wlPlacement{maximized: true}, 0, 0, 1280, 800},
	}
	for _, c := range cases {
		ww := &wlWindow{logW: 1280, logH: 800, place: c.place}
		if w, h := ww.configuredSize(c.pendW, c.pendH); w != c.wantW || h != c.wantH {
			t.Errorf("%s: %d×%d, want %d×%d", c.name, w, h, c.wantW, c.wantH)
		}
	}
}

// Only a floating window's size is the one to come back to.
func TestSyncPlacementKeepsFloatingSize(t *testing.T) {
	t.Parallel()
	ww := &wlWindow{b: &Backend{}}
	ww.syncPlacement(300, 200)
	ww.place.pendMaximized = true
	ww.syncPlacement(1280, 800)
	if ww.place.floatW != 300 || ww.place.floatH != 200 {
		t.Errorf("float size %d×%d, want 300×200", ww.place.floatW, ww.place.floatH)
	}
	if !ww.place.maximized {
		t.Error("pending maximize not adopted")
	}
	// A compositor dropping maximized in fullscreen does not end it.
	ww.place.pendMaximized, ww.place.pendFullscreen = false, true
	ww.syncPlacement(1280, 800)
	if !ww.place.maximized || !ww.place.fullscreen {
		t.Errorf("in fullscreen: %+v, want maximized and fullscreen", ww.place)
	}
	// Leaving with no maximized state does end it.
	ww.place.pendFullscreen = false
	ww.syncPlacement(300, 200)
	if ww.place.maximized || ww.place.fullscreen {
		t.Errorf("after leaving: %+v, want neither", ww.place)
	}
}

// pumpWindow runs posted requests and answers configures with frames, as
// the run loop does, until cond holds.
func pumpWindow(t *testing.T, b *Backend, what string, cond func() bool) {
	t.Helper()
	d := b.plat.wl.d
	render := func() {
		if b.plat.wl.dirty {
			b.plat.wl.dirty = false
			b.plat.makeCurrent()
			b.renderFrame(b.plat.w)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		d.runPosted()
		render()
		if err := d.conn.Dispatch(20 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	// Answer the configure that met cond, as the loop would.
	render()
}

// sway tiles: it ignores a maximize, and a client's own size.
func skipOnTiling(t *testing.T) {
	t.Helper()
	if os.Getenv("COMPOSITOR") == "sway" {
		t.Skip("sway tiles its windows")
	}
}

// waylandDecorations covers both paths: libdecor or a compositor frame
// for the default, and a plain xdg_toplevel with no frame.
var waylandDecorations = []struct {
	name string
	deco gui.WindowDecoration
}{
	{"framed", gui.DecorationDefault},
	{"frameless", gui.DecorationNone},
}

func TestWaylandFullscreenRoundTrip(t *testing.T) {
	for _, c := range waylandDecorations {
		t.Run(c.name, func(t *testing.T) { fullscreenRoundTrip(t, c.deco) })
	}
}

func fullscreenRoundTrip(t *testing.T, deco gui.WindowDecoration) {
	b := newWaylandTestBackend(t, gui.WindowCfg{
		Width: 300, Height: 200, Decorations: deco,
	})
	defer b.Destroy()
	settleWindow(t, b)
	w := b.plat.w
	n := &nativePlatform{b: b}
	before, _ := w.WindowSize()

	n.SetWindowFullscreen(true)
	pumpWindow(t, b, "fullscreen", w.IsFullscreen)
	if ww, wh := w.WindowSize(); ww != 1280 || wh != 800 {
		t.Errorf("fullscreen size %d×%d, want 1280×800", ww, wh)
	}

	n.SetWindowFullscreen(false)
	pumpWindow(t, b, "leaving fullscreen", func() bool { return !w.IsFullscreen() })
	if ww, _ := w.WindowSize(); ww != before {
		t.Errorf("width after fullscreen %d, want %d", ww, before)
	}
}

func TestWaylandMaximizeRoundTrip(t *testing.T) {
	skipOnTiling(t)
	for _, c := range waylandDecorations {
		t.Run(c.name, func(t *testing.T) { maximizeRoundTrip(t, c.deco) })
	}
}

func maximizeRoundTrip(t *testing.T, deco gui.WindowDecoration) {
	b := newWaylandTestBackend(t, gui.WindowCfg{
		Width: 300, Height: 200, Decorations: deco,
	})
	defer b.Destroy()
	settleWindow(t, b)
	w := b.plat.w
	n := &nativePlatform{b: b}

	n.MaximizeWindow()
	pumpWindow(t, b, "maximize", w.IsMaximized)
	if ww, _ := w.WindowSize(); ww <= 300 {
		t.Errorf("maximized width %d, want more than 300", ww)
	}

	// Fullscreen and back: still maximized, and Restore in between did
	// not touch it.
	n.SetWindowFullscreen(true)
	pumpWindow(t, b, "fullscreen", w.IsFullscreen)
	n.RestoreWindow()
	b.plat.wl.d.runPosted()
	if !w.IsMaximized() {
		t.Error("IsMaximized = false in fullscreen from maximized")
	}
	n.SetWindowFullscreen(false)
	pumpWindow(t, b, "leaving fullscreen", func() bool { return !w.IsFullscreen() })
	if !w.IsMaximized() {
		t.Error("IsMaximized = false after fullscreen from maximized")
	}

	n.RestoreWindow()
	pumpWindow(t, b, "restore", func() bool { return !w.IsMaximized() })
	if ww, wh := w.WindowSize(); ww != 300 || wh != 200 {
		t.Errorf("restored size %d×%d, want 300×200", ww, wh)
	}
}

func TestWaylandResizeTo(t *testing.T) {
	skipOnTiling(t)
	b := newWaylandTestBackend(t, gui.WindowCfg{Width: 300, Height: 200})
	defer b.Destroy()
	settleWindow(t, b)
	w := b.plat.w

	(&nativePlatform{b: b}).ResizeWindow(420, 260)
	pumpWindow(t, b, "resize", func() bool {
		ww, wh := w.WindowSize()
		return ww == 420 && wh == 260
	})
	if err := b.plat.wl.d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}

// A request for a window that went away before the loop ran is dropped.
func TestWaylandRequestAfterDestroy(t *testing.T) {
	first := newWaylandTestBackend(t, gui.WindowCfg{})
	defer first.Destroy()
	settleWindow(t, first)
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	settleWindow(t, b)
	d := first.plat.wl.d
	(&nativePlatform{b: b}).MaximizeWindow()
	b.Destroy()
	first.plat.makeCurrent()
	d.runPosted()
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}

// A Wayland client cannot place itself: no position, and a move is
// dropped before it reaches the protocol.
func TestWaylandNoPosition(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	defer b.Destroy()
	n := &nativePlatform{b: b}
	n.MoveWindow(10, 10)
	if _, _, ok := n.WindowPosition(); ok {
		t.Error("WindowPosition ok = true on Wayland")
	}
}

// A ceiling must not hold a window back from fullscreen, in any of the
// ways a window gets one, and it comes back on leaving.
func TestWaylandFullscreenIgnoresLimits(t *testing.T) {
	for _, c := range []struct {
		name string
		cfg  gui.WindowCfg
	}{
		{"fixed framed", gui.WindowCfg{Width: 300, Height: 200, FixedSize: true}},
		{"fixed frameless", gui.WindowCfg{Width: 300, Height: 200, FixedSize: true,
			Decorations: gui.DecorationNone}},
		{"max framed", gui.WindowCfg{Width: 300, Height: 200, MaxWidth: 400, MaxHeight: 300}},
		{"max frameless", gui.WindowCfg{Width: 300, Height: 200, MaxWidth: 400, MaxHeight: 300,
			Decorations: gui.DecorationNone}},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newWaylandTestBackend(t, c.cfg)
			defer b.Destroy()
			settleWindow(t, b)
			w := b.plat.w
			n := &nativePlatform{b: b}

			n.SetWindowFullscreen(true)
			pumpWindow(t, b, "fullscreen", w.IsFullscreen)
			if ww, wh := w.WindowSize(); ww != 1280 || wh != 800 {
				t.Errorf("fullscreen size %d×%d, want 1280×800", ww, wh)
			}
			if !b.plat.wl.place.lifted {
				t.Error("ceiling still on in fullscreen")
			}
			n.SetWindowFullscreen(false)
			pumpWindow(t, b, "leaving", func() bool { return !w.IsFullscreen() })
			if b.plat.wl.place.lifted {
				t.Error("ceiling not restored after leaving")
			}
			if err := b.plat.wl.d.conn.Roundtrip(); err != nil {
				t.Fatal(err)
			}
			if os.Getenv("COMPOSITOR") == "sway" {
				return // sway tiles: the window fills the output again
			}
			if ww, wh := w.WindowSize(); ww != 300 || wh != 200 {
				t.Errorf("size after leaving %d×%d, want 300×200", ww, wh)
			}
		})
	}
}
