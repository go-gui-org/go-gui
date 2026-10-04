//go:build linux && !android && (amd64 || arm64)

package wl

import (
	"strings"
	"testing"
	"unsafe"
)

// These tests need neither libwayland nor a compositor.

// TestInterfaceLayout pins the Go structs to libwayland's C layout. A wrong
// offset here makes libwayland read garbage for every message.
func TestInterfaceLayout(t *testing.T) {
	var i Interface
	var m message
	var a wlArray
	checks := []struct {
		name      string
		got, want uintptr
	}{
		{"sizeof wl_interface", unsafe.Sizeof(i), 40},
		{"wl_interface.version", unsafe.Offsetof(i.version), 8},
		{"wl_interface.method_count", unsafe.Offsetof(i.methodCount), 12},
		{"wl_interface.methods", unsafe.Offsetof(i.methods), 16},
		{"wl_interface.event_count", unsafe.Offsetof(i.eventCount), 24},
		{"wl_interface.events", unsafe.Offsetof(i.events), 32},
		{"sizeof wl_message", unsafe.Sizeof(m), 24},
		{"wl_message.signature", unsafe.Offsetof(m.signature), 8},
		{"wl_message.types", unsafe.Offsetof(m.types), 16},
		{"sizeof wl_array", unsafe.Sizeof(a), 24},
		{"wl_array.data", unsafe.Offsetof(a.data), 16},
		{"sizeof wl_argument", unsafe.Sizeof(argSlots{}) / maxArgs, 8},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// messages views a C-layout message array as a slice.
func messages(p *message, n int32) []message {
	if n == 0 {
		return nil
	}
	return unsafe.Slice(p, n)
}

// sigArgs counts the arguments in a libwayland signature string.
func sigArgs(sig string) int {
	n := 0
	for _, r := range sig {
		if r != '?' && (r < '0' || r > '9') {
			n++
		}
	}
	return n
}

// TestTablesConsistent checks every generated table: names and versions are
// set, and each object/new_id argument with a types entry names a real
// interface. The types array is walked as libwayland walks it, by argument
// index, so a short array reads past its end and shows up here.
func TestTablesConsistent(t *testing.T) {
	defineInterfaces()
	for _, it := range allInterfaces {
		name := it.Name()
		if name == "" || it.version < 1 {
			t.Fatalf("table %p not defined (name %q, version %d)", it, name, it.version)
		}
		for _, list := range [][]message{messages(it.methods, it.methodCount), messages(it.events, it.eventCount)} {
			for _, m := range list {
				sig := goString(uintptr(unsafe.Pointer(m.signature)))
				n := sigArgs(sig)
				if n == 0 {
					continue
				}
				if m.types == nil {
					t.Errorf("%s.%s: signature %q has no types array", name, goString(uintptr(unsafe.Pointer(m.name))), sig)
					continue
				}
				types := unsafe.Slice(m.types, n)
				for _, ti := range types {
					if ti != nil && ti.Name() == "" {
						t.Errorf("%s: types entry with no name", name)
					}
				}
			}
		}
	}
}

// TestTablesCrossReference spot-checks types entries across protocol files.
func TestTablesCrossReference(t *testing.T) {
	defineInterfaces()
	// wl_surface.attach(?o buffer, i x, i y): types[0] is wl_buffer.
	attach := messages(SurfaceInterface.methods, SurfaceInterface.methodCount)[1]
	if got := unsafe.Slice(attach.types, 3)[0].Name(); got != "wl_buffer" {
		t.Errorf("wl_surface.attach types[0] = %q, want wl_buffer", got)
	}
	// xdg_wm_base.get_xdg_surface(n xdg_surface, o wl_surface).
	get := messages(XdgWmBaseInterface.methods, XdgWmBaseInterface.methodCount)[2]
	ts := unsafe.Slice(get.types, 2)
	if ts[0].Name() != "xdg_surface" || ts[1].Name() != "wl_surface" {
		t.Errorf("xdg_wm_base.get_xdg_surface types = %q %q", ts[0].Name(), ts[1].Name())
	}
}

func TestFixed(t *testing.T) {
	for _, c := range []struct {
		in   float64
		want Fixed
	}{{1.5, 384}, {-1.5, -384}, {0, 0}, {0.001, 0}, {100.25, 25664}} {
		if got := FixedFrom(c.in); got != c.want {
			t.Errorf("FixedFrom(%v) = %d, want %d", c.in, got, c.want)
		}
	}
	if got := Fixed(-384).Float(); got != -1.5 {
		t.Errorf("Fixed(-384).Float() = %v", got)
	}
}

func TestStringsAndArrays(t *testing.T) {
	b := cString("hello", false)
	if len(b) != 6 || b[5] != 0 {
		t.Fatalf("cString = %q", b)
	}
	if got := goString(cStringPtr(b)); got != "hello" {
		t.Errorf("goString = %q", got)
	}
	if goString(0) != "" || cString("", true) != nil || cStringPtr(nil) != 0 {
		t.Error("null string handling")
	}
	if b := cString("", false); len(b) != 1 {
		t.Error(`non-nullable "" must still send an empty string`)
	}
	data := []byte{1, 2, 3, 4}
	a := newArray(data)
	got := arrayBytes(uintptr(unsafe.Pointer(a)))
	if len(got) != 4 || got[3] != 4 {
		t.Errorf("arrayBytes = %v", got)
	}
	if arrayBytes(0) != nil || arrayBytes(uintptr(unsafe.Pointer(newArray(nil)))) != nil {
		t.Error("empty array handling")
	}
}

// TestDispatchRoutesAndRecovers drives the dispatcher directly with a fake
// proxy address: events reach the right handler, and a handler's panic is
// held back from the C caller and raised again by rethrow.
func TestDispatchRoutesAndRecovers(t *testing.T) {
	t.Cleanup(func() { clear(handlers); panicked = nil })
	var fake [1]byte
	target := unsafe.Pointer(&fake[0])
	var gotOp uint32
	var gotArg uintptr
	handlers[uintptr(target)] = func(op uint32, a *argSlots) { gotOp, gotArg = op, a[0] }

	args := argSlots{0xFFFFFFFF_00000007} // high bytes undefined in C
	if r := dispatch(nil, target, 3, nil, unsafe.Pointer(&args)); r != 0 {
		t.Fatalf("dispatch returned %d", r)
	}
	if gotOp != 3 || uint32(gotArg) != 7 {
		t.Fatalf("handler got op %d arg %#x", gotOp, gotArg)
	}

	// An event for a proxy with no handler is dropped.
	var other [1]byte
	dispatch(nil, unsafe.Pointer(&other[0]), 0, nil, unsafe.Pointer(&args))

	handlers[uintptr(target)] = func(uint32, *argSlots) { panic("boom") }
	dispatch(nil, target, 0, nil, unsafe.Pointer(&args)) // must not panic
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "boom") {
			t.Fatalf("rethrow recovered %v, want boom", r)
		}
		if panicked != nil {
			t.Error("panicked not cleared")
		}
	}()
	rethrow()
}

func TestMarshalTooManyArgs(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want a panic for more than maxArgs arguments")
		}
	}()
	Proxy{}.marshal(0, nil, 0, 0, make([]uintptr, maxArgs+1)...)
}
