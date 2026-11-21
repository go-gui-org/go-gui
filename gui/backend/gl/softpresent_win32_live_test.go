//go:build windows && !js

package gl

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// Live tests of the GOGUI_RENDERER=soft Win32 path (#1310). They open real
// windows and read them back through a window DC, so they need a desktop
// session; a headless runner that cannot create a window skips them, or
// fails them when GOGUI_REQUIRE_GL is set (tools/glrendertest).

var (
	pGetCurrentProcess = kernel32.NewProc("GetCurrentProcess")
	pGetGuiResources   = user32.NewProc("GetGuiResources")
	pInvalidateRect    = user32.NewProc("InvalidateRect")
	pPatBlt            = gdi32.NewProc("PatBlt")
	pGetLayeredAttrs   = user32.NewProc("GetLayeredWindowAttributes")
)

const (
	grGDIObjects = 0
	dstInvert    = 0x00550009
	// swpResize changes only the size: no move, no z-order, no activation.
	swpResize = swpNoMove | swpNoZOrder | swpNoActivate
)

// softWin32Skip skips, or fails when GOGUI_REQUIRE_GL is set: the
// glrendertest selections set it, so a runner that cannot open the window
// shows up as red, not as a silent pass.
func softWin32Skip(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("GOGUI_REQUIRE_GL") != "" {
		t.Fatalf("GOGUI_REQUIRE_GL is set but: "+format, args...)
	}
	t.Skipf(format, args...)
}

// newSoftWin32Test opens a real soft window, as Run would before its loop:
// OnInit has run and the window procedure routes messages to b.
func newSoftWin32Test(t *testing.T, cfg gui.WindowCfg) (*Backend, *gui.Window) {
	t.Helper()
	t.Setenv("GOGUI_RENDERER", "soft")
	w := gui.NewWindow(cfg)
	b, err := New(w)
	if err != nil {
		if strings.Contains(err.Error(), "CreateWindowExW") {
			softWin32Skip(t, "no window: %v", err)
		}
		t.Fatalf("New on soft: %v", err)
	}
	t.Cleanup(b.Destroy)
	if !b.plat.softMode || b.plat.soft == nil || b.plat.hglrc != 0 || b.plat.hdc != 0 {
		t.Fatalf("GOGUI_RENDERER=soft did not select the soft path: softMode %v, "+
			"hglrc %#x, hdc %#x", b.plat.softMode, b.plat.hglrc, b.plat.hdc)
	}
	b.plat.w = w
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	return b, w
}

// softWin32Cfg is a static view (softTextView's content, without
// animation) at a size that fits any screen.
func softWin32Cfg() gui.WindowCfg {
	return gui.WindowCfg{Width: 320, Height: 200,
		OnInit: func(w *gui.Window) { w.SetView(softWin32View) }}
}

// softWin32View is a static frame with shapes and text at several sizes.
func softWin32View(_ *gui.Window) gui.View {
	content := make([]gui.View, 0, 4)
	for _, size := range []float32{11, 16, 24} {
		style := gui.CurrentTheme().TextStyleDef
		style.Size = size
		content = append(content, gui.Text(gui.TextCfg{
			Text:      "Soft on Win32 — 0123456789",
			TextStyle: style,
		}))
	}
	content = append(content, gui.Column(gui.ContainerCfg{
		Width:  120,
		Height: 40,
		Sizing: gui.FixedFixed,
		Radius: gui.RadiusLarge,
		Color:  gui.RGB(70, 130, 220),
	}))
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PaddingMedium,
		Content: content,
	})
}

// frame runs one loop turn's frame call, as Run does.
func frame(b *Backend, w *gui.Window) {
	w.FrameFn()
	b.presentWin32(w)
}

