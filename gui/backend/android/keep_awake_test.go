//go:build android

package android

import "testing"

// The host polls the state: it follows each edge and is not cleared on
// read (issue #1172).
func TestPendingKeepAwake(t *testing.T) {
	n := &nativePlatform{}
	t.Cleanup(func() { _ = n.SetKeepAwake(false, "", "") })
	if PendingKeepAwake() {
		t.Fatal("PendingKeepAwake() = true before any request")
	}
	if err := n.SetKeepAwake(true, "app", "reason"); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if !PendingKeepAwake() {
			t.Errorf("read %d: PendingKeepAwake() = false while on", i+1)
		}
	}
	_ = n.SetKeepAwake(false, "", "")
	if PendingKeepAwake() {
		t.Error("PendingKeepAwake() = true after off")
	}
}
