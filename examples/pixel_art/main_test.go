package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoFindings(t *testing.T) {
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 360, Height: 500})
	w.TestRender(mainView)
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// The sprite registers as an in-memory image of the right size.
func TestSpriteRegisters(t *testing.T) {
	if len(spritePix) != spriteSize*spriteSize*4 {
		t.Fatalf("sprite is %d bytes, want %d",
			len(spritePix), spriteSize*spriteSize*4)
	}
	if src := spriteSrc(); src == "" {
		t.Fatal("UseImage refused the sprite")
	}
}

// Both copies draw, left then right, and Restore leaves the context
// back on the linear filter. Which entry carries which filter is
// pinned in gui by the canvas_image_filter golden.
func TestDrawSpritesFilters(t *testing.T) {
	dc := gui.NewDrawContext(canvasW, canvasH, nil)
	drawSprites(dc, spriteSrc())
	if dc.ImageFilter() != gui.ImageFilterLinear {
		t.Error("Restore did not put the linear filter back")
	}
	imgs := dc.Images()
	if len(imgs) != 2 {
		t.Fatalf("drew %d images, want 2", len(imgs))
	}
	if imgs[0].X >= imgs[1].X {
		t.Fatalf("image order: x %v then %v", imgs[0].X, imgs[1].X)
	}
}
