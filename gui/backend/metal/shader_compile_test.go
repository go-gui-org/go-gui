//go:build darwin && !ios

package metal

import "testing"

// TestBuiltinShadersCompile compiles the built-in MSL library against
// the pinned language version (MSL 3.0 / macOS 13 Ventura) that
// metalCtxCreate uses.
//
// This is the regression guard for issue 126, where a Metal 3.2-only
// lambda in fs_gradient compiled fine on macOS 15+ but failed on
// Sonoma — and because the library is built once at context init,
// broke every app rather than just gradient-drawing ones.
//
// It exists as a separate probe because the real compile only happens
// once a window is created, which requires the Cocoa main thread and
// is gated behind GO_GUI_MAIN_THREAD_TESTS. No CI job sets that, so
// without this test nothing compiles a shader on any runner.
//
// To confirm the test actually bites, reintroduce a lambda into
// gui/backend/internal/msl/shaders.h and check that it fails.
func TestBuiltinShadersCompile(t *testing.T) {
	switch rc := testCompileShaders(); rc {
	case 0:
		// compiled
	case 1:
		t.Skip("no Metal device (headless or virtualized runner)")
	default:
		t.Fatalf("built-in MSL library failed to compile at the "+
			"pinned language version (probe rc=%d); see the logged "+
			"MTLLibraryError above. A shader construct newer than "+
			"the pin in mslCompileOptions() will break older macOS "+
			"at runtime — fix the shader, do not raise the pin", rc)
	}
}
