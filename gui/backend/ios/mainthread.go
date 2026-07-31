//go:build ios

package ios

import "runtime"

// Pin the main goroutine to the process's initial thread before main()
// runs.
//
// UIKit has the same rule as AppKit: UIApplicationMain and every
// UIView / CAMetalLayer call must happen on the process's initial
// thread. The Go runtime starts the main goroutine on thread 0 but
// does not keep it there — any blocking syscall or cgo call before the
// backend starts can let the scheduler resume main on a different M,
// after which the LockOSThread in Run pins the wrong thread and
// C.iosStartApp enters UIApplicationMain off the main thread.
//
// Locking from init runs while the main goroutine is still on thread
// 0, so the lock captures the actual main thread. This covers the
// Go-driven pattern (backend.Run). The Swift-driven c-archive pattern
// is unaffected: there the host app owns the main thread and Go is
// entered from it.
//
// See gui/backend/metal/mainthread.go for the macOS case that made
// this concrete.
//
// The lock is never released; the main goroutine stays on thread 0 for
// the process lifetime.
func init() {
	runtime.LockOSThread()
}
