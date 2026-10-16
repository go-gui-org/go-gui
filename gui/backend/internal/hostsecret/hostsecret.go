// Package hostsecret adapts a host-provided credential store to go-gui's
// secret hook contract.
package hostsecret

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// Store is the credential store supplied by a native host.
type Store interface {
	Load(service, key string) ([]byte, error)
	Save(service, key string, value []byte) error
	Delete(service, key string) error
}

// Registry holds the currently registered host store. It does not hold its
// lock across host calls, which may block.
type Registry struct {
	// mu is unexported so callers cannot take the lock from outside.
	mu    sync.RWMutex
	store Store
}

// Set replaces the registered store. A nil store unregisters it.
func (r *Registry) Set(store Store) {
	r.mu.Lock()
	r.store = store
	r.mu.Unlock()
}

func (r *Registry) current() Store {
	r.mu.RLock()
	store := r.store
	r.mu.RUnlock()
	return store
}

// Load forwards a load and normalizes the host's empty missing-value sentinel
// to the go-gui hook contract.
func (r *Registry) Load(service, key string) ([]byte, error) {
	store := r.current()
	if store == nil {
		return nil, gui.ErrSecretsUnsupported
	}
	value, err := store.Load(service, key)
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return nil, gui.ErrSecretNotFound
	}
	return value, nil
}

// Save forwards a save to the registered host store.
func (r *Registry) Save(service, key string, value []byte) error {
	store := r.current()
	if store == nil {
		return gui.ErrSecretsUnsupported
	}
	return store.Save(service, key, value)
}

// Delete forwards a delete to the registered host store.
func (r *Registry) Delete(service, key string) error {
	store := r.current()
	if store == nil {
		return gui.ErrSecretsUnsupported
	}
	return store.Delete(service, key)
}
