//go:build linux && !js && !android

package gl

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// buildXcursorFile synthesizes a valid Xcursor file with one image
// per given size. Pixels repeat the marker value. The format is
// LSBFirst throughout, as on disk.
func buildXcursorFile(sizes []int, marker uint32) []byte {
	buf := make([]byte, 0, 1024)
	put := func(v uint32) {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], v)
		buf = append(buf, b[:]...)
	}
	put(xcursorMagic)
	put(xcursorFileHeaderLen)
	put(xcursorVersion)
	put(uint32(len(sizes)))
	tocAt := len(buf)
	// Leave room for the TOC; chunk positions are backfilled.
	for range len(sizes) {
		put(0)
		put(0)
		put(0)
	}
	for _, s := range sizes {
		// TOC entry: image type, subtype = size, position.
		pos := uint32(len(buf))
		binary.LittleEndian.PutUint32(buf[tocAt:], xcursorTypeImage)
		binary.LittleEndian.PutUint32(buf[tocAt+4:], uint32(s))
		binary.LittleEndian.PutUint32(buf[tocAt+8:], pos)
		tocAt += xcursorTocEntryLen
		// Chunk: 16-byte header, then width/height/xhot/yhot/delay,
		// then pixels. The header field counts the bytes before the
		// pixels: xcursorImageHeaderLen for well-formed images.
		chunkSize := xcursorImageHeaderLen + s*s*4
		put(uint32(chunkSize))
		put(xcursorTypeImage)
		put(uint32(s))
		put(1) // image format version
		put(uint32(s))
		put(uint32(s))
		put(uint32(s / 2)) // centered hotspot
		put(uint32(s / 2))
		put(0) // delay
		for range s * s {
			put(marker)
		}
	}
	return buf
}

