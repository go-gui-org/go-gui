//go:build ((linux && !android) || windows) && !js

package gl

import (
	"encoding/binary"
	"log"
	"os"
	"sync"
	"time"
)

// What the Linux and Windows CPU renderer paths share: how GOGUI_RENDERER is
// read, the pixel swizzle into the window system's byte order, and the
// frame interval used when the display's rate is unknown (#1265, #1310).

// rendererEnv is what GOGUI_RENDERER asks for. The X11 path (pickRenderer),
// the Wayland path (openWaylandDisplay) and the Windows New read it through
// one function, so they cannot disagree on spelling.
type rendererEnv uint8

const (
	// rendererEnvAuto: unset or empty; a probe decides.
	rendererEnvAuto rendererEnv = iota
	// rendererEnvGL: "gl", or any value that is neither gl nor soft.
	rendererEnvGL
	// rendererEnvSoft: "soft".
	rendererEnvSoft
)

// readRendererEnv reads GOGUI_RENDERER. Matching is exact and case-sensitive,
// like GOGUI_WAYLAND. A value that is neither gl nor soft reads as gl, with
// one warning per process: a typo such as "Soft" must not silently cost a
// probe or pick a renderer the user did not name.
func readRendererEnv() rendererEnv {
	switch v := os.Getenv("GOGUI_RENDERER"); v {
	case "soft":
		return rendererEnvSoft
	case "gl":
		return rendererEnvGL
	case "":
		return rendererEnvAuto
	default:
		unknownRendererOnce.Do(func() {
			log.Printf("gl: GOGUI_RENDERER=%q is not gl or soft; using GL", v)
		})
		return rendererEnvGL
	}
}

// unknownRendererOnce keeps readRendererEnv's warning to one line per process.
var unknownRendererOnce sync.Once

// defaultFrameInterval paces soft frames when the monitor's refresh rate is
// unknown: no RandR, no CRTC, or a mode without timing on X11; a display
// frequency of 0 or 1 ("hardware default") on Windows.
const defaultFrameInterval = time.Second / 60

// swizzleBGRA converts premultiplied RGBA to the B, G, R, A byte order of a
// little-endian 32 bpp TrueColor ZPixmap, which is also the order of a 32 bpp
// BI_RGB DIB section on Windows. Alpha is copied: a depth-24 visual and an
// opaque Windows window ignore the byte, and a depth-32 one wants it
// premultiplied, which it is.
// Only whole pixels that fit both slices are written. dst and src may be
// the same slice.
//
// Two pixels go through as one 64-bit word: red and blue change places by
// mask and shift, green and alpha stay (#1327). The word is read and
// written little-endian by name, so the bytes land the same on any host. A
// 32-bit word, one pixel a turn, measured slower than the byte loop this
// replaced.
func swizzleBGRA(dst, src []byte) {
	const (
		greenAlpha = 0xff00ff00_ff00ff00
		red        = 0x000000ff_000000ff // byte 0 of each pixel
		blue       = 0x00ff0000_00ff0000 // byte 2 of each pixel
	)
	n := min(len(dst), len(src)) &^ 3
	dst, src = dst[:n], src[:n]
	for len(src) >= 8 {
		v := binary.LittleEndian.Uint64(src)
		binary.LittleEndian.PutUint64(dst, v&greenAlpha|(v&red)<<16|(v&blue)>>16)
		dst, src = dst[8:], src[8:]
	}
	if len(src) == 4 {
		// The odd pixel. The right side is read before the left is
		// written, so this is safe in place too.
		dst[0], dst[1], dst[2], dst[3] = src[2], src[1], src[0], src[3]
	}
}
