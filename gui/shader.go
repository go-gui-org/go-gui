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
	// MetalDecls and GLSLDecls hold program-scope declarations:
	// helper functions, structs and constants that a body calls but
	// cannot define, because a body runs inside main. Each is
	// spliced before main, after the wrapper's own declarations.
	// In GLSL a helper can read the built-ins (iTime, ...), which
	// are globals there. In MSL they are locals of the fragment
	// function, so a helper must take them as parameters.
	// cmd/shaderport fills both from a Shadertoy shader.
	MetalDecls string
	GLSLDecls  string
	// Animate keeps frames coming while a container with this shader
	// is drawn, so iTime advances without app code. Each
	// frame rebuilds only the render commands: no view function runs
	// and nothing is re-arranged. Frames stop once no Animate shader
	// is drawn. For a user who asked for reduced motion the shader
	// holds still instead, as in a headless capture: iTime, iTimeDelta
	// and iFrame read zero and no frames are started. Leave it false
	// for a shader that does not change.
	Animate bool
	// Surface feeds the shader textures (issue #1168): plane i of the
	// surface is iChannel i, a sampler2D in GLSL (read it with
	// texture(iChannel0, p)) and a texture2d<float> in MSL (read it with
	// iChannel0.sample(iChannelSampler, p)). iChannelResolution[i] holds
	// plane i's size in pixels, Shadertoy style, with z = 1. Sampling is
	// linear and clamps to the edge; p = uv * 0.5 + 0.5 puts (0,0) at the
	// plane's top-left pixel. A channel the surface has no plane for, or
	// any channel while the surface is closed, reads as a 1x1 transparent
	// texture. A new frame from the surface redraws the container
	// without Animate. Leave it nil for a shader that reads no texture:
	// only a shader with a Surface declares the iChannel names, so
	// existing shaders compile unchanged.
	//
	// The soft backend and PDF export run no shaders: nothing is drawn
	// in place of the shader fill, while the container's shadow, border
	// and children still draw (the web backend without WebGL2 draws the
	// fill color). Window.RendersShaders reports which case applies.
	Surface *PixelSurface
}

// BuildGLSLFragment wraps a user-supplied GLSL body with the
// standard preamble and epilogue for desktop GL 3.3.
func BuildGLSLFragment(body string) string {
	return buildGLSLCustomFragment("#version 330", "", body, false)
}

// BuildGLSLFragmentDecls is BuildGLSLFragment with program-scope
// declarations (Shader.GLSLDecls) spliced before main.
func BuildGLSLFragmentDecls(decls, body string) string {
	return buildGLSLCustomFragment("#version 330", decls, body, false)
}

// BuildGLSLFragmentChannels is BuildGLSLFragmentDecls for a shader with
// a Surface: it also declares iChannel0..3 and iChannelResolution.
func BuildGLSLFragmentChannels(decls, body string) string {
	return buildGLSLCustomFragment("#version 330", decls, body, true)
}

// BuildGLSLESFragment wraps a user-supplied GLSL body exactly
// like BuildGLSLFragment, but for WebGL2 and GLES
// (#version 300 es). A body that compiles under one compiles
// under the other; only the version header differs. The web and
// Android backends share this instead of carrying their own copy
// of the wrapper.
func BuildGLSLESFragment(body string) string {
	return BuildGLSLESFragmentDecls("", body)
}

// BuildGLSLESFragmentDecls is BuildGLSLESFragment with
// program-scope declarations (Shader.GLSLDecls) spliced before main.
func BuildGLSLESFragmentDecls(decls, body string) string {
	return buildGLSLCustomFragment(glslESHeader, decls, body, false)
}

// BuildGLSLESFragmentChannels is BuildGLSLESFragmentDecls for a shader
// with a Surface: it also declares iChannel0..3 and iChannelResolution.
func BuildGLSLESFragmentChannels(decls, body string) string {
	return buildGLSLCustomFragment(glslESHeader, decls, body, true)
}

// glslESHeader opens a WebGL2 / GLES 3.0 fragment shader.
const glslESHeader = "#version 300 es\n" +
	"precision highp float;\n" +
	"precision highp int;"

