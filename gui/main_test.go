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
	os.Exit(RunTestsCheckingLeaks(m))
}