// readWindow reads the top-left width x height pixels of hwnd's client
// area through a window DC, as B, G, R, x bytes, top row first.
func readWindow(t *testing.T, hwnd uintptr, width, height int32) []byte {
	t.Helper()
	wdc, _, _ := pGetDC.Call(hwnd)
	if wdc == 0 {
		t.Fatal("GetDC failed")
	}
	defer pReleaseDC.Call(hwnd, wdc)
	mdc, _, _ := pCreateCompatibleDC.Call(wdc)
	if mdc == 0 {
		t.Fatal("CreateCompatibleDC failed")
	}
	defer pDeleteDC.Call(mdc)
	// Spelled out rather than taken from dibHeader, so a wrong row order
	// in the frame store cannot cancel out here.
	bmi := bitmapInfo{header: bitmapInfoHeader{biSize: 40, biWidth: width,
		biHeight: -height, biPlanes: 1, biBitCount: 32, biCompression: biRGB}}
	var bits unsafe.Pointer
	bmp, err := createDIBSection(mdc, &bmi, &bits)
	if err != nil {
		t.Fatal(err)
	}
	defer pDeleteObject.Call(bmp)
	old, err := selectObject(mdc, bmp)
	if err != nil {
		t.Fatal(err)
	}
	defer pSelectObject.Call(mdc, old)
	if r, _, e := pBitBlt.Call(mdc, 0, 0, uintptr(width), uintptr(height),
		wdc, 0, 0, srcCopy); r == 0 {
		t.Fatalf("BitBlt from the window: %v", e)
	}
	pGdiFlush.Call()
	return append([]byte(nil), unsafe.Slice((*byte)(bits), int(width)*int(height)*4)...)
}

// renderReference draws cfg with soft.RenderToImage at b's scale: what the
// window must show.
func renderReference(t *testing.T, b *Backend, cfg gui.WindowCfg) []byte {
	t.Helper()
	ref := gui.NewWindow(cfg)
	want, err := soft.RenderToImage(ref, b.dpiScale)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	t.Cleanup(func() { soft.Release(ref) })
	// The window truncates a fractional client size and RenderToImage
	// rounds it, so at a scale such as 1.33 there is nothing to compare.
	// Skipped as the X11 test skips a resized window.
	if want.Rect.Dx() != int(b.physW) || want.Rect.Dy() != int(b.physH) {
		t.Skipf("window is %dx%d, reference %dx%d at scale %v",
			b.physW, b.physH, want.Rect.Dx(), want.Rect.Dy(), b.dpiScale)
	}
	return want.Pix
}

// diffBGR counts the pixels where got (B, G, R, x) differs from want
// (R, G, B, A) in color, and returns the first one's index, or -1.
func diffBGR(got, want []byte, pixels int) (count, first int) {
	first = -1
	for i := range pixels {
		g, r := got[i*4:i*4+3], want[i*4:i*4+3]
		if g[0] != r[2] || g[1] != r[1] || g[2] != r[0] {
			if first < 0 {
				first = i
			}
			count++
		}
	}
	return count, first
}

// assertWindowShows fails when b's window does not show want.
func assertWindowShows(t *testing.T, b *Backend, want []byte, when string) {
	t.Helper()
	pw, ph := b.physW, b.physH
	got := readWindow(t, b.plat.hwnd, pw, ph)
	if n, first := diffBGR(got, want, int(pw*ph)); n > 0 {
		t.Fatalf("%s: %d of %d pixels differ; first at (%d,%d): window BGR %v, want RGB %v",
			when, n, pw*ph, first%int(pw), first/int(pw), got[first*4:first*4+3],
			want[first*4:first*4+3])
	}
}

// repaint invalidates rect (nil: the whole client area) and has the real
// WM_PAINT answered at once: UpdateWindow sends it to the window
// procedure, which routes it to b.
func repaint(hwnd uintptr, rect *rectW) {
	pInvalidateRect.Call(hwnd, uintptr(unsafe.Pointer(rect)), 0)
	pUpdateWindow.Call(hwnd)
}

