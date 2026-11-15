package gui

import "testing"

// debugCheckWindowHide belongs to DebugWindowDegraded, like the other
// window features a platform can refuse, and DebugAll covers it. Kept
// beside TestCheckCategoryMapping's table (debug_test.go), which lists
// the other checks.
func TestWindowHideCheckCategory(t *testing.T) {
	if got := checkCategory(debugCheckWindowHide); got != DebugWindowDegraded {
		t.Errorf("checkCategory(debugCheckWindowHide) = %v, want DebugWindowDegraded", got)
	}
	if DebugAll&DebugWindowDegraded == 0 {
		t.Error("DebugAll does not cover DebugWindowDegraded")
	}
}
