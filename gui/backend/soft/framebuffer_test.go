package soft

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math/rand/v2"
	"testing"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
)

// TestBufferClearFillsEveryPixel: clear stores one pixel and doubles it
// with copy, so the sizes here include ones whose last copy is a partial
// one. Every pixel must hold the premultiplied color, whatever the buffer
// held before, and the whole buffer must be marked written.
func TestBufferClearFillsEveryPixel(t *testing.T) {
	sizes := []image.Point{{0, 0}, {1, 1}, {3, 1}, {5, 3}, {64, 64}, {257, 3}}
	colors := []gui.Color{
		gui.RGB(10, 20, 30),
		gui.RGBA(200, 100, 50, 128),
		gui.RGBA(255, 255, 255, 0),
	}
	for _, sz := range sizes {
		for _, c := range colors {
			b := newBuffer(sz.X, sz.Y)
			for i := range b.img.Pix {
				b.img.Pix[i] = 0xAA
			}
			b.clear(c)
			r, g, bl, a := premul(c)
			for i := 0; i < len(b.img.Pix); i += 4 {
				p := b.img.Pix[i : i+4]
				if p[0] != r || p[1] != g || p[2] != bl || p[3] != a {
					t.Fatalf("%dx%d clear(%v): pixel %d = %v, want [%d %d %d %d]",
						sz.X, sz.Y, c, i/4, p, r, g, bl, a)
				}
			}
			if b.dirty != b.img.Bounds() {
				t.Fatalf("%dx%d clear(%v): dirty = %v, want %v",
					sz.X, sz.Y, c, b.dirty, b.img.Bounds())
			}
		}
	}
}

// BenchmarkBufferClear times the frame clear alone. Every Draw starts with
// one.
func BenchmarkBufferClear(b *testing.B) {
	for _, sz := range []image.Point{{1920, 1080}, {3840, 2160}} {
		b.Run(fmt.Sprintf("%dx%d", sz.X, sz.Y), func(b *testing.B) {
			buf := newBuffer(sz.X, sz.Y)
			c := gui.RGB(32, 33, 36)
			b.SetBytes(int64(len(buf.img.Pix)))
			for b.Loop() {
				buf.clear(c)
			}
		})
	}
}

// newBackgroundBuffer returns a w x h buffer for bg: "noise" is fixed
// pseudo-random bytes, so a blend that reads the wrong channel or pixel
// shows; "white" and "transparent" are the two ends of the arithmetic's
// range, where a shortcut that rounds differently shows.
func newBackgroundBuffer(w, h int, bg string) *buffer {
	b := newBuffer(w, h)
	switch bg {
	case "white":
		b.clear(gui.RGB(255, 255, 255))
	case "noise":
		rng := rand.New(rand.NewPCG(3, 4))
		for i := range b.img.Pix {
			b.img.Pix[i] = uint8(rng.UintN(256))
		}
	}
	return b
}

// exactCoverage returns the 16-bit coverage of the path emit draws into
// region, indexed from zero. draw.Src of an opaque color onto an Alpha16
// takes the rasterizer's generic loop, which stores it unrounded.
func exactCoverage(region image.Rectangle,
	emit func(z *vector.Rasterizer, ox, oy float32)) *image.Alpha16 {

	var z vector.Rasterizer
	z.Reset(region.Dx(), region.Dy())
	emit(&z, float32(region.Min.X), float32(region.Min.Y))
	z.DrawOp = draw.Src
	cov := image.NewAlpha16(z.Bounds())
	z.Draw(cov, z.Bounds(), image.NewUniform(color.Opaque), image.Point{})
	return cov
}

