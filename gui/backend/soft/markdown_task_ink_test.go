//go:build !wasm

package soft

import (
	"image"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestMarkdownTaskCheckInkInset draws a checked GFM task item with the
// real icon font and measures the painted check against its box
// (#1254). A gui-package test can pin only the font size, and a font
// size says nothing about where a glyph's ink lands: the check used to
// be sized from the box and drawn edge to edge, touching the border.
// The fill is found by its color, and the check by the near-white
// pixels inside the fill.
func TestMarkdownTaskCheckInkInset(t *testing.T) {
	const scale = 2
	win := newWin(t, 200, 60, func(w *gui.Window) gui.View {
		return w.Markdown(gui.MarkdownCfg{
			ID:     "md",
			Source: "- [x] done\n",
		})
	})
	img, err := RenderToImage(win, scale)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	fill := gui.CurrentTheme().ColorSelect

	box, ok := bboxWhere(img, img.Bounds(), func(r, g, b uint8) bool {
		return near(r, fill.R) && near(g, fill.G) && near(b, fill.B)
	})
	if !ok {
		t.Fatalf("no task box fill (%v) in the frame", fill)
	}
	// Near white: the check is drawn White over the fill, and its
	// anti-aliased edge blends toward the fill, so a strict threshold
	// keeps only the body of the mark.
	ink, ok := bboxWhere(img, box, func(r, g, b uint8) bool {
		return r > 200 && g > 200 && b > 200
	})
	if !ok {
		t.Fatalf("no check ink inside box %v", box)
	}

	// Logical pixels between the ink and each side of the box fill.
	left := float32(ink.Min.X-box.Min.X) / scale
	right := float32(box.Max.X-ink.Max.X) / scale
	top := float32(ink.Min.Y-box.Min.Y) / scale
	bottom := float32(box.Max.Y-ink.Max.Y) / scale
	t.Logf("box %v ink %v; inset l=%v r=%v t=%v b=%v",
		box, ink, left, right, top, bottom)
	// At least 2px clear of the border on every side: the visible
	// margin the issue asks for. The old sizing left about 0.
	const minInset = 2
	for name, v := range map[string]float32{
		"left": left, "right": right, "top": top, "bottom": bottom,
	} {
		if v < minInset {
			t.Errorf("%s inset %vpx, want >= %vpx", name, v, minInset)
		}
	}
	// centerGlyphOnInk centres the ink; allow one logical pixel for
	// anti-aliasing and the baseline snap.
	if d := left - right; d > 1 || d < -1 {
		t.Errorf("check off centre horizontally: l=%v r=%v", left, right)
	}
	if d := top - bottom; d > 1 || d < -1 {
		t.Errorf("check off centre vertically: t=%v b=%v", top, bottom)
	}
}

// near reports whether a channel is within a small tolerance of want,
// for colors that pass through blending and rounding.
func near(got, want uint8) bool {
	d := int(got) - int(want)
	return d >= -3 && d <= 3
}

// bboxWhere returns the bounding box of the pixels inside r that match.
func bboxWhere(
	img *image.RGBA, r image.Rectangle, match func(r, g, b uint8) bool,
) (image.Rectangle, bool) {
	out := image.Rectangle{}
	found := false
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if !match(c.R, c.G, c.B) {
				continue
			}
			p := image.Rect(x, y, x+1, y+1)
			if !found {
				out, found = p, true
			} else {
				out = out.Union(p)
			}
		}
	}
	return out, found
}
