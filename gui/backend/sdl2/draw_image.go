//go:build !js && !darwin

package sdl2

import (
	"fmt"
	"log"
	"math"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
	"github.com/veandco/go-sdl2/sdl"
)

func newSDLTexCache(maxSize int) texcache.Cache[string, *sdl.Texture] {
	return texcache.New[string, *sdl.Texture](maxSize,
		func(tex *sdl.Texture) {
			if tex != nil {
				_ = tex.Destroy()
			}
		})
}

// loadTexture opens, validates, decodes, and uploads an image
// to an SDL texture.
func (b *Backend) loadTexture(path string) (*sdl.Texture, error) {
	f, err := imgload.OpenSafe(path, b.allowedImageRoots)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	nrgba, err := imgload.DecodeNRGBA(
		path, f, b.maxImageBytes, b.maxImagePixels)
	if err != nil {
		return nil, err
	}

	bounds := nrgba.Bounds()
	w := int32(bounds.Dx())
	h := int32(bounds.Dy())
	// Guard against empty/zero-sized decodes to avoid panics when
	// passing &nrgba.Pix[0] to SDL.
	if w <= 0 || h <= 0 || len(nrgba.Pix) == 0 {
		return nil, fmt.Errorf(
			"sdl2: decoded image has invalid dimensions (%dx%d) or empty pixel buffer (len=%d)",
			w, h, len(nrgba.Pix),
		)
	}

	// Go NRGBA stores [R,G,B,A] per pixel. On little-endian
	// this maps to SDL's PIXELFORMAT_ABGR8888.
	tex, err := b.renderer.CreateTexture(
		sdl.PIXELFORMAT_ABGR8888,
		sdl.TEXTUREACCESS_STATIC,
		w, h,
	)
	if err != nil {
		return nil, err
	}

	if err := tex.Update(
		nil, unsafe.Pointer(&nrgba.Pix[0]), nrgba.Stride,
	); err != nil {
		_ = tex.Destroy()
		return nil, err
	}
	_ = tex.SetBlendMode(sdl.BLENDMODE_BLEND)
	return tex, nil
}

// drawImage renders an image from a file path, using a
// texture cache.
func (b *Backend) drawImage(r *gui.RenderCmd) {
	path, ok := b.imagePathCache.Get(r.Resource)
	if !ok {
		var err error
		path, err = imgload.ResolveValidatedPath(
			r.Resource, b.allowedImageRoots)
		if err != nil {
			log.Printf("sdl2: drawImage: %s: %v",
				r.Resource, err)
			path = "-"
		}
		b.imagePathCache.Set(r.Resource, path)
	}
	if path == "-" {
		return
	}

	tex, ok := b.texCache.Get(path)
	if !ok {
		var err error
		tex, err = b.loadTexture(path)
		if err != nil {
			log.Printf("sdl2: drawImage: %v", err)
		}
		b.texCache.Set(path, tex)
	}
	if tex == nil {
		return
	}

	s := b.dpiScale
	dst := sdl.Rect{
		X: int32(r.X * s),
		Y: int32(r.Y * s),
		W: int32(r.W * s),
		H: int32(r.H * s),
	}

	// Fill background when BgColor has alpha (e.g. mermaid/math PNGs).
	if r.Color.A > 0 {
		_ = b.renderer.SetDrawColor(
			r.Color.R, r.Color.G, r.Color.B, r.Color.A)
		_ = b.renderer.FillRect(&dst)
	}

	if r.ClipRadius > 0 {
		dst := sdl.FRect{
			X: r.X * s,
			Y: r.Y * s,
			W: r.W * s,
			H: r.H * s,
		}
		b.drawTextureRoundedRegion(
			tex, dst, sdl.FRect{X: 0, Y: 0, W: 1, H: 1},
			r.ClipRadius*s,
		)
		return
	}
	_ = b.renderer.Copy(tex, nil, &dst)
}

func textureRegionUV(
	tex *sdl.Texture, dst sdl.FRect,
) (sdl.FRect, bool) {
	_, _, texW, texH, err := tex.Query()
	if err != nil || texW <= 0 || texH <= 0 ||
		dst.W <= 0 || dst.H <= 0 {
		return sdl.FRect{}, false
	}
	return sdl.FRect{
		X: dst.X / float32(texW),
		Y: dst.Y / float32(texH),
		W: dst.W / float32(texW),
		H: dst.H / float32(texH),
	}, true
}

// drawTextureRoundedRegion renders a textured rounded rectangle
// using normalized source UVs.
func (b *Backend) drawTextureRoundedRegion(
	tex *sdl.Texture, dst sdl.FRect, src sdl.FRect, radius float32,
) {
	x := dst.X
	y := dst.Y
	w := dst.W
	h := dst.H
	rad := min(radius, w/2, h/2)

	white := sdl.Color{R: 255, G: 255, B: 255, A: 255}
	vert := func(px, py float32) sdl.Vertex {
		return sdl.Vertex{
			Position: sdl.FPoint{X: px, Y: py},
			Color:    white,
			TexCoord: sdl.FPoint{
				X: src.X + ((px-x)/w)*src.W,
				Y: src.Y + ((py-y)/h)*src.H,
			},
		}
	}

	const segments = 8
	// 3 quads (6 verts each) + 4 corners (segments*3 verts each).
	numVerts := 18 + 4*segments*3
	if cap(b.svgVerts) < numVerts {
		b.svgVerts = make([]sdl.Vertex, 0, numVerts)
	}
	verts := b.svgVerts[:0]

	quad := func(x0, y0, x1, y1 float32) {
		verts = append(verts,
			vert(x0, y0), vert(x1, y0), vert(x1, y1),
			vert(x0, y0), vert(x1, y1), vert(x0, y1),
		)
	}

	// Center strip.
	quad(x+rad, y, x+w-rad, y+h)
	// Left strip.
	quad(x, y+rad, x+rad, y+h-rad)
	// Right strip.
	quad(x+w-rad, y+rad, x+w, y+h-rad)

	// Corner fans. Start angles (screen coords, Y down):
	// top-left=π, top-right=3π/2, bottom-right=0, bottom-left=π/2.
	type corner struct {
		cx, cy float32
		start  float64
	}
	corners := [4]corner{
		{x + rad, y + rad, math.Pi},
		{x + w - rad, y + rad, 3 * math.Pi / 2},
		{x + w - rad, y + h - rad, 0},
		{x + rad, y + h - rad, math.Pi / 2},
	}
	step := (math.Pi / 2) / float64(segments)
	for _, c := range corners {
		for j := range segments {
			a0 := c.start + float64(j)*step
			a1 := c.start + float64(j+1)*step
			verts = append(verts,
				vert(c.cx, c.cy),
				vert(c.cx+rad*float32(math.Cos(a0)),
					c.cy+rad*float32(math.Sin(a0))),
				vert(c.cx+rad*float32(math.Cos(a1)),
					c.cy+rad*float32(math.Sin(a1))),
			)
		}
	}

	_ = b.renderer.RenderGeometry(tex, verts, nil)
	b.svgVerts = verts[:0]
}
