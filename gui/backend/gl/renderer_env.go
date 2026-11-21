//go:build ((linux && !android) || windows) && !js

package gl

import (
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
// Only whole pixels that fit both slices are written.
func swizzleBGRA(dst, src []byte) {
	n := min(len(dst), len(src)) &^ 3
	for i := 0; i < n; i += 4 {
		// One bounds check per pixel instead of four.
		d, s := dst[i:i+4:i+4], src[i:i+4:i+4]
		d[0], d[1], d[2], d[3] = s[2], s[1], s[0], s[3]
	}
}
