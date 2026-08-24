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
// the timer must be armed only while the main runloop runs in a nested
// mode — it must fire (and reach Go) there, must not exist at all in
// NSDefaultRunLoopMode where the backend's own event loop owns frame
// timing, and must be retired by Destroy.
//
// Regression test for windows freezing — no repaint, no command flush,
// no terminal reflow on resize — for as long as a modal dialog or open
// menu keeps a nested AppKit runloop on the main thread. Also guards
// issue #406: the 60 Hz timer must not wake the run loop in the default
// mode, where its only act used to be an early return.
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

	// metalAppFinishLaunch registers the mode observer, not a timer.
	testActivateNow()
	if testFramePumpActive() {
		panic("frame pump: timer armed outside a nested mode (issue #406)")
	}

	// Nested mode: the pump must run. 200 ms is ~12 ticks at 60 Hz, so
	// this does not depend on precise timer scheduling.
	before := framePumpCount.Load()
	testRunModalMode(200)
	if framePumpCount.Load() == before {
		panic("frame pump: no frames pumped in NSModalPanelRunLoopMode")
	}

	// A second nested mode over the first (menu over a modal dialog)
	// must keep the pump armed — only the exit of the last nested mode
	// may retire the timer.
	if testNestedModesPumpStaysArmed() {
		panic("frame pump: timer disarmed while a nested mode was still active")
	}

	// Default mode: the pump must not exist, let alone pump. Frames
	// doubled here would double every frame the Go loop renders.
	before = framePumpCount.Load()
	testRunDefaultMode(200)
	if got := framePumpCount.Load(); got != before {
		panic("frame pump: pumped in NSDefaultRunLoopMode — frames doubled")
	}
	if testFramePumpActive() {
		panic("frame pump: timer left armed after returning to the default mode")
	}

	// Destroy must retire the observer; a live pump would keep pumping
	// frames for windows that no longer exist.
	b.Destroy()
	if testFramePumpActive() {
		panic("frame pump: timer still valid after Destroy")
	}
}
