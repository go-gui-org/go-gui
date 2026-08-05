//go:build linux && !js && !android

package gl

// cursors_x11.go — server-side wiring for themed Xcursor cursors.
// Loads the diagonal resize images parsed in xcursor.go and uploads
// them via the RENDER extension. Split from platform_x11.go to stay
// under the 800-line gate for gui/ source files.

import (
	"os"
	"strconv"
	"strings"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/render"
	"github.com/jezek/xgb/xproto"
)

// xcursorThemeCursor loads the best-fitting image for one of the
// logical cursor names from the active Xcursor theme and uploads it to
// the server as a RENDER cursor. Returns 0 when no theme image is
// available or the RENDER extension is missing, so the caller can fall
// back to a font glyph.
func xcursorThemeCursor(p *platformState, theme string, size int, names []string) xproto.Cursor {
	img, err := findThemeCursor(names, size, xcursorSearchDirs(), theme)
	if err != nil {
		return 0
	}
	return createRenderCursor(p.conn, p.root, img)
}

// xcursorThemeName resolves the active cursor theme the way GTK does:
// the XCURSOR_THEME env var, the XSETTINGS property (GNOME), the
// Xcursor.theme X resource (classic .Xresources), the GTK settings.ini
// files (KDE, manual config), then the "default" theme.
func xcursorThemeName(conn *xgb.Conn, root xproto.Window) string {
	if t := os.Getenv("XCURSOR_THEME"); t != "" {
		return t
	}
	if v := readXSettings(conn, root)["Gtk/CursorThemeName"]; v != "" {
		return v
	}
	if t := readXResource(conn, root, "Xcursor.theme"); t != "" {
		return t
	}
	if dir, err := os.UserConfigDir(); err == nil {
		if t := settingsIniValue(dir, "gtk-cursor-theme-name"); t != "" {
			return t
		}
	}
	return "default"
}

// xcursorThemeSize resolves the preferred cursor size: XCURSOR_SIZE,
// the XSETTINGS property, the Xcursor.size X resource, the GTK
// settings.ini files, then the Xcursor spec default of 24.
func xcursorThemeSize(conn *xgb.Conn, root xproto.Window) int {
	if v := os.Getenv("XCURSOR_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if v := readXSettings(conn, root)["Gtk/CursorThemeSize"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if v := readXResource(conn, root, "Xcursor.size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if dir, err := os.UserConfigDir(); err == nil {
		if v := settingsIniValue(dir, "gtk-cursor-theme-size"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return n
			}
		}
	}
	return 24
}

// readXSettings fetches and decodes the _XSETTINGS_SETTINGS property
// from the root window (set by GNOME's settings daemon and xsettingsd).
// Returns nil when absent or unreadable.
func readXSettings(conn *xgb.Conn, root xproto.Window) map[string]string {
	atom := internAtom(conn, "_XSETTINGS_SETTINGS")
	if atom == 0 {
		return nil
	}
	reply, err := xproto.GetProperty(conn, false, root, atom,
		atom, 0, 1<<20).Reply()
	if err != nil || reply == nil {
		return nil
	}
	return parseXSettings(reply.Value)
}

// readXResource returns the value of an X resource (e.g.
// "Xcursor.theme") from the root RESOURCE_MANAGER property. Returns ""
// when absent.
func readXResource(conn *xgb.Conn, root xproto.Window, key string) string {
	atom := internAtom(conn, "RESOURCE_MANAGER")
	if atom == 0 {
		return ""
	}
	reply, err := xproto.GetProperty(conn, false, root, atom,
		xproto.AtomString, 0, 1<<16).Reply()
	if err != nil || reply == nil || len(reply.Value) == 0 {
		return ""
	}
	for line := range strings.SplitSeq(string(reply.Value), "\n") {
		k, val, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		return strings.TrimSpace(val)
	}
	return ""
}

// createRenderCursor uploads img to the X server via the RENDER
// extension and returns a cursor with its hotspot. The image becomes a
// server-side ARGB32 picture; the client-side pixmap, GC, and picture
// are released before returning. Returns 0 on any failure.
func createRenderCursor(conn *xgb.Conn, root xproto.Window, img *xcursorImage) xproto.Cursor {
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return 0
	}
	if render.Init(conn) != nil {
		return 0
	}
	format, ok := renderPictFormatARGB32(conn)
	if !ok {
		return 0
	}

	pixID, err := conn.NewId()
	if err != nil {
		return 0
	}
	gcID, err := conn.NewId()
	if err != nil {
		return 0
	}
	pix := xproto.Pixmap(pixID)
	gc := xproto.Gcontext(gcID)
	xproto.CreatePixmap(conn, 32, pix, xproto.Drawable(root),
		uint16(img.Width), uint16(img.Height))
	xproto.CreateGC(conn, gc, xproto.Drawable(pix), 0, nil)

	// Pixels are host-endian; the server expects its own image byte
	// order (LSBFirst on x86). Swap only when the server is MSBFirst.
	data := make([]byte, len(img.Pixels)*4)
	if xproto.Setup(conn).ImageByteOrder == xproto.ImageOrderMSBFirst {
		for i, px := range img.Pixels {
			data[i*4] = byte(px >> 24)
			data[i*4+1] = byte(px >> 16)
			data[i*4+2] = byte(px >> 8)
			data[i*4+3] = byte(px)
		}
	} else {
		for i, px := range img.Pixels {
			data[i*4] = byte(px)
			data[i*4+1] = byte(px >> 8)
			data[i*4+2] = byte(px >> 16)
			data[i*4+3] = byte(px >> 24)
		}
	}
	xproto.PutImage(conn, xproto.ImageFormatZPixmap, xproto.Drawable(pix), gc,
		uint16(img.Width), uint16(img.Height), 0, 0, 0, 32, data)

	pict, err := render.NewPictureId(conn)
	if err != nil {
		xproto.FreeGC(conn, gc)
		xproto.FreePixmap(conn, pix)
		return 0
	}
	render.CreatePicture(conn, pict, xproto.Drawable(pix), format, 0, nil)
	cid, err := conn.NewId()
	if err != nil {
		render.FreePicture(conn, pict)
		xproto.FreeGC(conn, gc)
		xproto.FreePixmap(conn, pix)
		return 0
	}
	cursor := xproto.Cursor(cid)
	render.CreateCursor(conn, cursor, pict, uint16(img.Xhot), uint16(img.Yhot))

	// The cursor keeps a server-side reference to the pixels, so the
	// upload scaffolding can be torn down.
	render.FreePicture(conn, pict)
	xproto.FreeGC(conn, gc)
	xproto.FreePixmap(conn, pix)
	return cursor
}

// renderPictFormatARGB32 finds the ARGB32 picture format in the
// RENDER extension's format list: direct, 32-bit, with the standard
// 8-bit channels at shifts 24/16/8/0.
func renderPictFormatARGB32(conn *xgb.Conn) (render.Pictformat, bool) {
	reply, err := render.QueryPictFormats(conn).Reply()
	if err != nil || reply == nil {
		return 0, false
	}
	for _, f := range reply.Formats {
		d := f.Direct
		if f.Type == render.PictTypeDirect && f.Depth == 32 &&
			d.AlphaShift == 24 && d.AlphaMask == 0xff &&
			d.RedShift == 16 && d.RedMask == 0xff &&
			d.GreenShift == 8 && d.GreenMask == 0xff &&
			d.BlueShift == 0 && d.BlueMask == 0xff {
			return f.Id, true
		}
	}
	return 0, false
}
