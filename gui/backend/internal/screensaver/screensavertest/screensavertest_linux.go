//go:build linux && !android

// Package screensavertest runs a fake org.freedesktop.ScreenSaver service
// for tests: the screensaver package's own, and the GL backend's Wayland
// keep-awake tests (issue #1287), which also watch what mutter forwards.
package screensavertest

import (
	"bufio"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
)

const (
	dest  = "org.freedesktop.ScreenSaver"
	path  = dbus.ObjectPath("/org/freedesktop/ScreenSaver")
	iface = "org.freedesktop.ScreenSaver"
)

// StartBus starts a private session bus, points this test at it, and
// returns its address. It skips where dbus-daemon is not installed.
func StartBus(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("dbus-daemon"); err != nil {
		t.Skip("dbus-daemon is not installed")
	}
	cmd := exec.Command("dbus-daemon", "--session", "--nofork", "--print-address")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatalf("start dbus-daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatalf("read the bus address: %v", err)
	}
	addr := strings.TrimSpace(line)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", addr)
	return addr
}

// Call is one Inhibit the fake service got.
type Call struct{ App, Reason string }

// Fake is a ScreenSaver service that records its calls. godbus runs its
// methods on its own goroutines, so every read takes the lock.
type Fake struct {
	mu     sync.Mutex
	calls  []Call
	active map[uint32]bool
	next   uint32
}

// Inhibit is the D-Bus method.
func (f *Fake) Inhibit(app, reason string) (uint32, *dbus.Error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.calls = append(f.calls, Call{app, reason})
	f.active[f.next] = true
	return f.next, nil
}

// UnInhibit is the D-Bus method.
func (f *Fake) UnInhibit(cookie uint32) *dbus.Error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.active, cookie)
	return nil
}

// Active returns how many requests are held.
func (f *Fake) Active() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.active)
}

// Calls returns a copy of the Inhibit calls.
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// Serve puts a fake ScreenSaver service on the bus at addr.
func Serve(t *testing.T, addr string) *Fake {
	t.Helper()
	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatalf("connect the fake service: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	f := &Fake{active: map[uint32]bool{}}
	if err = conn.Export(f, path, iface); err != nil {
		t.Fatal(err)
	}
	reply, err := conn.RequestName(dest, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: reply %v, err %v", dest, reply, err)
	}
	return f
}
