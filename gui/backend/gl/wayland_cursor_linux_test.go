//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

func TestWlCursorTables(t *testing.T) {
	for mc := gui.CursorDefault; mc <= gui.CursorGrabbing; mc++ {
		if wlCursorShape(mc) == 0 {
			t.Errorf("cursor %d has no shape", mc)
		}
		if int(mc) >= len(wlCursorNames) || len(wlCursorNames[mc]) == 0 {
			t.Errorf("cursor %d has no Xcursor names", mc)
		}
	}
	if got := wlCursorShape(gui.CursorGrabbing + 1); got != wl.WpCursorShapeDeviceV1ShapeDefault {
		t.Errorf("an unknown cursor maps to shape %d, want the default", got)
	}
}

func TestWlResizeEdges(t *testing.T) {
	seen := map[uint32]bool{}
	for e := gui.EdgeTopLeft; e <= gui.EdgeLeft; e++ {
		v := wlResizeEdges[e]
		if v == wl.XdgToplevelResizeEdgeNone || seen[v] {
			t.Errorf("edge %d maps to %d: none or a duplicate", e, v)
		}
		seen[v] = true
	}
	if wlResizeEdges[gui.EdgeTopLeft] != wl.XdgToplevelResizeEdgeTop|wl.XdgToplevelResizeEdgeLeft {
		t.Error("top-left is not top|left")
	}
}

