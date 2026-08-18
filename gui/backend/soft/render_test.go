//go:build !wasm

//
// Every test here drives RenderToImage, which builds a go-glyph text
// system; on wasm that construction needs a DOM (syscall/js), which the
// node-based CI harness does not provide. draw_test.go covers the pure
// rasterizer paths on every platform.

package soft

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newWin builds a headless window whose root is view.
func newWin(t *testing.T, w, h int, view func(*gui.Window) gui.View) *gui.Window {
	t.Helper()
	win := gui.NewWindow(gui.WindowCfg{
		Width:   w,
		Height:  h,
		BgColor: gui.RGB(0, 0, 0),
		OnInit:  func(win *gui.Window) { win.UpdateView(view) },
	})
	t.Cleanup(func() { Release(win) })
	return win
}

func TestRenderRectPixels(t *testing.T) {
	win := newWin(t, 100, 100, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FixedFixed,
			Width:      40,
			Height:     40,
			Color:      gui.RGB(255, 0, 0),
			SizeBorder: gui.NoBorder,
		})
	})
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 100, 100) {
		t.Fatalf("bounds = %v, want 100x100", got)
	}
	// Inside the rect: pure red.
	if r, g, b, a := at(img, 20, 20); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("centre = %d,%d,%d,%d, want 255,0,0,255", r, g, b, a)
	}
	// Outside it: the window background.
	if r, g, b, a := at(img, 90, 90); r != 0 || g != 0 || b != 0 || a != 255 {
		t.Errorf("corner = %d,%d,%d,%d, want 0,0,0,255", r, g, b, a)
	}
}

func TestRenderScaleDoublesGeometry(t *testing.T) {
	view := func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FixedFixed,
			Width:      40,
			Height:     40,
			Color:      gui.RGB(255, 0, 0),
			SizeBorder: gui.NoBorder,
		})
	}
	win := newWin(t, 100, 100, view)
	img, err := RenderToImage(win, 2)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 200, 200) {
		t.Fatalf("bounds = %v, want 200x200", got)
	}
	// The 40pt rect covers 80 device px, so (70,70) is inside at
	// scale 2 and outside at scale 1.
	if r, _, _, _ := at(img, 70, 70); r != 255 {
		t.Errorf("(70,70) red = %d, want 255 (rect should span 80px)", r)
	}
	if r, _, _, _ := at(img, 90, 90); r != 0 {
		t.Errorf("(90,90) red = %d, want 0 (past the rect)", r)
	}
}

func TestRenderText(t *testing.T) {
	win := newWin(t, 200, 60, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Label("HHHH", gui.TextStyle{
					Size:  32,
					Color: gui.RGB(255, 255, 255),
				}),
			},
		})
	})
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	if win.TextMeasurer() == nil {
		t.Fatal("text measurer not installed")
	}
	if lit := countLitPixels(img); lit == 0 {
		t.Fatal("no glyph pixels rendered; the atlas warm pass failed")
	} else {
		t.Logf("lit pixels: %d", lit)
	}
}

// countLitPixels counts pixels with any channel above the antialiasing
// threshold; glyph edges are never exact white, so exact equality would be
// fragile.
func countLitPixels(img *image.RGBA) int {
	var lit int
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			if r, _, _, _ := at(img, x, y); r > 40 {
				lit++
			}
		}
	}
	return lit
}

func TestRenderToPNG(t *testing.T) {
	win := newWin(t, 60, 60, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FillFill,
			Color:      gui.RGB(0, 128, 255),
			SizeBorder: gui.NoBorder,
		})
	})
	path := filepath.Join(t.TempDir(), "out", "shot.png")
	if err := RenderToPNG(win, 1, path); err != nil {
		t.Fatalf("RenderToPNG: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("empty PNG")
	}
}

// redView is a window-filling red column, used by the lifecycle tests.
func redView(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Color:      gui.RGB(255, 0, 0),
		SizeBorder: gui.NoBorder,
	})
}

func TestRenderScaleChangeRebuilds(t *testing.T) {
	onInit := 0
	win := gui.NewWindow(gui.WindowCfg{
		Width:   100,
		Height:  100,
		BgColor: gui.RGB(0, 0, 0),
		OnInit: func(w *gui.Window) {
			onInit++
			w.UpdateView(redView)
		},
	})
	t.Cleanup(func() { Release(win) })

	img1, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("RenderToImage(scale 1): %v", err)
	}
	if got := img1.Bounds(); got != image.Rect(0, 0, 100, 100) {
		t.Fatalf("bounds = %v, want 100x100", got)
	}

	// A second render at a different scale frees the scale-1 text
	// system and builds a fresh one; the frame must still paint.
	img2, err := RenderToImage(win, 2)
	if err != nil {
		t.Fatalf("RenderToImage(scale 2): %v", err)
	}
	if got := img2.Bounds(); got != image.Rect(0, 0, 200, 200) {
		t.Fatalf("bounds = %v, want 200x200", got)
	}
	if r, _, _, _ := at(img2, 40, 40); r != 255 {
		t.Errorf("scaled rect red = %d, want 255", r)
	}
	if onInit != 1 {
		t.Fatalf("OnInit ran %d times, want 1", onInit)
	}
}

func TestReleaseThenRenderRebuilds(t *testing.T) {
	win := newWin(t, 60, 60, redView)
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("first RenderToImage: %v", err)
	}
	Release(win)
	if win.TextMeasurer() != nil {
		t.Fatal("Release left the text measurer installed")
	}
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("render after Release: %v", err)
	}
	if r, _, _, _ := at(img, 30, 30); r != 255 {
		t.Errorf("rect red = %d, want 255", r)
	}
}

func TestRenderToImageNilWindow(t *testing.T) {
	if _, err := RenderToImage(nil, 1); err == nil {
		t.Fatal("nil window: want error")
	}
}

func TestRenderToImageZeroSizeWindow(t *testing.T) {
	win := gui.NewWindow(gui.WindowCfg{})
	t.Cleanup(func() { Release(win) })
	if _, err := RenderToImage(win, 1); err == nil {
		t.Fatal("zero-size window: want error")
	}
}

func TestRenderTextFallbackStyle(t *testing.T) {
	win := newWin(t, 120, 60, func(w *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Label("HHHH", gui.TextStyle{
					Size:  32,
					Color: gui.RGB(255, 255, 255),
				}),
			},
		})
	})
	tm, err := prepare(win, 1)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// Drive the command stream directly with a fallback-style
	// RenderText (no TextStylePtr) to cover drawText's else branch
	// and the textured-quad blend path.
	r := &renderer{
		buf:       newBuffer(120, 60),
		scale:     1,
		textSys:   tm.textSys,
		glyphBack: tm.back,
	}
	cmd := gui.RenderCmd{Kind: gui.RenderText, X: 10, Y: 10,
		Text: "HHHH", FontSize: 32, Color: gui.RGB(255, 255, 255)}

	tm.back.buf = nil
	r.drawAll([]gui.RenderCmd{cmd})
	tm.textSys.Commit()
	tm.back.buf = r.buf
	r.buf.clear(gui.RGB(0, 0, 0))
	r.drawAll([]gui.RenderCmd{cmd})
	tm.textSys.Commit()

	if lit := countLitPixels(r.buf.img); lit == 0 {
		t.Fatal("no glyph pixels via fallback style")
	}
}
