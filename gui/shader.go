package gui

import (
	"runtime"
	"strings"
)

// Shader holds custom fragment shader bodies and parameters.
//
// Each body is trusted app-authored source for one fragment-shader
// main, not a full shader: declare a local frag_color
// (float4 in MSL, vec4 in GLSL) and the framework wraps it with
// the SDF round-rect clipping and the output. Do not put a
// #version line or a main function in a body, and do not build
// one from untrusted input — it is spliced into the compiled
// source as-is. Provide both bodies for cross-platform builds;
// a backend compiles only its own and ignores the other.
//
// Params carries up to 16 custom floats to p0..p3. Longer slices
// are silently truncated to the first 16; NaN or Inf entries
// fail render validation and drop the draw.
type Shader struct {
	Metal  string    // MSL fragment body
	GLSL   string    // GLSL fragment body. Keep syntax compatible with desktop GL 3.3 and WebGL2 GLSL ES 3.00.
	Params []float32 // up to 16 custom floats
}

// BuildGLSLFragment wraps a user-supplied GLSL body with the
// standard preamble and epilogue for desktop GL 3.3.
func BuildGLSLFragment(body string) string {
	return buildGLSLCustomFragment("#version 330", body)
}

// BuildGLSLESFragment wraps a user-supplied GLSL body exactly
// like BuildGLSLFragment, but for WebGL2 and GLES
// (#version 300 es). A body that compiles under one compiles
// under the other; only the version header differs. The web and
// Android backends share this instead of carrying their own copy
// of the wrapper.
func BuildGLSLESFragment(body string) string {
	return buildGLSLCustomFragment("#version 300 es\n"+
		"precision highp float;\n"+
		"precision highp int;", body)
}

// buildGLSLCustomFragment is the single copy of the custom-shader
// GLSL wrapper. The body must declare a local vec4 frag_color;
// the wrapper adds the SDF round-rect clipping around it.
func buildGLSLCustomFragment(header, body string) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString(`
uniform sampler2D tex;
in vec2 uv;
in vec4 color;
in float params;
in vec4 p0;
in vec4 p1;
in vec4 p2;
in vec4 p3;

out vec4 _frag_out;

void main() {
    float radius = floor(params / 4096.0) / 4.0;

    vec2 uv_to_px = 1.0 / (vec2(fwidth(uv.x), fwidth(uv.y)) + 1e-6);
    vec2 half_size = uv_to_px;
    vec2 pos = uv * half_size;

    vec2 q = abs(pos) - half_size + vec2(radius);
    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;

    float grad_len = length(vec2(dFdx(d), dFdy(d)));
    d = d / max(grad_len, 0.001);
    float sdf_alpha = 1.0 - smoothstep(-0.59, 0.59, d);

    // --- user body ---
    `)
	b.WriteString(body)
	b.WriteString(`
    // --- end user body ---

    _frag_out = vec4(frag_color.rgb, frag_color.a * sdf_alpha);

    if (_frag_out.a < 0.0) {
        _frag_out += texture(tex, uv);
    }
}
`)
	return b.String()
}

// ShaderHash computes a cache key from the shader source.
// Uses Metal source on macOS, GLSL otherwise, so shaders that
// differ only in the unused platform's body share a cache entry
// there by design. Params are uniforms, not source, and are not
// part of the key. A nil shader hashes to 0.
func ShaderHash(s *Shader) uint64 {
	if s == nil {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return hashString(s.Metal)
	}
	return hashString(s.GLSL)
}

// hashString computes a 64-bit FNV-1a hash.
func hashString(s string) uint64 {
	return Fnv64Str(Fnv64Offset, s)
}
