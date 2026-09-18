package gpu

import (
	"os"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
	"github.com/go-gui-org/go-gui/gui/shader"
)

// TestBlurShadersGuardZeroBlur holds every copy of the blur shader
// to a guarded smoothstep. smoothstep with identical edges is
// undefined, and a blur below a quarter pixel packs to exactly 0
// (PackParams floors), so smoothstep(-blur, blur, d) is reachable
// with edge0 == edge1. The shadow shader already guards this with
// max(1.0, blur); the blur shader must match.
func TestBlurShadersGuardZeroBlur(t *testing.T) {
	t.Parallel()
	android, err := os.ReadFile(androidShaderSrc)
	if err != nil {
		t.Fatalf("read %s: %v", androidShaderSrc, err)
	}
	srcs := map[string]string{
		"shader.FsBlurGLSL": shader.FsBlurGLSL,
		"msl.Source":        msl.Source,
		androidShaderSrc:    string(android),
	}
	for name, src := range srcs {
		if strings.Contains(src, "smoothstep(-blur, blur,") {
			t.Errorf("%s: unguarded smoothstep(-blur, blur, ...) "+
				"is undefined when blur packs to 0; guard "+
				"with max(1.0, blur) like the shadow shader",
				name)
		}
	}
}
