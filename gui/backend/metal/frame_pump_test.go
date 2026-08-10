//go:build darwin && cgo && !ios

package metal

import (
	"runtime"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestFramePumpNestedMode is a compile-time guard only; the real check
// needs Cocoa on the initial main thread and runs from TestMain via
// runMainThreadTests → runFramePumpMainThreadTests.
func TestFramePumpNestedMode(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Cocoa requires main thread; tested via TestMain")
	}
}

// runFramePumpMainThreadTests verifies the nested-runloop frame pump:
// the timer must fire (and reach Go) while the main runloop runs in a
// nested mode, and must stay silent in NSDefaultRunLoopMode where the
// backend's own event loop owns frame timing.
//
// Regression test for windows freezing — no repaint, no command flush,
// no terminal reflow on resize — for as long as a modal dialog or open
// menu keeps a nested AppKit runloop on the main thread.
//
// Panics on failure, matching runMainThreadTests.
func runFramePumpMainThreadTests() {
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
		panic("frame pump: metal.New: " + err.Error())
	}

	// metalAppFinishLaunch arms the pump.
	testActivateNow()
	if !testFramePumpActive() {
		panic("frame pump: timer not installed after metalAppFinishLaunch")
	}

	// Nested mode: the pump must run. 200 ms is ~12 ticks at 60 Hz, so
	// this does not depend on precise timer scheduling.
	before := framePumpCount.Load()
	testRunModalMode(200)
	if framePumpCount.Load() == before {
		panic("frame pump: no frames pumped in NSModalPanelRunLoopMode")
	}

	// Default mode: the pump must stay out of the way. The timer still
	// fires here (it is installed in common modes) but the handler must
	// return before calling into Go.
	before = framePumpCount.Load()
	testRunDefaultMode(200)
	if got := framePumpCount.Load(); got != before {
		panic("frame pump: pumped in NSDefaultRunLoopMode — frames doubled")
	}

	// Destroy must retire the timer; a live one would keep pumping
	// frames for windows that no longer exist.
	b.Destroy()
	if testFramePumpActive() {
		panic("frame pump: timer still valid after Destroy")
	}
}
