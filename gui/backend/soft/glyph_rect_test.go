package soft

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// The soft glyph backend must be a RectTextureUpdater: go-glyph then uploads
// newly rasterized glyphs mid-frame, before it emits the quads that sample
// them, so a single draw pass shows a glyph on its first appearance (#1265).
// Without it, uploads wait for Commit and a one-pass frame draws new glyphs
// blank — the reason RenderToImage carries a warm pass.
var _ glyph.RectTextureUpdater = (*glyphBackend)(nil)

// TestUpdateTextureRect checks that a sub-rectangle upload copies exactly the
// (x, y, w, h) region from a full-page source with the given stride, and
// leaves every texel outside it untouched.
func TestUpdateTextureRect(t *testing.T) {
	const pw, ph = 8, 6
	gb := newGlyphBackend(1)
	id := gb.NewTexture(pw, ph)
	if id == 0 {
		t.Fatal("NewTexture returned 0")
	}
	tex := gb.textures[id]
	for i := range tex.pix {
		tex.pix[i] = 0xEE // sentinel: must survive outside the rect
	}
	// Source is the whole page; each byte encodes its own offset so a
	// wrong stride or origin shows up as a wrong value, not just a nonzero.
	src := make([]byte, pw*ph*4)
	for i := range src {
		src[i] = byte(i % 251)
	}
	const x, y, w, h = 2, 1, 3, 4
	gb.UpdateTextureRect(id, src, pw*4, x, y, w, h)

	for py := range ph {
		for px := range pw {
			off := (py*pw + px) * 4
			inside := px >= x && px < x+w && py >= y && py < y+h
			for c := range 4 {
				got := tex.pix[off+c]
				want := byte(0xEE)
				if inside {
					want = src[off+c]
				}
				if got != want {
					t.Fatalf("texel (%d,%d) ch %d = %#x, want %#x (inside=%v)",
						px, py, c, got, want, inside)
				}
			}
		}
	}
}

// TestUpdateTextureRectRejectsBadInput: glyph promises an in-bounds rect, but
// the backend is the last line before a slice panic, so an unknown id, a rect
// past the texture, a short source or a negative origin is ignored.
func TestUpdateTextureRectRejectsBadInput(t *testing.T) {
	const pw, ph = 4, 4
	gb := newGlyphBackend(1)
	id := gb.NewTexture(pw, ph)
	full := make([]byte, pw*ph*4)
	for i := range full {
		full[i] = 1
	}
	cases := []struct {
		name               string
		id                 glyph.TextureID
		data               []byte
		stride, x, y, w, h int
	}{
		{"unknown id", id + 7, full, pw * 4, 0, 0, 1, 1},
		{"past right edge", id, full, pw * 4, 3, 0, 2, 1},
		{"past bottom edge", id, full, pw * 4, 0, 3, 1, 2},
		{"negative origin", id, full, pw * 4, -1, 0, 1, 1},
		{"empty rect", id, full, pw * 4, 0, 0, 0, 1},
		{"short source", id, full[:8], pw * 4, 0, 1, 1, 1},
		{"stride below row", id, full, 4, 0, 0, 2, 1},
		// Values whose sums wrap int: the guards must not overflow into a
		// negative bound that passes and then slices out of range.
		{"origin wraps", id, full, pw * 4, math.MaxInt, 0, 1, 1},
		{"stride wraps", id, full, math.MaxInt/2 + 1, 0, 3, 1, 1},
	}
	for _, c := range cases {
		gb.UpdateTextureRect(c.id, c.data, c.stride, c.x, c.y, c.w, c.h)
		for i, v := range gb.textures[id].pix {
			if v != 0 {
				t.Fatalf("%s: texel byte %d written (%d)", c.name, i, v)
			}
		}
	}
}
