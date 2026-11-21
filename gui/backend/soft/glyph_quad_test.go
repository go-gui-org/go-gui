package soft

import (
	"bytes"
	"fmt"
	"image"
	"math/rand/v2"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// newQuadTestBackend returns a backend over a w x h buffer with a
// non-empty background, and a tw x th texture of fixed pseudo-random
// straight RGBA, a quarter of it fully transparent.
func newQuadTestBackend(w, h, tw, th int) (*glyphBackend, glyph.TextureID) {
	rng := rand.New(rand.NewPCG(1, 2))
	gb := newGlyphBackend(1)
	gb.buf = newBuffer(w, h)
	for i := range gb.buf.img.Pix {
		gb.buf.img.Pix[i] = uint8(rng.UintN(256))
	}
	id := gb.NewTexture(tw, th)
	pix := gb.textures[id].pix
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2] = uint8(rng.UintN(256)), uint8(rng.UintN(256)), uint8(rng.UintN(256))
		if rng.UintN(4) != 0 {
			pix[i+3] = uint8(rng.UintN(256))
		}
	}
	return gb, id
}

// perPixelQuad is DrawTexturedQuad as it was before it blended by row
// (#1327): one blendTexel call per device pixel.
func perPixelQuad(gb *glyphBackend, id glyph.TextureID, src, dst glyph.Rect, c glyph.Color) {
	tex := gb.textures[id]
	s := gb.scale
	x0, y0 := dst.X*s, dst.Y*s
	x1, y1 := (dst.X+dst.Width)*s, (dst.Y+dst.Height)*s
	region := gb.buf.region(x0, y0, x1-x0, y1-y0)
	invW := 1 / (x1 - x0)
	invH := 1 / (y1 - y0)
	for py := region.Min.Y; py < region.Max.Y; py++ {
		v := (float32(py) + 0.5 - y0) * invH
		sy := int(src.Y + v*src.Height)
		for px := region.Min.X; px < region.Max.X; px++ {
			u := (float32(px) + 0.5 - x0) * invW
			sx := int(src.X + u*src.Width)
			gb.blendTexel(tex, sx, sy, px, py, c)
		}
	}
}

// TestDrawTexturedQuadMatchesPerPixelBlend: the row loop must write the
// bytes the per-pixel path wrote. The cases are the ones where the two
// could part: a quad cut by the buffer edge and by a clip rect, a source
// rect that reaches outside the atlas, a translucent tint, a scaled quad
// and a fractional position.
func TestDrawTexturedQuadMatchesPerPixelBlend(t *testing.T) {
	opaque := glyph.Color{R: 255, G: 255, B: 255, A: 255}
	tinted := glyph.Color{R: 200, G: 90, B: 30, A: 140}
	cases := []struct {
		name     string
		src, dst glyph.Rect
		c        glyph.Color
		clip     image.Rectangle
	}{
		{"one to one", glyph.Rect{X: 2, Y: 3, Width: 12, Height: 9},
			glyph.Rect{X: 5, Y: 4, Width: 12, Height: 9}, opaque, image.Rectangle{}},
		{"tinted", glyph.Rect{X: 0, Y: 0, Width: 16, Height: 16},
			glyph.Rect{X: 3, Y: 2, Width: 16, Height: 16}, tinted, image.Rectangle{}},
		{"cut by the buffer edge", glyph.Rect{X: 0, Y: 0, Width: 16, Height: 16},
			glyph.Rect{X: -6, Y: -5, Width: 16, Height: 16}, tinted, image.Rectangle{}},
		{"cut by the far edge", glyph.Rect{X: 0, Y: 0, Width: 16, Height: 16},
			glyph.Rect{X: 30, Y: 14, Width: 16, Height: 16}, opaque, image.Rectangle{}},
		{"clipped", glyph.Rect{X: 0, Y: 0, Width: 16, Height: 16},
			glyph.Rect{X: 4, Y: 4, Width: 16, Height: 16}, tinted, image.Rect(9, 7, 15, 12)},
		{"source outside the atlas", glyph.Rect{X: -4, Y: -3, Width: 26, Height: 24},
			glyph.Rect{X: 2, Y: 1, Width: 26, Height: 22}, tinted, image.Rectangle{}},
		{"scaled up", glyph.Rect{X: 1, Y: 1, Width: 8, Height: 6},
			glyph.Rect{X: 2, Y: 2, Width: 30, Height: 19}, tinted, image.Rectangle{}},
		{"fractional position", glyph.Rect{X: 0.5, Y: 0.25, Width: 10.5, Height: 9.75},
			glyph.Rect{X: 3.3, Y: 2.7, Width: 11.4, Height: 8.2}, tinted, image.Rectangle{}},
	}
	for _, tc := range cases {
		got, id := newQuadTestBackend(40, 24, 16, 16)
		want, _ := newQuadTestBackend(40, 24, 16, 16)
		if !tc.clip.Empty() {
			got.buf.clip, want.buf.clip = tc.clip, tc.clip
		}
		before := bytes.Clone(got.buf.img.Pix)
		got.DrawTexturedQuad(id, tc.src, tc.dst, tc.c)
		perPixelQuad(want, id, tc.src, tc.dst, tc.c)
		if bytes.Equal(got.buf.img.Pix, before) {
			t.Errorf("%s: the quad drew nothing; the case compares nothing", tc.name)
		}
		if !bytes.Equal(got.buf.img.Pix, want.buf.img.Pix) {
			for i := range got.buf.img.Pix {
				if got.buf.img.Pix[i] != want.buf.img.Pix[i] {
					t.Errorf("%s: pixel (%d,%d) channel %d = %d, per-pixel path wrote %d",
						tc.name, i/4%40, i/4/40, i%4, got.buf.img.Pix[i], want.buf.img.Pix[i])
					break
				}
			}
		}
	}
}

// BenchmarkDrawTexturedQuad times the glyph blit alone, on 16x32 px quads:
// a line of text is one quad per glyph.
func BenchmarkDrawTexturedQuad(b *testing.B) {
	for _, n := range []int{1, 200} {
		b.Run(fmt.Sprintf("%dquads", n), func(b *testing.B) {
			gb, id := newQuadTestBackend(1920, 1080, 64, 64)
			src := glyph.Rect{X: 8, Y: 8, Width: 16, Height: 32}
			c := glyph.Color{R: 230, G: 230, B: 230, A: 255}
			b.ReportAllocs()
			for b.Loop() {
				for i := range n {
					gb.DrawTexturedQuad(id, src,
						glyph.Rect{X: float32(8 + i%100*18), Y: float32(8 + i/100*40), Width: 16, Height: 32}, c)
				}
			}
		})
	}
}
