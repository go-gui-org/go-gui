//go:build !wasm

package soft

// Pixel-level golden files for the software rasterizer (issue #361).
//
// The command goldens in gui/golden_test.go pin colors, geometry and
// insets; they cannot see blend order, antialiasing, glyph placement
// or rounded-corner geometry. Those are exactly the bugs a CPU
// rasterizer introduces, and this suite pins them as pixels: build a
// text-free view, run the real pipeline through RenderToImage, and
// compare the buffer against a committed PNG in testdata/.
//
// Re-record after reading the diff:
//
//	go test ./gui/backend/soft/ -run TestPixelGolden -update
//
// Text-free by design. Every case is checked to emit no text render
// kind, because text goldens cannot be stable across machines: the
// repo bundles no text font (only the icon font), so a family
// resolves through the host catalog, and go-glyph is consumed through
// a local go.work working copy, so glyph outlines can differ between
// the machine that records and the CI machine that checks. No text
// kinds means go-glyph never runs and neither drift source exists.
// Icons are text kinds too, so a case cannot cheat with one.
//
// Comparison is tolerant, not exact. x/image/vector and x/image/draw
// accumulate coverage with arch-specific SIMD, so edge pixels can
// differ by a bit or two across GOARCH (recording happens on macOS
// arm64; CI runs amd64 Linux and Windows). pixelTol absorbs the
// per-channel delta and maxDiffFraction caps how many pixels may
// differ at all. The trade is named in the issue: a small real
// regression inside the tolerance is missed. On failure the actual,
// expected and diff images are written under testdata/failures/ so a
// red run is reviewable; CI uploads that directory as an artifact.

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

var updatePixelGolden = flag.Bool("update", false,
	"re-record pixel golden PNG files")

// Golden window size. Matches the command goldens' 320x240 so the two
// suites see the same layout, at scale 1 so the PNGs stay small.
const (
	pixelGoldenW = 320
	pixelGoldenH = 240

	// pixelTol is the per-channel (R, G, B, A) delta that a pixel may
	// differ by before it counts as differing. Calibrated for the
	// ±1-bit coverage differences SIMD accumulation can produce
	// between GOARCHs, while a flat-fill color change — the common
	// styling regression — still moves every pixel of the shape by
	// well over 3 and is caught.
	pixelTol = 3

	// maxDiffFraction is the share of pixels that may differ before
	// the case fails. Antialiasing edges are where SIMD shows up, and
	// a 320x240 frame has a few hundred edge pixels; 0.5% (384 px) is
	// far above that and far below a moved or resized shape.
	maxDiffFraction = 0.005
)

// pixelTheme is one recorded theme for a pixel case.
type pixelTheme struct {
	theme gui.Theme
	name  string
}

// pixelCase is one recorded view. build returns the whole window
// content: the harness has no wrapper, so a case that wants the theme
// background visible around its content must size itself smaller than
// the window.
type pixelCase struct {
	name string
	// focusID, when set, is focused before the frame renders, so a
	// focus ring is recorded rather than inferred.
	focusID string
	// themes overrides the dark/light pair. Cases whose geometry is
	// where a platform theme's override surface meets the base ladder
	// record the platform themes too (visual-refresh §10): an
	// unrecorded platform theme is where a base-ladder change silently
	// breaks a hand-tuned override. Nil records pixelThemes().
	themes []pixelTheme
	build  func(*gui.Window) gui.View
}

// pixelThemes are recorded for every case, mirroring the command
// goldens: de-emphasis alphas and the window background are
// per-theme, and only a side-by-side recording catches a drift.
func pixelThemes() []pixelTheme {
	return []pixelTheme{
		{gui.ThemeDark, "dark"},
		{gui.ThemeLight, "light"},
	}
}

func TestPixelGolden(t *testing.T) {
	for _, c := range pixelCases() {
		themes := c.themes
		if themes == nil {
			themes = pixelThemes()
		}
		for _, th := range themes {
			name := c.name + "." + th.name
			t.Run(name, func(t *testing.T) {
				img, err := renderPixelGolden(t, th.theme, c)
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				checkPixelGolden(t, name, img)
			})
		}
	}
}

// renderPixelGolden drives one frame of one case under one theme and
// returns the rendered buffer. The window carries no BgColor, so the
// clear color is the theme's background — which is part of what the
// recording pins.
func renderPixelGolden(
	t *testing.T, theme gui.Theme, c pixelCase,
) (*image.RGBA, error) {
	t.Helper()
	w := gui.NewWindow(gui.WindowCfg{
		Width:  pixelGoldenW,
		Height: pixelGoldenH,
		OnInit: func(w *gui.Window) { w.UpdateView(c.build) },
	})
	t.Cleanup(func() { Release(w) })
	w.SetTheme(theme)
	if c.focusID != "" {
		w.SetFocus(c.focusID)
	}
	img, err := RenderToImage(w, 1)
	if err != nil {
		return nil, err
	}
	assertTextFree(t, w)
	return img, nil
}

// assertTextFree fails a case whose command stream carries a text or
// shader kind. The gate is what makes the recordings stable across
// machines (see the file header); it also catches a case that
// silently loses content to RenderCustomShader, which the renderer
// skips by design.
func assertTextFree(t *testing.T, w *gui.Window) {
	t.Helper()
	for i := range w.Renderers() {
		switch w.Renderers()[i].Kind {
		case gui.RenderText, gui.RenderLayout, gui.RenderRTF,
			gui.RenderLayoutTransformed, gui.RenderTextPath,
			gui.RenderTermGrid, gui.RenderCustomShader:
			t.Fatalf("case emits %v; pixel goldens are text-free "+
				"by design", w.Renderers()[i].Kind)
		}
	}
}

