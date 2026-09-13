package gui

import (
	"runtime"
	"strings"
)

// Shader holds custom fragment shader bodies and parameters.
type Shader struct {
	Metal  string    // MSL fragment body
	GLSL   string    // GLSL fragment body. Keep syntax compatible with desktop GL 3.3 and WebGL2 GLSL ES 3.00.
	Params []float32 // up to 16 custom floats
}

// BuildGLSLFragment wraps a user-supplied GLSL body with the
// standard preamble and epilogue.
func BuildGLSLFragment(body string) string {
	var b strings.Builder
	b.WriteString(`
    #version 330
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
// Uses Metal source on macOS, GLSL otherwise.
func ShaderHash(s *Shader) uint64 {
	if runtime.GOOS == "darwin" {
		return hashString(s.Metal)
	}
	return hashString(s.GLSL)
}

// hashString computes a 64-bit FNV-1a hash.
func hashString(s string) uint64 {
	return Fnv64Str(Fnv64Offset, s)
}
