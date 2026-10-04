//go:build linux && !android

package keyring

import (
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/go-gui-org/go-gui/gui"
)

// TestMapErrNoDaemon checks that a bus with no Secret Service owner
// reports ErrSecretsUnsupported, the error an app can act on, and that
// other D-Bus errors do not.
func TestMapErrNoDaemon(t *testing.T) {
	for _, name := range []string{
		"org.freedesktop.DBus.Error.ServiceUnknown",
		"org.freedesktop.DBus.Error.NameHasNoOwner",
		"org.freedesktop.DBus.Error.Spawn.ServiceNotFound",
	} {
		// godbus returns a reply error as a dbus.Error value.
		if err := mapErr("op", dbus.Error{Name: name}); !errors.Is(err, gui.ErrSecretsUnsupported) {
			t.Errorf("%s: got %v, want ErrSecretsUnsupported", name, err)
		}
	}
	err := mapErr("op", dbus.Error{Name: "org.freedesktop.Secret.Error.IsLocked"})
	if errors.Is(err, gui.ErrSecretsUnsupported) {
		t.Errorf("IsLocked mapped to ErrSecretsUnsupported: %v", err)
	}
}
