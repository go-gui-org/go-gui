//go:build linux && !android

package screensaver

import (
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui/backend/internal/screensaver/screensavertest"
)

// endHeld ends a request a failed test left behind.
func endHeld(t *testing.T) {
	t.Cleanup(func() { _ = Set(false, "", "") })
}

// The request reaches the service with the app's strings and ends with
// UnInhibit.
func TestSetInhibitsAndEnds(t *testing.T) {
	f := screensavertest.Serve(t, screensavertest.StartBus(t))
	endHeld(t)

	if err := Set(true, "org.example.Player", "Playing a video"); err != nil {
		t.Fatalf("Set(true): %v", err)
	}
	if got := f.Active(); got != 1 {
		t.Fatalf("%d requests held after on, want 1", got)
	}
	want := screensavertest.Call{App: "org.example.Player", Reason: "Playing a video"}
	if calls := f.Calls(); len(calls) != 1 || calls[0] != want {
		t.Errorf("Inhibit calls = %+v, want [%+v]", calls, want)
	}
	if err := Set(false, "", ""); err != nil {
		t.Fatalf("Set(false): %v", err)
	}
	if got := f.Active(); got != 0 {
		t.Errorf("%d requests held after off, want 0", got)
	}
}

// A repeated on does not stack a second request; a repeated off does
// nothing.
func TestSetRepeatedEdges(t *testing.T) {
	f := screensavertest.Serve(t, screensavertest.StartBus(t))
	endHeld(t)

	for range 2 {
		if err := Set(true, "app", "reason"); err != nil {
			t.Fatalf("Set(true): %v", err)
		}
	}
	if n := len(f.Calls()); n != 1 {
		t.Errorf("%d Inhibit calls for two ons, want 1", n)
	}
	for range 2 {
		if err := Set(false, "", ""); err != nil {
			t.Fatalf("Set(false): %v", err)
		}
	}
	if got := f.Active(); got != 0 {
		t.Errorf("%d requests held after off, want 0", got)
	}
}

// Without the service (sway, Hyprland), on fails and holds nothing, so
// a later off does nothing.
func TestSetWithoutService(t *testing.T) {
	screensavertest.StartBus(t)
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
