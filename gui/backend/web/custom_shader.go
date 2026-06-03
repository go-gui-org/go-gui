//go:build js && wasm

package web

import (
	"log"
	"math"
	"strings"
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

const maxCustomShaderPrograms = 32

type customShaderRenderer struct {
	canvas   js.Value
	gl       js.Value
	vbo      js.Value
	ibo      js.Value
	whiteTex js.Value
	lost     bool
	canvasW  int
	canvasH  int

	mvpBuf    js.Value
	tmBuf     js.Value
	vertexBuf js.Value

	programs texcache.Cache[uint64, *customShaderProgram]
}

type customShaderProgram struct {
	program js.Value
	uMVP    js.Value
	uTM     js.Value
	uTex    js.Value
}

func newCustomShaderRenderer(
	doc js.Value, callbacks *[]js.Func,
) *customShaderRenderer {
	canvas := doc.Call("createElement", "canvas")
	attrs := js.Global().Get("Object").New()
	attrs.Set("alpha", true)
	attrs.Set("antialias", false)
	attrs.Set("depth", false)
	attrs.Set("stencil", false)
	attrs.Set("preserveDrawingBuffer", true)

	gl := canvas.Call("getContext", "webgl2", attrs)
	if gl.IsNull() || gl.IsUndefined() {
		log.Printf("web: webgl2 unavailable; custom shaders use solid fallback")
		return nil
	}

	r := &customShaderRenderer{
		canvas:    canvas,
		gl:        gl,
		mvpBuf:    js.Global().Get("Float32Array").New(16),
		tmBuf:     js.Global().Get("Float32Array").New(16),
		vertexBuf: js.Global().Get("Float32Array").New(4 * 9),
	}
	r.resetProgramCache()
	r.registerContextCallbacks(callbacks)
	if !r.initResources() {
		log.Printf("web: custom shader init failed; using solid fallback")
		return nil
	}
	return r
}

func (b *Backend) drawCustomShader(r *gui.RenderCmd) {
	if r.Shader == nil || r.Shader.GLSL == "" {
		b.drawCustomShaderFallback(r)
		return
	}
	if b.shaders == nil || !b.shaders.draw(r, b.ctx2d, b.dpiScale) {
		b.drawCustomShaderFallback(r)
	}
}

func (b *Backend) drawCustomShaderFallback(r *gui.RenderCmd) {
	fill := r.Color
	if fill == (gui.Color{}) {
		fill = gui.CurrentTheme().ColorActive
	}
	fallback := *r
	fallback.Fill = true
	fallback.Color = fill
	b.drawRect(&fallback)
}

func (r *customShaderRenderer) draw(
	cmd *gui.RenderCmd, ctx2d js.Value, dpiScale float32,
) bool {
	if cmd.W <= 0 || cmd.H <= 0 {
		return true
	}
	if r.lost || r.gl.Call("isContextLost").Bool() {
		return false
	}

	p, ok := r.getProgram(cmd.Shader)
	if !ok || p == nil {
		return false
	}

	physW := max(1, int(math.Ceil(float64(cmd.W*dpiScale))))
	physH := max(1, int(math.Ceil(float64(cmd.H*dpiScale))))
	r.ensureCanvasSize(physW, physH)

	gl := r.gl
	gl.Call("viewport", 0, 0, physW, physH)
	gl.Call("disable", gl.Get("DEPTH_TEST"))
	gl.Call("disable", gl.Get("CULL_FACE"))
	gl.Call("enable", gl.Get("BLEND"))
	gl.Call("blendFunc", gl.Get("SRC_ALPHA"), gl.Get("ONE_MINUS_SRC_ALPHA"))
	gl.Call("clearColor", 0, 0, 0, 0)
	gl.Call("clear", gl.Get("COLOR_BUFFER_BIT"))
	gl.Call("useProgram", p.program)

	r.setOrtho(float32(physW), float32(physH))
	gl.Call("uniformMatrix4fv", p.uMVP, false, r.mvpBuf)

	for i := range 16 {
		v := float32(0)
		if i < len(cmd.Shader.Params) {
			v = cmd.Shader.Params[i]
		}
		r.tmBuf.SetIndex(i, v)
	}
	gl.Call("uniformMatrix4fv", p.uTM, false, r.tmBuf)

	gl.Call("activeTexture", gl.Get("TEXTURE0"))
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), r.whiteTex)
	gl.Call("uniform1i", p.uTex, 0)

	gl.Call("bindBuffer", gl.Get("ARRAY_BUFFER"), r.vbo)
	gl.Call("bindBuffer", gl.Get("ELEMENT_ARRAY_BUFFER"), r.ibo)
	gl.Call("enableVertexAttribArray", 0)
	gl.Call("vertexAttribPointer", 0, 3, gl.Get("FLOAT"), false, 9*4, 0)
	gl.Call("enableVertexAttribArray", 1)
	gl.Call("vertexAttribPointer", 1, 2, gl.Get("FLOAT"), false, 9*4, 3*4)
	gl.Call("enableVertexAttribArray", 2)
	gl.Call("vertexAttribPointer", 2, 4, gl.Get("FLOAT"), false, 9*4, 5*4)

	r.setQuad(float32(physW), float32(physH), cmd.Radius*dpiScale, cmd.Color)
	gl.Call("bufferSubData", gl.Get("ARRAY_BUFFER"), 0, r.vertexBuf)
	gl.Call("drawElements", gl.Get("TRIANGLES"), 6, gl.Get("UNSIGNED_SHORT"), 0)

	ctx2d.Call("drawImage",
		r.canvas,
		float64(cmd.X), float64(cmd.Y),
		float64(cmd.W), float64(cmd.H),
	)
	return true
}

