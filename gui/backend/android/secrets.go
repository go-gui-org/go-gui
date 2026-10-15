//go:build android

package android

import (
	"github.com/go-gui-org/go-gui/gui/backend/internal/hostsecret"
)

// SecretStore is implemented by the Android host to keep secrets in
// Android's credential storage. Register it with SetSecretStore before
// Start runs the window's OnInit callback.
//
// Load returns an empty slice when the item does not exist. Delete succeeds
// when the item is already absent. Implementations must be safe for calls
// from concurrent goroutines.
type SecretStore interface {
	Load(service, key string) ([]byte, error)
	Save(service, key string, value []byte) error
	Delete(service, key string) error
}

var androidSecretStore hostsecret.Registry

// SetSecretStore registers the credential store supplied by the Android
// host. A nil store unregisters it. Call this before Start.
func SetSecretStore(store SecretStore) {
	androidSecretStore.Set(store)
}

func (n *nativePlatform) SecretLoad(service, key string) ([]byte, error) {
	return androidSecretStore.Load(service, key)
}

func (n *nativePlatform) SecretSave(service, key string, value []byte) error {
	return androidSecretStore.Save(service, key, value)
}

func (n *nativePlatform) SecretDelete(service, key string) error {
	return androidSecretStore.Delete(service, key)
}

// gui finds the hook by an unexported structural interface. Keep an explicit
// assertion so a changed method signature cannot silently disable secrets.
var _ interface {
	SecretLoad(service, key string) ([]byte, error)
	SecretSave(service, key string, value []byte) error
	SecretDelete(service, key string) error
} = (*nativePlatform)(nil)
