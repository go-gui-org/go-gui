//go:build ios && cgo

package ios

// These tests need a Metal device, so they run on a booted iOS
// simulator, as gui/internal/darwinffi/spike_ios_test.go does:
//
//	SDK=$(xcrun --sdk iphonesimulator --show-sdk-path)
//	FLAGS="-isysroot $SDK -arch arm64 -mios-simulator-version-min=17.0"
//	CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
//	  CC="$(xcrun --sdk iphonesimulator -f clang)" \
//	  CGO_CFLAGS="$FLAGS" CGO_LDFLAGS="$FLAGS" \
//	  go test -c -o /tmp/ios.test ./gui/backend/ios
//	xcrun simctl spawn booted /tmp/ios.test -test.v

import (
	"math"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// testF reads float results (CGSize) back from a send. An FCaller must
// live in the heap.
var testF = new(darwinffi.FCaller)

// newTestBackend makes a CAMetalLayer the way a Pattern B host does and
// starts the backend on it at 100×100 points, scale 2. The caller owns
// the layer's +1 from alloc/init.
func newTestBackend(t *testing.T) (*Backend, objc.ID) {
	t.Helper()
	darwinffi.Open(darwinffi.PathMetal)
	darwinffi.Open(darwinffi.PathQuartzCore)
	layer := objc.ID(darwinffi.Class("CAMetalLayer")).
		Send(darwinffi.Sel("alloc")).Send(darwinffi.Sel("init"))
	if layer == 0 {
		t.Fatal("CAMetalLayer alloc/init returned nil")
	}
	iosWindow = gui.NewWindow(gui.WindowCfg{})
	// The layer is an ObjC object, not Go memory: pass its address bits
	// as the pointer UIKit would hand over.
	initBackend(*(*unsafe.Pointer)(unsafe.Pointer(&layer)), 100, 100, 2)
	if iosBackend == nil || !iosBackend.r.Ready() {
		t.Skip("no Metal device")
	}
	return iosBackend, layer
}

func retainCount(o objc.ID) uintptr {
	return uintptr(o.Send(darwinffi.Sel("retainCount")))
}

// drawableSize reads the layer's CGSize drawableSize, which comes back
// in the float result registers.
func drawableSize(layer objc.ID) (float64, float64) {
	testF.Send(uintptr(layer), uintptr(darwinffi.Sel("drawableSize")))
	return testF.RD[0], testF.RD[1]
}

// TestBackendRetainsLayer checks that the backend holds its own
// reference to the layer for its lifetime (#1342 review). The renderer
// keeps the layer unretained and asks it for a drawable every frame, so
// without this a Pattern B host that releases its layer leaves the
// renderer messaging a freed object.
func TestBackendRetainsLayer(t *testing.T) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	darwinffi.Open(darwinffi.PathQuartzCore)
	probe := objc.ID(darwinffi.Class("CAMetalLayer")).
		Send(darwinffi.Sel("alloc")).Send(darwinffi.Sel("init"))
	before := retainCount(probe)
	probe.Send(darwinffi.Sel("release"))

	b, layer := newTestBackend(t)
	defer layer.Send(darwinffi.Sel("release"))
	if got := retainCount(layer); got != before+1 {
		t.Errorf("retainCount after init = %d, want %d (host +1, backend +1)",
			got, before+1)
	}
	b.Destroy()
	if got := retainCount(layer); got != before {
		t.Errorf("retainCount after Destroy = %d, want %d (host only)",
			got, before)
	}
}

// TestResizeKeepsAcceptedScale sends scales that SetDPIScale rejects
// through handleResize. The drawable must stay sized at the scale the
// renderer kept (2), so it matches the projection and text density
// (#1342 review). Sized from the raw value, 9 gave a 900×900 drawable
// while text stayed at 2×.
func TestResizeKeepsAcceptedScale(t *testing.T) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	b, layer := newTestBackend(t)
	defer layer.Send(darwinffi.Sel("release"))
	defer b.Destroy()

	for _, s := range []float32{9, 0, -1, float32(math.NaN())} {
		b.handleResize(100, 100, s)
		if got := b.r.DPIScale(); got != 2 {
			t.Fatalf("scale %v: DPIScale = %v, want 2 kept", s, got)
		}
		if w, h := drawableSize(layer); w != 200 || h != 200 {
			t.Errorf("scale %v: drawableSize = %v×%v, want 200×200", s, w, h)
		}
	}
	// An accepted change still resizes.
	b.handleResize(100, 100, 3)
	if w, h := drawableSize(layer); w != 300 || h != 300 {
		t.Errorf("scale 3: drawableSize = %v×%v, want 300×300", w, h)
	}
}
