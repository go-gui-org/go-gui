//go:build linux && !js && !android

package gl

import (
	"os"
	"testing"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// newUnmappedXWindow creates a plain, unmapped window on a live X
// server, as New does before its first map. It skips without a server.
func newUnmappedXWindow(t *testing.T) (*xgb.Conn, xproto.Window) {
	t.Helper()
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY; the test needs an X server")
	}
	conn, err := xgb.NewConn()
	if err != nil {
		t.Skipf("x11 connect: %v", err)
	}
	t.Cleanup(conn.Close)
	screen := xproto.Setup(conn).DefaultScreen(conn)
	win, err := xproto.NewWindowId(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err = xproto.CreateWindowChecked(conn, screen.RootDepth, win, screen.Root,
		0, 0, 200, 100, 0, xproto.WindowClassInputOutput, screen.RootVisual,
		0, nil).Check(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { xproto.DestroyWindow(conn, win) })
	return conn, win
}

func xMapState(t *testing.T, conn *xgb.Conn, win xproto.Window) byte {
	t.Helper()
	attrs, err := xproto.GetWindowAttributes(conn, win).Reply()
	if err != nil {
		t.Fatal(err)
	}
	return attrs.MapState
}

// Issue #1270: a window the app hid before the backend started is not
// mapped, and Window.Show maps it.
func TestHiddenBeforeStartIsNotMapped(t *testing.T) {
	conn, win := newUnmappedXWindow(t)
	w := gui.NewTestWindow(t, gui.WindowCfg{})
	w.Hide() // before the platform attaches

	mapAtStart(w, conn, win)
	if got := xMapState(t, conn, win); got != xproto.MapStateUnmapped {
		t.Fatalf("map state %d after start, want unmapped", got)
	}

	b := &Backend{}
	b.plat.conn = conn
	b.plat.window = win
	w.SetNativePlatform(&nativePlatform{b: b})
	w.Show()
	if got := xMapState(t, conn, win); got == xproto.MapStateUnmapped {
		t.Error("Show did not map a window that started hidden")
	}
	if !w.IsVisible() {
		t.Error("IsVisible = false after Show")
	}
}

// The control: a window the app did not hide is mapped at start.
func TestVisibleAtStartIsMapped(t *testing.T) {
	conn, win := newUnmappedXWindow(t)
	if got := xMapState(t, conn, win); got != xproto.MapStateUnmapped {
		t.Fatalf("the test window was created with map state %d", got)
	}

	mapAtStart(gui.NewTestWindow(t, gui.WindowCfg{}), conn, win)
	if got := xMapState(t, conn, win); got == xproto.MapStateUnmapped {
		t.Error("a window that was not hidden was not mapped at start")
	}
}
