package gpu

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
)

const (
	customTestBodyGLSL = "vec4 frag_color = vec4(p0.x, uv.x, " +
		"color.r, 1.0);"
	customTestBodyMSL = "float4 frag_color = float4(in.p0.x, " +
		"in.uv.x, in.color.r, 1.0);"
)

// TestCustomGLSLWrappersAgree holds the desktop and ES fragment
// wrappers to one copy: the outputs must differ only in the
// version header, so a body compiling under one compiles under
// the other.
func TestCustomGLSLWrappersAgree(t *testing.T) {
	t.Parallel()
	desktop := gui.BuildGLSLFragment(customTestBodyGLSL)
	es := gui.BuildGLSLESFragment(customTestBodyGLSL)

	if !strings.Contains(desktop, "#version 330") {
		t.Error("desktop wrapper missing #version 330")
	}
	if !strings.Contains(es, "#version 300 es") {
		t.Error("ES wrapper missing #version 300 es")
	}
	for _, want := range []string{
		customTestBodyGLSL,
		"floor(params / 4096.0) / 4.0",
		"smoothstep(-0.59, 0.59,",
		"frag_color",
		"in vec4 p0;",
		"in vec4 p1;",
		"in vec4 p2;",
		"in vec4 p3;",
		// The body runs in a nested block, so its frag_color is
		// copied out before the block closes.
		"_gg_color = frag_color;\n    }",
	} {
		if !strings.Contains(desktop, want) {
			t.Errorf("desktop wrapper missing %q", want)
		}
		if !strings.Contains(es, want) {
			t.Errorf("ES wrapper missing %q", want)
		}
	}

	const esHeader = "#version 300 es\n" +
		"precision highp float;\n" +
		"precision highp int;"
	relabeled := strings.Replace(es, esHeader, "#version 330", 1)
	if relabeled != desktop {
		t.Error("desktop and ES wrappers differ past the " +
			"version header; keep them one shared core")
	}
}

// TestCustomGLSLWrapperCompiles compiles wrapped bodies with glslc,
// Khronos glslang's front end as shipped in the Android NDK
// (ndk/<version>/shader-tools/<host>/glslc) and the Vulkan SDK. Set
// GOGUI_GLSLC to its path, or put it on PATH; the test skips without
// it. String checks cannot catch a type error or a name the wrapper
// fails to declare; a compiler does.
//
// glslc emits SPIR-V, which accepts ES shaders only from version 310,
// so the ES wrapper is checked as 310 es. 310 es is a superset of the
// 300 es WebGL2 and Android compile, so a pass here is strong evidence,
// not proof; the real check is drawing on those backends.
func TestCustomGLSLWrapperCompiles(t *testing.T) {
	glslc := os.Getenv("GOGUI_GLSLC")
	if glslc == "" {
		var err error
		if glslc, err = exec.LookPath("glslc"); err != nil {
			t.Skip("no glslc: set GOGUI_GLSLC or put glslc on PATH")
		}
	}
	bodies := map[string]string{
		"params": customTestBodyGLSL,
		// Every built-in, used with its declared type.
		"builtins": "vec2 st = gl_FragCoord.xy / iResolution.xy;\n" +
			"float m = iMouse.z > 0.0 ? iMouse.w : 0.0;\n" +
			"float f = float(iFrame % 2) * iTimeDelta;\n" +
			"vec4 frag_color = vec4(st, cos(iTime) + m + f, 1.0);",
		// A body written before the built-ins existed may declare
		// its own; it must shadow the wrapper's, not collide.
		"own iTime": "float iTime = p0.x;\n" +
			"vec2 iResolution = vec2(1.0);\n" +
			"vec4 frag_color = vec4(cos(iTime), iResolution, 1.0);",
		// The wrapper's own locals are common names; a body that
		// reuses them must compile, as it does on Metal.
		"wrapper names": "float d = 1.0; vec2 q = uv; float radius = 2.0;\n" +
			"vec2 pos = uv; float sdf_alpha = 0.5;\n" +
			"vec4 frag_color = vec4(d * sdf_alpha, q.x, radius, pos.y);",
	}
	dir := t.TempDir()
	for name, body := range bodies {
		for _, src := range []struct{ lang, text string }{
			{"gl330", gui.BuildGLSLFragment(body)},
			{"es310", strings.Replace(gui.BuildGLSLESFragment(body),
				"#version 300 es", "#version 310 es", 1)},
		} {
			path := filepath.Join(dir, "shader.frag")
			if err := os.WriteFile(path, []byte(src.text), 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(glslc, "--target-env=opengl",
				"-fauto-map-locations", "-fauto-bind-uniforms",
				"-fshader-stage=frag", "-o", os.DevNull, path).CombinedOutput()
			if err != nil {
				t.Errorf("%s body, %s: %v\n%s", name, src.lang, err, out)
			}
		}
	}
}

// TestCustomMSLWrapperAgrees holds the shared MSL wrapper to the
// same contract: user body spliced in, SDF clipping around a
// body-declared frag_color, params in p0..p3.
func TestCustomMSLWrapperAgrees(t *testing.T) {
	t.Parallel()
	out := msl.BuildCustom(customTestBodyMSL)
	for _, want := range []string{
		customTestBodyMSL,
		"/ 4096.0) / 4.0",
		"smoothstep(-0.59, 0.59,",
		"frag_color",
		"float4 p0;",
		"float4 p3;",
		"constant float4x4 &_gg_builtins [[buffer(0)]]",
		// The body runs in a nested block, so its frag_color is
		// copied out before the block closes.
		"_gg_color = frag_color;\n    }",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("msl.BuildCustom output missing %q", want)
		}
	}
}

