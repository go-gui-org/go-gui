package gui

import (
	"errors"
	"fmt"
)

// ErrSecretNotFound is returned by LoadSecret when the store holds no
// secret under the key.
// exportaudit:keep — caller-facing errors.Is target (issue #920)
var ErrSecretNotFound = errors.New("gui: secret not found")

// ErrSecretsUnsupported is returned by LoadSecret, SaveSecret and
// DeleteSecret when the platform has no OS credential store go-gui can
// use: web, Android, a Linux session with no Secret Service daemon, or
// a window with no platform attached yet. The secret is never written
// to a plain file in its place; the app decides what to do instead.
// exportaudit:keep — caller-facing errors.Is target (issue #920)
var ErrSecretsUnsupported = errors.New("gui: no secure credential store on this platform")

// maxSecretBytes caps one secret. It is the Windows Credential Manager
// limit (CRED_MAX_CREDENTIAL_BLOB_SIZE, 5*512 bytes). The same cap on
// every platform means a secret an app saves on macOS also fits on
// Windows, so the limit is found in development, not by a user.
const maxSecretBytes = 5 * 512

// maxSecretKeyLen caps a key. Keys are short names ("github",
// "refresh-token"); the cap keeps a store's item name bounded.
const maxSecretKeyLen = 128

// secretPlatform is the optional NativePlatform hook for an OS
// credential store (Keychain, Credential Manager, Secret Service). It
// is not part of NativePlatform, so no existing implementer breaks. A
// platform without it gets ErrSecretsUnsupported.
//
// service is the app ID; key is the caller's checked key. SecretLoad
// returns ErrSecretNotFound when nothing is stored, and SecretDelete
// returns nil for a missing secret. A store that is absent at run time
// (no daemon) returns an error that wraps ErrSecretsUnsupported.
type secretPlatform interface {
	SecretLoad(service, key string) ([]byte, error)
	SecretSave(service, key string, value []byte) error
	SecretDelete(service, key string) error
}

// LoadSecret returns the secret the app saved under key, read from the
// OS credential store: the Keychain on macOS and iOS, Credential
// Manager on Windows, and the Secret Service (GNOME Keyring, KWallet)
// on Linux. The service name is the app ID (WindowCfg.AppInfo.ID), so
// every window of the app sees the same secrets.
//
// It returns ErrSecretNotFound when nothing is saved under key, and
// ErrSecretsUnsupported when the platform has no store. The returned
// slice is the caller's own copy; clear it when done with it.
//
// A window from NewTestWindow uses a memory store, so tests never
// touch the real credential store.
//
// The call can block while the OS asks the user to unlock the store or
// to allow access (macOS asks again after each rebuild of an unsigned
// binary). Do not call it from a view function or an event callback:
// call it from a goroutine and post the result with QueueCommand. The
// call holds no window lock while it waits.
func LoadSecret(w *Window, key string) ([]byte, error) {
	appID, err := w.secretCheck(key)
	if err != nil {
		return nil, err
	}
	if v, ok, memErr := w.secretMemoryLoad(key); ok {
		return v, memErr
	}
	hook := w.secretHook()
	if hook == nil {
		return nil, ErrSecretsUnsupported
	}
	v, err := hook.SecretLoad(appID, key)
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			return nil, ErrSecretNotFound
		}
		return nil, fmt.Errorf("gui: load secret %q: %w", key, err)
	}
	// SaveSecret never stores an empty value, so an empty item came from
	// another program or a half-written entry. Report it as missing
	// rather than as a zero-byte token the app would try to use.
	if len(v) == 0 {
		return nil, ErrSecretNotFound
	}
	// The store may hold an item another program wrote under the same
	// name. Bound what reaches the app.
	if len(v) > maxSecretBytes {
		clear(v)
		return nil, fmt.Errorf("gui: stored secret %q is %d bytes, over the %d-byte limit",
			key, len(v), maxSecretBytes)
	}
	return v, nil
}

