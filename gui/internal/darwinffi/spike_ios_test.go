//go:build ios && arm64

package darwinffi

// Spike for #1342: can darwinffi (purego) drive Metal on iOS?
//
// The test loads the frameworks by path, makes a device and a
// CAMetalLayer, compiles MSL source, clears a render target through a
// render pass and reads the pixels back. If all of it works on the
// simulator, the purego renderer in gui/backend/metal can serve iOS.
//
// Run it on a booted simulator (no device needed):
//
//	SDK=$(xcrun --sdk iphonesimulator --show-sdk-path)
//	FLAGS="-isysroot $SDK -arch arm64 -mios-simulator-version-min=17.0"
//	CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
//	  CC="$(xcrun --sdk iphonesimulator -f clang)" \
//	  CGO_CFLAGS="$FLAGS" CGO_LDFLAGS="$FLAGS" \
//	  go test -c -o /tmp/ffi.test ./gui/internal/darwinffi
//	xcrun simctl spawn booted /tmp/ffi.test -test.v -test.run IOS

import (
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// iosF keeps the FCaller in the heap, as FCaller requires.
var iosF = new(FCaller)

// blitOrigin and blitSize are the MTLOrigin and MTLSize of the readback
// blit. They are globals because C gets only their addresses.
var blitOrigin, blitSize [3]uintptr

func TestIOSMetalClearReadback(t *testing.T) {
	Open(PathMetal)
	Open(PathQuartzCore)
	pool := PoolPush()
	defer PoolPop(pool)

	// MTLCreateSystemDefaultDevice is a plain C function: no arguments,
	// one pointer result in X0.
	iosF.Call(Func(Open(PathMetal), "MTLCreateSystemDefaultDevice"))
	dev := objc.ID(iosF.R0)
	if dev == 0 {
		t.Fatal("MTLCreateSystemDefaultDevice returned nil")
	}
	name := dev.Send(Sel("name"))
	t.Logf("device: %s", GoString(uintptr(name)))

	// The iOS backend gets its layer from UIView.layerClass; here one is
	// made by hand. setDevice: must stick.
	layer := objc.ID(Class("CAMetalLayer")).Send(Sel("alloc")).Send(Sel("init"))
	if layer == 0 {
		t.Fatal("CAMetalLayer alloc/init returned nil")
	}
	layer.Send(Sel("setDevice:"), dev)
	layer.Send(Sel("setPixelFormat:"), uintptr(80)) // BGRA8Unorm
	if got := layer.Send(Sel("device")); got != dev {
		t.Fatalf("layer device = %#x, want %#x", got, dev)
	}

	// The shader compiler runs on the simulator too.
	const src = `#include <metal_stdlib>
using namespace metal;
fragment float4 spike_fs() { return float4(1, 0, 0, 1); }
`
	var errOut objc.ID
	lib := dev.Send(Sel("newLibraryWithSource:options:error:"), NSString(src), objc.ID(0),
		unsafe.Pointer(&errOut))
	if lib == 0 {
		t.Fatalf("newLibraryWithSource failed: %s",
			GoString(uintptr(errOut.Send(Sel("localizedDescription")))))
	}
	if fn := lib.Send(Sel("newFunctionWithName:"), NSString("spike_fs")); fn == 0 {
		t.Fatal("newFunctionWithName: spike_fs returned nil")
	}

	// 4x4 render target. Private storage, then a blit into a shared
	// buffer: the simulator may refuse shared-storage textures, and the
	// readback path must not depend on them.
	const n = 4
	td := objc.ID(Class("MTLTextureDescriptor")).Send(
		Sel("texture2DDescriptorWithPixelFormat:width:height:mipmapped:"),
		uintptr(80), uintptr(n), uintptr(n), false)
	td.Send(Sel("setUsage:"), uintptr(4))       // RenderTarget
	td.Send(Sel("setStorageMode:"), uintptr(2)) // Private
	tex := dev.Send(Sel("newTextureWithDescriptor:"), td)
	if tex == 0 {
		t.Fatal("newTextureWithDescriptor returned nil")
	}

	rpd := objc.ID(Class("MTLRenderPassDescriptor")).Send(Sel("renderPassDescriptor"))
	ca := rpd.Send(Sel("colorAttachments")).Send(Sel("objectAtIndexedSubscript:"), uintptr(0))
	ca.Send(Sel("setTexture:"), tex)
	ca.Send(Sel("setLoadAction:"), uintptr(2))  // Clear
	ca.Send(Sel("setStoreAction:"), uintptr(1)) // Store
	// MTLClearColor is four doubles, an HFA: it goes in D0..D3. This is
	// the FCaller path the renderer uses for float arguments.
	iosF.D[0], iosF.D[1], iosF.D[2], iosF.D[3] = 0, 1, 0, 1 // green
	iosF.Send(uintptr(ca), uintptr(Sel("setClearColor:")))

	queue := dev.Send(Sel("newCommandQueue"))
	cb := queue.Send(Sel("commandBuffer"))
	enc := cb.Send(Sel("renderCommandEncoderWithDescriptor:"), rpd)
	if enc == 0 {
		t.Fatal("renderCommandEncoderWithDescriptor returned nil")
	}
	enc.Send(Sel("endEncoding"))

	buf := dev.Send(Sel("newBufferWithLength:options:"), uintptr(n*n*4), uintptr(0))
	blit := cb.Send(Sel("blitCommandEncoder"))
	// copyFromTexture:sourceSlice:sourceLevel:sourceOrigin:sourceSize:
	// toBuffer:destinationOffset:destinationBytesPerRow:
	// destinationBytesPerImage: takes 9 arguments, 11 integer arguments
	// with self and _cmd. FCaller has room for 10 (X0-X7 and S[0:2]), so
	// this setup-only call goes through objc.ID.Send, which allocates
	// but takes any count. Origin and size are 24-byte structs: arm64
	// passes them by reference, so they go as pointers to globals that
	// stay reachable across the call.
	blitSize = [3]uintptr{n, n, 1}
	blit.Send(Sel("copyFromTexture:sourceSlice:sourceLevel:sourceOrigin:sourceSize:"+
		"toBuffer:destinationOffset:destinationBytesPerRow:destinationBytesPerImage:"),
		tex, uintptr(0), uintptr(0), unsafe.Pointer(&blitOrigin), unsafe.Pointer(&blitSize),
		buf, uintptr(0), uintptr(n*4), uintptr(n*n*4))
	blit.Send(Sel("endEncoding"))
	cb.Send(Sel("commit"))
	cb.Send(Sel("waitUntilCompleted"))
	if st := cb.Send(Sel("status")); st != 4 { // Completed
		t.Fatalf("command buffer status = %d, want 4 (completed)", st)
	}

	px := unsafe.Slice((*byte)(objc.Send[unsafe.Pointer](buf, Sel("contents"))), n*n*4)
	// BGRA: green is 00 FF 00 FF. Check every pixel, so a wrong row
	// stride or a short copy shows up, not only the first pixel.
	for i := range n * n {
		p := px[i*4 : i*4+4]
		if p[0] != 0 || p[1] != 0xFF || p[2] != 0 || p[3] != 0xFF {
			t.Fatalf("pixel %d = % x, want 00 ff 00 ff", i, p)
		}
	}
	t.Logf("all %d pixels green: clear + blit readback works", n*n)
}

// TestIOSSharedTextureUpload covers the desktop renderer's upload path
// (metalCtx.upload): a shared-storage, shader-read texture, filled with
// replaceRegion: and its 48-byte MTLRegion passed by pointer, then read
// back with getBytes:.
func TestIOSSharedTextureUpload(t *testing.T) {
	Open(PathMetal)
	pool := PoolPush()
	defer PoolPop(pool)
	iosF.Call(Func(Open(PathMetal), "MTLCreateSystemDefaultDevice"))
	dev := objc.ID(iosF.R0)
	if dev == 0 {
		t.Fatal("no Metal device")
	}
	const n = 4
	td := objc.ID(Class("MTLTextureDescriptor")).Send(
		Sel("texture2DDescriptorWithPixelFormat:width:height:mipmapped:"),
		uintptr(80), uintptr(n), uintptr(n), false)
	td.Send(Sel("setUsage:"), uintptr(1))       // ShaderRead
	td.Send(Sel("setStorageMode:"), uintptr(0)) // Shared, as sampledTexture
	tex := dev.Send(Sel("newTextureWithDescriptor:"), td)
	if tex == 0 {
		t.Fatal("shared texture refused")
	}
	region := &[6]uintptr{0, 0, 0, n, n, 1}
	src := make([]byte, n*n*4)
	for i := range src {
		src[i] = byte(i)
	}
	tex.Send(Sel("replaceRegion:mipmapLevel:withBytes:bytesPerRow:"),
		unsafe.Pointer(region), uintptr(0), unsafe.Pointer(&src[0]), uintptr(n*4))
	dst := make([]byte, n*n*4)
	tex.Send(Sel("getBytes:bytesPerRow:fromRegion:mipmapLevel:"),
		unsafe.Pointer(&dst[0]), uintptr(n*4), unsafe.Pointer(region), uintptr(0))
	for i := range dst {
		if dst[i] != src[i] {
			t.Fatalf("byte %d = %d, want %d", i, dst[i], src[i])
		}
	}
}