// resizeClient resizes the window so its outer size grows by dw x dh,
// which sends WM_SIZE through the window procedure.
func resizeClient(t *testing.T, hwnd uintptr, dw, dh int32) {
	t.Helper()
	var r rectW
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ok, _, e := pSetWindowPos.Call(hwnd, 0, 0, 0,
		uintptr(r.right-r.left+dw), uintptr(r.bottom-r.top+dh), swpResize); ok == 0 {
		t.Fatalf("SetWindowPos: %v", e)
	}
}

// gdiObjects is the process's GDI object count.
func gdiObjects() uintptr {
	proc, _, _ := pGetCurrentProcess.Call()
	n, _, _ := pGetGuiResources.Call(proc, grGDIObjects)
	return n
}

// openGL32Loaded reports whether any module named opengl32.dll is mapped,
// the system one or a Mesa beside the binary.
func openGL32Loaded() bool {
	h, _, _ := pGetModuleHandleW.Call(uintptr(unsafe.Pointer(
		windows.StringToUTF16Ptr("opengl32.dll"))))
	return h != 0
}

// noGLChildEnv marks the subprocess TestSoftWin32LoadsNoGL starts.
const noGLChildEnv = "GOGUI_TEST_SOFT_NOGL_CHILD"

// TestSoftWin32LoadsNoGL drives every soft entry point — frames, live
// resizes, a scale change, WM_PAINT, a display change, destroy — and
// requires that no opengl32.dll was mapped: GOGUI_RENDERER=soft must not
// load the system DLL, nor Mesa's when it is beside the binary (as in the
// glrendertest forced-soft selection). In a test process where an earlier
// test already loaded the DLL, it reruns itself alone in a subprocess.
func TestSoftWin32LoadsNoGL(t *testing.T) {
	if openGL32Loaded() {
		if os.Getenv(noGLChildEnv) != "" {
			t.Fatal("opengl32.dll was loaded before the test started, in a process that runs only this test")
		}
		cmd := exec.Command(os.Args[0], "-test.run", "^TestSoftWin32LoadsNoGL$", "-test.v") // #nosec G204 -- this test binary, a fixed test name
		cmd.Env = append(os.Environ(), noGLChildEnv+"=1", "GOGUI_RENDERER=soft")
		out, err := cmd.CombinedOutput()
		t.Logf("opengl32.dll already loaded in this process; ran in a subprocess:\n%s", out)
		// A subprocess that cannot open a window skips; with
		// GOGUI_REQUIRE_GL set it fails instead and lands below.
		if err == nil && strings.Contains(string(out), "--- SKIP: TestSoftWin32LoadsNoGL") {
			t.Skip("the subprocess skipped")
		}
		if err != nil || !strings.Contains(string(out), "--- PASS: TestSoftWin32LoadsNoGL") {
			t.Fatalf("subprocess did not pass: %v", err)
		}
		return
	}

	b, w := newSoftWin32Test(t, softWin32Cfg())
	for range 3 {
		frame(b, w)
	}
	resizeClient(t, b.plat.hwnd, 40, 30)
	resizeClient(t, b.plat.hwnd, -40, -30)
	b.applyDPIScale(2)
	if err := b.plat.soft.r.SetScale(b.dpiScale); err != nil {
		t.Fatal(err)
	}
	b.resizeWin32()
	frame(b, w)
	repaint(b.plat.hwnd, nil)
	b.handleMessage(wmDisplayChange, 32, 0)
	frame(b, w)
	var msg msgW
	pumpMessages(&msg)
	if b.plat.hglrc != 0 || b.plat.hdc != 0 {
		t.Fatalf("soft window has GL state: hglrc %#x, hdc %#x", b.plat.hglrc, b.plat.hdc)
	}
	b.Destroy()
	if openGL32Loaded() {
		t.Fatal("the soft path loaded opengl32.dll")
	}
}

