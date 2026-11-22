//go:build windows && !js

package gl

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// Live tests of a transparent frameless window on the CPU renderer (#1310,
// phase 4): a layered window presented with UpdateLayeredWindow. Like the
// other soft live tests they need a desktop session.

var (
	pWindowFromPoint = user32.NewProc("WindowFromPoint")
	pGetClassNameW   = user32.NewProc("GetClassNameW")
)

const (
	// captureBlt is CAPTUREBLT: a BitBlt from the screen includes
	// layered windows only with it.
	captureBlt = 0x40000000
	// hwndTopmost is HWND_TOPMOST, (HWND)-1.
	hwndTopmost  = ^uintptr(0)
	swpShowWin   = 0x0040
	layeredInset = 40
)

// layeredCfg is a transparent frameless window: a margin of alpha 0 around
// a card of alpha 128.
func layeredCfg(fixed bool) gui.WindowCfg {
	return gui.WindowCfg{Width: 240, Height: 160, Transparent: true,
		Decorations: gui.DecorationNone, FixedSize: fixed,
		OnInit: func(w *gui.Window) { w.SetView(layeredView) }}
}

func layeredView(_ *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    gui.PadAll(layeredInset),
		SizeBorder: gui.NoBorder,
		Content: []gui.View{gui.Column(gui.ContainerCfg{
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Color:      gui.RGBA(40, 120, 220, 128),
		})},
	})
}

// windowAndClient returns the window rect's size and the client size.
func windowAndClient(hwnd uintptr) (ww, wh, cw, ch int32) {
	var r rectW
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	cw, ch = clientSize(hwnd)
	return r.right - r.left, r.bottom - r.top, cw, ch
}

// assertLayeredFits fails unless the client area is the whole window and
// the frame store is that size: UpdateLayeredWindow is then sent the
// window's own size and never resizes it.
func assertLayeredFits(t *testing.T, b *Backend, when string) {
	t.Helper()
	ww, wh, cw, ch := windowAndClient(b.plat.hwnd)
	sp := b.plat.soft
	if ww != cw || wh != ch || sp.dibW != ww || sp.dibH != wh {
		t.Fatalf("%s: window %dx%d, client %dx%d, frame %dx%d; want all equal",
			when, ww, wh, cw, ch, sp.dibW, sp.dibH)
	}
}

// layeredWant is the frame store a layered window must hold for the
// reference frame ref (RGBA, premultiplied): B, G, R, A, with alpha 0
// raised to 1.
func layeredWant(ref []byte) []byte {
	want := make([]byte, len(ref))
	for i := 0; i < len(ref); i += 4 {
		want[i], want[i+1], want[i+2], want[i+3] = ref[i+2], ref[i+1], ref[i], max(ref[i+3], 1)
	}
	return want
}

