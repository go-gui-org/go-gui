//go:build linux

package ibus

import (
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// Integration tests against the input method actually running on this
// machine. Gated like the X11 and clipboard tests: a build machine has
// no ibus-daemon, and a developer machine's engine list is unknown.
func requireIBus(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("GOGUI_IBUS_IT") != "1" {
		t.Skip("set GOGUI_IBUS_IT=1 to run the live input-method tests")
	}
	c := New("go-gui-test", nil)
	if c == nil {
		t.Fatal("New: no input method reachable")
	}
	t.Cleanup(c.Close)
	return c
}

func TestLiveConnect(t *testing.T) {
	c := requireIBus(t)
	if c.disabled.Load() {
		t.Fatal("client latched off during setup")
	}
}

// A round trip through the daemon: focus the context, offer a key, and
// confirm the transport survived. Whether the key is consumed depends
// on the engine the user has active, so only the transport is asserted.
func TestLiveProcessKey(t *testing.T) {
	c := requireIBus(t)
	c.FocusIn()
	c.SetCursorLocation(100, 200, 2, 20)

	const xkA, evdevA = 0x61, 30
	consumed := c.ProcessKey(xkA, evdevA, 0)
	c.ProcessKeyRelease(xkA, evdevA, 0)
	t.Logf("engine consumed 'a': %v", consumed)

	if c.disabled.Load() {
		t.Fatal("client latched off: the daemon rejected a call")
	}
	c.FocusOut()
	if c.disabled.Load() {
		t.Fatal("client latched off during FocusOut")
	}
}

// Signals have to reach the queue and fire the wake callback. Only runs
// meaningfully with a composing engine selected; with a latin engine
// nothing is produced and the test reports that rather than failing.
func TestLiveSignals(t *testing.T) {
	if os.Getenv("GOGUI_IBUS_IT") != "1" {
		t.Skip("set GOGUI_IBUS_IT=1 to run the live input-method tests")
	}
	var wakes atomic.Int32
	c := New("go-gui-test", func() { wakes.Add(1) })
	if c == nil {
		t.Fatal("New: no input method reachable")
	}
	defer c.Close()

	c.FocusIn()
	// "nihon" on a kana engine composes; on a latin engine it does not.
	for _, k := range []struct{ keyval, code uint32 }{
		{'n', 49}, {'i', 23}, {'h', 35}, {'o', 24}, {'n', 49},
	} {
		c.ProcessKey(k.keyval, k.code, 0)
		c.ProcessKeyRelease(k.keyval, k.code, 0)
	}
	time.Sleep(200 * time.Millisecond)

	evs := c.Drain(nil)
	if len(evs) == 0 {
		t.Skip("no composition: the active engine is not a composing one")
	}
	for _, e := range evs {
		t.Logf("kind=%d text=%q cursor=%d", e.Kind, e.Text, e.Cursor)
	}
	if wakes.Load() == 0 {
		t.Error("events were queued but wake was never called")
	}
	c.FocusOut()
}