// TestSoftWin32PixelsMatchRenderToImage: what the window shows is exactly
// what soft.RenderToImage draws for the same view (RGB; the window is
// opaque), read back through a window DC, so it covers the swizzle and the
// DIB's row order as well as the renderer. Then a patch of the window is
// inverted behind the backend's back and invalidated: the real WM_PAINT
// must put the frame back from the DIB without drawing a new one.
func TestSoftWin32PixelsMatchRenderToImage(t *testing.T) {
	cfg := softWin32Cfg()
	b, w := newSoftWin32Test(t, cfg)
	frame(b, w)
	want := renderReference(t, b, cfg)
	assertWindowShows(t, b, want, "after present")

	patch := rectW{left: 10, top: 10, right: 90, bottom: 60}
	wdc, _, _ := pGetDC.Call(b.plat.hwnd)
	pPatBlt.Call(wdc, uintptr(patch.left), uintptr(patch.top),
		uintptr(patch.right-patch.left), uintptr(patch.bottom-patch.top), dstInvert)
	pGdiFlush.Call()
	pReleaseDC.Call(b.plat.hwnd, wdc)
	got := readWindow(t, b.plat.hwnd, b.physW, b.physH)
	if n, _ := diffBGR(got, want, int(b.physW*b.physH)); n == 0 {
		t.Fatal("the inverted patch does not show in the read-back; the test checks nothing")
	}

	frames := w.FrameCount() // FrameFn counts every call, an idle frame too
	repaint(b.plat.hwnd, &patch)
	assertWindowShows(t, b, want, "after WM_PAINT")
	if n := w.FrameCount(); n != frames {
		t.Fatalf("WM_PAINT ran a frame: frame count %d, was %d", n, frames)
	}
}

// TestSoftWin32ResizeKeepsHandles: repeated live resizes replace the DIB
// without leaking a GDI object. A DIB that cannot be created, or created
// and not selected, leaves the last frame in place (bitmap, size and bits
// unchanged, and WM_PAINT still shows it) with no object leaked; the next
// present compares sizes again and allocates the new DIB.
func TestSoftWin32ResizeKeepsHandles(t *testing.T) {
	b, w := newSoftWin32Test(t, softWin32Cfg())
	sp := b.plat.soft
	hwnd := b.plat.hwnd
	frame(b, w)
	resizeClient(t, hwnd, 8, 8)
	resizeClient(t, hwnd, -8, -8)
	before := gdiObjects()
	for i := range 20 {
		d := int32(4 + i%5*6)
		resizeClient(t, hwnd, d, d)
		resizeClient(t, hwnd, -d, -d)
	}
	if after := gdiObjects(); after != before {
		t.Fatalf("GDI objects %d before 40 resizes, %d after", before, after)
	}
	if sp.dibW != b.physW || sp.dibH != b.physH {
		t.Fatalf("DIB %dx%d, client %dx%d", sp.dibW, sp.dibH, b.physW, b.physH)
	}

	failures := []struct {
		name string
		set  func() func()
	}{
		{"CreateDIBSection", func() func() {
			orig := createDIBSectionFunc
			createDIBSectionFunc = func(uintptr, *bitmapInfo, *unsafe.Pointer) (uintptr, error) {
				return 0, errors.New("test: out of GDI handles")
			}
			return func() { createDIBSectionFunc = orig }
		}},
		{"SelectObject", func() func() {
			orig := selectObjectFunc
			selectObjectFunc = func(uintptr, uintptr) (uintptr, error) {
				return 0, errors.New("test: selection refused")
			}
			return func() { selectObjectFunc = orig }
		}},
	}
	for _, f := range failures {
		frame(b, w)
		oldW, oldH := sp.dibW, sp.dibH
		last := readWindow(t, hwnd, oldW, oldH)
		bmp, bits := sp.bmp, unsafe.SliceData(sp.pix)
		count := gdiObjects()

		restore := f.set()
		resizeClient(t, hwnd, 24, 16) // WM_SIZE presents at the new size
		if sp.bmp != bmp || sp.dibW != oldW || sp.dibH != oldH || unsafe.SliceData(sp.pix) != bits {
			restore()
			t.Fatalf("%s failed: DIB changed to %#x %dx%d", f.name, sp.bmp, sp.dibW, sp.dibH)
		}
		if got := gdiObjects(); got != count {
			restore()
			t.Fatalf("%s failed: GDI objects %d, were %d", f.name, got, count)
		}
		repaint(hwnd, nil)
		if got := readWindow(t, hwnd, oldW, oldH); string(got) != string(last) {
			restore()
			t.Fatalf("%s failed: WM_PAINT does not show the last frame", f.name)
		}
		restore()

		frame(b, w)
		if sp.dibW != b.physW || sp.dibH != b.physH || sp.bmp == bmp {
			t.Fatalf("after %s failed, the next present did not allocate: DIB %dx%d, client %dx%d",
				f.name, sp.dibW, sp.dibH, b.physW, b.physH)
		}
		resizeClient(t, hwnd, -24, -16)
	}
}

