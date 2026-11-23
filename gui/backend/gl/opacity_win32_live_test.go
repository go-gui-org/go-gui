//go:build windows && !js

package gl

import (
	"testing"
	"time"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// solidCfg is a frameless window filled with one color, rgb as R, G, B.
func solidCfg(rgb [3]int) gui.WindowCfg {
	return gui.WindowCfg{Width: 300, Height: 220, Decorations: gui.DecorationNone,
		OnInit: func(w *gui.Window) {
			w.SetView(func(*gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder,
					Color: gui.RGB(uint8(rgb[0]), uint8(rgb[1]), uint8(rgb[2]))})
			})
		}}
}

// TestGLWin32FadeOnOwnDCClass: the GL window class has CS_OWNDC, which
// Microsoft documents as excluded for WS_EX_LAYERED, and the fade sets
// WS_EX_LAYERED on it anyway (#1316). This checks that the combination
// still fades on the driver at hand: a GL window at opacity 0.5 over an
// opaque backdrop must show the blend of the two on screen, faded after its
// first frame and faded before it was created (the replay in
// openWin32Window), at the first frame, after more swaps, and after a
// resize in both the old area and the newly exposed strip. An unfaded
// window shows its own color, so the check can fail.
func TestGLWin32FadeOnOwnDCClass(t *testing.T) {
	bd := [3]int{200, 60, 40}
	fg := [3]int{40, 120, 220}
	var want [3]int
	for i := range want {
		want[i] = (fg[i]*128 + bd[i]*127) / 255
	}
	for _, early := range []bool{false, true} {
		name := "fade-after-paint"
		if early {
			name = "fade-at-create"
		}
		t.Run(name, func(t *testing.T) {
			// Both windows on this subtest's goroutine: New locks the OS
			// thread, and SetWindowPos on a window of another thread
			// waits for that thread to pump messages.
			back, bw := newSoftWin32Test(t, solidCfg(bd))
			t.Setenv("GOGUI_RENDERER", "gl")
			cfg := solidCfg(fg)
			gw := gui.NewWindow(cfg)
			if early {
				gw.SetWindowOpacity(0.5)
			}
			gb, err := New(gw)
			if err != nil {
				// No GL context on this runner (GDI Generic without Mesa):
				// a skip, which glrendertest turns into a failure.
				softWin32Skip(t, "no GL window: %v", err)
			}
			t.Cleanup(gb.Destroy)
			if gb.plat.softMode {
				t.Fatal("GOGUI_RENDERER=gl opened a soft window")
			}
			gb.plat.w = gw
			cfg.OnInit(gw)

			const x, y = 120, 140
			// Both topmost, the GL window above; raise puts them back before
			// every read, as another topmost window can come up over them.
			raise := func() {
				pSetWindowPos.Call(back.plat.hwnd, hwndTopmost, x, y, 0, 0,
					swpNoSize|swpNoActivate|swpShowWin)
				pSetWindowPos.Call(gb.plat.hwnd, hwndTopmost, x, y, 0, 0,
					swpNoSize|swpNoActivate|swpShowWin)
			}
			raise()
			var msg msgW
			pumpMessages(&msg)
			frame(back, bw)
			frame(gb, gw)
			if !early {
				(&nativePlatform{b: gb}).SetWindowOpacity(0.5)
			}

			style, _, _ := pGetWindowLongPtrW.Call(gb.plat.hwnd, gwlExStyle)
			var alpha byte
			var flags uint32
			ok, _, _ := pGetLayeredAttrs.Call(gb.plat.hwnd, 0,
				uintptr(unsafe.Pointer(&alpha)), uintptr(unsafe.Pointer(&flags)))
			if ok == 0 || style&wsExLayered == 0 || flags&lwaAlpha == 0 || alpha != 128 {
				t.Fatalf("fade not applied: ex-style %#x, GetLayeredWindowAttributes %d, "+
					"flags %#x, alpha %d", style, ok, flags, alpha)
			}

			// DWM composes after the swap returns: poll for the blend.
			expect := func(when string, px, py int32) {
				t.Helper()
				deadline := time.Now().Add(3 * time.Second)
				for {
					raise()
					pumpMessages(&msg)
					frame(gb, gw)
					s := screenPixels(t, x+px, y+py, 1, 1)
					got := [3]int{int(s[2]), int(s[1]), int(s[0])}
					if near(got, want) {
						return
					}
					if time.Now().After(deadline) {
						t.Fatalf("%s: screen %v, want the blend %v of %v over %v; "+
							"window there: %s", when, got, want, fg, bd,
							windowClass(windowAt(x+px, y+py)))
					}
					time.Sleep(20 * time.Millisecond)
				}
			}
			expect("first frame", 150, 110)
			for range 30 {
				frame(gb, gw)
			}
			expect("after 30 frames", 150, 110)
			// Grow from the current physical size, which depends on the
			// display scale, so the strip is new at any scale. The backdrop
			// grows further, so the strip still has it behind.
			ww, wh, _, _ := windowAndClient(gb.plat.hwnd)
			pSetWindowPos.Call(back.plat.hwnd, hwndTopmost, x, y,
				uintptr(ww+40), uintptr(wh+40), swpNoActivate)
			pSetWindowPos.Call(gb.plat.hwnd, hwndTopmost, x, y,
				uintptr(ww+20), uintptr(wh+20), swpNoActivate)
			pumpMessages(&msg)
			frame(back, bw)
			expect("after a resize, old area", 150, 110)
			expect("after a resize, new strip", ww+10, wh+10)
		})
	}
}