// checkPixelGolden compares img against testdata/<name>.png, or
// records it under -update.
func checkPixelGolden(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	path := filepath.Join("testdata", name+".png")
	if *updatePixelGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		if err = png.Encode(f, img); err != nil {
			_ = f.Close()
			t.Fatalf("encode %s: %v", path, err)
		}
		if err = f.Close(); err != nil {
			t.Fatalf("close %s: %v", path, err)
		}
		t.Logf("recorded %s", path)
		return
	}

	want, err := loadPremul(path)
	if err != nil {
		t.Fatalf("read %s: %v (run with -update to record)", path, err)
	}
	if diffPixels, maxDelta, err := pixelDiff(img, want, pixelTol); err != nil {
		t.Fatalf("compare: %v", err)
	} else if frac := float64(diffPixels) / float64(img.Bounds().Dx()*img.Bounds().Dy()); frac > maxDiffFraction {
		dir := filepath.Join("testdata", "failures", name)
		writeFailure(dir, img, want)
		t.Errorf("golden mismatch for %s: %d/%d pixels differ "+
			"(%.2f%% > %.2f%%), max channel delta %d — artifacts in %s/",
			name, diffPixels, img.Bounds().Dx()*img.Bounds().Dy(),
			frac*100, maxDiffFraction*100, maxDelta, dir)
	}
}

// loadPremul decodes a PNG reference into a premultiplied RGBA
// buffer, the same color space RenderToImage returns. PNG stores
// straight alpha; draw.Draw converts through the color model,
// including the 16-bit intermediate rounding that costs an edge pixel
// a unit now and then — inside pixelTol.
func loadPremul(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst, nil
}

// pixelDiff counts the pixels whose channels differ from the
// reference by more than tol, and the largest per-channel delta
// anywhere. It walks got and want's bounds, which must agree.
func pixelDiff(
	got, want *image.RGBA, tol uint8,
) (diffPixels int, maxDelta uint8, err error) {
	b := got.Bounds()
	if b != want.Bounds() {
		return 0, 0, fmt.Errorf("bounds: got %v, want %v",
			b, want.Bounds())
	}
	if b.Dx()*b.Dy() == 0 {
		return 0, 0, fmt.Errorf("empty image %v", b)
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gi := got.PixOffset(x, y)
			wi := want.PixOffset(x, y)
			differing := false
			for c := range 4 {
				d := absDiff(got.Pix[gi+c], want.Pix[wi+c])
				if d > maxDelta {
					maxDelta = d
				}
				if d > tol {
					differing = true
				}
			}
			if differing {
				diffPixels++
			}
		}
	}
	return diffPixels, maxDelta, nil
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// writeFailure saves the actual, expected and a diff (differing
// pixels in magenta) under dir so a red run is reviewable, locally
// and as a CI artifact.
func writeFailure(dir string, got, want *image.RGBA) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = writePNG(filepath.Join(dir, "actual.png"), got)
	_ = writePNG(filepath.Join(dir, "expected.png"), want)

	diff := image.NewRGBA(got.Bounds())
	draw.Draw(diff, diff.Bounds(), got, got.Bounds().Min, draw.Src)
	b := got.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gi := got.PixOffset(x, y)
			wi := want.PixOffset(x, y)
			for c := range 4 {
				if absDiff(got.Pix[gi+c], want.Pix[wi+c]) > pixelTol {
					di := diff.PixOffset(x, y)
					diff.Pix[di] = 255
					diff.Pix[di+1] = 0
					diff.Pix[di+2] = 255
					diff.Pix[di+3] = 255
					break
				}
			}
		}
	}
	_ = writePNG(filepath.Join(dir, "diff.png"), diff)
}

func writePNG(path string, img *image.RGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// TestPixelDiffGuardsHarness pins the comparator itself: identical
// images pass, a channel shift beyond the tolerance counts every
// affected pixel, and a shift inside the tolerance passes.
func TestPixelDiffGuardsHarness(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range base.Pix {
		base.Pix[i] = 128
	}
	copy := image.NewRGBA(base.Bounds())
	draw.Draw(copy, copy.Bounds(), base, base.Bounds().Min, draw.Src)

	if n, maxD, err := pixelDiff(base, copy, pixelTol); err != nil || n != 0 || maxD != 0 {
		t.Fatalf("identical images: n=%d max=%d err=%v", n, maxD, err)
	}

	shift := image.NewRGBA(base.Bounds())
	draw.Draw(shift, shift.Bounds(), base, base.Bounds().Min, draw.Src)
	shift.Pix[0] = 255 // one pixel, one channel, +127
	if n, maxD, err := pixelDiff(base, shift, pixelTol); err != nil || n != 1 || maxD != 127 {
		t.Fatalf("shifted pixel: n=%d max=%d err=%v, want 1/127", n, maxD, err)
	}

	// A change inside the tolerance (e.g. SIMD coverage noise) must
	// not count as a differing pixel.
	noise := image.NewRGBA(base.Bounds())
	draw.Draw(noise, noise.Bounds(), base, base.Bounds().Min, draw.Src)
	noise.Pix[4] = 130 // +2
	if n, _, err := pixelDiff(base, noise, pixelTol); err != nil || n != 0 {
		t.Fatalf("within-tolerance noise: n=%d err=%v, want 0", n, err)
	}
}
