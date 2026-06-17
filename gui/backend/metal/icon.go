//go:build darwin && !ios

package metal

/*
#cgo LDFLAGS: -framework Cocoa
#include <stddef.h>
void metalActivateApp(void);
void metalSetDockIcon(const void *data, int len);
*/
import "C"
import (
	"bytes"
	"image"
	"image/draw"
	_ "image/png"
	"log"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

// setWindowIcon decodes PNG data and sets it as the SDL window icon.
func setWindowIcon(win *sdl.Window, png []byte) {
	if len(png) == 0 {
		return
	}
	src, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		log.Printf("metal: decode icon PNG: %v", err)
		return
	}
	bounds := src.Bounds()
	// Guard against malformed PNG decodes with zero-sized
	// image buffers to avoid panics when taking &rgba.Pix[0].
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, src, bounds.Min, draw.Src)
	if len(rgba.Pix) == 0 {
		return
	}

	w := int32(bounds.Dx())
	h := int32(bounds.Dy())
	surface, err := sdl.CreateRGBSurfaceFrom(
		unsafe.Pointer(&rgba.Pix[0]),
		w, h, 32, int(w*4),
		0x000000FF, 0x0000FF00, 0x00FF0000, 0xFF000000,
	)
	if err != nil {
		log.Printf("metal: create icon surface: %v", err)
		return
	}
	defer surface.Free()
	win.SetIcon(surface)
}

// activateApp sets the NSApplication activation policy to Regular
// and brings the app to the foreground. Must be called before
// creating any windows for CLI-launched (non-bundled) binaries.
func activateApp() {
	C.metalActivateApp()
}

// setAppIcon sets the macOS Dock icon from PNG data.
func setAppIcon(png []byte) {
	if len(png) == 0 {
		return
	}
	C.metalSetDockIcon(unsafe.Pointer(&png[0]), C.int(len(png)))
}
