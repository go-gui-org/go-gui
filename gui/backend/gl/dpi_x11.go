//go:build linux && !js && !android

package gl

import (
	"bytes"
	"strconv"

	"github.com/go-gui-org/go-gui/gui/backend/internal/devscale"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
)

// Accepted Xft.dpi range: scale 0.5 to 8. Any client can write
// RESOURCE_MANAGER, and Xft.dpi now wins over RandR, so a value such as
// 100000 would otherwise give a scale of about 1000. The window size in
// physical pixels would then overflow the 16-bit X window size. A value
// outside the range counts as unset, so RandR decides.
const (
	minXftDPI = 48
	maxXftDPI = 768
)

// parseXftDPIScale turns the Xft.dpi value from the root RESOURCE_MANAGER
// property into a UI scale. ok is false when the value is unset, not an
// integer, or outside [minXftDPI, maxXftDPI].
func parseXftDPIScale(s string) (float32, bool) {
	dpi, err := strconv.Atoi(s)
	if err != nil || dpi < minXftDPI || dpi > maxXftDPI {
		return 0, false
	}
	return float32(dpi) / 96.0, true
}

// pickDPIScale chooses the UI scale source. Xft.dpi comes first because it
// is the scale the user chose: GNOME, KDE, Xfce and xrdb publish it, and GTK
// and Qt follow it. RandR physical DPI is not that choice. Under XWayland
// fractional scaling it is also wrong: the CRTC is the ceil-scaled virtual
// size (3072x1728 for a 1920x1080 panel at 125%) while the millimetres are
// the real panel, so it gives 2.62 where the desktop asks for 2.0 (#871).
// rr runs only when Xft.dpi is unset and RandR is present, so a desktop
// that sets Xft.dpi costs no RandR round trips. The fallback is 1.0.
//
// Under XWayland with no Xft.dpi, RandR is skipped and the scale is 1.0.
// The compositor already scales X11 buffers by the output scale, so a
// RandR physical DPI scale is applied a second time: on Phosh at 174% the
// app drew at 1.81 and phoc enlarged that by 1.74 (#918). GTK and Qt also
// draw at 1.0 on XWayland unless Xft.dpi or their own variable says else.
func pickDPIScale(xft float32, xftOK, haveRandr, xwayland bool, rr func() (float32, randr.Crtc, bool)) (float32, randr.Crtc) {
	if xftOK {
		return xft, 0
	}
	if haveRandr && !xwayland {
		if s, crtc, ok := rr(); ok {
			return s, crtc
		}
	}
	return 1, 0
}

// Plausible bounds for a physical display DPI. Values outside this range
// usually mean a bogus EDID physical size, so the RandR path is rejected
// in favor of the 1.0 fallback.
const (
	minPlausibleDPI = 50
	maxPlausibleDPI = 400
)

// dpiScaleForWindow computes the UI scale for the window at the
// root-relative point (x,y). The global Xft.dpi scale wins when it is set.
// Otherwise the scale comes from the RandR physical size of the monitor
// containing (x,y), and falls back to 1.0 when RandR is unavailable or
// reports no usable physical size. See pickDPIScale for the order. Returns
// the scale and the CRTC the point lands on (0 when none was resolved).
func dpiScaleForWindow(conn *xgb.Conn, root xproto.Window, haveRandr, xwayland bool, x, y int32) (float32, randr.Crtc) {
	xft, xftOK := parseXftDPIScale(readXResource(conn, root, "Xft.dpi"))
	scale, crtc := pickDPIScale(xft, xftOK, haveRandr, xwayland, func() (float32, randr.Crtc, bool) {
		return randrDPIScale(conn, root, x, y)
	})
	// GOGUI_DEVICE_SCALE replaces the monitor's scale here, so window
	// creation and monitor moves both use it (#971). The window keeps its
	// logical size and gets more physical pixels.
	return devscale.Apply(scale), crtc
}

// xwaylandExt is the extension name Xwayland 23.1 and later advertise.
const xwaylandExt = "XWAYLAND"

// detectXWayland reports whether the X server is Xwayland. It runs once at
// startup. The XWAYLAND extension is the direct check. Older Xwayland
// servers do not have it, but they name every RandR output XWAYLAND<n>
// (the same check SDL uses), so that is the fallback when RandR is present.
func detectXWayland(conn *xgb.Conn, root xproto.Window, haveRandr bool) bool {
	ext, err := xproto.QueryExtension(conn, uint16(len(xwaylandExt)), xwaylandExt).Reply()
	if err == nil && ext != nil && ext.Present {
		return true
	}
	if !haveRandr {
		return false
	}
	res, err := randr.GetScreenResourcesCurrent(conn, root).Reply()
	if err != nil || res == nil || len(res.Outputs) == 0 {
		return false
	}
	// Xwayland names every output XWAYLAND<n>, so the first output decides.
	// One query, not one per output: the server sets the output count, and
	// each query is a blocking round trip.
	out, err := randr.GetOutputInfo(conn, res.Outputs[0], res.ConfigTimestamp).Reply()
	return err == nil && out != nil && isXWaylandOutputName(out.Name)
}

// isXWaylandOutputName reports whether a RandR output name is one Xwayland
// gives its outputs (XWAYLAND0, XWAYLAND1, ...).
func isXWaylandOutputName(name []byte) bool {
	return bytes.HasPrefix(name, []byte(xwaylandExt))
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
// a relayout. It does nothing unless perMonitorDPI is true.
// ConfigureNotify coordinates are frame-relative under a
// reparenting WM, so the true root position is queried explicitly and a
// RandR rescan runs only when that position changed. Cursors are not
// reloaded: the Xcursor size X clients see is already in device pixels
// and does not track the per-monitor scale. When Xft.dpi is set, the
// scale is the same on every monitor (the same as GTK on X11), so a move
// never changes it.
func (b *Backend) maybeRescaleDPI() bool {
	if !b.plat.perMonitorDPI() {
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

	scale, crtc := dpiScaleForWindow(b.plat.conn, b.plat.root, true, false,
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
