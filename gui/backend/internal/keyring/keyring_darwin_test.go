//go:build darwin && cgo

package keyring

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestRealStoreOversizeLoad writes an item over gui's cap straight
// through Save (which, unlike gui.SaveSecret, does not check the size)
// and checks Load refuses it before copying it into Go memory. It uses
// the real Keychain, so it runs only when GOGUI_SECRET_IT=1.
func TestRealStoreOversizeLoad(t *testing.T) {
	if os.Getenv("GOGUI_SECRET_IT") != "1" {
		t.Skip("set GOGUI_SECRET_IT=1 to use the real credential store")
	}
	const service = "org.go-gui.keyring-test"
	key := fmt.Sprintf("it-big-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = Delete(service, key) })
	if err := Save(service, key, make([]byte, maxItemBytes+1)); err != nil {
		t.Fatal(err)
	}
	if v, err := Load(service, key); err == nil {
		t.Fatalf("Load returned %d bytes, want an oversize error", len(v))
	}
}
