//go:build linux && !js && !android

package gl

// xcursor.go — pure-Go Xcursor cursor-file parsing and theme resolution.
//
// Cursor shapes are loaded from the desktop's Xcursor theme (the same
// .cursor files GTK renders) and uploaded via the RENDER extension's
// CreateCursor. The X11 core cursor font ignores the Xcursor size and
// never scales for HiDPI, so it is only the fallback when no themed
// cursor is available (issue #453). Everything in this file is X-free
// so it can be unit-tested headlessly; the X wiring lives in
// platform_x11.go.
//
// Format reference: the Xcursor library spec (magic "Xcur", LSBFirst
// 32-bit header/TOC, chunk types 0xfffd0001=comment, 0xfffd0002=image
// with ARGB32 premultiplied pixels). Theme resolution follows
// libXcursor: XCURSOR_THEME env, XSETTINGS, GTK settings.ini, the
// Xcursor.* X resources, then the "default" theme with index.theme
// Inherits chains.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Xcursor file constants. The format is stored LSBFirst (the magic
// constant is the little-endian reading of "Xcur"); see
// /usr/include/X11/Xcursor/Xcursor.h.
const (
	xcursorMagic            = 0x72756358 // "Xcur", LSBFirst
	xcursorVersion          = 0x00010000
	xcursorTypeComment      = 0xfffd0001
	xcursorTypeImage        = 0xfffd0002
	xcursorFileHeaderLen    = 16 // magic + header + version + ntoc
	xcursorTocEntryLen      = 12 // type + subtype + position
	xcursorChunkHeaderLen   = 16 // header + type + subtype + version
	xcursorImageHeaderLen   = 36 // chunk header + width/height/xhot/yhot/delay
	xcursorMaxImageDim      = 256
	xcursorMaxStringSetting = 4096     // XSETTINGS string value cap
	xcursorMaxFileSize      = 16 << 20 // sanity cap on cursor files (real ones are < 200 KB)
)

// xcursorImage is a single cursor image frame. Pixels are ARGB32 with
// premultiplied alpha, as the on-disk format (and the ARGB32 picture
// format) expects.
type xcursorImage struct {
	Width, Height int
	Xhot, Yhot    int
	Delay         uint32
	Pixels        []uint32
}

