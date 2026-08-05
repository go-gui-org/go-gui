//go:build cgo && !js && !darwin

package gl

import "testing"

// TestBackendRenderSmoke is skipped when cgo is linked into the test binary
// (the toolchain default). A cgo-enabled binary segfaults at process exit on
// Linux with a display: during shutdown the runtime starts a new M
// (startm → _cgo_thread_start → pthread_create) whose first Go execution
// jumps to freed heap memory left by the test goroutine's purego EGL
// teardown (SEGV_ACCERR). The backend itself is cgo-free; run
// `CGO_ENABLED=0 go test ./gui/backend/gl/` to exercise it.
// See go-gui#162.
func TestBackendRenderSmoke(t *testing.T) {
	t.Skip("cgo test binaries crash at exit on Linux with a display " +
		"(Go runtime exit race, go-gui#162); run with CGO_ENABLED=0")
}
