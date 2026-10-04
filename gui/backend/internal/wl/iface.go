//go:build linux && !android && (amd64 || arm64)

package wl

import "unsafe"

// Interface has the memory layout of libwayland's struct wl_interface:
//
//	struct wl_interface {
//		const char *name;
//		int version;
//		int method_count;
//		const struct wl_message *methods;
//		int event_count;
//		const struct wl_message *events;
//	};
//
// Go lays this struct out the same way on 64-bit Linux, which
// TestInterfaceLayout pins. The generated <Type>Interface vars are filled by
// defineInterfaces from Load. libwayland keeps pointers to them in every
// proxy, which is safe because they are package globals: always reachable,
// and the Go GC never moves memory.
//
// libwayland compares interfaces by name, so these tables also match the
// ones compiled into libEGL, libwayland-egl and libdecor.
type Interface struct {
	name        *byte
	version     int32
	methodCount int32
	methods     *message
	eventCount  int32
	events      *message
}

// message has the layout of struct wl_message:
//
//	struct wl_message {
//		const char *name;
//		const char *signature;
//		const struct wl_interface **types;
//	};
type message struct {
	name      *byte
	signature *byte
	types     **Interface
}

// Name returns the protocol name of the interface, such as "wl_surface".
func (i *Interface) Name() string {
	if i == nil || i.name == nil {
		return ""
	}
	return goString(uintptr(unsafe.Pointer(i.name)))
}

// Version returns the highest version the generated bindings know.
func (i *Interface) Version() uint32 { return uint32(i.version) }

// newMessage builds one wl_message. types has one entry per argument in the
// signature (a generic new_id counts as three: interface, version, id); the
// entry is the argument's interface for an object or new_id, nil otherwise.
func newMessage(name, signature string, types []*Interface) message {
	m := message{name: cBytes(name), signature: cBytes(signature)}
	if len(types) > 0 {
		m.types = &types[0]
	}
	return m
}

// defineInterface fills one generated table.
func defineInterface(i *Interface, name string, version int32, requests, events []message) {
	*i = Interface{name: cBytes(name), version: version}
	if len(requests) > 0 {
		i.methodCount = int32(len(requests))
		i.methods = &requests[0]
	}
	if len(events) > 0 {
		i.eventCount = int32(len(events))
		i.events = &events[0]
	}
}

// cBytes returns a NUL-terminated copy of s that stays alive as long as the
// table holding it.
func cBytes(s string) *byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return &b[0]
}
