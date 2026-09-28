//go:build linux && !js && !android

package gl

import (
	"os"
	"testing"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// TestSetWindowTitleUTF8 writes a non-Latin-1 title and reads both name
// properties back. Before the fix WM_NAME was typed STRING (Latin-1), so a
// window manager rendered "✳ Claude Code" as "â<U+009C>³ Claude Code".
//
// Opt-in for the same reason as the clipboard tests (extra X connections
// alongside the GL smoke test can trip a Mesa/Xlib crash). Run with:
//
//	GOGUI_X11_IT=1 go test -run TestSetWindowTitleUTF8 ./gui/backend/gl/
func TestSetWindowTitleUTF8(t *testing.T) {
	if os.Getenv("GOGUI_X11_IT") == "" {
		t.Skip("set GOGUI_X11_IT=1 to run X11 integration tests")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	conn, err := xgb.NewConn()
	if err != nil {
		t.Skipf("x11 connect: %v", err)
	}
	defer conn.Close()
	root := xproto.Setup(conn).DefaultScreen(conn).Root
	wid, err := conn.NewId()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}
	win := xproto.Window(wid)
	xproto.CreateWindow(conn, 0, win, root, 0, 0, 1, 1, 0,
		xproto.WindowClassInputOnly, 0, 0, nil)
	defer xproto.DestroyWindow(conn, win)

	p := &platformState{
		atomUTF8:      internAtom(conn, "UTF8_STRING"),
		atomNetWMName: internAtom(conn, "_NET_WM_NAME"),
	}
	const title = "✳ Claude Code"
	setWindowTitle(conn, win, p, title)

	for _, prop := range []struct {
		name string
		atom xproto.Atom
	}{
		{"WM_NAME", xproto.AtomWmName},
		{"_NET_WM_NAME", p.atomNetWMName},
	} {
		r, err := xproto.GetProperty(conn, false, win, prop.atom,
			xproto.GetPropertyTypeAny, 0, 1024).Reply()
		if err != nil {
			t.Fatalf("%s: get property: %v", prop.name, err)
		}
		if r.Type != p.atomUTF8 {
			t.Errorf("%s type = %d, want UTF8_STRING (%d)", prop.name, r.Type, p.atomUTF8)
		}
		if got := string(r.Value); got != title {
			t.Errorf("%s = %q, want %q", prop.name, got, title)
		}
	}
}
