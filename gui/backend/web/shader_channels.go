//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// webChannels holds a surface's planes as the textures a custom shader
// reads as iChannel0..3 (gui.Shader.Surface), in the shader renderer's
// own WebGL2 context. buf keeps one JS byte array per plane, so a new
// frame is one copy into it instead of a new array.
type webChannels struct {
	tex  [4]js.Value
	buf  [4]js.Value
	w, h [4]int
	fmt  [4]gui.PixelFormat
	n    int
}

// planeFormat maps a plane format to its WebGL2 internal format and the
// format of its pixel data.
func (r *customShaderRenderer) planeFormat(f gui.PixelFormat) (internal, format js.Value) {
	gl := r.gl
	switch f {
	case gui.PixelFormatR8:
		return gl.Get("R8"), gl.Get("RED")
	case gui.PixelFormatRG8:
		return gl.Get("RG8"), gl.Get("RG")
	}
	return gl.Get("RGBA8"), gl.Get("RGBA")
}

// uploadChannels puts a frame's planes into c, reusing each texture while
// its plane keeps its format and size. Textures sample linearly and clamp
// to the edge, the contract of gui.Shader.Surface.
func (r *customShaderRenderer) uploadChannels(c webChannels, planes []gui.SurfacePlane) webChannels {
	gl := r.gl
	target := gl.Get("TEXTURE_2D")
	// R8 and RG8 rows need not be 4-byte aligned.
	gl.Call("pixelStorei", gl.Get("UNPACK_ALIGNMENT"), 1)
	for i, p := range planes {
		if c.buf[i].IsUndefined() || c.buf[i].Get("length").Int() != len(p.Pix) {
			c.buf[i] = js.Global().Get("Uint8Array").New(len(p.Pix))
		}
		js.CopyBytesToJS(c.buf[i], p.Pix)
		internal, format := r.planeFormat(p.Format)
		if !validJS(c.tex[i]) || c.w[i] != p.Width || c.h[i] != p.Height || c.fmt[i] != p.Format {
			if validJS(c.tex[i]) {
				gl.Call("deleteTexture", c.tex[i])
			}
			tex := gl.Call("createTexture")
			gl.Call("bindTexture", target, tex)
			gl.Call("texParameteri", target, gl.Get("TEXTURE_MIN_FILTER"), gl.Get("LINEAR"))
			gl.Call("texParameteri", target, gl.Get("TEXTURE_MAG_FILTER"), gl.Get("LINEAR"))
			gl.Call("texParameteri", target, gl.Get("TEXTURE_WRAP_S"), gl.Get("CLAMP_TO_EDGE"))
			gl.Call("texParameteri", target, gl.Get("TEXTURE_WRAP_T"), gl.Get("CLAMP_TO_EDGE"))
			gl.Call("texImage2D", target, 0, internal, p.Width, p.Height, 0,
				format, gl.Get("UNSIGNED_BYTE"), c.buf[i])
			c.tex[i], c.w[i], c.h[i], c.fmt[i] = tex, p.Width, p.Height, p.Format
			continue
		}
		gl.Call("bindTexture", target, c.tex[i])
		gl.Call("texSubImage2D", target, 0, 0, 0, p.Width, p.Height,
			format, gl.Get("UNSIGNED_BYTE"), c.buf[i])
	}
	for i := len(planes); i < c.n; i++ {
		if validJS(c.tex[i]) {
			gl.Call("deleteTexture", c.tex[i])
		}
		c.tex[i], c.buf[i] = js.Value{}, js.Value{}
	}
	c.n = len(planes)
	gl.Call("pixelStorei", gl.Get("UNPACK_ALIGNMENT"), 4)
	gl.Call("bindTexture", target, js.Null())
	return c
}

// deleteChannels releases a surface's plane textures.
func (r *customShaderRenderer) deleteChannels(c webChannels) {
	for i := range c.n {
		if validJS(c.tex[i]) && !r.lost {
			r.gl.Call("deleteTexture", c.tex[i])
		}
	}
}

// bindChannels binds plane i to texture unit i+1, and the transparent
// texture to every unit without a plane (or all of them when ok is false:
// a closed surface). Unit 0 is left active.
func (r *customShaderRenderer) bindChannels(c webChannels, ok bool) {
	gl := r.gl
	for i := range 4 {
		gl.Call("activeTexture", gl.Get("TEXTURE0").Int()+i+1)
		tex := r.transparentTex
		if ok && i < c.n {
			tex = c.tex[i]
		}
		gl.Call("bindTexture", gl.Get("TEXTURE_2D"), tex)
	}
	gl.Call("activeTexture", gl.Get("TEXTURE0"))
}

// setChannelUnits points a channel program's iChannel samplers at units
// 1..4. A sampler's unit is program state, so this runs once per build.
func (r *customShaderRenderer) setChannelUnits(program js.Value) {
	gl := r.gl
	gl.Call("useProgram", program)
	for i, name := range [4]string{"iChannel0", "iChannel1", "iChannel2", "iChannel3"} {
		if loc := gl.Call("getUniformLocation", program, name); !loc.IsNull() {
			gl.Call("uniform1i", loc, i+1)
		}
	}
}

// validJS reports whether v holds a JS object rather than undefined or null.
func validJS(v js.Value) bool {
	return !v.IsUndefined() && !v.IsNull()
}
