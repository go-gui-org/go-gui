package gui

import (
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui/appinfo"
)

// secretHookPlatform keeps secrets in a map through the optional
// secretPlatform hook, as a backend with an OS credential store does.
type secretHookPlatform struct {
	noopNativePlatform
	items map[string][]byte
}

func (p *secretHookPlatform) SecretLoad(service, key string) ([]byte, error) {
	v, ok := p.items[service+"/"+key]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return v, nil
}

func (p *secretHookPlatform) SecretSave(service, key string, value []byte) error {
	p.items[service+"/"+key] = value
	return nil
}

func (p *secretHookPlatform) SecretDelete(service, key string) error {
	delete(p.items, service+"/"+key)
	return nil
}

func TestSecretTestWindowRoundTrip(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	in := []byte("token-123")
	if err := SaveSecret(w, "github", in); err != nil {
		t.Fatal(err)
	}
	// The store must keep its own copy: a caller that zeroes its
	// buffer after the save must not wipe the stored secret.
	clear(in)
	out, err := LoadSecret(w, "github")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "token-123" {
		t.Fatalf("LoadSecret = %q, want token-123", out)
	}
	// The same holds for the slice Load returns.
	clear(out)
	again, err := LoadSecret(w, "github")
	if err != nil || string(again) != "token-123" {
		t.Fatalf("second LoadSecret = %q, %v", again, err)
	}
}

func TestSecretNotFound(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	if _, err := LoadSecret(w, "missing"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("got %v, want ErrSecretNotFound", err)
	}
}

func TestSecretDelete(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	if err := SaveSecret(w, "k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSecret(w, "k"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSecret(w, "k"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("after delete: got %v, want ErrSecretNotFound", err)
	}
	// Deleting a missing secret is not an error: a sign-out path can
	// call it without first checking that a token exists.
	if err := DeleteSecret(w, "k"); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestSecretNoAppID(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	if _, err := LoadSecret(w, "k"); !errors.Is(err, ErrNoAppID) {
		t.Fatalf("Load: got %v, want ErrNoAppID", err)
	}
	if err := SaveSecret(w, "k", []byte("v")); !errors.Is(err, ErrNoAppID) {
		t.Fatalf("Save: got %v, want ErrNoAppID", err)
	}
	if err := DeleteSecret(w, "k"); !errors.Is(err, ErrNoAppID) {
		t.Fatalf("Delete: got %v, want ErrNoAppID", err)
	}
}

func TestSecretRejectsBadKey(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	long := strings.Repeat("k", maxSecretKeyLen+1)
	for _, key := range []string{"", "a/b", `a\b`, "a b", "a:b", "ü", long} {
		if err := SaveSecret(w, key, []byte("v")); err == nil {
			t.Errorf("SaveSecret key %q: want error", key)
		}
		if _, err := LoadSecret(w, key); err == nil || errors.Is(err, ErrSecretNotFound) {
			t.Errorf("LoadSecret key %q: got %v, want a key error", key, err)
		}
		if err := DeleteSecret(w, key); err == nil {
			t.Errorf("DeleteSecret key %q: want error", key)
		}
	}
	for _, key := range []string{"github", "api.token", "user-1_refresh"} {
		if err := SaveSecret(w, key, []byte("v")); err != nil {
			t.Errorf("SaveSecret key %q: %v", key, err)
		}
	}
}

func TestSecretValueBounds(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: settingsTestInfo})
	if err := SaveSecret(w, "k", nil); err == nil {
		t.Error("empty value: want error")
	}
	if err := SaveSecret(w, "k", make([]byte, maxSecretBytes)); err != nil {
		t.Errorf("value at the cap: %v", err)
	}
	if err := SaveSecret(w, "k", make([]byte, maxSecretBytes+1)); err == nil {
		t.Error("value over the cap: want error")
	}
}

// TestSecretUnsupportedWithoutHook covers a real window with no
// credential store (web, Android, an unknown NativePlatform, or no
// platform yet). It must fail; it must never write the secret to a
// plaintext file.
func TestSecretUnsupportedWithoutHook(t *testing.T) {
	cfgDir := useTempConfigDir(t)
	for _, np := range []NativePlatform{nil, noopNativePlatform{}} {
		w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
		w.nativePlatform = np
		if err := SaveSecret(w, "k", []byte("v")); !errors.Is(err, ErrSecretsUnsupported) {
			t.Errorf("Save with %T: got %v, want ErrSecretsUnsupported", np, err)
		}
		if _, err := LoadSecret(w, "k"); !errors.Is(err, ErrSecretsUnsupported) {
			t.Errorf("Load with %T: got %v, want ErrSecretsUnsupported", np, err)
		}
		if err := DeleteSecret(w, "k"); !errors.Is(err, ErrSecretsUnsupported) {
			t.Errorf("Delete with %T: got %v, want ErrSecretsUnsupported", np, err)
		}
	}
	assertNoFiles(t, cfgDir)
}

