//go:build darwin && cgo && !ios

package metal

import (
	"os/exec"
	"strings"
	"testing"
)

// assertions returns what pmset lists as held power assertions.
func assertions(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("pmset", "-g", "assertions").Output()
	if err != nil {
		t.Skipf("pmset -g assertions: %v", err)
	}
	return string(out)
}

// Issue #1172: the request reaches macOS. While it is held, pmset lists
// an assertion with its reason; after the release it does not. A second
// on or off changes nothing.
func TestKeepAwakeAssertion(t *testing.T) {
	const reason = "go-gui keep-awake test"
	n := &nativePlatform{}
	t.Cleanup(func() { _ = n.SetKeepAwake(false, "", "") })

	for range 2 {
		if err := n.SetKeepAwake(true, "go-gui", reason); err != nil {
			t.Fatalf("SetKeepAwake(true): %v", err)
		}
	}
	held := assertions(t)
	if strings.Count(held, reason) != 1 {
		t.Errorf("pmset lists the reason %d times while held, want 1:\n%s",
			strings.Count(held, reason), held)
	}
	for range 2 {
		if err := n.SetKeepAwake(false, "", ""); err != nil {
			t.Fatalf("SetKeepAwake(false): %v", err)
		}
	}
	if after := assertions(t); strings.Contains(after, reason) {
		t.Errorf("pmset still lists the reason after release:\n%s", after)
	}
}
