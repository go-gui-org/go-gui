//go:build linux && !js && !android && !(amd64 || arm64)

package gl

import (
	"log"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// The Wayland bindings read event arguments as 8-byte little-endian slots,
// so they build for amd64 and arm64 only (gui/backend/internal/wl). On
// other CPUs GOGUI_WAYLAND=1 warns once and X11 is used.

// wlWindow is never created here; the type keeps platformState's field.
type wlWindow struct{}

var wlUnsupportedOnce sync.Once

func tryWayland(*gui.Window) (*Backend, bool) {
	if waylandRequested() {
		wlUnsupportedOnce.Do(func() {
			log.Printf("gl: GOGUI_WAYLAND=1 is not supported on this CPU, using X11")
		})
	}
	return nil, false
}

func runAppWayland(*gui.App, []*gui.Window) (bool, error) {
	_, _ = tryWayland(nil)
	return false, nil
}

func (b *Backend) runWayland(*gui.Window) {}

func (p *platformState) destroyWayland() {}