func TestParseXcursorFile(t *testing.T) {
	data := buildXcursorFile([]int{24, 32}, 0x80000000|0xdeadbeef)
	images, err := parseXcursorFile(data)
	if err != nil {
		t.Fatalf("parseXcursorFile: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("got %d images, want 2", len(images))
	}
	for i, want := range []int{24, 32} {
		img := images[i]
		if img.Width != want || img.Height != want {
			t.Errorf("image %d: size %dx%d, want %dx%d", i, img.Width, img.Height, want, want)
		}
		if img.Xhot != want/2 || img.Yhot != want/2 {
			t.Errorf("image %d: hotspot %d,%d, want %d,%d", i, img.Xhot, img.Yhot, want/2, want/2)
		}
		if img.Delay != 0 {
			t.Errorf("image %d: delay %d, want 0", i, img.Delay)
		}
		if len(img.Pixels) != want*want {
			t.Errorf("image %d: %d pixels, want %d", i, len(img.Pixels), want*want)
		}
		for _, px := range img.Pixels {
			if px != 0x80000000|0xdeadbeef {
				t.Errorf("image %d: pixel %#x, want marker", i, px)
				break
			}
		}
	}
}

func TestParseXcursorFileMalformed(t *testing.T) {
	good := buildXcursorFile([]int{16}, 0)
	cases := []struct {
		name string
		mut  func([]byte) []byte
	}{
		{"empty", func(_ []byte) []byte { return nil }},
		{"bad-magic", func(b []byte) []byte { b[0] = 0; return b }},
		{"short-header", func(b []byte) []byte { return b[:8] }},
		{"bad-version", func(b []byte) []byte { b[11] = 2; return b }},
		{"toc-overrun", func(b []byte) []byte { b[15] = 0xff; return b }},
		{"truncated-pixels", func(b []byte) []byte { return b[:len(b)-16] }},
		{"short-chunk-header", func(b []byte) []byte {
			// Chunk header field starts after file header + one TOC entry;
			// a value below the image header length is malformed.
			off := xcursorFileHeaderLen + xcursorTocEntryLen
			for i := range 4 {
				b[off+i] = 0
			}
			return b
		}},
		{"hotspot-outside", func(b []byte) []byte {
			off := xcursorFileHeaderLen + xcursorTocEntryLen +
				xcursorChunkHeaderLen + 8
			b[off+3] = 0xff
			return b
		}},
		{"huge-dimension", func(b []byte) []byte {
			off := xcursorFileHeaderLen + xcursorTocEntryLen +
				xcursorChunkHeaderLen
			b[off] = 0x01 // width = 0x01000000
			return b
		}},
	}
	for _, c := range cases {
		data := c.mut(append([]byte(nil), good...))
		if _, err := parseXcursorFile(data); err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}

func TestBestFit(t *testing.T) {
	img := func(w int) xcursorImage { return xcursorImage{Width: w, Height: w} }
	cases := []struct {
		name   string
		images []xcursorImage
		size   int
		want   int // -1 = no result
	}{
		{"exact", []xcursorImage{img(16), img(24), img(32)}, 24, 24},
		{"between-sizes", []xcursorImage{img(16), img(32)}, 24, 16},
		{"all-larger", []xcursorImage{img(32), img(48)}, 24, 32},
		{"all-smaller", []xcursorImage{img(8), img(16)}, 48, 16},
		{"single", []xcursorImage{img(24)}, 100, 24},
		{"none", nil, 24, -1},
	}
	for _, c := range cases {
		got, ok := bestFit(c.images, c.size)
		if c.want == -1 {
			if ok {
				t.Errorf("%s: expected no image", c.name)
			}
			continue
		}
		if !ok || got.Width != c.want {
			t.Errorf("%s: got %d (ok=%v), want %d", c.name, got.Width, ok, c.want)
		}
	}
}

// buildXSettings synthesizes a _XSETTINGS_SETTINGS blob in the given
// byte order (msbFirst) from name/value pairs. val "color" emits a
// color-typed setting.
func buildXSettings(msbFirst bool, settings map[string]string) []byte {
	order := binary.ByteOrder(binary.LittleEndian)
	if msbFirst {
		order = binary.BigEndian
	}
	buf := make([]byte, 0, 256)
	put := func(v uint32) {
		var b [4]byte
		order.PutUint32(b[:], v)
		buf = append(buf, b[:]...)
	}
	if msbFirst {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	put(0) // serial
	put(uint32(len(settings)))
	putStr := func(s string) {
		put(uint32(len(s)))
		buf = append(buf, s...)
		if n := len(buf) % 4; n != 0 {
			buf = append(buf, make([]byte, 4-n)...)
		}
	}
	for name, val := range settings {
		if val == "color" {
			buf = append(buf, 2) // color type, 8 bytes payload
			put(1)
			buf = append(buf, make([]byte, 4)...)
			put(uint32(len(name)))
			buf = append(buf, name...)
			if n := len(buf) % 4; n != 0 {
				buf = append(buf, make([]byte, 4-n)...)
			}
			continue
		}
		buf = append(buf, 1) // string type
		putStr(name)
		putStr(val)
	}
	return buf
}

func TestParseXSettings(t *testing.T) {
	want := map[string]string{
		"Gtk/CursorThemeName": "Adwaita",
		"Gtk/CursorThemeSize": "48",
		"Gtk/ColorScheme":     "color", // must be skipped, not returned
	}
	for name, msb := range map[string]bool{
		"big-endian":    true,
		"little-endian": false,
	} {
		got := parseXSettings(buildXSettings(msb, want))
		for k, v := range want {
			if v == "color" {
				if _, ok := got[k]; ok {
					t.Errorf("%s: color setting %s leaked into result", name, k)
				}
				continue
			}
			if got[k] != v {
				t.Errorf("%s: %s=%q, want %q", name, k, got[k], v)
			}
		}
	}
}

func TestParseXSettingsTruncated(t *testing.T) {
	// An empty blob and a blob cut mid-name must not panic or spin,
	// and must yield nothing usable.
	if got := parseXSettings(nil); len(got) != 0 {
		t.Errorf("nil blob: got %v, want empty", got)
	}
	data := buildXSettings(false, map[string]string{"Gtk/CursorThemeName": "X"})
	if got := parseXSettings(data[:30]); len(got) != 0 {
		t.Errorf("truncated blob: got %v, want empty", got)
	}
}

func TestIniValue(t *testing.T) {
	data := []byte(`
# comment
[Settings]
gtk-cursor-theme-name=Yaru
gtk-cursor-theme-size = 32

[Other]
gtk-cursor-theme-name=wrong
`)
	if got := iniValue(data, "Settings", "gtk-cursor-theme-name"); got != "Yaru" {
		t.Errorf("name: got %q, want Yaru", got)
	}
	if got := iniValue(data, "Settings", "gtk-cursor-theme-size"); got != "32" {
		t.Errorf("size: got %q, want 32", got)
	}
	if got := iniValue(data, "Settings", "missing"); got != "" {
		t.Errorf("missing key: got %q, want empty", got)
	}
	if got := iniValue(data, "Other", "gtk-cursor-theme-name"); got != "wrong" {
		t.Errorf("other section: got %q, want wrong", got)
	}
}

func TestThemeInherits(t *testing.T) {
	data := []byte("[Icon Theme]\nName=X\nInherits=B, A, C\n")
	got := themeInherits(data)
	want := []string{"B", "A", "C"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if got := themeInherits([]byte("[Icon Theme]\nInherits=\n")); got != nil {
		t.Errorf("empty inherits: got %v, want nil", got)
	}
}

// writeTheme creates a cursor theme dir under root with index.theme
// (optionally inheriting) and the given cursor files (name → sizes).
func writeTheme(t *testing.T, root, theme string, inherits []string, cursors map[string][]int) {
	t.Helper()
	dir := filepath.Join(root, theme)
	if inherits != nil {
		_ = os.MkdirAll(dir, 0o755)
		var b strings.Builder
		b.WriteString("[Icon Theme]\nName=")
		b.WriteString(theme)
		b.WriteString("\nInherits=")
		b.WriteString(strings.Join(inherits, ","))
		b.WriteString("\n")
		_ = os.WriteFile(filepath.Join(dir, "index.theme"), []byte(b.String()), 0o644)
	}
	for name, sizes := range cursors {
		cur := filepath.Join(dir, "cursors")
		_ = os.MkdirAll(cur, 0o755)
		_ = os.WriteFile(filepath.Join(cur, name), buildXcursorFile(sizes, 0xffffffff), 0o644)
	}
}

func TestFindThemeCursor(t *testing.T) {
	root := t.TempDir()
	// Base theme provides the cursor; child theme inherits it.
	writeTheme(t, root, "base", nil, map[string][]int{"size_fdiag": {24}})
	writeTheme(t, root, "child", []string{"base"}, nil)

	img, err := findThemeCursor(nwseCursorNames, 24, []string{root}, "child")
	if err != nil {
		t.Fatalf("inherited lookup: %v", err)
	}
	if img.Width != 24 {
		t.Errorf("got %dx%d, want 24x24", img.Width, img.Height)
	}

	// Adwaita-style alias naming.
	writeTheme(t, root, "adwaita", nil, map[string][]int{"nwse-resize": {32}})
	img, err = findThemeCursor(nwseCursorNames, 24, []string{root}, "adwaita")
	if err != nil {
		t.Fatalf("alias lookup: %v", err)
	}
	if img.Width != 32 {
		t.Errorf("alias: got %d, want 32", img.Width)
	}

	// Theme misses but the default theme (with its own inheritance)
	// provides the cursor.
	writeTheme(t, root, "default", []string{"base"}, nil)
	img, err = findThemeCursor(nwseCursorNames, 24, []string{root}, "missing")
	if err != nil {
		t.Fatalf("default fallback: %v", err)
	}
	if img.Width != 24 {
		t.Errorf("default fallback: got %d, want 24", img.Width)
	}

	// Nothing anywhere → error.
	if _, err := findThemeCursor(neswCursorNames, 24, []string{root}, "missing"); err == nil {
		t.Error("expected error when no theme provides the cursor")
	}
}

func TestFindThemeCursorCycle(t *testing.T) {
	root := t.TempDir()
	writeTheme(t, root, "a", []string{"b"}, nil)
	writeTheme(t, root, "b", []string{"a"}, nil)
	writeTheme(t, root, "c", []string{"c"}, map[string][]int{"size_bdiag": {16}})
	img, err := findThemeCursor(neswCursorNames, 16, []string{root}, "c")
	if err != nil {
		t.Fatalf("self-inheriting theme: %v", err)
	}
	if img.Width != 16 {
		t.Errorf("got %d, want 16", img.Width)
	}
	// Mutual cycle without a cursor anywhere must terminate with an error.
	if _, err := findThemeCursor(neswCursorNames, 16, []string{root}, "a"); err == nil {
		t.Error("expected error for cycle with no cursor")
	}
}

func TestSettingsIniValue(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "gtk-3.0"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "gtk-3.0", "settings.ini"),
		[]byte("[Settings]\ngtk-cursor-theme-name=Bibata-Modern-Classic\n"), 0o644)
	if got := settingsIniValue(dir, "gtk-cursor-theme-name"); got != "Bibata-Modern-Classic" {
		t.Errorf("got %q, want Bibata-Modern-Classic", got)
	}
	if got := settingsIniValue(dir, "gtk-cursor-theme-size"); got != "" {
		t.Errorf("size: got %q, want empty", got)
	}
	// gtk-4.0 is consulted when gtk-3.0 is absent.
	dir2 := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir2, "gtk-4.0"), 0o755)
	_ = os.WriteFile(filepath.Join(dir2, "gtk-4.0", "settings.ini"),
		[]byte("[Settings]\ngtk-cursor-theme-size=48\n"), 0o644)
	if got := settingsIniValue(dir2, "gtk-cursor-theme-size"); got != "48" {
		t.Errorf("gtk-4.0 size: got %q, want 48", got)
	}
}

