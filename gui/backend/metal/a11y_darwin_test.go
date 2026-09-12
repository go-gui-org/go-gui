//go:build darwin && cgo && !ios

package metal

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestCStringOrNil_Empty(t *testing.T) {
	var buf []cchar
	got := cStringOrNil("", &buf)
	if got != nil {
		t.Fatal("empty string must return nil")
	}
	if len(buf) != 0 {
		t.Fatalf("collector empty: got %d", len(buf))
	}
}

func TestCStringOrNil_NonEmpty(t *testing.T) {
	var buf []cchar
	got := cStringOrNil("hello", &buf)
	if got == nil {
		t.Fatal("non-empty string must return non-nil")
	}
	if len(buf) != 1 {
		t.Fatalf("collector len: got %d, want 1", len(buf))
	}
	if s := cGoString(got); s != "hello" {
		t.Fatalf("round-trip: got %q, want %q", s, "hello")
	}
	cFree(got)
}

func TestCStringOrNil_MultipleStrings(t *testing.T) {
	var buf []cchar
	s1 := cStringOrNil("first", &buf)
	s2 := cStringOrNil("second", &buf)
	s3 := cStringOrNil("", &buf)

	if s1 == nil || s2 == nil {
		t.Fatal("non-empty strings must return non-nil")
	}
	if s3 != nil {
		t.Fatal("empty string must return nil")
	}
	if len(buf) != 2 {
		t.Fatalf("collector len: got %d, want 2", len(buf))
	}
	if cGoString(s1) != "first" {
		t.Errorf("s1: got %q, want %q", cGoString(s1), "first")
	}
	if cGoString(s2) != "second" {
		t.Errorf("s2: got %q, want %q", cGoString(s2), "second")
	}
	for _, cs := range buf {
		cFree(cs)
	}
}

func TestA11ySyncBridge_ZeroCount(t *testing.T) {
	// Zero or negative count must push the empty clear without
	// panicking, even with no window behind the token.
	a11ySyncBridge(nil, nil, 0, 0, 0)
	a11ySyncBridge(nil, nil, -1, 0, 0)
}

func TestA11ySyncBridge_CountBeyondNodesClears(t *testing.T) {
	// A count past the slice end clamps instead of panicking: nil
	// nodes clear, a short slice syncs only what it holds. A nil
	// window resolves to no context, so both paths return headless.
	a11ySyncBridge(nil, nil, 3, 0, 0)
	a11ySyncBridge(nil, []gui.A11yNode{{Role: gui.AccessRoleButton}}, 3, 0, 600)
}

func TestSetA11yCallback(t *testing.T) {
	called := false
	const token = 1
	setA11yCallback(token, func(action, index int) {
		called = true
	})
	goA11yAction(1, 2, token)
	if !called {
		t.Fatal("callback not invoked via goA11yAction")
	}
	// Nil callback must not panic.
	setA11yCallback(token, nil)
	goA11yAction(0, 0, token)
}

// Actions must reach the owning window's callback: with two windows
// live, window 1's VoiceOver press must not fire window 2's handler.
func TestA11yCallbacksRoutePerWindow(t *testing.T) {
	var got1, got2 [][2]int
	setA11yCallback(11, func(action, index int) {
		got1 = append(got1, [2]int{action, index})
	})
	setA11yCallback(22, func(action, index int) {
		got2 = append(got2, [2]int{action, index})
	})
	t.Cleanup(func() {
		clearA11yCallback(11)
		clearA11yCallback(22)
	})

	goA11yAction(0, 3, 11)
	if len(got1) != 1 || got1[0] != [2]int{0, 3} {
		t.Errorf("window 1 actions: got %v, want [[0 3]]", got1)
	}
	if len(got2) != 0 {
		t.Errorf("window 2 heard window 1's action: %v", got2)
	}

	// Destroying one window's callback must not disturb the other's.
	clearA11yCallback(11)
	goA11yAction(0, 3, 11)
	goA11yAction(4, 5, 22)
	if len(got1) != 1 {
		t.Errorf("cleared callback still fires: %v", got1)
	}
	if len(got2) != 1 || got2[0] != [2]int{4, 5} {
		t.Errorf("window 2 actions: got %v, want [[4 5]]", got2)
	}
}
