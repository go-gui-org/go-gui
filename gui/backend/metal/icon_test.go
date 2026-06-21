//go:build darwin && !ios

package metal

import (
	"runtime"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestActivateAppNoCrash verifies that activateApp() does not panic.
// On darwin this requires the Cocoa main thread; the real test runs
// in TestMain via runMainThreadTests. This function exists as a
// compile-time guard so the symbol is referenced in the test binary.
func TestActivateAppNoCrash(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Cocoa requires main thread; tested via TestMain")
	}
	activateApp()
}

// TestNewIncludesActivation verifies that New() calls activateApp
// before creating the window. The real test runs in TestMain.
func TestNewIncludesActivation(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Cocoa requires main thread; tested via TestMain")
	}
}

// runMainThreadTests executes Cocoa-dependent regression tests on the
// initial main thread (thread 0). Called from TestMain before m.Run()
// when GO_GUI_MAIN_THREAD_TESTS=1 is set.
//
// Panics on failure so the failure is reported clearly before the
// standard test runner output.
func runMainThreadTests() {
	// 1. activateApp must not crash — validates the ObjC function
	//    exists, compiles, links, and can be called.
	activateApp()

	// 2. activation policy must be Regular so the app appears in
	//    the Dock and can become active. Regression test for
	//    window not appearing on macOS (activation skipped).
	if !testActivationPolicyRegular() {
		panic("metalActivateApp: activation policy not Regular")
	}

	// 3. Menu bar must be fully wired — delegate, main menu, and
	//    Quit→handleQuit:. Regression test for Cmd+Q silently
	//    failing or falling through to terminate:.
	if !testDelegateSet() {
		panic("metalActivateApp: delegate not set")
	}
	if !testMenuExists() {
		panic("metalActivateApp: main menu not created")
	}
	if !testMenuQuitWired() {
		panic("metalActivateApp: Quit menu not wired to handleQuit:")
	}

	// 4. New() must succeed with activation — creates a real window
	//    and Metal context, then tears down. Validates that
	//    metalActivateApp + metalWindowCreate work end-to-end.
	//    Regression test for window opening behind terminal.
	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	b, err := New(w)
	if err != nil {
		panic("metal.New with activation: " + err.Error())
	}

	// 5. Window must have a delegate for close/resize/focus
	//    callbacks. Regression test for single-window close
	//    button not working.
	if !testWindowDelegateExists(b.window) {
		panic("metal.New: window delegate not set")
	}

	// 6. Verify the window is registered in the lookup table so
	//    C→Go callbacks (file drop, close, resize) can find it.
	//    Regression test for setAttachedWindow not being called.
	winID := testWindowID(b.window)
	if ws := lookupWindow(winID); ws == nil {
		panic("metal.New: window not registered in windowRegistry")
	}

	// 7. metalActivateNow must not crash — validates the C function
	//    exists, links, and can be called from Go. Regression test
	//    for the activation call added before the event loop.
	testActivateNow()

	b.Destroy()

	// 8. Destroy must unregister the window. Regression test for
	//    leaked entries in the windowRegistry after close.
	if ws := lookupWindow(winID); ws != nil {
		panic("metal.Destroy: window still in windowRegistry after destroy")
	}
}
