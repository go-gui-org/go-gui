//go:build linux

package ibus

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestDrainIsFIFOAndReusesBuffer(t *testing.T) {
	c := &Client{}
	for i := range 3 {
		c.push(Event{Kind: KindCommit, Text: string(rune('a' + i))})
	}

	buf := c.Drain(nil)
	if len(buf) != 3 || buf[0].Text != "a" || buf[2].Text != "c" {
		t.Fatalf("drain order wrong: %+v", buf)
	}
	if got := c.Drain(buf[:0]); len(got) != 0 {
		t.Fatalf("second drain returned %d events, want 0", len(got))
	}

	c.push(Event{Kind: KindPreedit, Text: "z"})
	if got := c.Drain(buf[:0]); len(got) != 1 || got[0].Text != "z" {
		t.Fatalf("after refill got %+v", got)
	}
}

func TestPushWakes(t *testing.T) {
	var wakes atomic.Int32
	c := &Client{wake: func() { wakes.Add(1) }}
	c.push(Event{Kind: KindPreedit})
	c.push(Event{Kind: KindCommit})
	if wakes.Load() != 2 {
		t.Fatalf("wake called %d times, want 2", wakes.Load())
	}
}

// TestConcurrentPushDrain is here for the race detector: signals are
// pushed from a D-Bus goroutine while the event loop drains on the main
// thread, which is the one genuinely concurrent path in this package.
func TestConcurrentPushDrain(t *testing.T) {
	const n = 200
	c := &Client{}

	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range n {
			c.push(Event{Kind: KindCommit, Cursor: int32(i)})
		}
	})

	var buf []Event
	got := 0
	for got < n {
		buf = c.Drain(buf[:0])
		got += len(buf)
	}
	wg.Wait()
	if rest := c.Drain(nil); len(rest) != 0 {
		t.Fatalf("%d events left after draining all %d", len(rest), n)
	}
}

func TestNilClientIsInert(t *testing.T) {
	var c *Client
	if c.ProcessKey(1, 2, 3) {
		t.Error("nil client consumed a key")
	}
	c.ProcessKeyRelease(1, 2, 3)
	c.FocusIn()
	c.FocusOut()
	c.SetCursorLocation(0, 0, 1, 1)
	c.Close()
	if got := c.Drain(nil); got != nil {
		t.Errorf("nil client drained %v", got)
	}
}

func TestDisabledClientStopsConsuming(t *testing.T) {
	c := &Client{}
	c.disabled.Store(true)
	if c.ProcessKey(0x61, 38, 0) {
		t.Error("disabled client consumed a key")
	}
	// call must not dereference the nil connection while disabled.
	c.FocusIn()
	c.SetCursorLocation(1, 2, 3, 4)
}
