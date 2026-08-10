//go:build windows

// Package hicon converts PNG data to Win32 HICON handles, shared by
// the SNI tray backend and the GL window backend.
package hicon

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procDestroyIcon        = user32.NewProc("DestroyIcon")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procCreateBitmap       = gdi32.NewProc("CreateBitmap")
	procCreateIconIndirect = user32.NewProc("CreateIconIndirect")
)

const (
	biRGB        = 0
	dibRGBColors = 0
)

type bitmapV5Header struct {
	bV5Size          uint32
	bV5Width         int32
	bV5Height        int32
	bV5Planes        uint16
	bV5BitCount      uint16
	bV5Compression   uint32
	bV5SizeImage     uint32
	bV5XPelsPerMeter int32
	bV5YPelsPerMeter int32
	bV5ClrUsed       uint32
	bV5ClrImportant  uint32
	bV5RedMask       uint32
	bV5GreenMask     uint32
	bV5BlueMask      uint32
	bV5AlphaMask     uint32
	bV5CSType        uint32
	bV5Endpoints     [36]byte
	bV5GammaRed      uint32
	bV5GammaGreen    uint32
	bV5GammaBlue     uint32
	bV5Intent        uint32
	bV5ProfileData   uint32
	bV5ProfileSize   uint32
	bV5Reserved      uint32
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

// FromPNG decodes pngData and creates a HICON. maxDim rejects
// oversized icons to prevent GDI memory blowup. The caller owns the
// returned handle and must release it with Destroy.
func FromPNG(pngData []byte, maxDim int) (uintptr, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return 0, fmt.Errorf("png decode: %w", err)
	}
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return 0, errors.New("empty icon")
	}
	if w > maxDim || h > maxDim {
		return 0, fmt.Errorf("icon too large: %dx%d (max %d)",
			w, h, maxDim)
	}

	// Build 32-bit BGRA pixel buffer (GDI expects BGR order).
	stride := w * 4
	pixels := make([]byte, stride*h)
	for y := range h {
		for x := range w {
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			off := y*stride + x*4
			pixels[off+0] = byte(b >> 8)
			pixels[off+1] = byte(g >> 8)
			pixels[off+2] = byte(r >> 8)
			pixels[off+3] = byte(a >> 8)
		}
	}

	// Get screen DC for DIB creation.
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return 0, errors.New("GetDC failed")
	}
	defer procReleaseDC.Call(0, hdc)

	// Create 32-bit DIB section.
	bmi := bitmapV5Header{
		bV5Size:        uint32(unsafe.Sizeof(bitmapV5Header{})),
		bV5Width:       int32(w),
		bV5Height:      int32(-h), // negative = top-down
		bV5Planes:      1,
		bV5BitCount:    32,
		bV5Compression: biRGB,
		bV5AlphaMask:   0xFF000000,
		bV5RedMask:     0x00FF0000,
		bV5GreenMask:   0x0000FF00,
		bV5BlueMask:    0x000000FF,
	}
	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0, 0)
	if hbm == 0 {
		return 0, errors.New("CreateDIBSection failed")
	}
	defer procDeleteObject.Call(hbm)

	// Copy pixels into the DIB section.
	dst := unsafe.Slice((*byte)(bits), len(pixels))
	copy(dst, pixels)

	// Create a 1bpp monochrome mask. CreateIconIndirect requires a
	// non-NULL mask for fIcon; an all-zero mask is fully opaque, so
	// the 32bpp color bitmap's alpha does the shaping.
	// CreateBitmap with a NULL lpBits leaves the bits undefined, so
	// pass an explicitly zeroed buffer.
	maskStride := ((w + 15) / 16) * 2 // 1bpp rows are word-aligned
	zeroMask := make([]byte, maskStride*h)
	hMask, _, _ := procCreateBitmap.Call(
		uintptr(w), uintptr(h), 1, 1,
		uintptr(unsafe.Pointer(&zeroMask[0])))
	if hMask == 0 {
		return 0, errors.New("CreateBitmap failed")
	}
	// CreateIconIndirect copies both bitmaps, so the mask (like the
	// color DIB above) can be deleted once the icon exists.
	defer procDeleteObject.Call(hMask)

	ii := iconInfo{
		fIcon:    1, // TRUE
		xHotspot: 0,
		yHotspot: 0,
		hbmColor: hbm,
		hbmMask:  hMask,
	}
	hIcon, _, _ := procCreateIconIndirect.Call(
		uintptr(unsafe.Pointer(&ii)))
	if hIcon == 0 {
		return 0, errors.New("CreateIconIndirect failed")
	}

	return hIcon, nil
}

// Destroy releases a handle returned by FromPNG. No-op on 0.
func Destroy(h uintptr) {
	if h != 0 {
		procDestroyIcon.Call(h)
	}
}
