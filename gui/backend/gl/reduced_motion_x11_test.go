//go:build linux && !js && !android

package gl

import "testing"

// PrefersReducedMotion passes the gsettings reading through: only a
// known reduced report counts (issue #757).
func TestPrefersReducedMotionPassesQuery(t *testing.T) {
	old := reducedMotionQuery
	defer func() { reducedMotionQuery = old }()
	n := &nativePlatform{}

	reducedMotionQuery = func() (bool, bool) { return true, true }
	if !n.PrefersReducedMotion() {
		t.Error("reduced known: want reduced motion")
	}

	reducedMotionQuery = func() (bool, bool) { return false, true }
	if n.PrefersReducedMotion() {
		t.Error("animations known on: want no reduced motion")
	}

	reducedMotionQuery = func() (bool, bool) { return false, false }
	if n.PrefersReducedMotion() {
		t.Error("no setting: want no preference")
	}
}
