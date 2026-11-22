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
// WS_POPUP), is WS_EX_LAYERED, its client area is the whole window, and a
// present goes through UpdateLayeredWindow and stores RenderToImage's
// premultiplied pixels with alpha 0 raised to 1. The sizes stay equal over
// a live resize and a maximize; a minimized window skips its frames
// without logging, and the restore presents again.
func TestSoftWin32LayeredWindow(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		name := "resizable"
		if fixed {
			name = "fixed"
		}
		t.Run(name, func(t *testing.T) {
			cfg := layeredCfg(fixed)
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
			margin := [2]int32{layeredInset / 2, ph / 2}
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
				for col := range int32(layeredInset) {
					i := (row*pw + col) * 4
					sp.pix[i], sp.pix[i+1], sp.pix[i+2], sp.pix[i+3] = 0, 0, 0, 0
				}
			}
			if err := sp.updateLayered(); err != nil {
				t.Fatal(err)
			}
			raise(0)
			if h := windowAt(mx, my); h != back.plat.hwnd {
				t.Fatalf("with alpha 0 the margin hit-tests to %s, want the backdrop; "+
					"the floor check cannot fail", windowClass(h))
			}
		})
	}
}
