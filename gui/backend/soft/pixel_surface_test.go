package soft

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestPixelSurfaceDrawUpdateResizeAndClose(t *testing.T) {
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Width: 2, Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	r := newRenderer(20, 10, 1)
	cmds := []gui.RenderCmd{{Kind: gui.RenderImage, Surface: surface,
		Resource: "missing-file-must-not-load", W: 20, H: 10, Opacity: 1,
		ImageFilter: gui.ImageFilterNearest}}
	if err = surface.Update([]byte{255, 0, 0, 255, 0, 0, 255, 255}, 8); err != nil {
		t.Fatal(err)
	}
	r.drawAll(cmds)
	red, _, blue, _ := at(r.buf.img, 9, 5)
	if red != 255 || blue != 0 {
		t.Fatalf("nearest seam = %d,%d", red, blue)
	}
	storage, _ := r.surfaces.Resolve(surface, uploadSurfaceImage)
	if err = surface.Update([]byte{0, 255, 0, 255, 0, 255, 0, 255}, 8); err != nil {
		t.Fatal(err)
	}
	r.drawAll(cmds)
	updated, _ := r.surfaces.Resolve(surface, uploadSurfaceImage)
	if updated != storage {
		t.Error("fixed-size update replaced backing image")
	}
	if _, green, _, _ := at(r.buf.img, 10, 5); green != 255 {
		t.Error("updated pixels not drawn")
	}
	if err = surface.Resize(1, 1); err != nil {
		t.Fatal(err)
	}
	if err = surface.Update([]byte{0, 0, 255, 255}, 4); err != nil {
		t.Fatal(err)
	}
	r.drawAll(cmds)
	if _, _, gotBlue, _ := at(r.buf.img, 10, 5); gotBlue != 255 {
		t.Error("resized pixels not drawn")
	}
	surface.Close()
	r.buf.clear(gui.Black)
	r.drawAll(cmds)
	if _, _, gotBlue, _ := at(r.buf.img, 10, 5); gotBlue != 0 {
		t.Error("closed surface still drew pixels")
	}
}