// TestSoftWin32LayeredWindow: a Transparent frameless soft window, both
// frameless styles (WS_POPUP with a resize border, and FixedSize's plain
// WS_POPUP), is WS_EX_LAYERED, its client area is the whole window at the
// size asked for, and a present goes through UpdateLayeredWindow and
// stores RenderToImage's premultiplied pixels with alpha 0 raised to 1.
// The sizes stay equal over a live resize and a maximize; a minimized
// window skips its frames without logging, and the restore presents
// again. One frame of another size than the window's is dropped without a
// log line, a second in a row is logged, and neither resizes the window.
// A resizable window narrower than the resize border's minimum
// track size (14x14 at 100 %) keeps its size: the frame fit once made an
// 8x80 window 14x80, which the equality alone does not see.
func TestSoftWin32LayeredWindow(t *testing.T) {
	for _, c := range []struct {
		name          string
		fixed         bool
		width, height int
	}{{"resizable", false, 240, 160}, {"fixed", true, 240, 160}, {"narrow", false, 8, 80}} {
		t.Run(c.name, func(t *testing.T) {
			cfg := layeredCfg(c.fixed)
			cfg.Width, cfg.Height = c.width, c.height
			b, w := newSoftWin32Test(t, cfg)
			sp := b.plat.soft
			ex, _, _ := pGetWindowLongPtrW.Call(b.plat.hwnd, gwlExStyle)
			if ex&wsExLayered == 0 || !b.plat.layered || !sp.layered {
				t.Fatalf("not layered: ex-style %#x, plat %v, present %v", ex, b.plat.layered, sp.layered)
			}
			w.FrameFn()
			if !sp.present(w, b.physW, b.physH) {
				t.Fatal("the first present failed")
			}
			assertLayeredFits(t, b, "after the first present")
			if ew, eh := clientExtent(c.width, b.dpiScale), clientExtent(c.height, b.dpiScale); sp.dibW != ew || sp.dibH != eh {
				t.Fatalf("window and frame %dx%d, want the %dx%d asked for", sp.dibW, sp.dibH, ew, eh)
			}
			if c.name == "narrow" {
				return // no room for the card; the pixels are checked above
			}
			ref := renderReference(t, b, cfg)
			var clear, part int
			for i := 3; i < len(ref); i += 4 {
				switch {
				case ref[i] == 0:
					clear++
				case ref[i] < 255:
					part++
				}
			}
			if clear == 0 || part == 0 {
				t.Fatalf("reference has %d clear and %d part-transparent pixels; the test checks nothing", clear, part)
			}
			if !bytes.Equal(sp.pix, layeredWant(ref)) {
				t.Fatal("the frame store is not the reference with alpha 0 raised to 1")
			}

			resizeClient(t, b.plat.hwnd, 30, 20) // WM_SIZE presents
			assertLayeredFits(t, b, "after a resize")
			pShowWindow.Call(b.plat.hwnd, swMaximize)
			assertLayeredFits(t, b, "maximized")
			pShowWindow.Call(b.plat.hwnd, swRestore)
			assertLayeredFits(t, b, "restored")

			pShowWindow.Call(b.plat.hwnd, swMinimize)
			w.FrameFn()
			if sp.present(w, b.physW, b.physH) || sp.skipLogged {
				t.Fatalf("a minimized window was presented or logged a skipped frame (logged %v)", sp.skipLogged)
			}
			pShowWindow.Call(b.plat.hwnd, swRestore)
			assertLayeredFits(t, b, "restored after minimize")
			w.FrameFn()
			if !sp.present(w, b.physW, b.physH) {
				t.Fatal("present after the restore failed")
			}

			// A frame of another size than the window's, as the loop draws
			// one between a resize from another process and its WM_SIZE.
			ww, wh, _, _ := windowAndClient(b.plat.hwnd)
			if sp.present(w, b.physW-8, b.physH) || sp.skipLogged {
				t.Fatalf("one frame of another size was sent or logged (logged %v)", sp.skipLogged)
			}
			if sp.present(w, b.physW-8, b.physH) || !sp.skipLogged {
				t.Fatalf("a second frame of another size in a row was sent or not logged (logged %v)", sp.skipLogged)
			}
			if nw, nh, _, _ := windowAndClient(b.plat.hwnd); nw != ww || nh != wh {
				t.Fatalf("a frame of another size resized the window from %dx%d to %dx%d", ww, wh, nw, nh)
			}
			if !sp.present(w, b.physW, b.physH) || sp.skipLogged {
				t.Fatalf("present at the window's size failed or left the skip logged (logged %v)", sp.skipLogged)
			}
			assertLayeredFits(t, b, "after frames of another size")
		})
	}
}

// TestSoftWin32TransparentFramedIsOpaque: a Transparent window with a
// system frame stays opaque on soft (#1310, decision 3): no WS_EX_LAYERED,
// the frame is blitted as for any opaque window, and its one report says
// why. A frameless one reports nothing about transparency.
func TestSoftWin32TransparentFramedIsOpaque(t *testing.T) {
	t.Setenv("GOGUI_RENDERER", "soft")
	// newSoftWin32 is called without New, which locks the thread: the
	// window's messages are sent to the thread that created it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, deco := range []gui.WindowDecoration{gui.DecorationDefault, gui.DecorationNone} {
		cfg := softWin32Cfg()
		cfg.Transparent = true
		cfg.Decorations = deco
		w := gui.NewWindow(cfg)
		var rep windowReports
		b, err := newSoftWin32(w, &rep, nil)
		if err != nil {
			softWin32Skip(t, "no soft window: %v", err)
		}
		ex, _, _ := pGetWindowLongPtrW.Call(b.plat.hwnd, gwlExStyle)
		layered := ex&wsExLayered != 0
		reports := rep.transparency
		b.Destroy()
		if deco == gui.DecorationNone {
			if !layered || len(reports) != 0 {
				t.Errorf("frameless: layered %v, reports %q; want layered, none", layered, reports)
			}
			continue
		}
		if layered || b.plat.layered {
			t.Errorf("framed: ex-style %#x is layered", ex)
		}
		if len(reports) != 1 || !strings.Contains(reports[0], "system frame") ||
			!strings.Contains(reports[0], "DecorationNone") || strings.Contains(reports[0], "GOGUI_RENDERER") {
			t.Errorf("framed: reports %q, want one that names the system frame and DecorationNone", reports)
		}
	}

	// The framed window draws opaque, through the blit.
	cfg := softWin32Cfg()
	cfg.Transparent = true
	b, w := newSoftWin32Test(t, cfg)
	frame(b, w)
	assertWindowShows(t, b, renderReference(t, b, cfg), "framed transparent")
}