// TestSoftWin32OpacityAfterPaint: SetWindowOpacity on a soft window that
// has already shown a frame makes it a layered window (#1310: whole-window
// opacity needs no soft-specific code). The frame must still be there
// once the messages that follow are handled, and a later frame, which
// differs, must reach the layered window through the same GDI blit.
func TestSoftWin32OpacityAfterPaint(t *testing.T) {
	cfg := softWin32Cfg()
	b, w := newSoftWin32Test(t, cfg)
	frame(b, w)
	want := renderReference(t, b, cfg)
	assertWindowShows(t, b, want, "before the fade")

	(&nativePlatform{b: b}).SetWindowOpacity(0.5)
	style, _, _ := pGetWindowLongPtrW.Call(b.plat.hwnd, gwlExStyle)
	var alpha byte
	var flags uint32
	pGetLayeredAttrs.Call(b.plat.hwnd, 0, uintptr(unsafe.Pointer(&alpha)),
		uintptr(unsafe.Pointer(&flags)))
	if style&wsExLayered == 0 || flags&lwaAlpha == 0 || alpha != 128 {
		t.Fatalf("fade not applied: ex-style %#x, flags %#x, alpha %d", style, flags, alpha)
	}
	var msg msgW
	pumpMessages(&msg)
	assertWindowShows(t, b, want, "after the fade")

	cfg2 := cfg
	cfg2.OnInit = func(w *gui.Window) { w.SetView(softWin32AltView) }
	w.SetView(softWin32AltView)
	frame(b, w)
	assertWindowShows(t, b, renderReference(t, b, cfg2),
		"after a present to the layered window")
}

// softWin32AltView is a second static frame, unlike softWin32View in every
// region, so a present that does not reach the window shows.
func softWin32AltView(_ *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Color:   gui.RGB(200, 60, 40),
		Padding: gui.PaddingMedium,
		Content: []gui.View{gui.Text(gui.TextCfg{Text: "Faded"})},
	})
}

// TestSoftWin32PresentAllocs pins what a steady-state soft frame costs the
// heap: nothing. Draw is at 0 (soft.TestRendererDrawAllocs); the swizzle
// writes into the DIB, and every GDI call takes scalars or a pointer into
// the present state, which lives on the heap already.
func TestSoftWin32PresentAllocs(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts include the race runtime's own under -race")
	}
	b, w := newSoftWin32Test(t, softWin32Cfg())
	sp := b.plat.soft
	sp.vsyncOff = true
	w.FrameFn()
	for range 3 {
		if !sp.present(w, b.physW, b.physH) {
			t.Fatal("present failed")
		}
	}
	const frames = 30
	// Best of a few rounds: the counters are process-wide, and go-glyph's
	// background warm runs beside us.
	best := math.Inf(1)
	for range 5 {
		var m0, m1 runtime.MemStats
		runtime.ReadMemStats(&m0)
		for range frames {
			sp.present(w, b.physW, b.physH)
		}
		runtime.ReadMemStats(&m1)
		best = min(best, float64(m1.Mallocs-m0.Mallocs)/frames)
	}
	t.Logf("%.2f allocations per frame", best)
	if best > 0 {
		t.Fatalf("%.2f allocations per frame, want 0", best)
	}
}
