package soft

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestMain pins the Control key binding: the tests send Ctrl for
// shortcuts, so they must mean the same on macOS as on Linux and
// Windows (#969). Without the pin, Ctrl+V on macOS pastes nothing.
//
// GOGUI_KEY_BINDING_MODE pins the mode for the whole process, and
// SetKeyBindingMode then does nothing, so the tests would report false
// failures. Stop with a clear message instead.
func TestMain(m *testing.M) {
	if v := os.Getenv("GOGUI_KEY_BINDING_MODE"); v != "" {
		fmt.Fprintf(os.Stderr,
			"soft tests: unset GOGUI_KEY_BINDING_MODE (now %q); "+
				"the tests set the key binding mode themselves\n", v)
		os.Exit(1)
	}
	gui.SetKeyBindingMode(gui.KeyBindingControl)
	os.Exit(m.Run())
}
