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
	if !PendingKeepAwake() || !PendingKeepAwake() {
		t.Error("PendingKeepAwake() is not true on every read while on")
	}
	_ = n.SetKeepAwake(false, "", "")
	if PendingKeepAwake() {
		t.Error("PendingKeepAwake() = true after off")
	}
}
