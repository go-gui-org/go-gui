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
