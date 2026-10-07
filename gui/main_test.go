package gui

import (
	"os"
	"testing"
)

// TestMain fails the package when a test leaves a window whose
// animation loop is still ticking (#836). The check itself is exported
// as RunTestsCheckingLeaks (#840) so consumers' test packages get the
// same gate; see its doc comment for why a leak matters.
func TestMain(m *testing.M) {
	// CI only: log the module load bases so a Windows crash in the same job
	// can be mapped to a DLL (#886). go test shows this output only when the
	// package fails, so passing runs stay quiet.
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		logLoadedModules(os.Stderr)
	}
	// Tests send Ctrl for shortcuts. Pin the Control mode so they mean
	// the same thing on a macOS dev box as on Linux CI (#969). Tests of
	// the macOS bindings switch with setKeyBindingForTest.
	keyBinding.Store(uint32(KeyBindingControl))
	os.Exit(RunTestsCheckingLeaks(m))
}