// assertNoFiles fails when anything was written under dir.
func assertNoFiles(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			t.Errorf("unexpected file %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSecretPlatformHookIsUsed(t *testing.T) {
	p := &secretHookPlatform{items: map[string][]byte{}}
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	w.nativePlatform = p
	if err := SaveSecret(w, "github", []byte("tok")); err != nil {
		t.Fatal(err)
	}
	if got := p.items[settingsTestInfo.ID+"/github"]; !bytes.Equal(got, []byte("tok")) {
		t.Fatalf("hook got %q, want tok under service %q", got, settingsTestInfo.ID)
	}
	out, err := LoadSecret(w, "github")
	if err != nil || string(out) != "tok" {
		t.Fatalf("LoadSecret = %q, %v", out, err)
	}
	if err = DeleteSecret(w, "github"); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadSecret(w, "github"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("after delete: got %v, want ErrSecretNotFound", err)
	}
}

// TestSecretHookOversizeLoad checks the cap also holds on the way in:
// a store item written by something else must not reach the app
// unbounded.
func TestSecretHookOversizeLoad(t *testing.T) {
	p := &secretHookPlatform{items: map[string][]byte{
		settingsTestInfo.ID + "/k": make([]byte, maxSecretBytes+1),
	}}
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	w.nativePlatform = p
	if _, err := LoadSecret(w, "k"); err == nil {
		t.Fatal("want an oversize error")
	}
}

// TestSecretHookEmptyLoad checks that an empty store item, which
// SaveSecret never writes, reads as missing, not as a zero-byte secret.
func TestSecretHookEmptyLoad(t *testing.T) {
	for _, v := range [][]byte{nil, {}} {
		p := &secretHookPlatform{items: map[string][]byte{settingsTestInfo.ID + "/k": v}}
		w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
		w.nativePlatform = p
		if _, err := LoadSecret(w, "k"); !errors.Is(err, ErrSecretNotFound) {
			t.Errorf("stored %#v: got %v, want ErrSecretNotFound", v, err)
		}
	}
}

func TestSecretBadAppID(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{AppInfo: appinfo.Info{ID: "a/b"}})
	if err := SaveSecret(w, "k", []byte("v")); err == nil {
		t.Fatal("want an app ID error")
	}
}

// blockingSecretPlatform's SecretLoad waits until release is closed, as
// an OS store does while its unlock prompt is open.
type blockingSecretPlatform struct {
	secretHookPlatform
	entered chan struct{}
	release chan struct{}
}

func (p *blockingSecretPlatform) SecretLoad(service, key string) ([]byte, error) {
	close(p.entered)
	<-p.release
	return p.secretHookPlatform.SecretLoad(service, key)
}

// TestSecretHookRunsWithoutSettingsLock checks that a secret call
// blocked in the OS store does not hold the settings lock: settings
// calls, and the UI thread that makes them, must go on.
func TestSecretHookRunsWithoutSettingsLock(t *testing.T) {
	useTempConfigDir(t)
	p := &blockingSecretPlatform{
		secretHookPlatform: secretHookPlatform{items: map[string][]byte{}},
		entered:            make(chan struct{}),
		release:            make(chan struct{}),
	}
	w := NewWindow(WindowCfg{AppInfo: settingsTestInfo})
	w.nativePlatform = p
	loaded := make(chan error, 1)
	go func() {
		_, err := LoadSecret(w, "k")
		loaded <- err
	}()
	<-p.entered
	saved := make(chan error, 1)
	go func() { saved <- SaveSettings(w, testSettings{FontSize: 12}) }()
	select {
	case err := <-saved:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		close(p.release)
		t.Fatal("SaveSettings blocked while a secret call waited in the OS store")
	}
	close(p.release)
	if err := <-loaded; !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("LoadSecret: got %v, want ErrSecretNotFound", err)
	}
}
