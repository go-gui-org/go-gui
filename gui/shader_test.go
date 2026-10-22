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

// Decls land at program scope, after the wrapper's globals and before
// main, in both GLSL wrappers: a helper function can then read iTime
// (#573). Built without decls, the source is the plain body wrapper.
func TestGLSLFragmentDeclsBeforeMain(t *testing.T) {
	decls := "float _helper(float x) { return x * iTime; }"
	body := "vec4 frag_color = vec4(_helper(1.0));"
	for name, src := range map[string]string{
		"330": BuildGLSLFragmentDecls(decls, body),
		"es":  BuildGLSLESFragmentDecls(decls, body),
	} {
		at := strings.Index(src, decls)
		if at < 0 {
			t.Errorf("%s: decls missing", name)
			continue
		}
		if g := strings.Index(src, "vec4  iMouse;"); g < 0 || g > at {
			t.Errorf("%s: decls precede the built-in globals", name)
		}
		if m := strings.Index(src, "void main()"); m < at {
			t.Errorf("%s: decls not before main", name)
		}
		if b := strings.Index(src, body); b < strings.Index(src, "void main()") {
			t.Errorf("%s: body not inside main", name)
		}
	}
	if BuildGLSLFragmentDecls("", body) != BuildGLSLFragment(body) {
		t.Error("empty decls should match BuildGLSLFragment")
	}
	if BuildGLSLESFragmentDecls("", body) != BuildGLSLESFragment(body) {
		t.Error("empty decls should match BuildGLSLESFragment")
	}
}

func TestShaderHashCoversDecls(t *testing.T) {
	base := Shader{Metal: "m", GLSL: "g", MetalDecls: "md", GLSLDecls: "gd"}
	h := ShaderHash(&base)
	for name, s := range map[string]Shader{
		"MetalDecls": {Metal: "m", GLSL: "g", MetalDecls: "x", GLSLDecls: "gd"},
		"GLSLDecls":  {Metal: "m", GLSL: "g", MetalDecls: "md", GLSLDecls: "x"},
		// Text moved across a field boundary still changes the key.
		"shifted": {Metal: "m", GLSL: "g", MetalDecls: "mdg", GLSLDecls: "d"},
	} {
		if ShaderHash(&s) == h {
			t.Errorf("%s: change did not change the hash", name)
		}
	}
}

// ShaderHash runs on every draw of every custom-shader command, in
// each backend's drawCustomShader. A ported Shadertoy shader is a few
// KB of source per language, so this measures that size.
func BenchmarkShaderHash(b *testing.B) {
	src := strings.Repeat("float x = sin(iTime) * 0.5;\n", 8<<10/28)
	s := &Shader{Metal: src, GLSL: src, MetalDecls: src, GLSLDecls: src}
	b.ReportAllocs()
	for b.Loop() {
		ShaderHash(s)
	}
}

func TestShaderHashCacheMatchesShaderHash(t *testing.T) {
	var c ShaderHashCache
	s := &Shader{Metal: "m", GLSL: "g", MetalDecls: "md", GLSLDecls: "gd"}
	if got, want := c.Hash(s), ShaderHash(s); got != want {
		t.Fatalf("miss: got %x, want %x", got, want)
	}
	if got, want := c.Hash(s), ShaderHash(s); got != want {
		t.Fatalf("hit: got %x, want %x", got, want)
	}
	if c.Hash(nil) != 0 {
		t.Error("nil shader should hash to 0")
	}
}

// A field reassigned on the same *Shader must not return the stale
// key: the cache checks each string's data pointer and length.
func TestShaderHashCacheSeesReassignment(t *testing.T) {
	var c ShaderHashCache
	s := &Shader{Metal: "m", GLSL: "g", MetalDecls: "md", GLSLDecls: "gd"}
	c.Hash(s)
	for name, set := range map[string]func(string){
		"Metal":      func(v string) { s.Metal = v },
		"GLSL":       func(v string) { s.GLSL = v },
		"MetalDecls": func(v string) { s.MetalDecls = v },
		"GLSLDecls":  func(v string) { s.GLSLDecls = v },
	} {
		set(strings.Repeat("x", len(name)+1)) // new backing array
		if got, want := c.Hash(s), ShaderHash(s); got != want {
			t.Errorf("%s reassigned: got stale %x, want %x", name, got, want)
		}
	}
}

// The cache is bounded: shaders built per frame (a new *Shader each
// time) must not grow it without limit.
func TestShaderHashCacheBounded(t *testing.T) {
	var c ShaderHashCache
	for i := range 10 * shaderHashCacheMax {
		c.Hash(&Shader{Metal: string(rune('a' + i%26))})
	}
	if n := len(c.entries); n > shaderHashCacheMax {
		t.Errorf("cache holds %d entries, want <= %d", n, shaderHashCacheMax)
	}
}

func TestShaderHashCacheHitAllocsZero(t *testing.T) {
	var c ShaderHashCache
	s := &Shader{Metal: "m", GLSL: "g"}
	c.Hash(s)
	if n := testing.AllocsPerRun(100, func() { c.Hash(s) }); n != 0 {
		t.Errorf("hit allocates %v times, want 0", n)
	}
}

func BenchmarkShaderHashCacheHit(b *testing.B) {
	src := strings.Repeat("float x = sin(iTime) * 0.5;\n", 8<<10/28)
	s := &Shader{Metal: src, GLSL: src, MetalDecls: src, GLSLDecls: src}
	var c ShaderHashCache
	c.Hash(s)
	b.ReportAllocs()
	for b.Loop() {
		c.Hash(s)
	}
}
