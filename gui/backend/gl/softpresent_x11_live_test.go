//go:build linux && !js && !android

package gl

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// Live tests of the GOGUI_RENDERER=soft X11 path (#1265). They need an X
// server (CI's Xvfb job, or XWayland on a desktop) and skip without one.

// softTextView is a static frame with shapes and text at several sizes: no
// animation, so a live frame and RenderToImage see the same commands.
func softTextView(_ *gui.Window) gui.View {
	content := make([]gui.View, 0, 6)
	for i, size := range []float32{11, 16, 24} {
		style := gui.CurrentTheme().TextStyleDef
		style.Size = size
		content = append(content, gui.Text(gui.TextCfg{
			Text:      fmt.Sprintf("Soft line %d — 0123456789", i),
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

// softSkip skips, or fails when GOGUI_REQUIRE_GL is set: the CI job that
// provides Xvfb (tools/glrendertest) sets it, so a broken runner shows up as
// red rather than as a silent pass. Same contract as the GL tests.
func softSkip(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("GOGUI_REQUIRE_GL") != "" {
		t.Fatalf("GOGUI_REQUIRE_GL is set but: "+format, args...)
	}
	t.Skipf(format, args...)
}

// newSoftX11 opens a real soft window, as Run would before its loop: OnInit
// has run and the platform knows the window. It waits until the window is
// viewable, since GetImage on an unmapped window fails.
//
// One soft backend per test process under Xvfb: Xvfb resets the server when
// its last client disconnects, so a second backend's connect can be reset.
// tools/glrendertest runs each of these tests in its own process.
func newSoftX11(t *testing.T, cfg gui.WindowCfg) (*Backend, *gui.Window) {
	t.Helper()
	if os.Getenv("DISPLAY") == "" {
		softSkip(t, "no DISPLAY; the test needs an X server")
	}
	t.Setenv("GOGUI_RENDERER", "soft")
	w := gui.NewWindow(cfg)
	b, err := newX11(w)
	if err != nil {
		softSkip(t, "x11 soft window: %v", err)
	}
	t.Cleanup(b.Destroy)
	if !b.plat.softMode || b.plat.soft == nil {
		t.Fatal("GOGUI_RENDERER=soft did not select the soft path")
	}
	b.plat.w = w
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		attrs, aerr := xproto.GetWindowAttributes(b.plat.conn, b.plat.window).Reply()
		if aerr == nil && attrs.MapState == xproto.MapStateViewable {
			break
		}
		if time.Now().After(deadline) {
			softSkip(t, "window never became viewable")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return b, w
}

// TestSoftX11NeverTouchesEGL drives every soft-path entry point — frames, a
// resize, a scale change, Expose, destroy — and checks no EGL object was ever
// created. A GL call reached on this path would also panic or fail here when
// the package's GL table is uninitialized, which is the case when the test
// runs alone (go test -run TestSoftX11).
func TestSoftX11NeverTouchesEGL(t *testing.T) {
	b, w := newSoftX11(t, gui.WindowCfg{Width: 240, Height: 160,
		OnInit: func(w *gui.Window) { w.SetView(softTextView) }})

	for range 3 {
		w.FrameFn()
		b.presentX11(w)
	}
	b.plat.physW, b.plat.physH = 300, 200
	b.resizeX11()
	w.FrameFn()
	b.presentX11(w)
	b.applyScaleX11(2)
	b.applyScaleX11(1)
	b.exposeX11(1) // more rects to come: no repaint yet
	b.exposeX11(0)

	p := &b.plat
	if p.eglDpy != 0 || p.eglSurface != 0 || p.eglContext != 0 {
		t.Fatalf("soft window created EGL state: dpy=%#x surface=%#x context=%#x",
			p.eglDpy, p.eglSurface, p.eglContext)
	}
	if b.physW != 300 || b.physH != 200 {
		t.Fatalf("resize: phys %dx%d, want 300x200", b.physW, b.physH)
	}
	if !p.presented {
		t.Fatal("presentX11 did not mark the frame presented")
	}
}

// TestSoftX11PixelsMatchRenderToImage: what reaches the X server is exactly
// what soft.RenderToImage draws for the same view (RGB; a depth-24 visual
// carries no alpha). This is the issue's "starts with soft and draws the same
// pixels" acceptance, checked on the server side of PutImage, so it covers
// the band split and the BGRA swizzle as well as the renderer.
func TestSoftX11PixelsMatchRenderToImage(t *testing.T) {
	const ww, wh = 320, 200
	cfg := gui.WindowCfg{Width: ww, Height: wh,
		OnInit: func(w *gui.Window) { w.SetView(softTextView) }}
	b, w := newSoftX11(t, cfg)
	scale := b.dpiScale

	w.FrameFn()
	b.presentX11(w)

	pw, ph := int(b.physW), int(b.physH)
	reply, err := xproto.GetImage(b.plat.conn, xproto.ImageFormatZPixmap,
		xproto.Drawable(b.plat.window), 0, 0, uint16(pw), uint16(ph),
		0xffffffff).Reply()
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}

	ref := gui.NewWindow(cfg)
	want, err := soft.RenderToImage(ref, scale)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	t.Cleanup(func() { soft.Release(ref) })
	if want.Rect.Dx() != pw || want.Rect.Dy() != ph {
		t.Skipf("window is %dx%d, reference %dx%d: the window manager resized it",
			pw, ph, want.Rect.Dx(), want.Rect.Dy())
	}
	if len(reply.Data) < pw*ph*4 {
		t.Fatalf("GetImage returned %d bytes, want %d", len(reply.Data), pw*ph*4)
	}

	diff := 0
	first := -1
	for i := 0; i < pw*ph; i++ {
		g, r := reply.Data[i*4:i*4+3], want.Pix[i*4:i*4+3]
		// Server bytes are B, G, R; the reference is R, G, B.
		if g[0] != r[2] || g[1] != r[1] || g[2] != r[0] {
			if first < 0 {
				first = i
			}
			diff++
		}
	}
	if diff > 0 {
		x, y := first%pw, first/pw
		t.Fatalf("%d of %d pixels differ; first at (%d,%d): server BGR %v, want RGB %v",
			diff, pw*ph, x, y, reply.Data[first*4:first*4+3], want.Pix[first*4:first*4+3])
	}
}
