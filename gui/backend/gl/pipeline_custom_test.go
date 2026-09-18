//go:build !js && !darwin && !android

package gl

import (
	"errors"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// TestGetOrBuildCustomPipelineCachesFailure holds a cached
// compile failure to its error without touching GL. A broken
// body must not recompile on every frame.
func TestGetOrBuildCustomPipelineCachesFailure(t *testing.T) {
	b := &Backend{}
	b.pipelines.customCache = texcache.New[uint64, pipeline](
		maxCustomPipelines, nil)
	s := &gui.Shader{GLSL: "broken body"}
	h := gui.ShaderHash(s)
	b.pipelines.customCache.Set(h, pipeline{})
	got, err := b.getOrBuildCustomPipeline(s)
	if !errors.Is(err, errCustomShaderCompile) {
		t.Fatalf("cached failure err = %v, want errCustomShaderCompile", err)
	}
	if got.program != 0 {
		t.Errorf("cached failure program = %d, want 0", got.program)
	}
	if b.pipelines.customCache.Len() != 1 {
		t.Errorf("cache len = %d, want 1 (no recompile, no dup)",
			b.pipelines.customCache.Len())
	}
}