func (r *customShaderRenderer) getProgram(
	s *gui.Shader,
) (*customShaderProgram, bool) {
	h := gui.ShaderHash(s)
	if p, ok := r.programs.Get(h); ok {
		return p, p != nil
	}

	p, err := r.buildProgram(
		webGL2VertexSource(),
		webGL2FragmentSource(s.GLSL),
	)
	if err != nil {
		log.Printf("web: custom shader compile: %v", err)
		r.programs.Set(h, nil)
		return nil, false
	}
	r.programs.Set(h, p)
	return p, true
}

func (r *customShaderRenderer) buildProgram(
	vsSrc, fsSrc string,
) (*customShaderProgram, error) {
	gl := r.gl
	vs, err := r.compileShader(vsSrc, gl.Get("VERTEX_SHADER"))
	if err != nil {
		return nil, err
	}
	defer gl.Call("deleteShader", vs)

	fs, err := r.compileShader(fsSrc, gl.Get("FRAGMENT_SHADER"))
	if err != nil {
		return nil, err
	}
	defer gl.Call("deleteShader", fs)

	program := gl.Call("createProgram")
	gl.Call("attachShader", program, vs)
	gl.Call("attachShader", program, fs)
	gl.Call("linkProgram", program)
	if !gl.Call("getProgramParameter", program, gl.Get("LINK_STATUS")).Bool() {
		msg := strings.TrimSpace(gl.Call("getProgramInfoLog", program).String())
		gl.Call("deleteProgram", program)
		if msg == "" {
			msg = "link failed"
		}
		return nil, errString(msg)
	}

	return &customShaderProgram{
		program: program,
		uMVP:    gl.Call("getUniformLocation", program, "mvp"),
		uTM:     gl.Call("getUniformLocation", program, "tm"),
		uTex:    gl.Call("getUniformLocation", program, "tex"),
	}, nil
}

func (r *customShaderRenderer) compileShader(
	src string, shaderType js.Value,
) (js.Value, error) {
	gl := r.gl
	shader := gl.Call("createShader", shaderType)
	gl.Call("shaderSource", shader, src)
	gl.Call("compileShader", shader)
	if gl.Call("getShaderParameter", shader, gl.Get("COMPILE_STATUS")).Bool() {
		return shader, nil
	}
	msg := strings.TrimSpace(gl.Call("getShaderInfoLog", shader).String())
	gl.Call("deleteShader", shader)
	if msg == "" {
		msg = "compile failed"
	}
	return js.Value{}, errString(msg)
}

func (r *customShaderRenderer) resetProgramCache() {
	r.programs = texcache.New[uint64, *customShaderProgram](
		maxCustomShaderPrograms,
		func(p *customShaderProgram) {
			if p != nil {
				r.gl.Call("deleteProgram", p.program)
			}
		},
	)
}

func (r *customShaderRenderer) initResources() bool {
	if r.gl.IsNull() || r.gl.IsUndefined() ||
		r.gl.Call("isContextLost").Bool() {
		return false
	}

	gl := r.gl
	r.vbo = gl.Call("createBuffer")
	r.ibo = gl.Call("createBuffer")
	r.whiteTex = gl.Call("createTexture")
	if r.vbo.IsNull() || r.vbo.IsUndefined() ||
		r.ibo.IsNull() || r.ibo.IsUndefined() ||
		r.whiteTex.IsNull() || r.whiteTex.IsUndefined() {
		return false
	}

	gl.Call("bindBuffer", gl.Get("ARRAY_BUFFER"), r.vbo)
	gl.Call("bufferData", gl.Get("ARRAY_BUFFER"), 4*9*4, gl.Get("DYNAMIC_DRAW"))

	indices := js.Global().Get("Uint16Array").New(6)
	for i, v := range []uint16{0, 1, 2, 0, 2, 3} {
		indices.SetIndex(i, v)
	}
	gl.Call("bindBuffer", gl.Get("ELEMENT_ARRAY_BUFFER"), r.ibo)
	gl.Call("bufferData", gl.Get("ELEMENT_ARRAY_BUFFER"), indices, gl.Get("STATIC_DRAW"))

	white := js.Global().Get("Uint8Array").New(4)
	for i, v := range []uint8{255, 255, 255, 255} {
		white.SetIndex(i, v)
	}
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), r.whiteTex)
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MIN_FILTER"), gl.Get("NEAREST"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MAG_FILTER"), gl.Get("NEAREST"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_WRAP_S"), gl.Get("CLAMP_TO_EDGE"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_WRAP_T"), gl.Get("CLAMP_TO_EDGE"))
	gl.Call("texImage2D",
		gl.Get("TEXTURE_2D"), 0, gl.Get("RGBA"),
		1, 1, 0, gl.Get("RGBA"), gl.Get("UNSIGNED_BYTE"), white)

	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), js.Null())
	gl.Call("bindBuffer", gl.Get("ARRAY_BUFFER"), js.Null())
	gl.Call("bindBuffer", gl.Get("ELEMENT_ARRAY_BUFFER"), js.Null())
	return true
}

