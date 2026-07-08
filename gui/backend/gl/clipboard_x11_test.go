//go:build linux && !js && !android

package gl

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// requireXClip skips the test unless clipboard integration testing is
// explicitly opted into and a live X server + xclip are available.
//
// Opt-in gate: these tests open extra X connections which, when sharing a
// process with the real EGL/GL context test (TestBackendRenderSmoke),
// intermittently trip a native Mesa/Xlib threading crash unrelated to the
// clipboard code under test. Gating keeps the default `go test ./...` suite
// deterministic (and headless CI already skips on DISPLAY). Run with:
//
//	GOGUI_CLIPBOARD_IT=1 go test -run TestClipboard ./gui/backend/gl/
func requireXClip(t *testing.T) {
	t.Helper()
	if os.Getenv("GOGUI_CLIPBOARD_IT") == "" {
		t.Skip("set GOGUI_CLIPBOARD_IT=1 to run clipboard integration tests")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY; clipboard test needs a live X server")
	}
	if _, err := exec.LookPath("xclip"); err != nil {
		t.Skip("xclip not installed; skipping clipboard round-trip test")
	}
}

// newClipTestState builds a minimal platformState (X connection, owner
// window, interned atoms) without the full GL backend.
func newClipTestState(t *testing.T) (*platformState, func()) {
	t.Helper()
	conn, err := xgb.NewConn()
	if err != nil {
		t.Skipf("x11 connect: %v", err)
	}
	root := xproto.Setup(conn).DefaultScreen(conn).Root
	wid, err := conn.NewId()
	if err != nil {
		conn.Close()
		t.Fatalf("new id: %v", err)
	}
	win := xproto.Window(wid)
	xproto.CreateWindow(conn, 0, win, root, 0, 0, 1, 1, 0,
		xproto.WindowClassInputOnly, 0, 0, nil)
	p := &platformState{conn: conn, window: win}
	p.atomClipboard = internAtom(conn, "CLIPBOARD")
	p.atomUTF8 = internAtom(conn, "UTF8_STRING")
	p.atomTargets = internAtom(conn, "TARGETS")
	p.atomClipProp = internAtom(conn, "_GOGUI_CLIPBOARD")
	cleanup := func() {
		if p.clipReadConn != nil {
			p.clipReadConn.Close()
		}
		xproto.DestroyWindow(conn, win)
		conn.Close()
	}
	return p, cleanup
}

// TestClipboardGetFromXclip verifies getClipboard reads text set by an
// external selection owner (xclip).
func TestClipboardGetFromXclip(t *testing.T) {
	requireXClip(t)
	want := "gogui read αβγ 42"

	cmd := exec.Command("xclip", "-selection", "clipboard", "-in")
	cmd.Stdin = strings.NewReader(want)
	if err := cmd.Run(); err != nil {
		t.Fatalf("xclip -in: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // let xclip fork and take ownership

	p, cleanup := newClipTestState(t)
	defer cleanup()
	if got := getClipboard(p); got != want {
		t.Errorf("getClipboard() = %q, want %q", got, want)
	}
}

// TestClipboardSetServesXclip verifies setClipboard takes ownership and
// serveSelectionRequest answers an external requestor (xclip -out).
func TestClipboardSetServesXclip(t *testing.T) {
	requireXClip(t)
	p, cleanup := newClipTestState(t)
	defer cleanup()

	want := "gogui write 12345"
	setClipboard(p, want)

	// Serve incoming SelectionRequest events while xclip pulls the value.
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			ev, err := p.conn.PollForEvent()
			if err != nil {
				return
			}
			if ev == nil {
				time.Sleep(2 * time.Millisecond)
				continue
			}
			if req, ok := ev.(xproto.SelectionRequestEvent); ok {
				p.serveSelectionRequest(req)
			}
		}
	}()

	out, err := exec.Command("xclip", "-selection", "clipboard", "-out").Output()
	close(stop)
	<-done
	if err != nil {
		t.Fatalf("xclip -out: %v", err)
	}
	if got := string(out); got != want {
		t.Errorf("xclip read %q, want %q", got, want)
	}
}
