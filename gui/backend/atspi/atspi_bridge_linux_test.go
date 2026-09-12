//go:build linux

package atspi

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Two bridges must route actions to their own window's callback: a
// shared bridge would deliver window 1's VoiceOver press to window 2.
func TestBridgesRouteActionsToOwningWindow(t *testing.T) {
	var got1, got2 [][2]int
	b1 := &Bridge{}
	b2 := &Bridge{}
	// No session bus headless: Init stays transport-less but must
	// retain the callback.
	b1.Init(func(action, index int) {
		got1 = append(got1, [2]int{action, index})
	})
	b2.Init(func(action, index int) {
		got2 = append(got2, [2]int{action, index})
	})

	ok, _ := (&nodeHandler{bridge: b1, index: 3}).DoAction(0)
	if !ok {
		t.Fatal("DoAction on bridge 1 failed")
	}
	if len(got1) != 1 || got1[0] != [2]int{gui.A11yActionPress, 3} {
		t.Errorf("bridge 1 actions: got %v, want [[press 3]]", got1)
	}
	if len(got2) != 0 {
		t.Errorf("bridge 2 heard window 1's action: %v", got2)
	}

	ok, _ = (&nodeHandler{bridge: b2, index: 5}).DoAction(1)
	if !ok {
		t.Fatal("DoAction on bridge 2 failed")
	}
	if len(got2) != 1 || got2[0] != [2]int{gui.A11yActionConfirm, 5} {
		t.Errorf("bridge 2 actions: got %v, want [[confirm 5]]", got2)
	}
	if len(got1) != 1 {
		t.Errorf("bridge 1 heard window 2's action: %v", got1)
	}
}
