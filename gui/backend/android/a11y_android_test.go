//go:build android

package android

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Counts past the slice end clamp instead of panicking on the
// slice expression; negative counts clear.
func TestSyncA11yClampsCount(t *testing.T) {
	nodes := []gui.A11yNode{{Role: gui.AccessRoleButton}}
	syncA11y(nil, 5, 0)
	if got := A11yNodeCount(); got != 0 {
		t.Fatalf("over-count: got %d nodes, want 0", got)
	}
	syncA11y(nodes, -1, 0)
	if got := A11yNodeCount(); got != 0 {
		t.Fatalf("negative: got %d nodes, want 0", got)
	}
	syncA11y(nodes, 3, 0)
	if got := A11yNodeCount(); got != 1 {
		t.Fatalf("clamped: got %d nodes, want 1", got)
	}
	destroyA11y()
}

// Kotlin can pass any int32, including the -1 sentinel that
// A11yNodeParent returns for the root. A negative index must hit
// the fallback, not panic on the slice index: a Go panic inside a
// gomobile call kills the app process.
func TestA11yGettersRejectNegativeIndex(t *testing.T) {
	syncA11y([]gui.A11yNode{{Role: gui.AccessRoleButton, Label: "x", ParentIdx: 0}}, 1, 0)
	defer destroyA11y()
	for _, idx := range []int32{-1, -2147483648, 1} {
		if got := A11yNodeRole(idx); got != 0 {
			t.Errorf("Role(%d) = %d, want 0", idx, got)
		}
		if got := A11yNodeLabel(idx); got != "" {
			t.Errorf("Label(%d) = %q, want empty", idx, got)
		}
		if got := A11yNodeValue(idx); got != "" {
			t.Errorf("Value(%d) = %q, want empty", idx, got)
		}
		if got := A11yNodeDescription(idx); got != "" {
			t.Errorf("Description(%d) = %q, want empty", idx, got)
		}
		for name, fn := range map[string]func(int32) float32{
			"BoundsX": A11yNodeBoundsX, "BoundsY": A11yNodeBoundsY,
			"BoundsW": A11yNodeBoundsW, "BoundsH": A11yNodeBoundsH,
			"ValueNum": A11yNodeValueNum, "ValueMin": A11yNodeValueMin,
			"ValueMax": A11yNodeValueMax,
		} {
			if got := fn(idx); got != 0 {
				t.Errorf("%s(%d) = %v, want 0", name, idx, got)
			}
		}
		if got := A11yNodeState(idx); got != 0 {
			t.Errorf("State(%d) = %d, want 0", idx, got)
		}
		if got := A11yNodeParent(idx); got != -1 {
			t.Errorf("Parent(%d) = %d, want -1", idx, got)
		}
		if got := A11yNodeChildStart(idx); got != 0 {
			t.Errorf("ChildStart(%d) = %d, want 0", idx, got)
		}
		if got := A11yNodeChildCount(idx); got != 0 {
			t.Errorf("ChildCount(%d) = %d, want 0", idx, got)
		}
	}
	// In-range index still reads through.
	if got := A11yNodeLabel(0); got != "x" {
		t.Errorf("Label(0) = %q, want x", got)
	}
}
