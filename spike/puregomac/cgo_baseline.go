//go:build darwin && cgo

package main

/*
#cgo LDFLAGS: -lobjc
#include <objc/message.h>
#include <objc/runtime.h>
#include <stdint.h>
static uintptr_t spikeHash(uintptr_t obj, uintptr_t sel) {
	return ((uintptr_t (*)(id, SEL))objc_msgSend)((id)obj, (SEL)sel);
}
*/
import "C"

// cgoHash sends -hash through cgo: the cost one ObjC message has today,
// for comparison with BenchmarkSend. Built only with cgo on.
func cgoHash(obj id, s uintptr) uintptr {
	return uintptr(C.spikeHash(C.uintptr_t(obj), C.uintptr_t(s)))
}
