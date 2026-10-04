//go:build linux && !android && (amd64 || arm64)

package wl

import (
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
)

// Raw libwayland-client entry points, resolved by Load.
var (
	fnDisplayConnect          uintptr
	fnDisplayDisconnect       uintptr
	fnDisplayGetFd            uintptr
	fnDisplayRoundtrip        uintptr
	fnDisplayDispatchPending  uintptr
	fnDisplayFlush            uintptr
	fnDisplayPrepareRead      uintptr
	fnDisplayReadEvents       uintptr
	fnDisplayCancelRead       uintptr
	fnDisplayGetError         uintptr
	fnDisplayGetProtocolError uintptr
	fnProxyMarshalArrayFlags  uintptr
	fnProxyAddDispatcher      uintptr
	fnProxyGetListener        uintptr
	fnProxyDestroy            uintptr
	fnProxyGetVersion         uintptr
	fnProxyGetID              uintptr
)

var (
	loadOnce sync.Once
	loadErr  error
)

// Load opens libwayland-client, resolves the entry points, installs the
// event dispatcher callback and builds the protocol tables. It runs once;
// later calls return the first result. Connect calls it.
func Load() error {
	loadOnce.Do(func() { loadErr = load() })
	return loadErr
}

func load() error {
	lib, err := purego.Dlopen("libwayland-client.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("dlopen libwayland-client.so.0: %w", err)
	}
	syms := []struct {
		fn   *uintptr
		name string
	}{
		{&fnDisplayConnect, "wl_display_connect"},
		{&fnDisplayDisconnect, "wl_display_disconnect"},
		{&fnDisplayGetFd, "wl_display_get_fd"},
		{&fnDisplayRoundtrip, "wl_display_roundtrip"},
		{&fnDisplayDispatchPending, "wl_display_dispatch_pending"},
		{&fnDisplayFlush, "wl_display_flush"},
		{&fnDisplayPrepareRead, "wl_display_prepare_read"},
		{&fnDisplayReadEvents, "wl_display_read_events"},
		{&fnDisplayCancelRead, "wl_display_cancel_read"},
		{&fnDisplayGetError, "wl_display_get_error"},
		{&fnDisplayGetProtocolError, "wl_display_get_protocol_error"},
		// Added in libwayland 1.20. Older libraries fail here, by name,
		// instead of crashing on the first request.
		{&fnProxyMarshalArrayFlags, "wl_proxy_marshal_array_flags"},
		{&fnProxyAddDispatcher, "wl_proxy_add_dispatcher"},
		{&fnProxyGetListener, "wl_proxy_get_listener"},
		{&fnProxyDestroy, "wl_proxy_destroy"},
		{&fnProxyGetVersion, "wl_proxy_get_version"},
		{&fnProxyGetID, "wl_proxy_get_id"},
	}
	for _, s := range syms {
		addr, symErr := purego.Dlsym(lib, s.name)
		if symErr != nil || addr == 0 {
			return fmt.Errorf("libwayland-client has no %s (needs libwayland 1.20 or later): %v", s.name, symErr)
		}
		*s.fn = addr
	}
	// One callback for every proxy; see dispatch.
	dispatcherFn = purego.NewCallback(dispatch)
	defineInterfaces()
	return nil
}