// parseXcursorFile decodes an Xcursor file into its image frames.
// Returns an error when the file is malformed in any way — callers
// treat that as "no themed cursor available".
func parseXcursorFile(data []byte) ([]xcursorImage, error) {
	if len(data) < xcursorFileHeaderLen {
		return nil, fmt.Errorf("xcursor: file too short (%d bytes)", len(data))
	}
	if binary.LittleEndian.Uint32(data[0:4]) != xcursorMagic {
		return nil, errors.New("xcursor: bad magic")
	}
	if v := binary.LittleEndian.Uint32(data[8:12]); v != xcursorVersion {
		return nil, fmt.Errorf("xcursor: unsupported version %d", v)
	}
	ntoc := binary.LittleEndian.Uint32(data[12:16])
	if int(ntoc)*xcursorTocEntryLen > len(data)-xcursorFileHeaderLen {
		return nil, fmt.Errorf("xcursor: %d toc entries overrun file", ntoc)
	}
	toc := data[xcursorFileHeaderLen:]
	// Chunk spans are delimited by the next TOC entry's position
	// (the per-chunk header field is a fixed layout marker, not a
	// length), so collect every entry, sorted by position. The
	// prealloc is capped: ntoc is bounded by the file size, and
	// oversized TOCs just fail later in parsing.
	entries := make([]tocEntry, 0, min(int(ntoc), 1024))
	for i := range int(ntoc) {
		entry := toc[i*xcursorTocEntryLen : (i+1)*xcursorTocEntryLen]
		entries = append(entries, tocEntry{
			typ: binary.LittleEndian.Uint32(entry[0:4]),
			pos: binary.LittleEndian.Uint32(entry[8:12]),
		})
	}
	slices.SortFunc(entries, func(a, b tocEntry) int {
		return int(a.pos) - int(b.pos)
	})
	images := make([]xcursorImage, 0, 4)
	for i, e := range entries {
		if e.typ != xcursorTypeImage {
			continue // comments and unknown chunk types are skippable
		}
		end := uint32(len(data))
		if i+1 < len(entries) && entries[i+1].pos < end {
			end = entries[i+1].pos
		}
		img, err := parseXcursorChunk(data, e.pos, end)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	if len(images) == 0 {
		return nil, errors.New("xcursor: no images in file")
	}
	return images, nil
}

// tocEntry is one decoded table-of-contents entry.
type tocEntry struct {
	typ, pos uint32
}

// parseXcursorChunk decodes the image chunk starting at pos, whose
// data runs until end. All bounds are validated before anything is
// read. Layout: 16-byte chunk header (length marker, type, subtype,
// version), then width/height/xhot/yhot/delay, then ARGB pixels.
func parseXcursorChunk(data []byte, pos, end uint32) (xcursorImage, error) {
	var img xcursorImage
	if int(pos)+xcursorImageHeaderLen > int(end) || int(end) > len(data) {
		return img, fmt.Errorf("xcursor: chunk at %d outside file", pos)
	}
	chunk := data[pos:end]
	// The header field marks the layout (36 for version-1 images);
	// anything shorter cannot hold the image fields.
	if binary.LittleEndian.Uint32(chunk[0:4]) < xcursorImageHeaderLen {
		return img, fmt.Errorf("xcursor: short chunk header at %d", pos)
	}
	if t := binary.LittleEndian.Uint32(chunk[4:8]); t != xcursorTypeImage {
		return img, fmt.Errorf("xcursor: chunk at %d is not an image", pos)
	}
	p := chunk[xcursorChunkHeaderLen:]
	w := int(binary.LittleEndian.Uint32(p[0:4]))
	h := int(binary.LittleEndian.Uint32(p[4:8]))
	if w <= 0 || h <= 0 || w > xcursorMaxImageDim || h > xcursorMaxImageDim {
		return img, fmt.Errorf("xcursor: bad image size %dx%d at %d", w, h, pos)
	}
	img.Width, img.Height = w, h
	img.Xhot = int(binary.LittleEndian.Uint32(p[8:12]))
	img.Yhot = int(binary.LittleEndian.Uint32(p[12:16]))
	img.Delay = binary.LittleEndian.Uint32(p[16:20])
	if img.Xhot > w || img.Yhot > h {
		return img, fmt.Errorf("xcursor: hotspot %d,%d outside image at %d",
			img.Xhot, img.Yhot, pos)
	}
	pixBytes := w * h * 4
	if len(p)-20 < pixBytes {
		return img, fmt.Errorf("xcursor: pixel data truncated at %d", pos)
	}
	px := p[20:]
	img.Pixels = make([]uint32, w*h)
	for i := range img.Pixels {
		img.Pixels[i] = binary.LittleEndian.Uint32(px[i*4:])
	}
	return img, nil
}

// scaledCursorSize converts the logical Xcursor size (logical pixels)
// to the physical-pixel size the monitor's scale needs. scale ≤ 0
// means "unknown" and leaves the size untouched; otherwise it
// multiplies, rounds to the nearest integer, and clamps to ≥ 1 so a
// fractional scale never rounds down to zero.
func scaledCursorSize(size int, scale float32) int {
	if scale <= 0 {
		return size
	}
	return max(1, int(float64(size)*float64(scale)+0.5))
}

// bestFit selects the image libXcursor would render for size: the
// largest image no larger than size, or the smallest when every image
// is larger. Images are compared by width, as the format is square.
func bestFit(images []xcursorImage, size int) (xcursorImage, bool) {
	if len(images) == 0 {
		return xcursorImage{}, false
	}
	best, bestDim := 0, images[0].Width
	for i := range images {
		dim := images[i].Width
		switch {
		case dim <= size && (bestDim > size || dim > bestDim):
			best, bestDim = i, dim // fits, and better than current pick
		case dim > size && bestDim > size && dim < bestDim:
			best, bestDim = i, dim // all too big: take the smallest
		}
	}
	return images[best], true
}

// parseXSettings decodes a _XSETTINGS_SETTINGS property value into a
// name→value map. Integer settings are stringified; colors are
// ignored. Malformed data stops parsing and returns what was read —
// the blob is advisory only.
func parseXSettings(data []byte) map[string]string {
	out := make(map[string]string)
	if len(data) < 9 { // byte order + serial + count
		return out
	}
	order := binary.ByteOrder(binary.BigEndian)
	if data[0] == 0 {
		order = binary.LittleEndian
	}
	n := order.Uint32(data[5:9])
	p := data[9:]
	// align returns the bytes needed to reach a 4-byte boundary from
	// the start of the blob — the spec pads relative to the blob, not
	// to each field (see gdk's xsettings parse).
	align := func() int { return (4 - (len(data)-len(p))%4) % 4 }
	for range int(n) {
		if len(p) < 5 {
			return out
		}
		typ := p[0]
		nameLen := order.Uint32(p[1:5])
		p = p[5:]
		if nameLen == 0 || nameLen > 256 || len(p) < int(nameLen) {
			return out
		}
		name := string(p[:nameLen])
		p = p[nameLen:]
		// Padding bytes are optional at EOF; skip them when present.
		if a := align(); a > len(p) {
			return out
		} else {
			p = p[a:]
		}
		switch typ {
		case 0: // integer
			if len(p) < 4 {
				return out
			}
			out[name] = strconv.FormatUint(uint64(order.Uint32(p)), 10)
			p = p[4:]
		case 1: // string
			if len(p) < 4 {
				return out
			}
			vlen := order.Uint32(p)
			p = p[4:]
			if vlen > xcursorMaxStringSetting || len(p) < int(vlen) {
				return out
			}
			out[name] = string(p[:vlen])
			p = p[vlen:]
			if a := align(); a > len(p) {
				return out
			} else {
				p = p[a:]
			}
		case 2: // color: 4 x CARD16, nothing useful for cursors
			if len(p) < 8 {
				return out
			}
			p = p[8:]
		default:
			return out
		}
	}
	return out
}

// iniValue returns the value of key in the named section of an
// INI-style file (index.theme, gtk settings.ini), or "" when absent.
// Comment lines start with '#'.
func iniValue(data []byte, section, key string) string {
	inSection := false
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = line == "["+section+"]"
			continue
		}
		if !inSection {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// settingsIniValue reads key from the [Settings] section of
// gtk-3.0/settings.ini then gtk-4.0/settings.ini under configDir
// (KDE and manual GTK configs set the cursor theme there).
func settingsIniValue(configDir, key string) string {
	for _, sub := range []string{"gtk-3.0", "gtk-4.0"} {
		// #nosec G304 — path is a fixed subdir under the caller's own
		// config dir; only the user's own settings file is read.
		data, err := os.ReadFile(filepath.Join(configDir, sub, "settings.ini"))
		if err != nil {
			continue
		}
		if v := iniValue(data, "Settings", key); v != "" {
			return v
		}
	}
	return ""
}

// themeInherits returns the comma-separated Inherits list of an
// [Icon Theme] index.theme file.
func themeInherits(data []byte) []string {
	v := iniValue(data, "Icon Theme", "Inherits")
	if v == "" {
		return nil
	}
	var out []string
	for t := range strings.SplitSeq(v, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// xcursorSearchDirs returns the cursor-theme search directories,
// most specific first: ~/.icons, the XDG data dirs, the XCURSOR_PATH
// override, then the legacy system paths from the Xcursor spec.
func xcursorSearchDirs() []string {
	var dirs []string
	home, _ := os.UserHomeDir()
	if home != "" {
		dirs = append(dirs, filepath.Join(home, ".icons"))
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		dirs = append(dirs, filepath.Join(xdg, "icons"))
	} else if home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "icons"))
	}
	if path := os.Getenv("XCURSOR_PATH"); path != "" {
		dirs = append(dirs, filepath.SplitList(path)...)
	} else if xdg := os.Getenv("XDG_DATA_DIRS"); xdg != "" {
		for _, d := range filepath.SplitList(xdg) {
			if d != "" {
				dirs = append(dirs, filepath.Join(d, "icons"))
			}
		}
	} else {
		dirs = append(dirs, "/usr/local/share/icons", "/usr/share/icons")
	}
	dirs = append(dirs,
		"/usr/local/share/cursors", "/usr/share/cursors",
		"/usr/share/cursors/xorg-x11",
	)
	if home != "" {
		dirs = append(dirs, filepath.Join(home, ".cursor"))
	}
	return dirs
}

// Logical Xcursor names per cursor shape, tried in order per theme
// dir. Themes use the Wayland-era names (Adwaita, Yaru) or one of the
// classic libXcursor aliases (Bibata, XCursor-Pro, DMZ); the historical
// core-font-derived names are last resorts. The lists cover every
// shape the backend can show: the core cursor font ignores both the
// Xcursor size and HiDPI scaling, so it is only a fallback (issue
// #453).
var (
	leftPtrCursorNames    = []string{"left_ptr", "default", "arrow", "top_left_arrow"}
	xtermCursorNames      = []string{"xterm", "ibeam", "text"}
	crosshairCursorNames  = []string{"crosshair", "cross", "cross_reverse", "diamond_cross", "tcross"}
	handCursorNames       = []string{"hand2", "pointer", "hand1", "pointing_hand"}
	hResizeCursorNames    = []string{"sb_h_double_arrow", "ew-resize", "col-resize", "h_double_arrow", "split_h", "size_hor"}
	vResizeCursorNames    = []string{"sb_v_double_arrow", "ns-resize", "row-resize", "v_double_arrow", "split_v", "size_ver"}
	nwseCursorNames       = []string{"nwse-resize", "size_fdiag", "bd_double_arrow"}
	neswCursorNames       = []string{"nesw-resize", "size_bdiag", "fd_double_arrow"}
	moveCursorNames       = []string{"fleur", "all-scroll", "move", "size_all"}
	notAllowedCursorNames = []string{"X_cursor", "not-allowed", "circle", "crossed_circle", "no-drop"}
)

// themeInheritsOf returns the Inherits chain of theme from the first
// readable index.theme (theme dir or its cursors subdir) across dirs.
func themeInheritsOf(dirs []string, theme string) []string {
	for _, d := range dirs {
		base := filepath.Join(d, theme)
		for _, rel := range []string{"index.theme", filepath.Join("cursors", "index.theme")} {
			// #nosec G304 — dirs are the standard theme paths plus the
			// user's own env vars; only the user's own files are read.
			data, err := os.ReadFile(filepath.Join(base, rel))
			if err != nil {
				continue
			}
			if list := themeInherits(data); len(list) > 0 {
				return list
			}
		}
	}
	return nil
}

// loadCursorFile reads and parses path, returning the image that best
// fits size. err is set when the file is missing or malformed. The
// size is checked before reading so an absurdly large (or hostile)
// file is never slurped into memory.
func loadCursorFile(path string, size int) (*xcursorImage, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > xcursorMaxFileSize {
		return nil, fmt.Errorf("xcursor: %s too large (%d bytes)", path, info.Size())
	}
	// #nosec G304 — path is a fixed name inside the theme dirs the
	// user's own environment selects; the file is only read, never
	// written or executed.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	images, err := parseXcursorFile(data)
	if err != nil {
		return nil, err
	}
	img, ok := bestFit(images, size)
	if !ok {
		return nil, fmt.Errorf("xcursor: no images in %s", path)
	}
	return &img, nil
}

// findThemeCursor loads the best-fitting image for one of the logical
// names from theme and its Inherits chain, falling back to the
// "default" theme (which itself may inherit a real theme). Returns an
// error when no theme provides the cursor.
func findThemeCursor(names []string, size int, dirs []string, theme string) (*xcursorImage, error) {
	seen := make(map[string]bool)
	queue := []string{theme}
	if theme != "default" {
		queue = append(queue, "default")
	}
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		if seen[t] {
			continue
		}
		seen[t] = true
		for _, d := range dirs {
			dir := filepath.Join(d, t, "cursors")
			for _, name := range names {
				img, err := loadCursorFile(filepath.Join(dir, name), size)
				if err == nil {
					return img, nil
				}
			}
		}
		queue = append(queue, themeInheritsOf(dirs, t)...)
	}
	return nil, fmt.Errorf("xcursor: no %s image in theme %s", names[0], theme)
}
