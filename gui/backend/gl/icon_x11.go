//go:build linux && !js && !android

package gl

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"log"
	"math"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// maxWindowIconDim caps the decoded _NET_WM_ICON size. App icons are
// typically 512x512; anything larger is rejected rather than letting a
// bogus PNG balloon memory.
const maxWindowIconDim = 1024

// maxNetWMIconPixels is the largest _NET_WM_ICON that fits one X11
// request. The protocol's request-length field is 16 bits in 4-byte
// units (65535 units ≈ 256 KiB), and xgb truncates a larger write,
// desyncing the connection. An icon needs 2 CARDINALs of header plus
// w*h pixels, so 255x255 is the largest square that fits.
const maxNetWMIconPixels = 65535 - 6

// setWindowIcon publishes cfg.IconPNG (or the go-gui default) as
// _NET_WM_ICON, the property window managers read for the taskbar and
// alt-tab icon. Without it the WM falls back to its default icon.
func setWindowIcon(conn *xgb.Conn, win xproto.Window, cfg gui.WindowCfg) {
	icon := cfg.IconPNG
	if len(icon) == 0 {
		icon = gui.DefaultIconPNG
	}
	w, h, argb, err := decodeIconARGB(icon, maxWindowIconDim)
	if err != nil {
		log.Printf("gl: window icon: %v", err)
		return
	}
	// The WM downscales to its taskbar size anyway, so shrink
	// oversized artwork to fit a single protocol request.
	if 2+w*h > maxNetWMIconPixels {
		scale := maxNetWMIconPixels - 2
		// Keep aspect: scale both dims by sqrt(scale/(w*h)).
		f := math.Sqrt(float64(scale) / float64(w*h))
		tw := int(float64(w) * f)
		th := int(float64(h) * f)
		if tw < 1 {
			tw = 1
		}
		if th < 1 {
			th = 1
		}
		argb = scaleARGB(argb, w, h, tw, th)
		w, h = tw, th
	}
	atom := internAtom(conn, "_NET_WM_ICON")
	if atom == 0 {
		return
	}
	// _NET_WM_ICON is [width, height, argb...] with one CARDINAL per
	// pixel in the client's byte order. Client and server share a
	// machine here (local X connection), so native endianness is the
	// de-facto encoding every WM expects.
	data := make([]uint32, 2, len(argb)+2)
	data[0], data[1] = uint32(w), uint32(h)
	data = append(data, argb...)
	xproto.ChangeProperty(conn, xproto.PropModeReplace, win,
		atom, xproto.AtomCardinal, 32,
		uint32(len(data)),
		unsafe.Slice((*byte)(unsafe.Pointer(&data[0])), len(data)*4))
}

// scaleARGB shrinks a w*h ARGB32 image to tw*th with nearest-neighbor
// sampling.
func scaleARGB(src []uint32, w, h, tw, th int) []uint32 {
	dst := make([]uint32, tw*th)
	for y := range th {
		sy := y * h / th
		for x := range tw {
			dst[y*tw+x] = src[sy*w+x*w/tw]
		}
	}
	return dst
}

// decodeIconARGB decodes a PNG into straight (non-premultiplied)
// ARGB32 pixels, the format _NET_WM_ICON expects.
func decodeIconARGB(pngData []byte, maxDim int) (w, h int, argb []uint32, err error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return 0, 0, nil, err
	}
	bounds := img.Bounds()
	w, h = bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return 0, 0, nil, errors.New("empty icon")
	}
	if w > maxDim || h > maxDim {
		return 0, 0, nil, fmt.Errorf(
			"icon too large: %dx%d (max %d)", w, h, maxDim)
	}
	argb = make([]uint32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			argb[y*w+x] = uint32(a>>8)<<24 | uint32(r>>8)<<16 |
				uint32(g>>8)<<8 | uint32(b>>8)
		}
	}
	return w, h, argb, nil
}

// setWMClass sets the WM_CLASS property — two NUL-terminated strings,
// instance and class, both from the same configured name. Window
// managers use it to group windows and to match a .desktop file's
// StartupWMClass.
func setWMClass(conn *xgb.Conn, win xproto.Window, s string) {
	if s == "" {
		return
	}
	buf := append([]byte(s), 0)
	buf = append(buf, s...)
	buf = append(buf, 0)
	atom := internAtom(conn, "WM_CLASS")
	if atom == 0 {
		return
	}
	xproto.ChangeProperty(conn, xproto.PropModeReplace, win,
		atom, xproto.AtomString, 8, uint32(len(buf)), buf)
}
