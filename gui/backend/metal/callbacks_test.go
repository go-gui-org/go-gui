//go:build darwin && cgo && !ios

package metal

import (
	"sync"
	"testing"
)

func resetRegistry() {
	windowRegistryMu.Lock()
	clear(windowRegistry)
	windowRegistryMu.Unlock()
}

func TestWindowRegistry_RegisterLookup(t *testing.T) {
	resetRegistry()

	ws := &windowState{}
	registerWindow(1, ws)
	if got := lookupWindow(1); got != ws {
		t.Fatal("lookupWindow did not return registered state")
	}
}

func TestWindowRegistry_Unregister(t *testing.T) {
	resetRegistry()

	ws := &windowState{}
	registerWindow(42, ws)
	unregisterWindow(42)
	if got := lookupWindow(42); got != nil {
		t.Fatal("lookupWindow returned state after unregister")
	}
}

func TestWindowRegistry_LookupMissing(t *testing.T) {
	resetRegistry()

	if got := lookupWindow(999); got != nil {
		t.Fatal("lookupWindow for nonexistent ID must return nil")
	}
}

func TestWindowRegistry_ConcurrentRegister(t *testing.T) {
	resetRegistry()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			registerWindow(uint32(id), &windowState{})
		}(i)
	}
	wg.Wait()

	// All registrations must be visible.
	for i := range 10 {
		if lookupWindow(uint32(i)) == nil {
			t.Errorf("id %d not found after concurrent register", i)
		}
	}
}

// TestPumpWindows_ReentrantKeepsSnapshot re-enters the pump from inside the
// pump callback, the way a nested runloop's frame timer does when app code
// run by PumpFrame opens a modal dialog. The outer iteration must still see
// every registered window, each once, and never a nil entry. Before the fix
// the inner call truncated and cleared the shared buffer the outer range
// was walking.
func TestPumpWindows_ReentrantKeepsSnapshot(t *testing.T) {
	resetRegistry()
	t.Cleanup(resetRegistry)

	const n = 4
	for i := range n {
		registerWindow(uint32(i), &windowState{})
	}

	seen := make(map[*windowState]int, n)
	depth := 0
	var pump func(*windowState)
	pump = func(ws *windowState) {
		if ws == nil {
			t.Fatal("pump received nil windowState: shared buffer clobbered")
		}
		if depth > 0 {
			return // inner pass: only its side effect on the buffer matters
		}
		seen[ws]++
		if len(seen) == 1 {
			depth++
			pumpWindows(pump)
			depth--
		}
	}
	pumpWindows(pump)

	if len(seen) != n {
		t.Fatalf("outer pass saw %d distinct windows, want %d", len(seen), n)
	}
	for ws, c := range seen {
		if c != 1 {
			t.Errorf("window %p pumped %d times, want 1", ws, c)
		}
	}
}

// TestPumpWindows_SteadyStateNoAlloc guards the 60 Hz pump: outside
// re-entry the snapshot buffer is reused, so a tick allocates nothing.
func TestPumpWindows_SteadyStateNoAlloc(t *testing.T) {
	resetRegistry()
	t.Cleanup(resetRegistry)
	for i := range 3 {
		registerWindow(uint32(i), &windowState{})
	}
	noop := func(*windowState) {}
	pumpWindows(noop) // warm the buffer
	if a := testing.AllocsPerRun(100, func() { pumpWindows(noop) }); a != 0 {
		t.Fatalf("pumpWindows allocated %v per run, want 0", a)
	}
}

// TestPumpWindows_EmptyRegistry checks a tick with no windows calls nothing.
func TestPumpWindows_EmptyRegistry(t *testing.T) {
	resetRegistry()
	t.Cleanup(resetRegistry)

	calls := 0
	pumpWindows(func(*windowState) { calls++ })
	if calls != 0 {
		t.Fatalf("pump called %d times with no windows, want 0", calls)
	}
}

// TestGoMetalPumpFrames_UnattachedWindowNoPanic covers the real pump path: a
// registered window with no attached gui.Window or Metal context (the gap
// between registerWindow and attach, or after Destroy) must be skipped.
func TestGoMetalPumpFrames_UnattachedWindowNoPanic(t *testing.T) {
	resetRegistry()
	t.Cleanup(resetRegistry)
	registerWindow(1, &windowState{})

	before := framePumpCount.Load()
	goMetalPumpFrames()
	if framePumpCount.Load() != before+1 {
		t.Fatal("framePumpCount not incremented")
	}
}
