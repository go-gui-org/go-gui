//go:build darwin && (!ios || cgo)

package keyring

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// statusErr maps the two "no keychain" codes to ErrSecretsUnsupported
// and gives every other code the framework's own message. The message
// comes from SecCopyErrorMessageString through purego (#1307), so this
// also checks the CFString copy-out.
func TestStatusErr(t *testing.T) {
	if !errors.Is(statusErr(errSecItemNotFound), gui.ErrSecretNotFound) {
		t.Error("errSecItemNotFound: want gui.ErrSecretNotFound")
	}
	for _, st := range []int32{errSecNotAvailable, errSecNoSuchKeychain} {
		if !errors.Is(statusErr(st), gui.ErrSecretsUnsupported) {
			t.Errorf("OSStatus %d: want gui.ErrSecretsUnsupported", st)
		}
	}
	// errSecAuthFailed (-25293) has a framework message. The exact text
	// changes between macOS releases, so only its presence is checked.
	msg := statusErr(-25293).Error()
	if !strings.HasSuffix(msg, "(OSStatus -25293)") || msg == "keychain: OSStatus -25293" {
		t.Errorf("statusErr(-25293) = %q, want the framework message and the code", msg)
	}
}

// A service or key that is not valid UTF-8 is an error, not a crash.
// CFString gives NULL for it, and before the fix that NULL went into
// the query dictionary, where CFRetain(NULL) stopped the process. It
// never reaches the real Keychain, so it runs without GOGUI_SECRET_IT.
func TestInvalidUTF8Name(t *testing.T) {
	const bad = "\xff\xfe"
	if _, err := Load(bad, "k"); !errors.Is(err, errInvalidName) {
		t.Errorf("Load(bad service) = %v, want errInvalidName", err)
	}
	if err := Save("org.go-gui.keyring-test", bad, []byte("v")); !errors.Is(err, errInvalidName) {
		t.Errorf("Save(bad key) = %v, want errInvalidName", err)
	}
	if err := Delete(bad, bad); !errors.Is(err, errInvalidName) {
		t.Errorf("Delete(bad names) = %v, want errInvalidName", err)
	}
}
