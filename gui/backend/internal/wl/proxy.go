//go:build linux && !android && (amd64 || arm64)

package wl

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

// maxArgs is WL_CLOSURE_MAX_ARGS: no message carries more arguments.
const maxArgs = 20

// marshalFlagDestroy is WL_MARSHAL_FLAG_DESTROY: destroy the proxy after
// sending the request (destructor requests).
const marshalFlagDestroy = 1

// argSlots is libwayland's union wl_argument array. On 64-bit Linux each
// union is 8 bytes; an int32/uint32/fixed value sits in the low 4 bytes and
// the high bytes are not defined, so decoders truncate to 32 bits first.
type argSlots [maxArgs]uintptr

// Proxy is one client-side Wayland object (struct wl_proxy *). The generated
// types embed it. The zero Proxy is a null object, which is what a nullable
// object argument sends.
type Proxy struct {
	ptr uintptr
	// ver caches the object version when it is known at creation (bind and
	// constructor requests), so a constructor called every frame (such as
	// wl_surface.frame) does not pay a library call for it. 0 means unknown.
	ver uint32
}

// Ptr returns the struct wl_proxy * address, for handing to C libraries
// such as libwayland-egl.
func (p Proxy) Ptr() uintptr { return p.ptr }

// Valid reports whether p refers to an object.
func (p Proxy) Valid() bool { return p.ptr != 0 }

// ID returns the protocol object id.
func (p Proxy) ID() uint32 {
	r, _, _ := purego.SyscallN(fnProxyGetID, p.ptr)
	return uint32(r)
}

// Version returns the version the object was bound or created with.
func (p Proxy) Version() uint32 {
	if p.ver != 0 {
		return p.ver
	}
	r, _, _ := purego.SyscallN(fnProxyGetVersion, p.ptr)
	return uint32(r)
}

// DestroyProxy frees the client-side proxy without sending a request. Use it
// for objects whose interface has no destructor request, such as
// wl_callback after its done event, or wl_registry.
func (p Proxy) DestroyProxy() {
	delete(handlers, p.ptr)
	_, _, _ = purego.SyscallN(fnProxyDestroy, p.ptr)
}

// marshalBuf holds the request arguments for the duration of one marshal
// call. A package array instead of a local one: a local passed by address
// through purego.SyscallN would escape to the heap on every request.
// Safe because the package is single-threaded (see doc.go) and libwayland
// copies the arguments out before the call returns.
var marshalBuf argSlots

// marshal sends request opcode on p. iface and version name the new object
// for a constructor request (nil and 0 otherwise); the return value is the
// new proxy, or 0.
func (p Proxy) marshal(opcode uint32, iface *Interface, version, flags uint32, args ...uintptr) uintptr {
	if len(args) > maxArgs {
		panic(fmt.Sprintf("wl: %d arguments, libwayland takes at most %d", len(args), maxArgs))
	}
	copy(marshalBuf[:], args)
	r, _, _ := purego.SyscallN(fnProxyMarshalArrayFlags, p.ptr, uintptr(opcode),
		uintptr(unsafe.Pointer(iface)), uintptr(version), uintptr(flags),
		uintptr(unsafe.Pointer(&marshalBuf[0])))
	if flags&marshalFlagDestroy != 0 {
		delete(handlers, p.ptr)
	}
	return r
}

// handlers maps a proxy address to the decoder SetHandlers installed for it.
var handlers = map[uintptr]func(opcode uint32, a *argSlots){}

// dispatcherFn is the C address of dispatch, made by Load.
var dispatcherFn uintptr

// dispatcherTag is passed to libwayland as each proxy's "implementation".
// libwayland only hands it back to dispatch, but wl_proxy_get_listener
// returns it too, which tells setDispatch whether this package already owns
// the proxy. The handlers map cannot answer that: a freed proxy's address
// can come back for a new one.
var dispatcherTag byte

func dispatcherTagPtr() uintptr { return uintptr(unsafe.Pointer(&dispatcherTag)) }

// setDispatch routes p's events to f.
func (p Proxy) setDispatch(f func(opcode uint32, a *argSlots)) {
	cur, _, _ := purego.SyscallN(fnProxyGetListener, p.ptr)
	if cur != dispatcherTagPtr() {
		r, _, _ := purego.SyscallN(fnProxyAddDispatcher, p.ptr, dispatcherFn, dispatcherTagPtr(), 0)
		if int32(r) != 0 {
			// The proxy has a C listener already: some other library owns
			// its events. Taking it over would break that library.
			panic("wl: proxy already has a listener")
		}
	}
	handlers[p.ptr] = f
}

// panicked holds a panic raised by a handler. A Go panic must not unwind
// through libwayland's C frames, so dispatch recovers it and the call that
// ran the dispatch (Roundtrip, Dispatch, DispatchPending) raises it again
// once libwayland has returned.
var panicked any

// dispatch is libwayland's wl_dispatcher_func_t:
//
//	int (*)(const void *implementation, void *target, uint32_t opcode,
//		const struct wl_message *msg, union wl_argument *args);
//
// target is the proxy the event is for.
func dispatch(_, target unsafe.Pointer, opcode uint32, _, args unsafe.Pointer) int32 {
	defer func() {
		if r := recover(); r != nil && panicked == nil {
			panicked = r
		}
	}()
	if f := handlers[uintptr(target)]; f != nil {
		f(opcode, (*argSlots)(args))
	}
	return 0
}

// rethrow raises a panic recovered in dispatch.
func rethrow() {
	if p := panicked; p != nil {
		panicked = nil
		panic(p)
	}
}

// cptr turns an address libwayland handed over (C memory) back into a
// pointer. Written this way so vet does not read it as a Go pointer
// smuggled through uintptr: it never is one.
func cptr(u uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&u)) }

// goString copies a NUL-terminated C string. A null pointer gives "".
func goString(u uintptr) string {
	if u == 0 {
		return ""
	}
	p := cptr(u)
	n := 0
	for *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(p), n))
}

// cString returns s as a NUL-terminated byte slice for a request. For a
// nullable argument, "" means null and gives nil.
func cString(s string, nullable bool) []byte {
	if s == "" && nullable {
		return nil
	}
	b := make([]byte, len(s)+1)
	copy(b, s)
	return b
}

func cStringPtr(b []byte) uintptr {
	if b == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(&b[0]))
}

// wlArray has the layout of struct wl_array.
type wlArray struct {
	size  uintptr
	alloc uintptr
	data  unsafe.Pointer
}

func newArray(b []byte) *wlArray {
	a := &wlArray{size: uintptr(len(b)), alloc: uintptr(len(b))}
	if len(b) > 0 {
		a.data = unsafe.Pointer(&b[0])
	}
	return a
}

// arrayBytes views an event's wl_array without copying. The slice is only
// valid until the handler returns.
func arrayBytes(u uintptr) []byte {
	if u == 0 {
		return nil
	}
	a := (*wlArray)(cptr(u))
	if a.size == 0 || a.data == nil {
		return nil
	}
	return unsafe.Slice((*byte)(a.data), a.size)
}

// Fixed is wl_fixed_t: a signed 24.8 fixed-point number.
type Fixed int32

// Float returns f as a float64.
func (f Fixed) Float() float64 { return float64(f) / 256 }

// FixedFrom converts v to the nearest Fixed.
func FixedFrom(v float64) Fixed {
	if v < 0 {
		return Fixed(v*256 - 0.5)
	}
	return Fixed(v*256 + 0.5)
}
