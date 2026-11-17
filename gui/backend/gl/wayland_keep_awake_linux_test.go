//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/screensaver/screensavertest"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// Tests of keeping the display awake on Wayland (issue #1287). They need
// a compositor, like the other Wayland window tests; scripts/wayland/test.sh
// runs them under sway, weston and mutter.

// keepAwakeCall is one call that reached the D-Bus fallback.
type keepAwakeCall struct {
	on          bool
	app, reason string
}

// spyScreensaver replaces the D-Bus fallback for the test and returns the
// calls it got.
func spyScreensaver(t *testing.T) *[]keepAwakeCall {
	t.Helper()
	calls := &[]keepAwakeCall{}
	saved := screensaverSet
	screensaverSet = func(on bool, app, reason string) error {
		*calls = append(*calls, keepAwakeCall{on, app, reason})
		return nil
	}
	t.Cleanup(func() { screensaverSet = saved })
	return calls
}

// requireIdleInhibit skips on a compositor without the protocol. In the
// harness, sway and mutter must offer it and weston must not, so the
// fallback branch is known to be the one weston runs.
func requireIdleInhibit(t *testing.T, d *wlDisplay) {
	t.Helper()
	has := d.keepAwake.mgr.Valid()
	switch os.Getenv("COMPOSITOR") {
	case "sway", "mutter":
		if !has {
			t.Fatal("compositor offers no zwp_idle_inhibit_manager_v1")
		}
	case "weston":
		if has {
			t.Fatal("weston now offers zwp_idle_inhibit_manager_v1: move it to the protocol cases")
		}
	}
	if !has {
		t.Skip("compositor offers no zwp_idle_inhibit_manager_v1")
	}
}

// openKeepAwakeWindow opens a Wayland window that the test closes.
func openKeepAwakeWindow(t *testing.T) *Backend {
	t.Helper()
	b := newWaylandTestBackend(t, gui.WindowCfg{Width: 200, Height: 150})
	t.Cleanup(func() {
		if b.plat.wl != nil {
			b.Destroy()
		}
	})
	return b
}

// checkInhibitors fails unless exactly the windows in want hold an
// inhibitor, and the connection is still healthy.
func checkInhibitors(t *testing.T, d *wlDisplay, want ...*Backend) {
	t.Helper()
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatalf("connection: %v", err)
	}
	if len(d.keepAwake.inhibitors) != len(want) {
		t.Fatalf("%d inhibitors, want %d", len(d.keepAwake.inhibitors), len(want))
	}
	for _, b := range want {
		if _, ok := d.keepAwake.inhibitors[b.plat.wl]; !ok {
			t.Fatalf("window %d holds no inhibitor", b.plat.wl.id())
		}
	}
}

// TestWaylandKeepAwakeEveryWindow turns the request on and off through one
// window's platform, and checks that every window follows: those open
// then, one opened while it is on, and none after a window closes.
func TestWaylandKeepAwakeEveryWindow(t *testing.T) {
	b1 := openKeepAwakeWindow(t)
	d := b1.plat.wl.d
	requireIdleInhibit(t, d)
	calls := spyScreensaver(t)
	b2 := openKeepAwakeWindow(t)
	np := &nativePlatform{b: b2}

	for range 3 {
		if err := np.SetKeepAwake(true, "app", "reason"); err != nil {
			t.Fatal(err)
		}
		checkInhibitors(t, d, b1, b2)
		if err := np.SetKeepAwake(false, "", ""); err != nil {
			t.Fatal(err)
		}
		checkInhibitors(t, d)
	}
	for _, c := range *calls {
		if c.on {
			t.Fatalf("D-Bus asked to keep awake with the protocol present: %+v", c)
		}
	}

	if err := np.SetKeepAwake(true, "app", "reason"); err != nil {
		t.Fatal(err)
	}
	b3 := openKeepAwakeWindow(t)
	checkInhibitors(t, d, b1, b2, b3)
	// A window that closes drops its inhibitor before its surface: the
	// compositor sees no request on a destroyed object.
	b2.Destroy()
	checkInhibitors(t, d, b1, b3)
	if err := (&nativePlatform{b: b1}).SetKeepAwake(false, "", ""); err != nil {
		t.Fatal(err)
	}
	checkInhibitors(t, d)
}

// TestWaylandKeepAwakeFallback checks the D-Bus fallback on a compositor
// without the protocol: weston has none, elsewhere it is hidden. A fake
// ScreenSaver service on a private bus gets the strings gui made, and no
// window gets an inhibitor.
func TestWaylandKeepAwakeFallback(t *testing.T) {
	b := openKeepAwakeWindow(t)
	d := b.plat.wl.d
	if d.keepAwake.mgr.Valid() {
		saved := d.keepAwake.mgr
		d.keepAwake.mgr = wl.ZwpIdleInhibitManagerV1{}
		t.Cleanup(func() { d.keepAwake.mgr = saved })
	}
	f := screensavertest.Serve(t, screensavertest.StartBus(t))
	t.Cleanup(func() { _ = screensaverSet(false, "", "") })

	np := &nativePlatform{b: b}
	if err := np.SetKeepAwake(true, "org.example.Player", "Playing a video"); err != nil {
		t.Fatal(err)
	}
	want := screensavertest.Call{App: "org.example.Player", Reason: "Playing a video"}
	if calls := f.Calls(); len(calls) != 1 || calls[0] != want || f.Active() != 1 {
		t.Fatalf("Inhibit calls %+v (%d held), want [%+v] held", calls, f.Active(), want)
	}
	checkInhibitors(t, d)
	if err := np.SetKeepAwake(false, "", ""); err != nil {
		t.Fatal(err)
	}
	if f.Active() != 0 {
		t.Fatal("the D-Bus request is still held after off")
	}
}