func (r *customShaderRenderer) ensureCanvasSize(w, h int) {
	if r.canvasW == w && r.canvasH == h {
		return
	}
	r.canvas.Set("width", w)
	r.canvas.Set("height", h)
	r.canvasW = w
	r.canvasH = h
}

func (r *customShaderRenderer) registerContextCallbacks(
	callbacks *[]js.Func,
) {
	lostFn := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			args[0].Call("preventDefault")
		}
		r.handleContextLost()
		return nil
	})
	restoredFn := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		r.handleContextRestored()
		return nil
	})
	*callbacks = append(*callbacks, lostFn, restoredFn)
	r.canvas.Call("addEventListener", "webglcontextlost", lostFn)
	r.canvas.Call("addEventListener", "webglcontextrestored", restoredFn)
}

func (r *customShaderRenderer) handleContextLost() {
	r.lost = true
	r.canvasW = 0
	r.canvasH = 0
	r.vbo = js.Null()
	r.ibo = js.Null()
	r.whiteTex = js.Null()
	r.resetProgramCache()
}

func (r *customShaderRenderer) handleContextRestored() {
	r.lost = false
	r.resetProgramCache()
	if !r.initResources() {
		log.Printf("web: custom shader restore failed")
		r.lost = true
	}
}

func (r *customShaderRenderer) setOrtho(w, h float32) {
	r.mvpBuf.SetIndex(0, 2/w)
	r.mvpBuf.SetIndex(1, 0)
	r.mvpBuf.SetIndex(2, 0)
	r.mvpBuf.SetIndex(3, 0)

	r.mvpBuf.SetIndex(4, 0)
	r.mvpBuf.SetIndex(5, -2/h)
	r.mvpBuf.SetIndex(6, 0)
	r.mvpBuf.SetIndex(7, 0)

	r.mvpBuf.SetIndex(8, 0)
	r.mvpBuf.SetIndex(9, 0)
	r.mvpBuf.SetIndex(10, -1)
	r.mvpBuf.SetIndex(11, 0)

	r.mvpBuf.SetIndex(12, -1)
	r.mvpBuf.SetIndex(13, 1)
	r.mvpBuf.SetIndex(14, 0)
	r.mvpBuf.SetIndex(15, 1)
}

func (r *customShaderRenderer) setQuad(
	w, h, radius float32, c gui.Color,
) {
	z := packShaderParams(radius, 0)
	col := shaderColor(c)
	verts := [4][9]float32{
		{0, 0, z, -1, -1, col[0], col[1], col[2], col[3]},
		{w, 0, z, 1, -1, col[0], col[1], col[2], col[3]},
		{w, h, z, 1, 1, col[0], col[1], col[2], col[3]},
		{0, h, z, -1, 1, col[0], col[1], col[2], col[3]},
	}
	idx := 0
	for _, v := range verts {
		for _, f := range v {
			r.vertexBuf.SetIndex(idx, f)
			idx++
		}
	}
}

func webGL2VertexSource() string {
	return `#version 300 es
precision highp float;
precision highp int;

layout(location=0) in vec3 position;
layout(location=1) in vec2 texcoord0;
layout(location=2) in vec4 color0;

uniform mat4 mvp;
uniform mat4 tm;

out vec2 uv;
out vec4 color;
out float params;
out vec4 p0;
out vec4 p1;
out vec4 p2;
out vec4 p3;

void main() {
    gl_Position = mvp * vec4(position.xy, 0.0, 1.0);
    uv = texcoord0;
    color = color0;
    params = position.z;
    p0 = tm[0];
    p1 = tm[1];
    p2 = tm[2];
    p3 = tm[3];
}
`
}

func webGL2FragmentSource(body string) string {
	return `#version 300 es
precision highp float;
precision highp int;

uniform sampler2D tex;
in vec2 uv;
in vec4 color;
in float params;
in vec4 p0;
in vec4 p1;
in vec4 p2;
in vec4 p3;

out vec4 outColor;

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

    ` + body + `

    outColor = vec4(frag_color.rgb, frag_color.a * sdf_alpha);
    if (outColor.a < 0.0) {
        outColor += texture(tex, uv);
    }
}
`
}

func packShaderParams(radius, thickness float32) float32 {
	r := float32(math.Floor(float64(radius)*4)) * 4096
	t := float32(math.Floor(float64(thickness) * 4))
	return r + t
}

func shaderColor(c gui.Color) [4]float32 {
	return [4]float32{
		float32(c.R) / 255,
		float32(c.G) / 255,
		float32(c.B) / 255,
		float32(c.A) / 255,
	}
}

type errString string

func (e errString) Error() string { return string(e) }