func TestWlCursorTheme(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XCURSOR_THEME", "")
	t.Setenv("XCURSOR_SIZE", "")
	if th, sz := wlCursorTheme(); th != "default" || sz != 24 {
		t.Errorf("no settings: %q %d, want default 24", th, sz)
	}
	if err := os.MkdirAll(filepath.Join(cfg, "gtk-3.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	ini := "[Settings]\ngtk-cursor-theme-name=Adwaita\ngtk-cursor-theme-size=32\n"
	if err := os.WriteFile(filepath.Join(cfg, "gtk-3.0", "settings.ini"), []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}
	if th, sz := wlCursorTheme(); th != "Adwaita" || sz != 32 {
		t.Errorf("settings.ini: %q %d, want Adwaita 32", th, sz)
	}
	t.Setenv("XCURSOR_THEME", "breeze")
	t.Setenv("XCURSOR_SIZE", "48")
	if th, sz := wlCursorTheme(); th != "breeze" || sz != 48 {
		t.Errorf("env: %q %d, want breeze 48", th, sz)
	}
	// A size out of range is ignored, not used to load a huge image.
	t.Setenv("XCURSOR_SIZE", "100000")
	if _, sz := wlCursorTheme(); sz != 32 {
		t.Errorf("huge XCURSOR_SIZE: size %d, want the settings.ini 32", sz)
	}
}

// virtualPointer makes a pointer on the test's own connection through
// wlr-virtual-pointer, so sway, headless with no input device, has one
// and sends its events back to us. Skips on other compositors.
func virtualPointer(t *testing.T, d *wlDisplay) wl.ZwlrVirtualPointerV1 {
	t.Helper()
	if d.seat == nil {
		t.Skip("no seat")
	}
	reg := d.conn.Display.GetRegistry()
	var mgr wl.ZwlrVirtualPointerManagerV1
	reg.SetHandlers(wl.RegistryHandlers{Global: func(name uint32, iface string, version uint32) {
		if iface == "zwlr_virtual_pointer_manager_v1" {
			mgr = wl.ZwlrVirtualPointerManagerV1{Proxy: reg.Bind(name,
				&wl.ZwlrVirtualPointerManagerV1Interface, 1)}
		}
	}})
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	reg.DestroyProxy()
	if !mgr.Valid() {
		t.Skip("no wlr-virtual-pointer")
	}
	vp := mgr.CreateVirtualPointer(d.seat.seat)
	mgr.Destroy()
	t.Cleanup(func() {
		vp.Destroy()
		_ = d.conn.Roundtrip()
	})
	return vp
}

// settleWindow renders the window as the run loop would, for half a
// second. A surface with no buffer is not mapped, and sway places a tiled
// window only once a frame at the size it configured arrives.
func settleWindow(t *testing.T, b *Backend) {
	t.Helper()
	ww := b.plat.wl
	ww.dirty = true
	for end := time.Now().Add(500 * time.Millisecond); time.Now().Before(end); {
		if ww.dirty {
			b.plat.w.FrameFn()
			b.renderFrame(b.plat.w)
			ww.dirty = false
		}
		if err := ww.d.conn.Dispatch(20 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
}

// pointAtWindow moves the virtual pointer to the middle of the output; sway
// tiles the one test window over all of it.
func pointAtWindow(t *testing.T, b *Backend, vp wl.ZwlrVirtualPointerV1) {
	t.Helper()
	d := b.plat.wl.d
	settleWindow(t, b)
	// The virtual device gives the seat its pointer capability. sway
	// sends no enter to a wl_pointer made after the pointer moved, so the
	// move waits for it (a real mouse exists long before the window).
	waitFor(t, d, "wl_pointer", func() bool { return d.seat.pointer.Valid() })
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	// sway starts the cursor in the middle; a move there would change
	// nothing and send no enter, so it goes somewhere else first.
	for _, at := range []uint32{40, 50} {
		vp.MotionAbsolute(0, at, at, 100, 100)
		vp.Frame()
	}
	waitFor(t, d, "pointer enter", func() bool { return d.seat.ptrFocus == b })
}

func TestWaylandCursor(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	// A cleanup, not a defer: the virtual pointer's cleanup, registered
	// later, runs first, while the connection is open.
	t.Cleanup(b.Destroy)
	d := b.plat.wl.d
	vp := virtualPointer(t, d)
	pointAtWindow(t, b, vp)
	s := d.seat

	if !d.cursorShape.Valid() {
		t.Fatal("sway offers cursor-shape-v1, but it is not bound")
	}
	s.setCursor(gui.CursorIBeam)
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatalf("cursor shape: %v", err)
	}
	if !s.cursor.device.Valid() || s.cursor.cur != gui.CursorIBeam {
		t.Fatalf("cursor state after set: %+v", s.cursor)
	}

	// The fallback, as on a compositor without cursor-shape-v1: the
	// image comes from a theme, here a made-up one.
	icons := t.TempDir()
	writeTheme(t, icons, "test", []string{}, map[string][]int{
		"left_ptr": {24, 48},
		"xterm":    {24, 48},
	})
	t.Setenv("XCURSOR_PATH", icons)
	t.Setenv("XCURSOR_THEME", "test")
	t.Setenv("XCURSOR_SIZE", "24")
	mgr := d.cursorShape
	d.cursorShape = wl.WpCursorShapeManagerV1{}
	defer func() { d.cursorShape = mgr }()
	s.cursor.dropDevice()
	s.cursor.theme = ""

	s.setCursor(gui.CursorIBeam)
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatalf("cursor image: %v", err)
	}
	img := s.cursor.images[gui.CursorIBeam]
	if !img.buf.Valid() || !s.cursor.surface.Valid() {
		t.Fatalf("no cursor image: %+v", img)
	}
	// 24 px at scale 1: the hotspot is the middle (buildXcursorFile).
	if img.hotX != 12 || img.hotY != 12 {
		t.Errorf("hotspot %d,%d, want 12,12", img.hotX, img.hotY)
	}
	// A cursor the theme lacks is searched once, then left alone.
	s.setCursor(gui.CursorCrosshair)
	if !s.cursor.images[gui.CursorCrosshair].failed {
		t.Error("a missing theme cursor was not marked failed")
	}
}

func TestWaylandCursorImageScale(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	// A cleanup, not a defer: the virtual pointer's cleanup, registered
	// later, runs first, while the connection is open.
	t.Cleanup(b.Destroy)
	d := b.plat.wl.d
	if !d.shm.Valid() {
		t.Skip("no wl_shm")
	}
	icons := t.TempDir()
	writeTheme(t, icons, "odd", []string{}, map[string][]int{"left_ptr": {25}})
	writeTheme(t, icons, "even", []string{}, map[string][]int{"left_ptr": {48}})
	t.Setenv("XCURSOR_PATH", icons)

	img := d.loadCursorImage(leftPtrCursorNames, "even", 24, 2)
	if !img.buf.Valid() || img.hotX != 12 {
		t.Errorf("scale 2: %+v, want a buffer with the hotspot at 48/2/2", img)
	}
	img.buf.Destroy()
	// An image that does not divide by the scale would be a protocol
	// error as a buffer.
	if img := d.loadCursorImage(leftPtrCursorNames, "odd", 25, 2); !img.failed {
		t.Errorf("odd size at scale 2 was not refused: %+v", img)
	}
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.shmBuffer(0, 4, nil); err == nil {
		t.Error("shmBuffer took an empty image")
	}
	if _, err := d.shmBuffer(4, 4, make([]uint32, 3)); err == nil {
		t.Error("shmBuffer took too few pixels")
	}
}

func TestWaylandMoveResize(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	// A cleanup, not a defer: the virtual pointer's cleanup, registered
	// later, runs first, while the connection is open.
	t.Cleanup(b.Destroy)
	d := b.plat.wl.d
	vp := virtualPointer(t, d)
	pointAtWindow(t, b, vp)

	vp.Button(0, btnLeft, wl.PointerButtonStatePressed)
	vp.Frame()
	waitFor(t, d, "button press", func() bool { return d.seat.buttons != 0 })
	if d.seat.pressSerial == 0 {
		t.Fatal("the press left no serial")
	}
	b.plat.startMoveResize(netMoveResizeMove)
	b.plat.startMoveResize(uint32(gui.EdgeBottomRight))
	b.plat.startMoveResize(99) // not an edge: ignored
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatalf("move/resize: %v", err)
	}
	vp.Button(0, btnLeft, wl.PointerButtonStateReleased)
	vp.Frame()
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}

func TestWlScaled(t *testing.T) {
	for _, c := range []struct{ v, s, want int32 }{
		{800, 120, 800},
		{801, 180, 1202}, // 1201.5 rounds up
		{800, 150, 1000},
		{0, 240, 0},
		{1 << 20, wlMaxScale120, 8 << 20}, // no int32 overflow on the way
	} {
		if got := wlScaled(c.v, c.s); got != c.want {
			t.Errorf("wlScaled(%d, %d) = %d, want %d", c.v, c.s, got, c.want)
		}
	}
}

func TestWaylandFractionalScale(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	t.Cleanup(b.Destroy)
	ww := b.plat.wl
	if !ww.fracScale.Valid() {
		t.Skip("no fractional-scale-v1")
	}
	settleWindow(t, b)
	check := func(scale120 int32) {
		t.Helper()
		if ww.scale120 != scale120 || b.dpiScale != float32(scale120)/120 ||
			b.physW != wlScaled(ww.logW, scale120) || b.physH != wlScaled(ww.logH, scale120) {
			t.Fatalf("scale %d/120: dpi %v, buffer %dx%d for %dx%d", ww.scale120, b.dpiScale,
				b.physW, b.physH, ww.logW, ww.logH)
		}
		if ww.cursorScale() != (scale120+119)/120 {
			t.Errorf("cursor scale %d at %d/120", ww.cursorScale(), scale120)
		}
	}
	ww.setFracScale(180)
	check(180)
	settleWindow(t, b) // a buffer at 1.5×, shown through the viewport
	// The integer event is ignored while fractional scaling is on.
	ww.setScale(3)
	check(180)
	// Out-of-range scales are clamped.
	ww.setFracScale(0)
	check(120)
	ww.setFracScale(1 << 30)
	check(wlMaxScale120)
	ww.setFracScale(120)
	settleWindow(t, b)

	// The compositor's own event: sway scales its output.
	if os.Getenv("SWAYSOCK") == "" {
		return
	}
	swaymsg := func(args ...string) {
		if out, err := exec.Command("swaymsg", args...).CombinedOutput(); err != nil {
			t.Fatalf("swaymsg %v: %v: %s", args, err, out)
		}
	}
	swaymsg("output", "*", "scale", "1.25")
	t.Cleanup(func() { swaymsg("output", "*", "scale", "1") })
	waitFor(t, ww.d, "the 1.25 scale", func() bool { return ww.scale120 == 150 })
	check(150)
	settleWindow(t, b)
}

func TestWaylandIntegerScale(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	t.Cleanup(b.Destroy)
	ww := b.plat.wl
	// As on a compositor without fractional scaling (weston).
	frac, vp := ww.fracScale, ww.viewport
	ww.fracScale, ww.viewport = wl.WpFractionalScaleV1{}, wl.WpViewport{}
	t.Cleanup(func() { ww.fracScale, ww.viewport = frac, vp })
	ww.setScale(2)
	if ww.scale120 != 240 || b.physW != 2*ww.logW || b.dpiScale != 2 {
		t.Fatalf("scale %d/120, buffer %d for %d", ww.scale120, b.physW, ww.logW)
	}
	ww.setScale(100) // clamped to 8
	if ww.scale120 != wlMaxScale120 {
		t.Fatalf("scale %d/120 after a huge factor", ww.scale120)
	}
	ww.setScale(1)
	if err := ww.d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}
