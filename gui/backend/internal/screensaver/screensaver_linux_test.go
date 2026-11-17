//go:build linux && !android

package screensaver

import (
	"bufio"
	"errors"
	"net"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// startBus starts a private session bus and points this test at it.
// It skips where dbus-daemon is not installed.
func startBus(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon is not installed")
	}
	cmd := exec.Command(bin, "--session", "--nofork", "--print-address")
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

// inhibitCall is one Inhibit the fake service got.
type inhibitCall struct{ app, reason string }

// fakeScreenSaver is a ScreenSaver service that records its calls.
type fakeScreenSaver struct {
	mu     sync.Mutex
	calls  []inhibitCall
	active map[uint32]bool
	next   uint32
}

func (f *fakeScreenSaver) Inhibit(app, reason string) (uint32, *dbus.Error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.calls = append(f.calls, inhibitCall{app, reason})
	f.active[f.next] = true
	return f.next, nil
}

func (f *fakeScreenSaver) UnInhibit(cookie uint32) *dbus.Error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.active, cookie)
	return nil
}

func (f *fakeScreenSaver) activeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.active)
}

// callList returns a copy of the Inhibit calls. godbus runs the methods
// on its own goroutines, so the test reads under the lock.
func (f *fakeScreenSaver) callList() []inhibitCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// serve puts a fake ScreenSaver service on the bus at addr.
func serve(t *testing.T, addr string) *fakeScreenSaver {
	t.Helper()
	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatalf("connect the fake service: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	f := &fakeScreenSaver{active: map[uint32]bool{}}
	if err = conn.Export(f, path, iface); err != nil {
		t.Fatal(err)
	}
	reply, err := conn.RequestName(dest, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: reply %v, err %v", dest, reply, err)
	}
	return f
}

// endHeld ends a request a failed test left behind.
func endHeld(t *testing.T) {
	t.Cleanup(func() { _ = Set(false, "", "") })
}

// The request reaches the service with the app's strings and ends with
// UnInhibit.
func TestSetInhibitsAndEnds(t *testing.T) {
	f := serve(t, startBus(t))
	endHeld(t)

	if err := Set(true, "org.example.Player", "Playing a video"); err != nil {
		t.Fatalf("Set(true): %v", err)
	}
	if got := f.activeCount(); got != 1 {
		t.Fatalf("%d requests held after on, want 1", got)
	}
	want := inhibitCall{"org.example.Player", "Playing a video"}
	if calls := f.callList(); len(calls) != 1 || calls[0] != want {
		t.Errorf("Inhibit calls = %+v, want [%+v]", calls, want)
	}
	if err := Set(false, "", ""); err != nil {
		t.Fatalf("Set(false): %v", err)
	}
	if got := f.activeCount(); got != 0 {
		t.Errorf("%d requests held after off, want 0", got)
	}
}

// A repeated on does not stack a second request; a repeated off does
// nothing.
func TestSetRepeatedEdges(t *testing.T) {
	f := serve(t, startBus(t))
	endHeld(t)

	for range 2 {
		if err := Set(true, "app", "reason"); err != nil {
			t.Fatalf("Set(true): %v", err)
		}
	}
	if n := len(f.callList()); n != 1 {
		t.Errorf("%d Inhibit calls for two ons, want 1", n)
	}
	for range 2 {
		if err := Set(false, "", ""); err != nil {
			t.Fatalf("Set(false): %v", err)
		}
	}
	if got := f.activeCount(); got != 0 {
		t.Errorf("%d requests held after off, want 0", got)
	}
}

// Without the service (sway, Hyprland), on fails and holds nothing, so
// a later off does nothing.
func TestSetWithoutService(t *testing.T) {
	startBus(t)
	endHeld(t)

	if err := Set(true, "app", "reason"); err == nil {
		t.Fatal("Set(true) without a ScreenSaver service returned no error")
	}
	if held.conn != nil {
		t.Error("a failed on left a connection open")
	}
	if err := Set(false, "", ""); err != nil {
		t.Errorf("Set(false) after a failed on: %v", err)
	}
}

// A bus that accepts the connection and then says nothing must not hang
// the caller: on fails once callTimeout is up, and the half-open
// connection is closed rather than left blocked in Auth.
func TestSetSilentBus(t *testing.T) {
	endHeld(t)
	sock := filepath.Join(t.TempDir(), "bus")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		c, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		t.Cleanup(func() { _ = c.Close() }) // answer nothing
		accepted <- c
	}()
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+sock)
	prev := callTimeout
	callTimeout = 200 * time.Millisecond
	t.Cleanup(func() { callTimeout = prev })

	start := time.Now()
	if err = Set(true, "app", "reason"); err == nil {
		t.Fatal("Set(true) on a silent bus returned no error")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Set(true) on a silent bus took %v", d)
	}

	// The server's end reads what the client sent, then EOF once the
	// client closed. A read that times out means the client still holds
	// the connection open.
	var c net.Conn
	select {
	case c = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("no client connected to the silent bus")
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 256)
	for {
		if _, err = c.Read(buf); err != nil {
			break
		}
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Error("the client left the silent connection open after the timeout")
	}
}

// Without a session bus, on fails.
func TestSetWithoutBus(t *testing.T) {
	endHeld(t)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/go-gui-test-bus")
	if err := Set(true, "app", "reason"); err == nil {
		t.Fatal("Set(true) without a session bus returned no error")
	}
}