// SaveSecret stores value under key in the OS credential store,
// replacing what was saved before. value must be 1 to 2560 bytes (the
// Windows limit, applied on every platform). The store keeps its own
// copy, so the caller can clear value after the call. See LoadSecret
// for where the secret goes and for blocking.
func SaveSecret(w *Window, key string, value []byte) error {
	appID, err := w.secretCheck(key)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		return errors.New("gui: SaveSecret: empty value; use DeleteSecret to remove a secret")
	}
	if len(value) > maxSecretBytes {
		return fmt.Errorf("gui: secret is %d bytes, over the %d-byte limit",
			len(value), maxSecretBytes)
	}
	if w.secretMemorySave(key, value) {
		return nil
	}
	hook := w.secretHook()
	if hook == nil {
		return ErrSecretsUnsupported
	}
	if err = hook.SecretSave(appID, key, value); err != nil {
		return fmt.Errorf("gui: save secret %q: %w", key, err)
	}
	return nil
}

// DeleteSecret removes the secret saved under key. A missing secret is
// not an error, so a sign-out path can call it unconditionally.
func DeleteSecret(w *Window, key string) error {
	appID, err := w.secretCheck(key)
	if err != nil {
		return err
	}
	if w.secretMemoryDelete(key) {
		return nil
	}
	hook := w.secretHook()
	if hook == nil {
		return ErrSecretsUnsupported
	}
	if err = hook.SecretDelete(appID, key); err != nil {
		return fmt.Errorf("gui: delete secret %q: %w", key, err)
	}
	return nil
}

// The secretMemory* helpers run the NewTestWindow memory store. Each
// reports ok=false for a real window. They hold w.settings.mu only for
// the map access: the hook path runs with the lock released, because an
// OS store can block for minutes on an unlock prompt, and holding the
// lock there would also block every settings call and the UI thread
// with it. The OS store does its own locking.

// secretMemoryLoad returns a copy of the stored value, so a caller that
// clears its slice does not wipe the stored secret.
func (w *Window) secretMemoryLoad(key string) (v []byte, ok bool, err error) {
	w.settings.mu.Lock()
	defer w.settings.mu.Unlock()
	if !w.settings.memory {
		return nil, false, nil
	}
	stored, found := w.settings.secrets[key]
	if !found {
		return nil, true, ErrSecretNotFound
	}
	return append([]byte(nil), stored...), true, nil
}

// secretMemorySave stores a copy of value, so the caller can clear it.
func (w *Window) secretMemorySave(key string, value []byte) bool {
	w.settings.mu.Lock()
	defer w.settings.mu.Unlock()
	if !w.settings.memory {
		return false
	}
	if w.settings.secrets == nil {
		w.settings.secrets = make(map[string][]byte)
	}
	w.settings.secrets[key] = append([]byte(nil), value...)
	return true
}

func (w *Window) secretMemoryDelete(key string) bool {
	w.settings.mu.Lock()
	defer w.settings.mu.Unlock()
	if !w.settings.memory {
		return false
	}
	if v, found := w.settings.secrets[key]; found {
		clear(v)
		delete(w.settings.secrets, key)
	}
	return true
}

// secretHook returns the platform's credential store hook, or nil when
// there is no platform or it has no store.
func (w *Window) secretHook() secretPlatform {
	sp, _ := w.nativePlatform.(secretPlatform)
	return sp
}

// secretCheck returns the app ID used as the service name, after it
// checks the app ID and the key. The key goes into store item names
// (the Windows target is "<app ID>/<key>"), so it gets a plain charset:
// letters, digits, '.', '-' and '_'.
func (w *Window) secretCheck(key string) (string, error) {
	appID, err := w.settingsAppID()
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", errors.New("gui: secret key is empty")
	}
	if len(key) > maxSecretKeyLen {
		return "", fmt.Errorf("gui: secret key is %d bytes, over the %d-byte limit",
			len(key), maxSecretKeyLen)
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		ok := c == '.' || c == '-' || c == '_' ||
			(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !ok {
			return "", fmt.Errorf("gui: secret key %q may hold only letters, digits, '.', '-' and '_'", key)
		}
	}
	return appID, nil
}
