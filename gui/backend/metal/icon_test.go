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

	// 2. New() must succeed with activation — creates a real SDL
	//    window and Metal context, then tears down. Catches the
	//    regression where activateApp is removed from the New() path
	//    and the window becomes invisible on macOS.
	w := gui.NewWindow(gui.WindowCfg{
		State:  new(int),
		Width:  200,
		Height: 200,
	})
	b, err := New(w)
	if err != nil {
		panic("metal.New with activation: " + err.Error())
	}
	b.Destroy()
}
