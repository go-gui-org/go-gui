package gui

import (
	"testing"
	"unsafe"
)

// Every widget holds at least one Shape, and each one in the view pool
// (scratchObjPool) is its own heap object from the 320-byte size class.
// Shape is 312 bytes on 64-bit after imageFilter (#1066). A field that
// pushes it past 320 moves every shape to the next class, so add it
// behind a pointer sub-struct instead.
func TestShapeFitsSizeClass(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("size class pinned for 64-bit only")
	}
	if got := unsafe.Sizeof(Shape{}); got > 320 {
		t.Errorf("Shape = %d bytes, want <= 320", got)
	}
}
