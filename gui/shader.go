package gui

import (
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
// A body reads Shadertoy's built-in inputs under their Shadertoy
// names, types and units: iResolution (the container's size in
// physical pixels), iTime, iTimeDelta and iFrame, while uv spans
// -1..1 across the container with y growing down. iMouse is declared
// but reads zero, Shadertoy's value for a canvas never clicked. A body
// runs in its own block, so a local of the same name as a built-in or
// as a wrapper variable hides it. Set Animate for a shader that
// changes over time.
//
// Params carries up to 16 custom floats to p0..p3. Longer slices
// are silently truncated to the first 16; NaN or Inf entries
// fail render validation and drop the draw.
type Shader struct {
	Metal  string    // MSL fragment body
	GLSL   string    // GLSL fragment body. Keep syntax compatible with desktop GL 3.3 and WebGL2 GLSL ES 3.00.
	Params []float32 // up to 16 custom floats
	// Animate keeps frames coming while a container with this shader
	// is drawn, so iTime advances without app code. Each
	// frame rebuilds only the render commands: no view function runs
	// and nothing is re-arranged. Frames stop once no Animate shader
	// is drawn. For a user who asked for reduced motion the shader
	// holds still instead, as in a headless capture: iTime, iTimeDelta
	// and iFrame read zero and no frames are started. Leave it false
	// for a shader that does not change.
	Animate bool
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

// Built-in inputs (#573): Shadertoy's names and units, unpacked from
// RenderCmd.ShaderBuiltinsAt (slots in gui/shader_builtins.go). Globals
// rather than locals of main, so a body that declares its own iTime
// shadows this one instead of failing to compile.
uniform mat4 _gg_builtins;
vec3  iResolution;
float iTime;
float iTimeDelta;
int   iFrame;
vec4  iMouse;

void main() {
    iResolution = _gg_builtins[0].xyz;
    iTime       = _gg_builtins[0].w;
    iMouse      = _gg_builtins[1];
    iTimeDelta  = _gg_builtins[2].x;
    iFrame      = int(_gg_builtins[2].y);

    float radius = floor(params / 4096.0) / 4.0;

    vec2 uv_to_px = 1.0 / (vec2(fwidth(uv.x), fwidth(uv.y)) + 1e-6);
    vec2 half_size = uv_to_px;
    vec2 pos = uv * half_size;

    vec2 q = abs(pos) - half_size + vec2(radius);
    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;

    float grad_len = length(vec2(dFdx(d), dFdy(d)));
    d = d / max(grad_len, 0.001);
    float sdf_alpha = 1.0 - smoothstep(-0.59, 0.59, d);

    // The body runs in a nested block, as in msl.BuildCustom, so its
    // locals may reuse the names above (d, q, pos, radius, ...).
    vec4 _gg_color;
    {
    // --- user body ---
    `)
	b.WriteString(body)
	b.WriteString(`
    // --- end user body ---
    _gg_color = frag_color;
    }

    _frag_out = vec4(_gg_color.rgb, _gg_color.a * sdf_alpha);

    if (_frag_out.a < 0.0) {
        _frag_out += texture(tex, uv);
    }
}
`)
	return b.String()
}

// ShaderHash computes a cache key from the shader source.
// Both bodies are hashed, with a NUL separator between them, so
// two shaders that differ in either body get distinct keys on
// every platform. Params are uniforms, not source, and are not
// part of the key. A nil shader hashes to 0.
func ShaderHash(s *Shader) uint64 {
	if s == nil {
		return 0
	}
	h := Fnv64Str(Fnv64Offset, s.Metal)
	h = Fnv64Byte(h, 0)
	return Fnv64Str(h, s.GLSL)
}

// hashString computes a 64-bit FNV-1a hash.
func hashString(s string) uint64 {
	return Fnv64Str(Fnv64Offset, s)
}
