// Package keyring reads and writes app secrets in the OS credential
// store, for gui.LoadSecret, gui.SaveSecret and gui.DeleteSecret
// (issue #920):
//
//   - macOS and iOS: the Keychain, as generic password items
//     (service = app ID, account = key);
//   - Windows: Credential Manager, as generic credentials with the
//     target "<app ID>/<key>";
//   - Linux: the Secret Service D-Bus API (GNOME Keyring, KWallet), in
//     the default collection;
//   - everywhere else: gui.ErrSecretsUnsupported.
//
// Each backend's NativePlatform forwards its SecretLoad, SecretSave and
// SecretDelete methods here. The package is shared so that the Metal and
// iOS backends use one Keychain implementation, and so that the GL
// backend gets the Linux and Windows code without build tags of its own.
//
// Load returns gui.ErrSecretNotFound for a missing item. Delete returns
// nil for a missing item. An error for a store that is absent at run
// time wraps gui.ErrSecretsUnsupported.
package keyring

import "github.com/go-gui-org/go-gui/gui"

// Platform implements gui's optional secret-store hook. A backend
// embeds it in its NativePlatform, so the three methods have one
// spelling and the hook's method set is asserted once, below.
type Platform struct{}

// SecretLoad implements the hook with Load.
func (Platform) SecretLoad(service, key string) ([]byte, error) { return Load(service, key) }

// SecretSave implements the hook with Save.
func (Platform) SecretSave(service, key string, value []byte) error {
	return Save(service, key, value)
}

// SecretDelete implements the hook with Delete.
func (Platform) SecretDelete(service, key string) error { return Delete(service, key) }

// The hook in gui is unexported and found by a type assertion, so a
// drifted method set here would drop every backend back to
// ErrSecretsUnsupported with no compile error. This assertion makes it
// one.
var _ interface {
	SecretLoad(service, key string) ([]byte, error)
	SecretSave(service, key string, value []byte) error
	SecretDelete(service, key string) error
} = Platform{}

// unsupported wraps gui.ErrSecretsUnsupported with the reason.
type unsupported struct{ reason string }

func (e unsupported) Error() string { return gui.ErrSecretsUnsupported.Error() + ": " + e.reason }

func (e unsupported) Unwrap() error { return gui.ErrSecretsUnsupported }
