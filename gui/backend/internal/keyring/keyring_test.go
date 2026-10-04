package keyring

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func TestUnsupportedWrapsSentinel(t *testing.T) {
	err := error(unsupported{reason: "no daemon"})
	if !errors.Is(err, gui.ErrSecretsUnsupported) {
		t.Fatalf("%v does not wrap ErrSecretsUnsupported", err)
	}
}

// TestRealStoreRoundTrip writes to the user's real credential store, so
// it runs only when GOGUI_SECRET_IT=1. On macOS the first run of each
// new test binary may show a Keychain access prompt.
func TestRealStoreRoundTrip(t *testing.T) {
	if os.Getenv("GOGUI_SECRET_IT") != "1" {
		t.Skip("set GOGUI_SECRET_IT=1 to use the real credential store")
	}
	const service = "org.go-gui.keyring-test"
	// A unique key, so a run left over from a crash does not collide.
	key := fmt.Sprintf("it-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = Delete(service, key) })

	if _, err := Load(service, key); errors.Is(err, gui.ErrSecretsUnsupported) {
		t.Skipf("no credential store: %v", err)
	} else if !errors.Is(err, gui.ErrSecretNotFound) {
		t.Fatalf("Load before Save: got %v, want ErrSecretNotFound", err)
	}
	if err := Save(service, key, []byte("first")); err != nil {
		t.Fatal(err)
	}
	// A second Save replaces the item; it must not fail as a duplicate.
	if err := Save(service, key, []byte("second\x00bytes")); err != nil {
		t.Fatal(err)
	}
	got, err := Load(service, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second\x00bytes" {
		t.Fatalf("Load = %q, want the second value", got)
	}
	if err = Delete(service, key); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(service, key); !errors.Is(err, gui.ErrSecretNotFound) {
		t.Fatalf("Load after Delete: got %v, want ErrSecretNotFound", err)
	}
	if err = Delete(service, key); err != nil {
		t.Fatalf("Delete of a missing item: %v", err)
	}
}
