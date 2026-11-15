//go:build !js && !darwin && !android && !cgo

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// solidPixels returns a w×h NRGBA buffer of one color.
func solidPixels(w, h int, c [4]byte) []byte {
	pix := make([]byte, w*h*4)
	fillPixels(pix, c)
	return pix
}

// A texture uploaded from a mem: image follows the registry (#1230): a
// drop deletes it, and a key registered again — after a drop, at a new
// size, or between a sweep and a draw — draws its new pixels.
func TestMemImageTextureFollowsRegistry(t *testing.T) {
	b := newBatchProbeBackend(t)
	target := newReadbackTarget(t, b)
	var deleted []uint32
	b.textures = texcache.New[string, glTexture](128, func(tex glTexture) {
		deleted = append(deleted, tex.id)
		gogl.DeleteTextures(1, &tex.id)
	})
	const key = "gltest/1230"
	t.Cleanup(func() { gui.DropImage(key) })

	src := gui.UseImage(key, 2, 2, solidPixels(2, 2, red))
	cmd := gui.RenderCmd{Kind: gui.RenderImage, Resource: src,
		W: float32(b.physW) / b.dpiScale, H: float32(b.physH) / b.dpiScale,
		Opacity: 1}
	// frame does what renderersDraw does for an image: sweep, then draw.
	frame := func() [4]byte {
		px := target.render(b, func() {
			b.sweepMemTextures()
			b.drawImage(&cmd)
		})
		return pixelAt(b, px, int(b.physW)/2, int(b.physH)/2)
	}
	black := [4]byte{0, 0, 0, 255}

	if got := frame(); got != red {
		t.Fatalf("first draw = %v, want red", got)
	}
	redTex, ok := b.textures.Get(src)
	if !ok || redTex.id == 0 {
		t.Fatal("no texture cached for the mem: image")
	}

	gui.DropImage(key)
	if got := frame(); got != black {
		t.Errorf("dropped image drew %v, want nothing", got)
	}
	if len(deleted) != 1 || deleted[0] != redTex.id {
		t.Errorf("deleted textures = %v, want [%d]", deleted, redTex.id)
	}
	if n := b.textures.Len(); n != 0 {
		t.Errorf("cache holds %d textures after the drop, want 0 "+
			"(a dropped mem: source must not be cached as a file path)", n)
	}

	gui.UseImage(key, 2, 2, solidPixels(2, 2, blue))
	if got := frame(); got != blue {
		t.Errorf("same key registered again drew %v, want blue", got)
	}

	gui.UseImage(key, 4, 4, solidPixels(4, 4, green)) // new size, no drop
	if got := frame(); got != green {
		t.Errorf("key registered at a new size drew %v, want green", got)
	}
	if tex, _ := b.textures.Get(src); tex.w != 4 || tex.h != 4 {
		t.Errorf("cached texture is %dx%d, want 4x4", tex.w, tex.h)
	}

	// A re-registration after this frame's sweep: the draw itself sees
	// the new serial and uploads again.
	b.sweepMemTextures()
	gui.DropImage(key)
	gui.UseImage(key, 4, 4, solidPixels(4, 4, red))
	px := target.render(b, func() { b.drawImage(&cmd) })
	if got := pixelAt(b, px, int(b.physW)/2, int(b.physH)/2); got != red {
		t.Errorf("re-registration between sweep and draw drew %v, want red", got)
	}
	if n := b.textures.Len(); n != 1 {
		t.Errorf("cache holds %d textures, want 1", n)
	}
	if len(deleted) != 3 {
		t.Errorf("deleted %d textures, want 3 (red, blue, green)", len(deleted))
	}
}

// The texture cache is bounded by bytes as well as count (#1239). A frame
// that draws more than the budget keeps every texture it draws, so later
// frames upload nothing; a texture not drawn for two frames is deleted
// when a new upload takes the cache over its budget.
func TestTextureByteBudget(t *testing.T) {
	b := newBatchProbeBackend(t)
	target := newReadbackTarget(t, b)
	const side = 16
	const texBytes = side * side * 4
	var deleted int
	// A budget of two textures; each frame below draws three.
	b.textures = texcache.NewSized[string, glTexture](128, 2*texBytes,
		glTextureBytes, func(tex glTexture) {
			deleted++
			gogl.DeleteTextures(1, &tex.id)
		})
	keys := []string{"gltest/1239a", "gltest/1239b", "gltest/1239c", "gltest/1239d"}
	srcs := make([]string, len(keys))
	for i, k := range keys {
		srcs[i] = gui.UseImage(k, side, side, solidPixels(side, side, red))
	}
	t.Cleanup(func() {
		for _, k := range keys {
			gui.DropImage(k)
		}
	})
	// frame does what renderersDraw does: begin the frame, then draw.
	frame := func(draw ...string) {
		target.render(b, func() {
			b.sweepMemTextures()
			b.textures.BeginFrame()
			for _, src := range draw {
				cmd := gui.RenderCmd{Kind: gui.RenderImage, Resource: src,
					W: side, H: side, Opacity: 1}
				b.drawImage(&cmd)
			}
		})
	}

	ids := map[uint32]bool{}
	for range 5 {
		frame(srcs[0], srcs[1], srcs[2])
		for _, src := range srcs[:3] {
			tex, _ := b.textures.Get(src)
			ids[tex.id] = true
		}
	}
	if len(ids) != 3 || deleted != 0 {
		t.Fatalf("over-budget frames used %d texture ids and deleted %d, "+
			"want 3 and 0 (no upload thrash)", len(ids), deleted)
	}

	// b and c leave the screen. Two frames later a new image d is drawn:
	// the cache is over budget, so b and c are deleted, oldest first.
	frame(srcs[0])
	frame(srcs[0])
	frame(srcs[0], srcs[3])
	if deleted != 2 {
		t.Errorf("deleted %d textures, want 2 (b and c)", deleted)
	}
	if got, want := b.textures.Bytes(), int64(2*texBytes); got != want {
		t.Errorf("cache holds %d bytes, want %d", got, want)
	}
	if _, ok := b.textures.Get(srcs[0]); !ok {
		t.Error("a, drawn every frame, was evicted")
	}
}
