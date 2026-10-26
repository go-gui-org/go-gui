//go:build !js && !darwin && !android

package gl

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
)

// glChannels holds a surface's planes as the textures a custom shader
// reads as iChannel0..3 (gui.Shader.Surface).
type glChannels struct {
	tex [4]glTexture
	fmt [4]gui.PixelFormat
	n   int
}

// glPlaneFormat maps a plane format to its GL internal format and the
// format of its pixel data.
func glPlaneFormat(f gui.PixelFormat) (internal int32, format uint32) {
	switch f {
	case gui.PixelFormatR8:
		return gogl.R8, gogl.RED
	case gui.PixelFormatRG8:
		return gogl.RG8, gogl.RG
	}
	return gogl.RGBA8, gogl.RGBA
}

// createPlaneTexture creates a texture for one plane. Like every texture
// here it samples linearly and clamps to the edge, the contract of
// gui.Shader.Surface.
func createPlaneTexture(f gui.PixelFormat, w, h int32, pix []byte) glTexture {
	internal, format := glPlaneFormat(f)
	var id uint32
	gogl.GenTextures(1, &id)
	gogl.BindTexture(gogl.TEXTURE_2D, id)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_MIN_FILTER, gogl.LINEAR)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_MAG_FILTER, gogl.LINEAR)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_WRAP_S, gogl.CLAMP_TO_EDGE)
	gogl.TexParameteri(gogl.TEXTURE_2D, gogl.TEXTURE_WRAP_T, gogl.CLAMP_TO_EDGE)
	// R8 and RG8 rows need not be 4-byte aligned; the default unpack
	// alignment would read past each row.
	gogl.PixelStorei(gogl.UNPACK_ALIGNMENT, 1)
	gogl.TexImage2D(gogl.TEXTURE_2D, 0, internal, w, h, 0, format,
		gogl.UNSIGNED_BYTE, unsafe.Pointer(&pix[0]))
	gogl.PixelStorei(gogl.UNPACK_ALIGNMENT, 4)
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
	return glTexture{id: id, w: w, h: h}
}

// uploadChannels puts a frame's planes into c, reusing each texture while
// its plane keeps its format and size.
func (b *Backend) uploadChannels(c glChannels, planes []gui.SurfacePlane) glChannels {
	for i, p := range planes {
		t := c.tex[i]
		w, h := int32(p.Width), int32(p.Height)
		if t.id == 0 || t.w != w || t.h != h || c.fmt[i] != p.Format {
			b.deleteSurfaceTexture(t)
			c.tex[i] = createPlaneTexture(p.Format, w, h, p.Pix)
			c.fmt[i] = p.Format
			continue
		}
		_, format := glPlaneFormat(p.Format)
		gogl.BindTexture(gogl.TEXTURE_2D, t.id)
		gogl.PixelStorei(gogl.UNPACK_ALIGNMENT, 1)
		gogl.TexSubImage2D(gogl.TEXTURE_2D, 0, 0, 0, w, h, format,
			gogl.UNSIGNED_BYTE, unsafe.Pointer(&p.Pix[0]))
		gogl.PixelStorei(gogl.UNPACK_ALIGNMENT, 4)
		gogl.BindTexture(gogl.TEXTURE_2D, 0)
	}
	for i := len(planes); i < c.n; i++ {
		b.deleteSurfaceTexture(c.tex[i])
		c.tex[i] = glTexture{}
	}
	c.n = len(planes)
	return c
}

// deleteChannels releases a surface's plane textures.
func (b *Backend) deleteChannels(c glChannels) {
	for i := range c.n {
		b.deleteSurfaceTexture(c.tex[i])
	}
}

// bindChannels binds plane i to texture unit i+1, and the transparent
// texture to every unit without a plane (or all of them when ok is false:
// a closed surface). Unit 0 is left active, as the other draws expect.
func (b *Backend) bindChannels(c glChannels, ok bool) {
	for i := range 4 {
		gogl.ActiveTexture(gogl.TEXTURE0 + uint32(i+1))
		id := b.transparentTex.id
		if ok && i < c.n {
			id = c.tex[i].id
		}
		gogl.BindTexture(gogl.TEXTURE_2D, id)
	}
	gogl.ActiveTexture(gogl.TEXTURE0)
}

// setChannelUnits points a channel shader's iChannel samplers at units
// 1..4. A sampler's unit is program state, so this runs once per build.
func setChannelUnits(p pipeline) {
	gogl.UseProgram(p.program)
	for i, name := range [4]string{"iChannel0\x00", "iChannel1\x00", "iChannel2\x00", "iChannel3\x00"} {
		if loc := gogl.GetUniformLocation(p.program, glStr(name)); loc >= 0 {
			gogl.Uniform1i(loc, int32(i+1))
		}
	}
}
