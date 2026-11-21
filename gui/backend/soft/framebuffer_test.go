package soft

import (
	"fmt"
	"image"
	"testing"

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
