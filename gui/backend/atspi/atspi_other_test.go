//go:build !linux

package atspi

import (
	"testing"
)

// TestBridgeNoopInit verifies Init does not panic on noop.
func TestBridgeNoopInit(t *testing.T) {
	var b Bridge
	b.Init(nil)
}

// TestBridgeNoopSync verifies Sync does not panic on noop.
func TestBridgeNoopSync(t *testing.T) {
	var b Bridge
	b.Sync(nil, 0, 0)
}

// TestBridgeNoopDestroy verifies Destroy does not panic on noop.
func TestBridgeNoopDestroy(t *testing.T) {
	var b Bridge
	b.Destroy()
}

// TestBridgeNoopAnnounce verifies Announce does not panic on noop.
func TestBridgeNoopAnnounce(t *testing.T) {
	var b Bridge
	b.Announce("test accessibility announcement")
}

// TestBridgeNoopMultipleCalls verifies repeated calls don't panic.
func TestBridgeNoopMultipleCalls(t *testing.T) {
	var b Bridge
	for range 5 {
		b.Init(nil)
		b.Sync(nil, 0, 0)
		b.Announce("msg")
	}
	b.Destroy()
	// Call after destroy should still be safe.
	b.Announce("after destroy")
}
