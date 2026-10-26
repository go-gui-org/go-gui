//go:build darwin && cgo && !ios

package metal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
	"github.com/go-gui-org/go-gui/internal/shaderport"
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

// TestCustomWrapperDeclsCompile compiles msl.BuildCustomDecls with
// program-scope helpers, the shape cmd/shaderport emits: a template
// helper, a struct, a program-scope constant, and a function that
// takes a built-in as a parameter and writes a thread reference
// (#573).
func TestCustomWrapperDeclsCompile(t *testing.T) {
	decls := `
template <typename T, typename U>
T _gg_mod(T x, U y) { return x - y * floor(x / y); }

constant float PI = 3.14159265;

struct Hit { float d; float3 col; };

Hit scene(float2 p, float iTime) {
    return Hit{length(p) - 0.5, float3(0.5 + 0.5 * cos(iTime))};
}

void mainImage(thread float4& O, float2 fragCoord, float3 iResolution,
               float iTime) {
    float2 p = (2.0 * fragCoord - iResolution.xy) / iResolution.y;
    Hit h = scene(_gg_mod(p, float2(PI)), iTime);
    if (h.d > 0.0) { O = float4(0.0); return; }
    O = float4(h.col, 1.0);
}
`
	body := "float4 frag_color;\n" +
		"mainImage(frag_color, float2((in.uv.x * 0.5 + 0.5) * iResolution.x, " +
		"(0.5 - in.uv.y * 0.5) * iResolution.y), iResolution, iTime);"
	switch rc := testCompileMSL(msl.BuildCustomDecls(decls, body)); rc {
	case 0:
	case 1:
		t.Skip("no Metal device (headless or virtualized runner)")
	default:
		t.Errorf("wrapped MSL with decls failed to compile (rc=%d); "+
			"see the logged MTLLibraryError above", rc)
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

// TestShaderportCompiles ports every fixture in the shaderport corpus
// and compiles its MSL output as the Metal backend would (#573). The
// GLSL side is checked by TestPortGLSLCompiles in internal/shaderport.
func TestShaderportCompiles(t *testing.T) {
	files, err := filepath.Glob("../../../internal/shaderport/testdata/*.frag")
	if err != nil || len(files) == 0 {
		t.Fatalf("no shaderport fixtures: %v", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		r, err := shaderport.Port(string(src))
		if err != nil {
			t.Fatalf("%s: Port: %v", f, err)
		}
		switch rc := testCompileMSL(msl.BuildCustomDecls(r.MetalDecls, r.Metal)); rc {
		case 0:
		case 1:
			t.Skip("no Metal device (headless or virtualized runner)")
		default:
			t.Errorf("%s: ported MSL failed to compile (rc=%d); "+
				"see the logged MTLLibraryError above", filepath.Base(f), rc)
		}
	}
}

// TestCustomWrapperChannelsCompile compiles msl.BuildCustomChannels, the
// wrapper of a shader with a Surface (#1168), around bodies that sample
// the channels with iChannelSampler and read iChannelResolution. The iOS
// backend uses the same wrapper, so this covers its MSL too. Binding and
// drawing are checked by hand on a Mac (examples/shader_channels): no
// runner here draws through Metal.
func TestCustomWrapperChannelsCompile(t *testing.T) {
	bodies := map[string]string{
		"all channels": "float2 p = in.uv * 0.5 + 0.5;\n" +
			"float4 a = iChannel0.sample(iChannelSampler, p);\n" +
			"float4 b = iChannel1.sample(iChannelSampler, p);\n" +
			"float4 c = iChannel2.sample(iChannelSampler, p) + iChannel3.sample(iChannelSampler, p);\n" +
			"float4 frag_color = float4(a.r, b.rg, 1.0 - c.a);",
		"resolution": "float4 frag_color = float4(iChannelResolution[0].xy / 255.0, " +
			"iChannelResolution[3].z, 1.0);",
		"nv12": "float2 p = in.uv * 0.5 + 0.5;\n" +
			"float y = (iChannel0.sample(iChannelSampler, p).r - 16.0 / 255.0) * (255.0 / 219.0);\n" +
			"float2 c = (iChannel1.sample(iChannelSampler, p).rg - 128.0 / 255.0) * (255.0 / 224.0);\n" +
			"float3 rgb = float3(y + 1.5748 * c.y, y - 0.1873 * c.x - 0.4681 * c.y, y + 1.8556 * c.x);\n" +
			"float4 frag_color = float4(clamp(rgb, 0.0, 1.0), 1.0);",
		"with builtins": "float4 frag_color = iChannel0.sample(iChannelSampler, " +
			"in.position.xy / iResolution.xy) * cos(iTime);",
	}
	for name, body := range bodies {
		switch rc := testCompileMSL(msl.BuildCustomChannels("", body)); rc {
		case 0:
		case 1:
			t.Skip("no Metal device (headless or virtualized runner)")
		default:
			t.Errorf("%s body: wrapped channel MSL failed to compile (rc=%d); "+
				"see the logged MTLLibraryError above", name, rc)
		}
	}
}
