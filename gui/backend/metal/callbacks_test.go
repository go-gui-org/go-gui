//go:build darwin && !ios

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
