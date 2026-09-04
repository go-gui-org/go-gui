//go:build linux && !js && !android

package gl

import (
	"strconv"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
)

// readDPIScale derives a UI scale from the Xft.dpi entry in the root
// RESOURCE_MANAGER property, falling back to 1.0.
func readDPIScale(conn *xgb.Conn, root xproto.Window) float32 {
	dpi, err := strconv.Atoi(readXResource(conn, root, "Xft.dpi"))
	if err == nil && dpi > 0 {
		return float32(dpi) / 96.0
	}
	return 1
}

// Plausible bounds for a physical display DPI. Values outside this range
// usually mean a bogus EDID physical size, so the RandR path is rejected
// in favor of the Xft.dpi fallback.
const (
	minPlausibleDPI = 50
	maxPlausibleDPI = 400
)

// dpiScaleForWindow computes the UI scale for the monitor containing the
// root-relative point (x,y), using that monitor's RandR-reported physical
// size. It falls back to the global Xft.dpi scale when RandR is
// unavailable or reports no usable physical size. Returns the scale and
// the CRTC the point lands on (0 when none was resolved).
func dpiScaleForWindow(conn *xgb.Conn, root xproto.Window, haveRandr bool, x, y int32) (float32, randr.Crtc) {
	if haveRandr {
		if s, crtc, ok := randrDPIScale(conn, root, x, y); ok {
			return s, crtc
		}
	}
	return readDPIScale(conn, root), 0
}

// randrDPIScale finds the CRTC covering (x,y) and derives a UI scale from
// its output's physical size. ok is false when RandR data is missing or
// implausible.
func randrDPIScale(conn *xgb.Conn, root xproto.Window, x, y int32) (float32, randr.Crtc, bool) {
	res, err := randr.GetScreenResourcesCurrent(conn, root).Reply()
	if err != nil || res == nil {
		return 0, 0, false
	}
	for _, crtc := range res.Crtcs {
		info, ierr := randr.GetCrtcInfo(conn, crtc, res.ConfigTimestamp).Reply()
		if ierr != nil || info == nil || info.Width == 0 || info.Height == 0 {
			continue // disabled/disconnected CRTC
		}
		if x < int32(info.X) || x >= int32(info.X)+int32(info.Width) ||
			y < int32(info.Y) || y >= int32(info.Y)+int32(info.Height) {
			continue
		}
		if len(info.Outputs) == 0 {
			return 0, 0, false
		}
		out, oerr := randr.GetOutputInfo(conn, info.Outputs[0], res.ConfigTimestamp).Reply()
		if oerr != nil || out == nil {
			return 0, 0, false
		}
		if dpi, ok := crtcDPI(info, out); ok {
			return float32(dpi / 96.0), crtc, true
		}
		return 0, 0, false
	}
	return 0, 0, false
}

// crtcDPI averages the horizontal and vertical DPI from the CRTC pixel
// size and the output's physical millimetre size. ok is false when no
// physical dimension is reported or the result is implausible.
func crtcDPI(info *randr.GetCrtcInfoReply, out *randr.GetOutputInfoReply) (float64, bool) {
	const mmPerInch = 25.4
	// A 90/270° rotation swaps the pixel axes relative to physical size.
	pw, ph := float64(info.Width), float64(info.Height)
	if info.Rotation&(randr.RotationRotate90|randr.RotationRotate270) != 0 {
		pw, ph = ph, pw
	}
	var sum float64
	var n int
	if out.MmWidth > 0 {
		sum += pw / (float64(out.MmWidth) / mmPerInch)
		n++
	}
	if out.MmHeight > 0 {
		sum += ph / (float64(out.MmHeight) / mmPerInch)
		n++
	}
	if n == 0 {
		return 0, false
	}
	dpi := sum / float64(n)
	if dpi < minPlausibleDPI || dpi > maxPlausibleDPI {
		return 0, false
	}
	return dpi, true
}

// maybeRescaleDPI re-evaluates the per-monitor scale when the window has
// moved to a CRTC with a different DPI, updating plat.scale and the text
// stack. It reports whether the scale changed so the caller can trigger
// a relayout. ConfigureNotify coordinates are frame-relative under a
// reparenting WM, so the true root position is queried explicitly and a
// RandR rescan runs only when that position changed. Cursors are not
// reloaded: the Xcursor size X clients see is already in device pixels
// and does not track the per-monitor scale.
func (b *Backend) maybeRescaleDPI() bool {
	if !b.plat.haveRandr {
		return false
	}
	t, err := xproto.TranslateCoordinates(b.plat.conn, b.plat.window,
		b.plat.root, 0, 0).Reply()
	if err != nil || t == nil {
		return false
	}
	if b.plat.haveLastPos && t.DstX == b.plat.lastRootX &&
		t.DstY == b.plat.lastRootY {
		return false // no move → still on the same monitor
	}
	b.plat.lastRootX, b.plat.lastRootY = t.DstX, t.DstY
	b.plat.haveLastPos = true

	scale, crtc := dpiScaleForWindow(b.plat.conn, b.plat.root, true,
		int32(t.DstX), int32(t.DstY))
	b.plat.curCrtc = crtc
	if scale == b.plat.scale {
		return false
	}
	b.plat.scale = scale
	b.applyDPIScale(scale)
	// The hints were written in physical pixels for the old scale, so a
	// monitor move would otherwise leave the floor enforcing the wrong
	// logical size.
	setSizeHints(b.plat.conn, b.plat.window, b.plat.limits.Scaled(scale))
	return true
}
