package hostsecret

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

type fakeStore struct {
	service string
	key     string
	value   []byte
	err     error
}

func (s *fakeStore) Load(service, key string) ([]byte, error) {
	s.service, s.key = service, key
	return s.value, s.err
}

func (s *fakeStore) Save(service, key string, value []byte) error {
	s.service, s.key = service, key
	s.value = append([]byte(nil), value...)
	return s.err
}

func (s *fakeStore) Delete(service, key string) error {
	s.service, s.key = service, key
	s.value = nil
	return s.err
}

func TestRegistryUnavailableWithoutHost(t *testing.T) {
	registry := &Registry{}
	if _, err := registry.Load("app.id", "token"); !errors.Is(err, gui.ErrSecretsUnsupported) {
		t.Fatalf("Load error = %v, want ErrSecretsUnsupported", err)
	}
	if err := registry.Save("app.id", "token", []byte("value")); !errors.Is(err, gui.ErrSecretsUnsupported) {
		t.Fatalf("Save error = %v, want ErrSecretsUnsupported", err)
	}
	if err := registry.Delete("app.id", "token"); !errors.Is(err, gui.ErrSecretsUnsupported) {
		t.Fatalf("Delete error = %v, want ErrSecretsUnsupported", err)
	}
}

func TestRegistryForwardsToHost(t *testing.T) {
	store := &fakeStore{}
	registry := &Registry{}
	registry.Set(store)

	if err := registry.Save("app.id", "token", []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if store.service != "app.id" || store.key != "token" || !bytes.Equal(store.value, []byte("secret")) {
		t.Fatalf("Save forwarded service=%q key=%q value=%q", store.service, store.key, store.value)
	}

	loaded, err := registry.Load("app.id", "token")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, []byte("secret")) {
		t.Fatalf("Load = %q, want secret", loaded)
	}

	if err = registry.Delete("app.id", "token"); err != nil {
		t.Fatal(err)
	}
	if store.value != nil {
		t.Fatalf("Delete left value %q", store.value)
	}
}

func TestRegistryMapsEmptyLoadToNotFound(t *testing.T) {
	registry := &Registry{}
	registry.Set(&fakeStore{})

	value, err := registry.Load("app.id", "token")
	if !errors.Is(err, gui.ErrSecretNotFound) {
		t.Fatalf("Load error = %v, want ErrSecretNotFound", err)
	}
	if value != nil {
		t.Fatalf("Load value = %q, want nil", value)
	}
}

func TestRegistryPreservesHostError(t *testing.T) {
	want := errors.New("host failed")
	registry := &Registry{}
	registry.Set(&fakeStore{err: want})

	if _, err := registry.Load("app.id", "token"); !errors.Is(err, want) {
		t.Fatalf("Load error = %v, want %v", err, want)
	}
}