// TestWaylandKeepAwakeMutter is a smoke test of the protocol under mutter,
// which forwards an inhibitor to org.freedesktop.ScreenSaver on the
// session bus as Inhibit("mutter", "idle-inhibit"). The harness gives
// mutter and the test one bus, where a fake service watches.
func TestWaylandKeepAwakeMutter(t *testing.T) {
	if os.Getenv("COMPOSITOR") != "mutter" {
		t.Skip("needs mutter")
	}
	addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if addr == "" {
		t.Fatal("no session bus shared with mutter")
	}
	b := openKeepAwakeWindow(t)
	requireIdleInhibit(t, b.plat.wl.d)
	f := screensavertest.Serve(t, addr)
	pumpWindow(t, b, "the first frame", func() bool { return !b.plat.wl.dirty })

	np := &nativePlatform{b: b}
	if err := np.SetKeepAwake(true, "app", "reason"); err != nil {
		t.Fatal(err)
	}
	pumpWindow(t, b, "mutter to forward the inhibitor", func() bool { return f.Active() == 1 })
	want := screensavertest.Call{App: "mutter", Reason: "idle-inhibit"}
	if calls := f.Calls(); len(calls) != 1 || calls[0] != want {
		t.Errorf("Inhibit calls %+v, want [%+v]", calls, want)
	}
	if err := np.SetKeepAwake(false, "", ""); err != nil {
		t.Fatal(err)
	}
	pumpWindow(t, b, "mutter to end the request", func() bool { return f.Active() == 0 })
}

// TestKeepAwakeOffWithoutDisplay ends the request after every window and
// the display are gone, as WindowCleanup may: nothing to destroy, and the
// D-Bus off, which does nothing when none is held. No compositor needed.
func TestKeepAwakeOffWithoutDisplay(t *testing.T) {
	if wlShared != nil {
		t.Skip("a Wayland display is open")
	}
	calls := spyScreensaver(t)
	if err := (&nativePlatform{}).SetKeepAwake(false, "", ""); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0].on {
		t.Fatalf("D-Bus calls %+v, want one off", *calls)
	}
}

// swayKeepAwakeState reads sway's view of the window with appID: whether
// an application inhibitor exists (idle_inhibitors.application) and
// whether idle is inhibited now (inhibit_idle, true only while the view
// is visible). Both are documented in sway-ipc(7).
func swayKeepAwakeState(t *testing.T, appID string) (found bool, application string, inhibited bool) {
	t.Helper()
	out, err := exec.Command("swaymsg", "-r", "-t", "get_tree").Output()
	if err != nil {
		t.Fatalf("swaymsg get_tree: %v", err)
	}
	type node struct {
		AppID          string `json:"app_id"`
		InhibitIdle    bool   `json:"inhibit_idle"`
		IdleInhibitors struct {
			Application string `json:"application"`
		} `json:"idle_inhibitors"`
		Nodes         []node `json:"nodes"`
		FloatingNodes []node `json:"floating_nodes"`
	}
	var root node
	if err = json.Unmarshal(out, &root); err != nil {
		t.Fatalf("swaymsg get_tree: %v", err)
	}
	var walk func(n node) bool
	walk = func(n node) bool {
		if n.AppID == appID {
			found, application, inhibited = true, n.IdleInhibitors.Application, n.InhibitIdle
			return true
		}
		return slices.ContainsFunc(append(n.Nodes, n.FloatingNodes...), walk)
	}
	walk(root)
	return found, application, inhibited
}

// TestWaylandKeepAwakeSway is the end-to-end check: an app calls
// Window.KeepAwake, and sway reports the window as inhibiting idle; after
// the release it does not.
func TestWaylandKeepAwakeSway(t *testing.T) {
	if os.Getenv("COMPOSITOR") != "sway" {
		t.Skip("needs sway's IPC")
	}
	const appID = "gogui-keepawake-test"
	requireWayland(t)
	t.Setenv("GOGUI_WAYLAND", "1")
	b, err := New(waylandTestWindow(gui.WindowCfg{Width: 200, Height: 150, WMClass: appID}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if b.plat.wl == nil {
		b.Destroy()
		t.Fatal("New fell back to X11 with a compositor present")
	}
	t.Cleanup(b.Destroy)
	w := b.plat.w
	requireIdleInhibit(t, b.plat.wl.d)
	pumpWindow(t, b, "the window on screen", func() bool {
		found, _, _ := swayKeepAwakeState(t, appID)
		return found
	})

	release := w.KeepAwake("Playing a test")
	w.FrameFn() // runs the queued reconcile, as the loop does
	pumpWindow(t, b, "sway to report the inhibitor", func() bool {
		_, app, inhibited := swayKeepAwakeState(t, appID)
		return app == "enabled" && inhibited
	})

	release()
	w.FrameFn()
	pumpWindow(t, b, "sway to report no inhibitor", func() bool {
		_, app, inhibited := swayKeepAwakeState(t, appID)
		return app == "none" && !inhibited
	})
}
