//go:build (linux && !js && !android) || (windows && !js)

package gl

import "runtime"

// Pin the main goroutine to the process's initial thread before main()
// runs.
//
// The Go runtime starts the main goroutine on thread 0 but does not
// keep it there: any blocking syscall or cgo call before the backend
// starts (config load, font registration, reading a file named on the
// command line) can let the scheduler resume main on a different M.
// The LockOSThread calls in New / RunAppE then pin whichever thread
// main happens to be on by that point, which is not necessarily the
// one the platform expects.
//
// That matters here for two reasons. An OpenGL context is current on
// one thread at a time, so every GL call — window creation through
// swap-buffers — must stay on the thread that made the context
// current. On Win32, a window's messages are delivered only to the
// thread that created it, so the create call and the message pump have
// to agree on a thread. Locking from init, while the main goroutine is
// still on thread 0, makes that thread deterministic instead of
// whatever the scheduler picked.
//
// See gui/backend/metal/mainthread.go, where the same migration caused
// a hard AppKit abort rather than a latent inconsistency.
//
// The lock is never released; the main goroutine stays on thread 0 for
// the process lifetime, which is what the event loop needs.
func init() {
	runtime.LockOSThread()
}
