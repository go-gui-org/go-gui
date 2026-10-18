//go:build darwin && cgo && !ios

package metal

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
)

// TestCustomWrapperCompiles compiles msl.BuildCustom around bodies
// that read the built-in inputs (#573), at the same pinned language
// version as the built-in library: stricter than the custom pipeline,
// which compiles with options:nil. A body that declares its own iTime
// must shadow the wrapper's, as in GLSL.
func TestCustomWrapperCompiles(t *testing.T) {
	bodies := map[string]string{
		"params": "float4 frag_color = float4(in.p0.x, in.uv.x, " +
			"in.color.r, 1.0);",
		"builtins": "float2 st = in.position.xy / iResolution.xy;\n" +
			"float m = iMouse.z > 0.0 ? iMouse.w : 0.0;\n" +
			"float f = float(iFrame % 2) * iTimeDelta;\n" +
			"float4 frag_color = float4(st, cos(iTime) + m + f, 1.0);",
		"own iTime": "float iTime = in.p0.x;\n" +
			"float2 iResolution = float2(1.0);\n" +
			"float4 frag_color = float4(cos(iTime), iResolution, 1.0);",
		"wrapper names": "float d = 1.0; float2 q = in.uv; float radius = 2.0;\n" +
			"float2 pos = in.uv; float sdf_alpha = 0.5;\n" +
			"float4 frag_color = float4(d * sdf_alpha, q.x, radius, pos.y);",
	}
	for name, body := range bodies {
		switch rc := testCompileMSL(msl.BuildCustom(body)); rc {
		case 0:
		case 1:
			t.Skip("no Metal device (headless or virtualized runner)")
		default:
			t.Errorf("%s body: wrapped MSL failed to compile (rc=%d); "+
				"see the logged MTLLibraryError above", name, rc)
		}
	}
}

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
