package gpu

import (
	"os"
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
	} {
		if !strings.Contains(out, want) {
			t.Errorf("msl.BuildCustom output missing %q", want)
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