// screenPixels reads width x height pixels of the screen at x, y, layered
// windows included, as B, G, R, x bytes.
func screenPixels(t *testing.T, x, y, width, height int32) []byte {
	t.Helper()
	sdc, _, _ := pGetDC.Call(0)
	if sdc == 0 {
		t.Fatal("GetDC(screen) failed")
	}
	defer pReleaseDC.Call(0, sdc)
	mdc, _, _ := pCreateCompatibleDC.Call(sdc)
	defer pDeleteDC.Call(mdc)
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
		sdc, uintptr(x), uintptr(y), srcCopy|captureBlt); r == 0 {
		t.Fatalf("BitBlt from the screen: %v", e)
	}
	pGdiFlush.Call()
	return append([]byte(nil), unsafe.Slice((*byte)(bits), int(width)*int(height)*4)...)
}

// windowAt is the window WindowFromPoint finds at screen point x, y.
func windowAt(x, y int32) uintptr {
	h, _, _ := pWindowFromPoint.Call(uintptr(uint32(x)) | uintptr(uint32(y))<<32)
	return h
}

// windowClass names hwnd's window class, for a failure message that says
// which window was found where the test's window was expected.
func windowClass(hwnd uintptr) string {
	var buf [128]uint16
	pGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return fmt.Sprintf("%#x (class %q)", hwnd, syscall.UTF16ToString(buf[:]))
}

// near reports whether every channel of got is within 2 of want.
func near(got, want [3]int) bool {
	for i := range got {
		if d := got[i] - want[i]; d < -2 || d > 2 {
			return false
		}
	}
	return true
}

