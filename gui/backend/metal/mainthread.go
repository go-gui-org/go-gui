//go:build darwin && !ios

package metal

import "runtime"

// AppKit requires every NSApplication / NSMenu / NSWindow call to
// happen on the process's initial thread (thread 0). The Go runtime
// starts the main goroutine on that thread, but does NOT keep it
// there: any blocking syscall, cgo call, or preemption point before
// the backend starts can let the scheduler resume main on a different
// M. Reaching backend.New / backend.RunApp from a migrated main
// goroutine makes the LockOSThread call there pin the *wrong* thread,
// and the first AppKit call aborts with:
//
//	API misuse: setting the main menu on a non-main thread
//
// Locking here instead is what makes the guarantee real: package init
// runs on the main goroutine before main(), while it is still on
// thread 0, so the lock captures the actual main thread. Importing
// this backend is therefore sufficient — no init boilerplate is
// required in the embedding program.
//
// The lock is never released; the main goroutine stays on thread 0
// for the process lifetime, which is exactly what an AppKit event
// loop needs.
func init() {
	runtime.LockOSThread()
}
