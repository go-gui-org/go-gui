//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
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
