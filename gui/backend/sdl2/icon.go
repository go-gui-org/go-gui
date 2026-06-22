//go:build !js && !darwin

package sdl2

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/png"
	"log"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

// setWindowIcon decodes PNG data and sets it as the window icon.
func setWindowIcon(win *sdl.Window, png []byte) {
	src, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		log.Printf("sdl2: decode icon PNG: %v", err)
		return
	}
	bounds := src.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, src, bounds.Min, draw.Src)

	w := int32(bounds.Dx())
	h := int32(bounds.Dy())
	surface, err := sdl.CreateRGBSurfaceFrom(
		unsafe.Pointer(&rgba.Pix[0]),
		w, h, 32, int(w*4),
		0x000000FF, 0x0000FF00, 0x00FF0000, 0xFF000000,
	)
	if err != nil {
		log.Printf("sdl2: create icon surface: %v", err)
		return
	}
	defer surface.Free()
	win.SetIcon(surface)
}
