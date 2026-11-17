//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// Timed tests of keeping the display awake under sway (issue #1287):
// swayidle reports when sway's idle timer fires, so these check what the
// user would see, not only that an inhibitor exists. They wait for real
// timeouts, so -test.short skips them.

// swayIdleTimeout is the idle timeout swayidle is started with, in seconds.
const swayIdleTimeout = 2

// startSwayidle starts swayidle with a short timeout and returns a channel
// that gets a value each time sway reports the session idle.
func startSwayidle(t *testing.T) <-chan struct{} {
	t.Helper()
	cmd := exec.Command("swayidle", "-w",
		"timeout", fmt.Sprint(swayIdleTimeout), "echo idle")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatalf("start swayidle: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	idle := make(chan struct{}, 8)
	go func() {
		s := bufio.NewScanner(out)
		for s.Scan() {
			if strings.TrimSpace(s.Text()) == "idle" {
				idle <- struct{}{}
			}
		}
	}()
	return idle
}

// idleWithin keeps the window's loop going for up to d and reports
// whether sway reported idle in that time. b may be nil once the window
// is gone.
func idleWithin(t *testing.T, b *Backend, idle <-chan struct{}, d time.Duration) bool {
	t.Helper()
	for end := time.Now().Add(d); time.Now().Before(end); {
		select {
		case <-idle:
			return true
		default:
		}
		if b == nil || b.plat.wl == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if b.plat.wl.dirty {
			b.plat.wl.dirty = false
			b.plat.makeCurrent()
			b.renderFrame(b.plat.w)
		}
		if err := b.plat.wl.d.conn.Dispatch(50 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	return false
}

// openSwayIdleWindow opens a window sway can name, and waits until it is
// on screen.
func openSwayIdleWindow(t *testing.T, appID string) *Backend {
	t.Helper()
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
	t.Cleanup(func() {
		if b.plat.wl != nil {
			b.Destroy()
		}
	})
	requireIdleInhibit(t, b.plat.wl.d)
	pumpWindow(t, b, "the window on screen", func() bool {
		found, _, _ := swayKeepAwakeState(t, appID)
		return found
	})
	return b
}

// TestWaylandKeepAwakeSwayIdle runs each case with a fresh window and a
// fresh swayidle, whose timer starts when it does.
func TestWaylandKeepAwakeSwayIdle(t *testing.T) {
	if os.Getenv("COMPOSITOR") != "sway" {
		t.Skip("needs sway")
	}
	if testing.Short() {
		t.Skip("waits for real idle timeouts")
	}
	if _, err := exec.LookPath("swayidle"); err != nil {
		t.Fatal("swayidle is not installed (scripts/wayland/Dockerfile)")
	}
	const appID = "gogui-keepawake-idle"
	// fires: idle comes within the timeout and a margin.
	fires := time.Duration(swayIdleTimeout)*time.Second + 1500*time.Millisecond
	// holds: well past the timeout with no idle.
	holds := time.Duration(2*swayIdleTimeout) * time.Second

	t.Run("no request", func(t *testing.T) {
		b := openSwayIdleWindow(t, appID)
		if !idleWithin(t, b, startSwayidle(t), fires) {
			t.Fatal("no idle without a request: the timer itself does not work")
		}
	})

	t.Run("held then released", func(t *testing.T) {
		b := openSwayIdleWindow(t, appID)
		np := &nativePlatform{b: b}
		if err := np.SetKeepAwake(true, "app", "reason"); err != nil {
			t.Fatal(err)
		}
		idle := startSwayidle(t)
		if idleWithin(t, b, idle, holds) {
			t.Fatal("idle while the request is held")
		}
		if err := np.SetKeepAwake(false, "", ""); err != nil {
			t.Fatal(err)
		}
		if !idleWithin(t, b, idle, fires) {
			t.Fatal("no idle after the release")
		}
	})

	t.Run("hidden", func(t *testing.T) {
		b := openSwayIdleWindow(t, appID)
		np := &nativePlatform{b: b}
		if err := np.SetKeepAwake(true, "app", "reason"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = np.SetKeepAwake(false, "", "") })
		idle := startSwayidle(t)
		// sway honours the inhibitor only while the window is visible.
		if out, err := exec.Command("swaymsg",
			"[app_id="+appID+"] move to workspace 2").CombinedOutput(); err != nil {
			t.Fatalf("swaymsg: %v: %s", err, out)
		}
		if !idleWithin(t, b, idle, fires) {
			t.Fatal("no idle with the window on another workspace")
		}
	})

	t.Run("closed", func(t *testing.T) {
		b := openSwayIdleWindow(t, appID)
		if err := (&nativePlatform{b: b}).SetKeepAwake(true, "app", "reason"); err != nil {
			t.Fatal(err)
		}
		idle := startSwayidle(t)
		b.Destroy()
		t.Cleanup(func() { _ = (&nativePlatform{}).SetKeepAwake(false, "", "") })
		if !idleWithin(t, nil, idle, fires) {
			t.Fatal("no idle after the window holding the request closed")
		}
	})
}
