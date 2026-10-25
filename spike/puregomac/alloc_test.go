//go:build darwin

package main

import (
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// TestSendAllocs measures heap allocations per objc_msgSend for each
// calling style. The metal backend's gate (TestDrawPathsDoNotAllocate)
// wants 0 per draw call, so only a style at 0 can sit on the hot path.
// No window is needed: the messages go to plain NSObject / Metal objects.
//
// Run: CGO_ENABLED=0 go test -run TestSendAllocs -v ./spike/puregomac
func TestSendAllocs(t *testing.T) {
	load()
	obj := sys0(sys0(cls("NSObject"), sel("alloc")), sel("init"))
	selHash := sel("hash")
	selResp := sel("respondsToSelector:")
	selPerf := sel("performSelector:withObject:")

	device := id(callC0(createDeviceFn))
	if device == 0 {
		t.Skip("no Metal device")
	}
	rpd := sys0(cls("MTLRenderPassDescriptor"), sel("renderPassDescriptor"))
	ca := sys1(sys0(rpd, sel("colorAttachments")), sel("objectAtIndexedSubscript:"), 0)
	selSetClear := sel("setClearColor:")
	selSetLoad := sel("setLoadAction:")
	layer := sys0(sys0(cls("CAMetalLayer"), sel("alloc")), sel("init"))
	selSetSize := sel("setDrawableSize:")
	sz := NSSize{640, 480}
	cc := MTLClearColor{0.1, 0.2, 0.3, 1}
	// getValue: is a real pointer-taking selector: it writes the boxed
	// CATransform3D (128 bytes) through its argument.
	boxed := sys1(sys0(cls("CALayer"), sel("layer")), sel("valueForKey:"),
		uintptr(nsString("transform")))
	selGetValue := sel("getValue:")

	cases := []struct {
		name string
		call func()
	}{
		// Option 1: variadic objc.ID.Send.
		{"Send/0 args", func() { obj.Send(selHash) }},
		{"Send/1 int arg", func() { obj.Send(selResp, selHash) }},
		{"Send/2 int args", func() { obj.Send(selPerf, selHash, 0) }},
		{"Send/clear color struct", func() { ca.Send(selSetClear, cc) }},
		{"Send[T]/0 args", func() { objc.Send[uintptr](obj, selHash) }},

		// Option 2: typed purego.RegisterFunc.
		{"RegisterFunc/0 args", func() { msg0(obj, selHash) }},
		{"RegisterFunc/1 int arg", func() { msg1(obj, selResp, uintptr(selHash)) }},
		{"RegisterFunc/2 int args", func() { msg2(obj, selPerf, uintptr(selHash), 0) }},
		{"RegisterFunc/clear color struct", func() { msgClear(ca, selSetClear, cc) }},
		{"RegisterFunc/NSSize struct", func() { msgSize(layer, selSetSize, sz) }},

		// Option 3: purego.SyscallN by address, integers only.
		{"SyscallN variadic/0 args", func() { vsys0(obj, selHash) }},
		{"SyscallN variadic/1 int arg", func() { vsys1(ca, selSetLoad, 2) }},
		{"SyscallN/0 args", func() { sys0(obj, selHash) }},
		{"SyscallN/1 int arg", func() { sys1(ca, selSetLoad, 2) }},
		{"SyscallN/2 int args", func() { sys2(obj, selPerf, uintptr(selHash), 0) }},
		// Buffered: the pointee is global, as argBuf requires.
		{"SyscallN/pointer arg, global pointee", func() {
			sys1(boxed, selGetValue, uintptr(unsafe.Pointer(&globalTransform)))
		}},
		// The form go:uintptrescapes protects: the conversion sits in the
		// SyscallN call, so a stack pointee is moved to the heap per call.
		{"SyscallN variadic/stack pointer arg", func() {
			var m [16]float64
			purego.SyscallN(msgSendAddr, uintptr(boxed), uintptr(selGetValue), uintptr(unsafe.Pointer(&m)))
		}},
		{"SyscallN/pool push+pop", func() { poolPop(poolPush()) }},
	}
	for _, tc := range cases {
		tc.call() // warm up any pool purego keeps
		n := testing.AllocsPerRun(1000, tc.call)
		t.Logf("%-38s %4.1f allocs/call", tc.name, n)
		// The buffered SyscallN form is the one the spec says passes the
		// gate; hold it to 0. The other styles are logged for the table.
		if strings.HasPrefix(tc.name, "SyscallN/") && n != 0 {
			t.Errorf("%s: %v allocs per call, want 0", tc.name, n)
		}
	}
}

// globalTransform receives CATransform3D values from getValue:.
var globalTransform [16]float64

// TestLargeStructSend checks the ABI claim send48 rests on, on the
// running architecture (run under Rosetta for amd64). It sets a layer's
// CGAffineTransform (six doubles, 48 bytes, the class of MTLViewport)
// through send48 and reads it back as a CATransform3D through NSValue
// getValue:, which takes only a pointer.
func TestLargeStructSend(t *testing.T) {
	load()
	layer := sys0(cls("CALayer"), sel("layer"))
	// sys1 takes the read-back buffer as a uintptr, which does not keep a
	// stack pointee in place, so it is global (see argBuf). send48 copies
	// its argument into a global itself.
	affine := [6]float64{2, 3, 5, 7, 11, 13} // a, b, c, d, tx, ty
	send48(layer, sel("setAffineTransform:"), (*[6]uintptr)(unsafe.Pointer(&affine)))
	val := sys1(layer, sel("valueForKey:"), uintptr(nsString("transform")))
	globalTransform = [16]float64{}
	sys1(val, sel("getValue:"), uintptr(unsafe.Pointer(&globalTransform)))
	m := globalTransform // CATransform3D, row-major m11..m44
	got := [6]float64{m[0], m[1], m[4], m[5], m[12], m[13]}
	if got != affine {
		t.Fatalf("%s: transform read back %v, want %v", runtime.GOARCH, got, affine)
	}
}
