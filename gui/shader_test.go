package gui

import (
	"strings"
	"testing"
)

func TestBuildGLSLFragment(t *testing.T) {
	body := "vec4 frag_color = color;"
	result := BuildGLSLFragment(body)
	if !strings.Contains(result, body) {
		t.Error("body not found in output")
	}
	if !strings.Contains(result, "#version 330") {
		t.Error("missing GLSL version")
	}
}

func TestShaderHash(t *testing.T) {
	s := &Shader{Metal: "test_metal", GLSL: "test_glsl"}
	h := ShaderHash(s)
	if h == 0 {
		t.Error("hash should not be zero")
	}
	// Same input → same hash.
	if ShaderHash(s) != h {
		t.Error("hash should be deterministic")
	}
	// Different input → different hash.
	s2 := &Shader{Metal: "other", GLSL: "other"}
	if ShaderHash(s2) == h {
		t.Error("different shaders should differ")
	}
}

func TestShaderHashNil(t *testing.T) {
	if got := ShaderHash(nil); got != 0 {
		t.Errorf("ShaderHash(nil) = %d, want 0", got)
	}
}

func TestShaderHashCoversBothBodies(t *testing.T) {
	h := ShaderHash(&Shader{Metal: "m", GLSL: "g"})
	// A difference in either body changes the key on every
	// platform. iOS reports GOOS "ios", compiles Metal, and
	// used to key on the GLSL body.
	if ShaderHash(&Shader{Metal: "other", GLSL: "g"}) == h {
		t.Error("Metal body should change the hash")
	}
	if ShaderHash(&Shader{Metal: "m", GLSL: "other"}) == h {
		t.Error("GLSL body should change the hash")
	}
}

func TestShaderHashSeparatesBodies(t *testing.T) {
	// Without a separator both hash the concatenation "abc".
	a := ShaderHash(&Shader{Metal: "ab", GLSL: "c"})
	b := ShaderHash(&Shader{Metal: "a", GLSL: "bc"})
	if a == b {
		t.Error("moving text across the body boundary should change the hash")
	}
}