// TestLoadCursorFileTooLarge verifies the size pre-check rejects an
// oversized cursor file before it is read into memory.
func TestLoadCursorFileTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "size_fdiag")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, xcursorMaxFileSize+1); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCursorFile(path, 24); err == nil {
		t.Error("expected error for oversized cursor file")
	}
}

func TestXcursorSearchDirs(t *testing.T) {
	home := t.TempDir()
	want := []string{
		filepath.Join(home, ".icons"),
		filepath.Join(home, ".local", "share", "icons"),
		"/usr/local/share/icons", "/usr/share/icons",
		"/usr/local/share/cursors", "/usr/share/cursors",
		"/usr/share/cursors/xorg-x11",
		filepath.Join(home, ".cursor"),
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_DATA_DIRS", "")
	t.Setenv("XCURSOR_PATH", "")
	got := xcursorSearchDirs()
	if len(got) != len(want) {
		t.Fatalf("got %d dirs %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dir[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// XCURSOR_PATH replaces the XDG_DATA_DIRS branch and wins over it.
	t.Setenv("XCURSOR_PATH", "/opt/cursors:/opt/more")
	t.Setenv("XDG_DATA_DIRS", "/custom/share")
	got = xcursorSearchDirs()
	if got[2] != "/opt/cursors" || got[3] != "/opt/more" {
		t.Errorf("XCURSOR_PATH not honored: %v", got)
	}
	for _, d := range got {
		if d == "/custom/share/icons" {
			t.Errorf("XDG_DATA_DIRS consulted despite XCURSOR_PATH: %v", got)
		}
	}
}

// TestXcursorThemeCursorLive exercises theme resolution and the RENDER
// cursor upload against the live X server. Opt-in via GOGUI_X11_IT=1.
// Skips when no cursor theme is installed (the glyph fallback in
// loadCursors covers that case).
func TestXcursorThemeCursorLive(t *testing.T) {
	if os.Getenv("GOGUI_X11_IT") == "" {
		t.Skip("set GOGUI_X11_IT=1 to run the live X11 cursor test")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY; live cursor test needs an X server")
	}
	conn, err := xgb.NewConn()
	if err != nil {
		t.Skipf("x11 connect: %v", err)
	}
	defer conn.Close()
	root := xproto.Setup(conn).DefaultScreen(conn).Root

	theme := xcursorThemeName(conn, root)
	size := xcursorThemeSize(conn, root)
	t.Logf("theme=%q size=%d", theme, size)
	if theme == "" || size <= 0 {
		t.Fatalf("theme=%q size=%d, want non-empty positive", theme, size)
	}

	for _, names := range [][]string{nwseCursorNames, neswCursorNames} {
		img, err := findThemeCursor(names, size, xcursorSearchDirs(), theme)
		if err != nil {
			t.Skipf("no themed %s cursor (fallback glyphs in use): %v", names[0], err)
		}
		t.Logf("%s: %dx%d hotspot %d,%d", names[0], img.Width, img.Height, img.Xhot, img.Yhot)
		cursor := createRenderCursor(conn, root, img)
		if cursor == 0 {
			t.Fatalf("%s: createRenderCursor returned 0", names[0])
		}
		xproto.FreeCursor(conn, cursor)
	}
}