// reBuiltinUnpack matches a wrapper line that unpacks a built-in from
// _gg_builtins, in either language: name, column, and swizzle (empty
// for a whole column).
var reBuiltinUnpack = regexp.MustCompile(
	`\b(iResolution|iTime|iTimeDelta|iFrame|iMouse)\s*=\s*` +
		`(?:int\()?_gg_builtins\[(\d)\](?:\.([xyzw]+))?`)

// builtinUnpacks maps each built-in name in src to "column.swizzle".
func builtinUnpacks(src string) map[string]string {
	got := map[string]string{}
	for _, m := range reBuiltinUnpack.FindAllStringSubmatch(src, -1) {
		got[m[1]] = m[2] + "." + m[3]
	}
	return got
}

// TestCustomWrappersUnpackBuiltinsAlike holds the MSL wrapper to the
// GLSL one: each built-in comes from the same column and components.
// gui's TestGLSLWrapperBuiltinSlots holds the GLSL side to the slot
// constants the block is packed with, so this pins MSL to them too.
func TestCustomWrappersUnpackBuiltinsAlike(t *testing.T) {
	t.Parallel()
	glsl := builtinUnpacks(gui.BuildGLSLFragment(customTestBodyGLSL))
	mslU := builtinUnpacks(msl.BuildCustom(customTestBodyMSL))
	if len(glsl) != 5 {
		t.Fatalf("GLSL wrapper unpacks %d built-ins, want 5: %v", len(glsl), glsl)
	}
	for name, want := range glsl {
		if got, ok := mslU[name]; !ok || got != want {
			t.Errorf("%s: MSL reads _gg_builtins[%s], GLSL reads [%s]",
				name, got, want)
		}
	}
	if len(mslU) != len(glsl) {
		t.Errorf("MSL unpacks %v, GLSL unpacks %v", mslU, glsl)
	}
}

// TestCustomShaderBackendsUploadBuiltins pins every GPU backend's
// custom-shader draw to an upload of the built-in block. On Metal and
// iOS a missed upload is not just stale values: fs_main reads fragment
// buffer(0) whatever the body uses, and an unbound buffer is undefined
// behavior.
func TestCustomShaderBackendsUploadBuiltins(t *testing.T) {
	t.Parallel()
	checks := map[string]string{
		"../../gl/draw.go":           "ShaderBuiltinsAt(",
		"../../web/custom_shader.go": "ShaderBuiltinsAt(",
		"../../android/draw.go":      "C.glesSetBuiltins(",
		"../../metal/draw.go":        "C.metalSetCustomBuiltins(",
		"../../ios/draw.go":          "C.metalSetCustomBuiltins(",
	}
	for path, want := range checks {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("%s: missing built-in upload %q", path, want)
		}
	}
}

// TestCustomWrappersAreShared pins the backends to the shared
// builders. A hand-edited wrapper copy in any backend drifts
// silently, the way the gradient copies once did.
func TestCustomWrappersAreShared(t *testing.T) {
	t.Parallel()
	checks := map[string]string{
		"../../metal/draw.go":        "msl.BuildCustom(",
		"../../ios/draw.go":          "msl.BuildCustom(",
		"../../android/draw.go":      "gui.BuildGLSLESFragment(",
		"../../web/custom_shader.go": "gui.BuildGLSLESFragment(",
	}
	for path, want := range checks {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("%s: missing shared-builder call %q; "+
				"do not carry a wrapper copy", path, want)
		}
	}
	stale := map[string]string{
		"../../web/custom_shader.go": "func webGL2FragmentSource(",
		"../../metal/draw.go":        "fragment float4 fs_main(",
		"../../ios/draw.go":          "fragment float4 fs_main(",
	}
	for path, want := range stale {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(data), want) {
			t.Errorf("%s: stale wrapper copy %q; use the "+
				"shared builder", path, want)
		}
	}
}