// TestSoftWin32LayeredShowsDesktop places the layered window over an opaque
// backdrop window and reads the screen: the margin, alpha 0 in the frame,
// shows the backdrop, and the card blends over it by its premultiplied
// alpha. The margin still hit-tests to the layered window (the alpha
// floor); with the floor taken out of the stored frame, the same point
// hit-tests to the backdrop, so the check can fail. Then the same for a
// window hidden at the start and shown with Show: the frame presented
// while it was hidden is on screen.
func TestSoftWin32LayeredShowsDesktop(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		name := "shown"
		if hidden {
			name = "hidden-start"
		}
		t.Run(name, func(t *testing.T) {
			bd := [3]int{200, 60, 40} // R, G, B
			bcfg := gui.WindowCfg{Width: 300, Height: 220, Decorations: gui.DecorationNone,
				OnInit: func(w *gui.Window) {
					w.SetView(func(*gui.Window) gui.View {
						return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder,
							Color: gui.RGB(uint8(bd[0]), uint8(bd[1]), uint8(bd[2]))})
					})
				}}
			back, bw := newSoftWin32Test(t, bcfg)
			lw := gui.NewWindow(layeredCfg(false))
			if hidden {
				lw.Hide()
			}
			t.Setenv("GOGUI_RENDERER", "soft")
			lb, err := New(lw)
			if err != nil {
				softWin32Skip(t, "no layered window: %v", err)
			}
			t.Cleanup(lb.Destroy)
			lb.plat.w = lw
			lw.SetView(layeredView)

			const x, y = 120, 140
			// Both windows are topmost, the layered one above. Another
			// topmost window can come up over them at any time (a
			// fullscreen one did on the workstation), so raise puts them
			// back before every read of the screen or hit-test.
			raise := func(show uintptr) {
				pSetWindowPos.Call(back.plat.hwnd, hwndTopmost, x, y, 0, 0,
					swpNoSize|swpNoActivate|swpShowWin)
				pSetWindowPos.Call(lb.plat.hwnd, hwndTopmost, x, y, 0, 0,
					swpNoSize|swpNoActivate|show)
			}
			if hidden {
				raise(0)
			} else {
				raise(swpShowWin)
			}
			var msg msgW
			pumpMessages(&msg)
			frame(back, bw)
			frame(lb, lw)
			if hidden {
				if windowStyle(lb.plat.hwnd)&wsVisible != 0 {
					t.Fatal("the hidden window is visible")
				}
				// No present after this: the frame sent while the window
				// was hidden must be what shows.
				lw.Show()
				pumpMessages(&msg)
			}
			sp := lb.plat.soft
			pw, ph := sp.dibW, sp.dibH
			// The margin in physical pixels: at a device scale of 0.5 the
			// logical inset's half is the first card pixel.
			inset := int32(float32(layeredInset) * lb.dpiScale)
			margin := [2]int32{inset / 2, ph / 2}
			card := [2]int32{pw / 2, ph / 2}
			pix := func(s []byte, p [2]int32) [3]int {
				i := (p[1]*pw + p[0]) * 4
				return [3]int{int(s[i+2]), int(s[i+1]), int(s[i])}
			}
			// What the card must show: the stored premultiplied pixel plus
			// the backdrop scaled by what its alpha leaves.
			ci := (card[1]*pw + card[0]) * 4
			ca := int(sp.pix[ci+3])
			if ca == 0 || ca == 255 {
				t.Fatalf("card alpha %d; the test needs a part-transparent card", ca)
			}
			src := [3]int{int(sp.pix[ci+2]), int(sp.pix[ci+1]), int(sp.pix[ci])}
			var wantCard [3]int
			for i := range wantCard {
				wantCard[i] = src[i] + bd[i]*(255-ca)/255
			}
			// DWM composes after the call returns: poll for the result.
			deadline := time.Now().Add(3 * time.Second)
			for {
				raise(0)
				s := screenPixels(t, x, y, pw, ph)
				gotMargin, gotCard := pix(s, margin), pix(s, card)
				if near(gotMargin, bd) && near(gotCard, wantCard) {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("screen: margin %v, want the backdrop %v; card %v, want %v; "+
						"window at the margin: %s", gotMargin, bd, gotCard, wantCard,
						windowClass(windowAt(x+margin[0], y+margin[1])))
				}
				time.Sleep(20 * time.Millisecond)
			}

			mx, my := x+margin[0], y+margin[1]
			raise(0)
			if h := windowAt(mx, my); h != lb.plat.hwnd {
				t.Fatalf("the margin hit-tests to %s, want the layered window", windowClass(h))
			}
			// The control: alpha 0 in the margin lets the point through.
			for row := range ph {
				for col := range inset {
					i := (row*pw + col) * 4
					sp.pix[i], sp.pix[i+1], sp.pix[i+2], sp.pix[i+3] = 0, 0, 0, 0
				}
			}
			if uerr := sp.updateLayered(); uerr != nil {
				t.Fatal(uerr)
			}
			raise(0)
			if h := windowAt(mx, my); h != back.plat.hwnd {
				t.Fatalf("with alpha 0 the margin hit-tests to %s, want the backdrop; "+
					"the floor check cannot fail", windowClass(h))
			}
		})
	}
}

