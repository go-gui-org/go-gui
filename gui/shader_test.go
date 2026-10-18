package gui

import (
	"regexp"
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

// The GLSL wrapper unpacks each built-in from the slot
// packShaderBuiltins writes it to. Column c of the mat4 holds slots
// 4c..4c+3, so _gg_builtins[c].x is slot 4c and .w is 4c+3.
func TestGLSLWrapperBuiltinSlots(t *testing.T) {
	src := BuildGLSLFragment("vec4 frag_color = vec4(1.0);")
	if !strings.Contains(src, "uniform mat4 _gg_builtins;") {
		t.Fatal("wrapper does not declare uniform mat4 _gg_builtins")
	}
	for _, tc := range []struct {
		name  string
		slot  int // first slot
		width int // components read
	}{
		{"iResolution", shaderBuiltinResX, 3},
		{"iTime", shaderBuiltinTime, 1},
		{"iMouse", shaderBuiltinMouse, 4},
		{"iTimeDelta", shaderBuiltinTimeDelta, 1},
		{"iFrame", shaderBuiltinFrame, 1},
	} {
		re := regexp.MustCompile(`\b` + tc.name +
			`\s*=\s*(?:int\()?_gg_builtins\[(\d)\](?:\.([xyzw]+))?`)
		m := re.FindStringSubmatch(src)
		if m == nil {
			t.Errorf("%s: no assignment from _gg_builtins", tc.name)
			continue
		}
		col := int(m[1][0] - '0')
		first, width := 0, 4 // a bare column reads all four
		if m[2] != "" {
			first, width = strings.IndexByte("xyzw", m[2][0]), len(m[2])
		}
		if got := col*4 + first; got != tc.slot || width != tc.width {
			t.Errorf("%s reads slot %d width %d, want slot %d width %d",
				tc.name, got, width, tc.slot, tc.width)
		}
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
