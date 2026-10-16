package soft

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A 2x1 red|blue image drawn 10x larger. Nearest sampling keeps a hard
// edge at the seam; linear sampling blends the pixels next to it.
func TestDrawImageFilterNearest(t *testing.T) {
	pix := []byte{255, 0, 0, 255, 0, 0, 255, 255}
	src := gui.UseImage("soft-test-filter", 2, 1, pix)
	if src == "" {
		t.Fatal("UseImage refused the buffer")
	}
	t.Cleanup(func() { gui.DropImage("soft-test-filter") })

	draw := func(f gui.ImageFilter) *renderer {
		r := newRenderer(20, 10, 1)
		r.drawAll([]gui.RenderCmd{{
			Kind: gui.RenderImage, X: 0, Y: 0, W: 20, H: 10,
			Resource: src, Opacity: 1, ImageFilter: f,
		}})
		return r
	}

	near := draw(gui.ImageFilterNearest)
	for x := range 20 {
		red, _, blue, _ := at(near.buf.img, x, 5)
		wantRed, wantBlue := uint8(255), uint8(0)
		if x >= 10 {
			wantRed, wantBlue = 0, 255
		}
		if red != wantRed || blue != wantBlue {
			t.Errorf("nearest x=%d: r=%d b=%d, want r=%d b=%d",
				x, red, blue, wantRed, wantBlue)
		}
	}

	// The linear pixel left of the seam mixes in some blue: this is
	// what the nearest case must not do.
	lin := draw(gui.ImageFilterLinear)
	if _, _, blue, _ := at(lin.buf.img, 9, 5); blue == 0 {
		t.Error("linear x=9 has no blue; the test no longer tells the filters apart")
	}
}