// buildGLSLCustomFragment is the single copy of the custom-shader
// GLSL wrapper. The body must declare a local vec4 frag_color;
// the wrapper adds the SDF round-rect clipping around it. decls
// goes at program scope, after the built-in globals so a helper
// can read them, and before main. channels adds the texture inputs of
// Shader.Surface; without it the source is exactly what it was before
// they existed.
func buildGLSLCustomFragment(header, decls, body string, channels bool) string {
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
`)
	if channels {
		b.WriteString(glslChannelDecls)
	}
	b.WriteString(decls)
	b.WriteString(`

void main() {
    iResolution = _gg_builtins[0].xyz;
    iTime       = _gg_builtins[0].w;
    iMouse      = _gg_builtins[1];
    iTimeDelta  = _gg_builtins[2].x;
    iFrame      = int(_gg_builtins[2].y);
`)
	if channels {
		b.WriteString(glslChannelInit)
	}
	b.WriteString(`
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
// Both bodies and both decls are hashed, with a NUL separator
// between fields, so two shaders that differ in any of them get
// distinct keys on every platform. Params are uniforms, not
// source, and are not part of the key. A nil shader hashes to 0.
// A shader with a Surface builds a different wrapper (with the
// iChannel inputs), so it hashes differently from the same source
// without one; which surface does not matter. The flag is mixed into
// the finished hash rather than hashed as one more byte: FNV-1a is
// byte-serial, so a byte after GLSLDecls could be reproduced by
// decls ending in that byte.
func ShaderHash(s *Shader) uint64 {
	if s == nil {
		return 0
	}
	h := Fnv64Str(Fnv64Offset, s.Metal)
	h = Fnv64Byte(h, 0)
	h = Fnv64Str(h, s.GLSL)
	h = Fnv64Byte(h, 0)
	h = Fnv64Str(h, s.MetalDecls)
	h = Fnv64Byte(h, 0)
	h = Fnv64Str(h, s.GLSLDecls)
	if s.Surface != nil {
		h ^= shaderChannelsSalt
	}
	return h
}

// shaderChannelsSalt marks the hash of a shader with a Surface. Any
// fixed nonzero value works; this one is the 64-bit golden ratio.
const shaderChannelsSalt = 0x9e3779b97f4a7c15

// hashString computes a 64-bit FNV-1a hash.
func hashString(s string) uint64 {
	return Fnv64Str(Fnv64Offset, s)
}

// glslChannelDecls declares the texture inputs of Shader.Surface at
// program scope, so a helper in GLSLDecls can read them too. The backend
// binds plane i to texture unit i+1 (unit 0 is tex).
const glslChannelDecls = `
// Texture inputs (#1168): plane i of Shader.Surface, or a 1x1
// transparent texture.
uniform sampler2D iChannel0;
uniform sampler2D iChannel1;
uniform sampler2D iChannel2;
uniform sampler2D iChannel3;
vec3 iChannelResolution[4];
`

// glslChannelInit fills iChannelResolution from the bound textures, so
// the sizes need no uniform of their own.
const glslChannelInit = `    iChannelResolution[0] = vec3(vec2(textureSize(iChannel0, 0)), 1.0);
    iChannelResolution[1] = vec3(vec2(textureSize(iChannel1, 0)), 1.0);
    iChannelResolution[2] = vec3(vec2(textureSize(iChannel2, 0)), 1.0);
    iChannelResolution[3] = vec3(vec2(textureSize(iChannel3, 0)), 1.0);
`

// RendersShaders reports whether the backend drawing w runs custom
// shaders (ContainerCfg.Shader): true on OpenGL, Metal, Android and iOS,
// and on the web when WebGL2 is available. It is false for the software
// renderer and headless tests, and a PDF export never runs shaders.
// Where it is false a shader is not drawn (soft and PDF draw nothing for
// it; the web backend without WebGL2 draws the fill), so a library
// that draws through a shader, such as a video view reading
// Shader.Surface, can show an RGBA8 Image instead. A lock-free read, safe
// from any goroutine.
func (w *Window) RendersShaders() bool {
	return w != nil && w.rendersShaders.Load()
}

// SetWindowRendersShaders records whether the backend drawing w runs
// custom shaders; see Window.RendersShaders. Intended for backend use:
// call it once the backend can compile and draw a shader. Safe for a nil
// window (no-op).
func SetWindowRendersShaders(w *Window, ok bool) {
	if w != nil {
		w.rendersShaders.Store(ok)
	}
}
