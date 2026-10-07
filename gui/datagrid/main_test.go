package datagrid

import (
	"fmt"
	"os"
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// TestMain pins the Control key binding: the tests send Ctrl for
// shortcuts, so they must mean the same on a macOS dev box as on Linux
// CI (#969).
//
// GOGUI_KEY_BINDING_MODE pins the mode for the whole process, and
// SetKeyBindingMode then does nothing, so the pin here and in
// setKeyBindingForTest would silently fail and the tests would report
// false failures. Stop with a clear message instead. The gui package's
// own tests do not need this: they write the mode directly.
func TestMain(m *testing.M) {
	if v := os.Getenv("GOGUI_KEY_BINDING_MODE"); v != "" {
		fmt.Fprintf(os.Stderr,
			"datagrid tests: unset GOGUI_KEY_BINDING_MODE (now %q); "+
				"the tests set the key binding mode themselves\n", v)
		os.Exit(1)
	}
	gg.SetKeyBindingMode(gg.KeyBindingControl)
	os.Exit(m.Run())
}

// setKeyBindingForTest switches the process-wide mode for one test and
// puts Control back after. The mode is global, so a caller must not run
// in parallel with other tests.
func setKeyBindingForTest(t *testing.T, m gg.KeyBindingMode) {
	t.Helper()
	gg.SetKeyBindingMode(m)
	t.Cleanup(func() { gg.SetKeyBindingMode(gg.KeyBindingControl) })
}