// TestWin32ResizeBeforeRun: a window resized after New and before its run
// loop takes it draws its first frame at the new size. The resize's
// WM_SIZE comes while wndProc still sends the window's messages to
// DefWindowProc, so only attachWin32 brings the client size the backend
// keeps up to date, and sendStartResize the window's logical size. A
// layered window shows nothing until a frame of the window's size reaches
// it: before the fix, every frame was skipped, with one log line. The
// application hears of the resize through OnEvent no earlier than it could
// before the reconcile: after OnInit, and in RunApp with the window
// registered (an App and a platform ID), with the real size. The first
// reconcile emitted it before OnInit, ahead of registration in RunApp, so
// a handler there could not reach the app. Run with SetWindowPos between
// New and Run, as the review reproduced it; RunApp, which calls New itself,
// with the resize made inside New after the size is read (the present
// seam). A GL window had the same stale size before phase 4; its case
// checks the reconcile with GL's context.
func TestWin32ResizeBeforeRun(t *testing.T) {
	const ww, wh = 320, 230 // the window after the resize, physical
	type resizeSeen struct {
		afterInit, app bool
		id             uint32
		w, h           int
	}
	type seen struct {
		recorded      bool
		phys, client  [2]int32
		window, dib   [2]int32
		guiW, guiH    int
		wantW, wantH  int
		skipped       bool
		dpiScale      float32
		layered, soft bool
		inited        bool         // OnInit has returned
		resizes       []resizeSeen // what each OnEvent EventResized saw
	}
	resize := func(hwnd uintptr) {
		pSetWindowPos.Call(hwnd, 0, 0, 0, ww, wh, swpNoMove|swpNoZOrder|swpNoActivate)
	}
	// open builds the window whose second frame records what the first
	// frame was drawn with, then closes it.
	open := func(cfg gui.WindowCfg, s *seen) *gui.Window {
		view := layeredView
		if !cfg.Transparent {
			view = softWin32View
		}
		queued := false
		cfg.OnEvent = func(e *gui.Event, w *gui.Window) {
			if e.Type == gui.EventResized {
				s.resizes = append(s.resizes, resizeSeen{afterInit: s.inited,
					app: w.App() != nil, id: w.PlatformID(),
					w: e.WindowWidth, h: e.WindowHeight})
			}
		}
		cfg.OnInit = func(w *gui.Window) {
			defer func() { s.inited = true }()
			w.SetView(func(*gui.Window) gui.View {
				if !queued {
					queued = true
					w.QueueCommand(func(*gui.Window) {
						b := w.NativePlatformBackend().(*nativePlatform).b
						s.recorded = true
						s.phys = [2]int32{b.physW, b.physH}
						cw, ch := clientSize(b.plat.hwnd)
						s.client = [2]int32{cw, ch}
						var r rectW
						pGetWindowRect.Call(b.plat.hwnd, uintptr(unsafe.Pointer(&r)))
						s.window = [2]int32{r.right - r.left, r.bottom - r.top}
						s.guiW, s.guiH = w.WindowSize()
						lw, lh := b.logicalXY(cw, ch)
						s.wantW, s.wantH = int(lw), int(lh)
						s.dpiScale = b.dpiScale
						s.layered = b.plat.layered
						if sp := b.plat.soft; sp != nil {
							s.soft = true
							s.dib = [2]int32{sp.dibW, sp.dibH}
							s.skipped = sp.skipLogged
						}
						w.Close()
					})
				}
				return view(w)
			})
		}
		return gui.NewWindow(cfg)
	}
	check := func(t *testing.T, s seen, inApp bool) {
		t.Helper()
		if !s.recorded {
			t.Fatal("the loop ended before its second frame")
		}
		t.Logf("%+v", s)
		if s.phys != s.client || s.guiW != s.wantW || s.guiH != s.wantH {
			t.Errorf("the first frame was drawn at %v, logical %dx%d; the client is %v, logical %dx%d",
				s.phys, s.guiW, s.guiH, s.client, s.wantW, s.wantH)
		}
		if len(s.resizes) == 0 {
			t.Error("no EventResized reached OnEvent")
		}
		for i, r := range s.resizes {
			if !r.afterInit || (inApp && (!r.app || r.id == 0)) {
				t.Errorf("EventResized %d: after OnInit %v, app %v, platform ID %d; want after OnInit, registered in RunApp", i, r.afterInit, r.app, r.id)
			}
		}
		if r := s.resizes; len(r) > 0 && (r[0].w != s.wantW || r[0].h != s.wantH) {
			t.Errorf("the first EventResized reported %dx%d, want the client's %dx%d", r[0].w, r[0].h, s.wantW, s.wantH)
		}
		if s.layered && (s.window != [2]int32{ww, wh} || s.dib != s.window || s.skipped) {
			t.Errorf("layered: window %v (want %dx%d), frame %v, skipped %v; want the frame at the window's size, sent",
				s.window, ww, wh, s.dib, s.skipped)
		}
	}
	runOnce := func(t *testing.T, renderer string, cfg gui.WindowCfg) {
		t.Setenv("GOGUI_RENDERER", renderer)
		var s seen
		w := open(cfg, &s)
		b, err := New(w)
		if err != nil {
			softWin32Skip(t, "no %s window: %v", renderer, err)
		}
		defer b.Destroy()
		resize(b.plat.hwnd)
		b.Run(w)
		check(t, s, false)
	}

	t.Run("soft-layered/Run", func(t *testing.T) {
		runOnce(t, "soft", layeredCfg(false))
	})
	t.Run("soft-layered/RunApp", func(t *testing.T) {
		t.Setenv("GOGUI_RENDERER", "soft")
		old := newSoftPresentWin32Func
		newSoftPresentWin32Func = func(hwnd uintptr, w *gui.Window, scale float32) (*softPresentWin32, error) {
			resize(hwnd)
			return old(hwnd, w, scale)
		}
		t.Cleanup(func() { newSoftPresentWin32Func = old })
		var s seen
		err := runAppE(gui.NewApp(), open(layeredCfg(false), &s))
		if err != nil {
			softWin32Skip(t, "no soft window: %v", err)
		}
		check(t, s, true)
	})
	t.Run("gl/Run", func(t *testing.T) {
		cfg := softWin32Cfg()
		cfg.Width, cfg.Height = 240, 160
		runOnce(t, "gl", cfg)
	})
}
