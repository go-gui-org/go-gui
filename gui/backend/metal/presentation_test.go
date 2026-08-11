//go:build darwin && cgo && !ios

package metal

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestLiveResizePresentationGuard is a compile-time guard only; the
// real check needs Cocoa on the initial main thread and runs from
// TestMain via runMainThreadTests → runLiveResizePresentationTests.
func TestLiveResizePresentationGuard(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Cocoa requires main thread; tested via TestMain")
	}
}

// runLiveResizePresentationTests verifies the layer presentation mode
// follows the live-resize switch: async outside the drag, transaction
// mode during it, and back to async when it ends — with a real frame
// rendered in each mode so both present paths in metalEndFrame run.
// The latch check (end-of-drag back to async) guards the regression
// that would otherwise leave every later frame paying the per-frame
// main-thread block.
//
// A real resize drag cannot be synthesized programmatically — AppKit
// exposes no API for it — so the test fires the same delegate methods
// the resize tracking loop would.
//
// Panics on failure, matching runMainThreadTests.
func runLiveResizePresentationTests() {
	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	w.UpdateView(func(_ *gui.Window) gui.View {
		return gui.Rectangle(gui.RectangleCfg{
			Width:  100,
			Height: 50,
			Color:  gui.Green,
		})
	})
	b, err := New(w)
	if err != nil {
		panic("live resize: metal.New: " + err.Error())
	}
	defer b.Destroy()

	// Default is async: frames must not block the main thread.
	if got := testLayerPresentsWithTransaction(b.window); got != 0 {
		panic(fmt.Sprintf("live resize: layer starts in mode %d, want 0 (async)", got))
	}
	if !w.FrameFn() {
		panic("live resize: FrameFn returned false")
	}
	b.renderFrame(w)

	// Drag start: transaction mode — the shift-free present path.
	testStartLiveResize(b.window)
	if got := testLayerPresentsWithTransaction(b.window); got != 1 {
		panic(fmt.Sprintf("live resize: windowWillStartLiveResize left mode %d, want 1", got))
	}
	b.renderFrame(w)

	// Drag end: back to async, or every later frame keeps the
	// main-thread block for the rest of the session.
	testEndLiveResize(b.window)
	if got := testLayerPresentsWithTransaction(b.window); got != 0 {
		panic(fmt.Sprintf("live resize: windowDidEndLiveResize left mode %d, want 0 (latch)", got))
	}
	b.renderFrame(w)
}
