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

// selCases enumerates the two selections the backend owns, so every
// round-trip test runs against both. xclipName is what `xclip -selection`
// calls them.
var selCases = []struct {
	name      string
	xclipName string
	set       func(*platformState, string)
	get       func(*platformState) string
}{
	{"clipboard", "clipboard", setClipboard, getClipboard},
	{"primary", "primary", setPrimary, getPrimary},
}

// serveRequests answers SelectionRequest events in the background for as long
// as the returned stop function has not been called. Standing in for the main
// event loop, which these tests do not run.
func serveRequests(p *platformState) (stop func()) {
	stopCh := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stopCh:
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
	return func() {
		close(stopCh)
		<-done
	}
}

// TestSelectionGetFromXclip verifies get{Clipboard,Primary} read text set by
// an external selection owner (xclip).
func TestSelectionGetFromXclip(t *testing.T) {
	requireXClip(t)
	for _, tc := range selCases {
		t.Run(tc.name, func(t *testing.T) {
			want := "gogui read αβγ 42 " + tc.name

			cmd := exec.Command("xclip", "-selection", tc.xclipName, "-in")
			cmd.Stdin = strings.NewReader(want)
			if err := cmd.Run(); err != nil {
				t.Fatalf("xclip -in: %v", err)
			}
			time.Sleep(50 * time.Millisecond) // let xclip fork and take ownership

			p, cleanup := newClipTestState(t)
			defer cleanup()
			if got := tc.get(p); got != want {
				t.Errorf("get() = %q, want %q", got, want)
			}
		})
	}
}

// TestSelectionSetServesXclip verifies set{Clipboard,Primary} takes ownership
// and serveSelectionRequest answers an external requestor (xclip -out).
func TestSelectionSetServesXclip(t *testing.T) {
	requireXClip(t)
	for _, tc := range selCases {
		t.Run(tc.name, func(t *testing.T) {
			p, cleanup := newClipTestState(t)
			defer cleanup()

			want := "gogui write 12345 " + tc.name
			tc.set(p, want)

			stop := serveRequests(p)
			out, err := exec.Command("xclip", "-selection", tc.xclipName, "-out").Output()
			stop()
			if err != nil {
				t.Fatalf("xclip -out: %v", err)
			}
			if got := string(out); got != want {
				t.Errorf("xclip read %q, want %q", got, want)
			}
		})
	}
}

// TestSelectionsAreIndependent is the point of PRIMARY: holding both at once
// must yield two different values, and serving a request for one must never
// answer with the other's text.
func TestSelectionsAreIndependent(t *testing.T) {
	requireXClip(t)
	p, cleanup := newClipTestState(t)
	defer cleanup()

	const clip = "value-on-clipboard"
	const prim = "value-on-primary"
	setClipboard(p, clip)
	setPrimary(p, prim)

	stop := serveRequests(p)
	gotClip, errClip := exec.Command("xclip", "-selection", "clipboard", "-out").Output()
	gotPrim, errPrim := exec.Command("xclip", "-selection", "primary", "-out").Output()
	stop()
	if errClip != nil || errPrim != nil {
		t.Fatalf("xclip -out: %v / %v", errClip, errPrim)
	}
	if string(gotClip) != clip {
		t.Errorf("clipboard = %q, want %q", gotClip, clip)
	}
	if string(gotPrim) != prim {
		t.Errorf("primary = %q, want %q", gotPrim, prim)
	}
}

// TestSelectionStateRouting covers the selection→cache mapping without an X
// server, including the zero-atom guard that keeps an uninitialized
// platformState from mistaking a nil selection for CLIPBOARD.
func TestSelectionStateRouting(t *testing.T) {
	p := &platformState{atomClipboard: 400}
	p.clipboardText, p.primaryText = "clip", "prim"

	tests := []struct {
		name string
		sel  xproto.Atom
		want string // "" means the selection must be rejected
	}{
		{"clipboard", 400, "clip"},
		{"primary", xproto.AtomPrimary, "prim"},
		{"secondary rejected", xproto.AtomSecondary, ""},
		{"zero rejected", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text, owns, ok := p.selectionState(tc.sel)
			if tc.want == "" {
				if ok {
					t.Fatalf("selectionState(%d) accepted an unowned selection", tc.sel)
				}
				return
			}
			if !ok {
				t.Fatalf("selectionState(%d) rejected a known selection", tc.sel)
			}
			if *text != tc.want {
				t.Errorf("text = %q, want %q", *text, tc.want)
			}
			// The owner flag must alias the right field: flipping it through
			// the pointer is how SelectionClear releases one selection only.
			*owns = true
		})
	}
	if !p.ownsClipboard || !p.ownsPrimary {
		t.Errorf("owner flags did not alias: clipboard=%v primary=%v",
			p.ownsClipboard, p.ownsPrimary)
	}
}

// TestSelectionStateZeroAtomGuard pins the uninitialized case: before the
// atoms are interned, atomClipboard is 0 and a zero selection must not route
// to the clipboard cache.
func TestSelectionStateZeroAtomGuard(t *testing.T) {
	p := &platformState{} // atoms not yet interned
	if _, _, ok := p.selectionState(0); ok {
		t.Error("zero selection routed to clipboard on an uninitialized state")
	}
	if _, _, ok := p.selectionState(xproto.AtomPrimary); !ok {
		t.Error("PRIMARY is a predefined atom and must resolve before interning")
	}
}