// TestFillPathSolidMatchesRasterizer: a solid fill goes through the 8-bit
// mask and compositeUniform (#1327). Before, the rasterizer blended it
// itself from 16-bit coverage. Every pixel must be within one level of
// what that wrote, and byte-identical where the 16-bit coverage is 0 or
// full. It must also be exactly the rasterizer's formula applied to the
// coverage's high byte, which is what the mask holds, so the loop's
// shortcuts for 0 and 0xff cannot change a pixel.
//
// The shapes are what the renderer fills: a rect at fractional
// coordinates, a rounded rect, a border ring, a circle, one wider than
// the rasterizer's floating-point threshold, ones cut by the clip and by
// the buffer's edges, and a shallow line.
func TestFillPathSolidMatchesRasterizer(t *testing.T) {
	const w, h = 640, 96
	type shape struct {
		name         string
		x, y, wd, ht float32
		clip         image.Rectangle
		emit         func(z *vector.Rasterizer, ox, oy float32)
	}
	box := func(name string, x, y, wd, ht, rad, ring float32) shape {
		return shape{name, x, y, wd, ht, image.Rectangle{},
			func(z *vector.Rasterizer, ox, oy float32) {
				pathRoundRect(z, ox, oy, x, y, wd, ht, rad, false)
				if ring > 0 {
					pathRoundRect(z, ox, oy, x+ring, y+ring,
						wd-2*ring, ht-2*ring, rad-ring, true)
				}
			}}
	}
	clipped := box("cut by the clip", 280.7, 2.2, 90, 40, 14, 0)
	clipped.clip = image.Rect(300, 10, 350, 40)
	shapes := []shape{
		box("fractional rect", 3.3, 2.7, 40.4, 17.45, 0, 0),
		box("rounded rect", 50.5, 4.25, 60, 30, 8, 0),
		box("border ring", 120.2, 3.6, 70, 32, 6, 1.5),
		box("circle", 200.4, 5.1, 33.3, 33.3, 16.65, 0),
		box("wider than 512 px", 4.6, 50.3, 600.8, 40.2, 12, 2.25),
		clipped,
		box("cut by the buffer edge", -10.5, -6.3, 40, 30, 8, 2),
		box("cut by the far edge", 615.3, 70.6, 40, 40, 10, 0),
		// A shallow line crosses pixels at every offset, so its edge
		// takes nearly every coverage value, 0x01 and 0xfe included.
		{"shallow line", 8, 20, 620, 9, image.Rectangle{},
			func(z *vector.Rasterizer, ox, oy float32) {
				pathQuad(z, ox, oy, [4][2]float32{
					{8.2, 20.1}, {627.9, 26.8}, {627.9, 28.1}, {8.2, 21.4}})
			}},
	}
	colors := []gui.Color{
		gui.RGB(200, 90, 30),
		gui.RGBA(30, 160, 220, 128),
		gui.RGBA(250, 250, 250, 254),
		gui.RGBA(90, 20, 240, 1),
		gui.RGBA(90, 20, 240, 0),
	}
	for _, sh := range shapes {
		for _, bg := range []string{"noise", "transparent", "white"} {
			t.Run(sh.name+"/"+bg, func(t *testing.T) {
				for _, c := range colors {
					got := newBackgroundBuffer(w, h, bg)
					want := newBackgroundBuffer(w, h, bg)
					if !sh.clip.Empty() {
						got.clip, want.clip = sh.clip, sh.clip
					}
					region := got.region(sh.x, sh.y, sh.wd, sh.ht)
					before := bytes.Clone(got.img.Pix)
					got.fillPath(region, got.solid(c), sh.emit)

					// What fillPath did before: the rasterizer's
					// uniform draw.Over onto the RGBA buffer.
					want.rast.Reset(region.Dx(), region.Dy())
					sh.emit(&want.rast, float32(region.Min.X), float32(region.Min.Y))
					want.rast.Draw(want.img, region, want.solid(c), region.Min)

					cov := exactCoverage(region, sh.emit)
					var src [4]uint32
					src[0], src[1], src[2], src[3] = got.solid(c).(*image.Uniform).RGBA()
					var edge, full int
					for i := 0; i < len(got.img.Pix); i += 4 {
						x, y := i/4%w, i/4/w
						g, wt := got.img.Pix[i:i+4], want.img.Pix[i:i+4]
						if !image.Pt(x, y).In(region) {
							if !bytes.Equal(g, before[i:i+4]) {
								t.Fatalf("%v: pixel (%d,%d) outside %v changed", c, x, y, region)
							}
							continue
						}
						a := cov.Alpha16At(x-region.Min.X, y-region.Min.Y).A
						switch {
						case a == 0xffff:
							full++
						case a != 0:
							edge++
						}
						ma := uint32(a>>8) * 0x101
						inv := 0xffff - src[3]*ma/0xffff
						for ch := range 4 {
							d := int(g[ch]) - int(wt[ch])
							if d > 1 || d < -1 || (d != 0 && (a == 0 || a == 0xffff)) {
								t.Fatalf("%v: pixel (%d,%d) coverage %#04x channel %d = %d, the rasterizer wrote %d",
									c, x, y, a, ch, g[ch], wt[ch])
							}
							e := uint8((uint32(before[i+ch])*0x101*inv + src[ch]*ma) / 0xffff >> 8)
							if g[ch] != e {
								t.Fatalf("%v: pixel (%d,%d) coverage %#04x channel %d = %d, the formula gives %d",
									c, x, y, a, ch, g[ch], e)
							}
						}
					}
					if edge == 0 || full == 0 {
						t.Fatalf("%d edge and %d fully covered pixels; the case compares too little",
							edge, full)
					}
					if c.A > 1 && bytes.Equal(got.img.Pix, before) {
						t.Fatalf("%v: the fill drew nothing", c)
					}
				}
			})
		}
	}
}

// BenchmarkFillPathSolid times solid fills alone, as the renderer issues
// them for a view of buttons: a rounded fill and a border ring over it,
// 100 times at 120x32 px, and one 1200x800 px panel, which is past the
// rasterizer's floating-point threshold.
func BenchmarkFillPathSolid(b *testing.B) {
	type box struct{ x, y, w, h float32 }
	buttons := make([]box, 100)
	for i := range buttons {
		buttons[i] = box{float32(16 + i%10*136), float32(16 + i/10*48), 120, 32}
	}
	cases := []struct {
		name  string
		boxes []box
	}{
		{"100buttons", buttons},
		{"panel", []box{{40.5, 40.5, 1200, 800}}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			buf := newBuffer(1920, 1080)
			buf.clear(gui.RGB(32, 33, 36))
			fill := gui.RGB(52, 54, 60)
			border := gui.RGBA(120, 124, 132, 200)
			fillAll := func() {
				for _, bx := range tc.boxes {
					buf.fillPath(buf.region(bx.x, bx.y, bx.w, bx.h), buf.solid(fill),
						func(z *vector.Rasterizer, ox, oy float32) {
							pathRoundRect(z, ox, oy, bx.x, bx.y, bx.w, bx.h, 6, false)
						})
					buf.fillPath(buf.region(bx.x, bx.y, bx.w, bx.h), buf.solid(border),
						func(z *vector.Rasterizer, ox, oy float32) {
							pathRoundRect(z, ox, oy, bx.x, bx.y, bx.w, bx.h, 6, false)
							pathRoundRect(z, ox, oy, bx.x+1, bx.y+1, bx.w-2, bx.h-2, 5, true)
						})
				}
			}
			// The first draw grows the rasterizer and mask scratch.
			fillAll()
			b.ReportAllocs()
			for b.Loop() {
				fillAll()
			}
		})
	}
}
