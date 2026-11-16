//go:build !wasm

package soft

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
)

// Steady-state allocation gate for every pixel-golden case (#1282).
//
// The pixel goldens cover every draw path the soft rasterizer has —
// solid fills, gradients, scaled images, blends, SVG meshes, effects —
// so pinning each of them at 0 allocations per Draw pins the whole
// renderer, not one view. TestRendererDrawAllocs covers text on top.

// pixelDumpDir, when set, writes each case's live Draw output there as
// PNG. Used to diff a change byte-for-byte against a baseline dump,
// which the tolerant golden comparison cannot do.
var pixelDumpDir = flag.String("pixeldump", "",
	"write each pixel-golden case's Draw output to this directory")

// newPixelCaseRenderer builds a live Renderer for one case and theme the
// way TestRendererMatchesPixelGoldens does, settled and ready to Draw.
func newPixelCaseRenderer(
	tb testing.TB, c pixelCase, th pixelTheme,
) (*Renderer, *gui.Window) {
	tb.Helper()
	w := gui.NewWindow(gui.WindowCfg{
		Width:  pixelGoldenW,
		Height: pixelGoldenH,
		OnInit: func(w *gui.Window) { w.SetView(c.build) },
	})
	w.SetTheme(th.theme)
	w.Config.OnInit(w)
	lr, err := NewRenderer(w, 1)
	if err != nil {
		tb.Fatalf("NewRenderer: %v", err)
	}
	tb.Cleanup(lr.Release)
	w.SetTextMeasurer(goldenMetrics{})
	if c.focusID != "" {
		w.SetFocus(c.focusID)
	}
	settle(w)
	// Warm frames: image decode, the layer pool and every scratch
	// buffer reach their steady size.
	for range 3 {
		if _, err = lr.Draw(w, pixelGoldenW, pixelGoldenH); err != nil {
			tb.Fatalf("Draw: %v", err)
		}
	}
	return lr, w
}

// forEachPixelCase runs fn once per case and theme as a subtest.
func forEachPixelCase(t *testing.T, fn func(*testing.T, pixelCase, pixelTheme)) {
	for _, c := range pixelCases() {
		themes := c.themes
		if themes == nil {
			themes = pixelThemes()
		}
		for _, th := range themes {
			t.Run(c.name+"."+th.name, func(t *testing.T) { fn(t, c, th) })
		}
	}
}

func TestPixelGoldenDrawAllocs(t *testing.T) {
	if inputRaceEnabled {
		t.Skip("AllocsPerRun is process-wide and unstable under race instrumentation")
	}
	forEachPixelCase(t, func(t *testing.T, c pixelCase, th pixelTheme) {
		lr, w := newPixelCaseRenderer(t, c, th)
		draw := func() { _, _ = lr.Draw(w, pixelGoldenW, pixelGoldenH) }
		n, stacks := drawAllocs(draw, 5)
		if n > 0 {
			t.Errorf("5 steady-state Draws allocate %d times, want 0:\n%s", n, stacks)
		}
	})
}

// rendererDrawFunc is the function every counted allocation must sit under.
const rendererDrawFunc = "/gui/backend/soft.(*Renderer).Draw"

// drawAllocs runs draw `runs` times and returns how many heap allocations
// happened under (*Renderer).Draw, with their stacks.
//
// testing.AllocsPerRun reads process-wide malloc counters. On a slow
// runner a gradient frame takes long enough to be preempted, and
// goroutines left behind by earlier tests allocate inside the window:
// Windows CI reported 12-32 per frame on the two slowest cases while
// their Draw allocated nothing (#1282). Attributing each allocation by
// stack from the memory profile, sampled at rate 1, counts only Draw's.
func drawAllocs(draw func(), runs int) (int64, string) {
	prev := runtime.MemProfileRate
	runtime.MemProfileRate = 1
	defer func() { runtime.MemProfileRate = prev }()

	// The profile publishes a GC cycle late; two cycles flush everything
	// allocated before the snapshot.
	runtime.GC()
	runtime.GC()
	before := drawAllocSites()
	for range runs {
		draw()
	}
	runtime.GC()
	runtime.GC()
	var total int64
	var sb strings.Builder
	for stack, n := range drawAllocSites() {
		if d := n - before[stack]; d > 0 {
			total += d
			fmt.Fprintf(&sb, "%d allocs at:\n%s", d, stack)
		}
	}
	return total, sb.String()
}

// drawAllocSites returns the cumulative allocation count of every
// memory-profile stack that passes through rendererDrawFunc, keyed by the
// formatted stack.
func drawAllocSites() map[string]int64 {
	var recs []runtime.MemProfileRecord
	n, _ := runtime.MemProfile(nil, true)
	for {
		// Headroom: the profile can grow between the sizing call and
		// this one.
		recs = make([]runtime.MemProfileRecord, n+64)
		var ok bool
		if n, ok = runtime.MemProfile(recs, true); ok {
			recs = recs[:n]
			break
		}
	}
	sites := make(map[string]int64)
	for i := range recs {
		var sb strings.Builder
		under := false
		frames := runtime.CallersFrames(recs[i].Stack())
		for {
			f, more := frames.Next()
			if strings.HasSuffix(f.Function, rendererDrawFunc) {
				under = true
			}
			fmt.Fprintf(&sb, "\t%s\n", f.Function)
			if !more {
				break
			}
		}
		if under {
			sites[sb.String()] += recs[i].AllocObjects
		}
	}
	return sites
}

// TestPixelGoldenDump writes every case's Draw output under -pixeldump;
// a no-op otherwise.
func TestPixelGoldenDump(t *testing.T) {
	if *pixelDumpDir == "" {
		t.Skip("set -pixeldump to write the frames")
	}
	if err := os.MkdirAll(*pixelDumpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	forEachPixelCase(t, func(t *testing.T, c pixelCase, th pixelTheme) {
		lr, w := newPixelCaseRenderer(t, c, th)
		img, err := lr.Draw(w, pixelGoldenW, pixelGoldenH)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(*pixelDumpDir, c.name+"."+th.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

// BenchmarkPixelGoldenDraw times one steady-state Draw per case, dark
// theme only: the themes share geometry, so one is enough for timing.
func BenchmarkPixelGoldenDraw(b *testing.B) {
	for _, c := range pixelCases() {
		th := pixelThemes()[0]
		if c.themes != nil {
			th = c.themes[0]
		}
		b.Run(c.name, func(b *testing.B) {
			lr, w := newPixelCaseRenderer(b, c, th)
			b.ReportAllocs()
			for b.Loop() {
				_, _ = lr.Draw(w, pixelGoldenW, pixelGoldenH)
			}
		})
	}
}

// TestFillPathSampledMatchesGeneric pins compositeSampled to the generic
// rasterizer loop it replaces: fully covered pixels byte-identical, edge
// pixels within one level (the mask keeps 8 of vector's 16 coverage bits).
func TestFillPathSampledMatchesGeneric(t *testing.T) {
	// A non-uniform, translucent source: an NRGBA ramp, so At varies per
	// pixel and RGBA() returns premultiplied values below full alpha.
	const n = 16
	src := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			src.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 16), G: uint8(y * 16), B: 0x80, A: uint8(0x40 + x*8),
			})
		}
	}
	// A circle-ish octagon: full coverage inside, partial on the edges.
	emit := func(z *vector.Rasterizer, ox, oy float32) {
		z.MoveTo(5.3-ox, 1.2-oy)
		z.LineTo(10.7-ox, 1.2-oy)
		z.LineTo(14.8-ox, 5.3-oy)
		z.LineTo(14.8-ox, 10.7-oy)
		z.LineTo(10.7-ox, 14.8-oy)
		z.LineTo(5.3-ox, 14.8-oy)
		z.LineTo(1.2-ox, 10.7-oy)
		z.LineTo(1.2-ox, 5.3-oy)
		z.ClosePath()
	}
	region := image.Rect(0, 0, n, n)
	got := newBuffer(n, n)
	want := newBuffer(n, n)
	bg := gui.RGB(30, 60, 90)
	got.clear(bg)
	want.clear(bg)

	got.fillPath(region, src, emit)

	// Reference: the generic path fillPath used before #1282.
	var z vector.Rasterizer
	z.Reset(n, n)
	emit(&z, 0, 0)
	z.Draw(want.img, region, src, region.Min)

	for i := range got.img.Pix {
		d := int(got.img.Pix[i]) - int(want.img.Pix[i])
		if d < -1 || d > 1 {
			t.Fatalf("byte %d: got %d, want %d (±1)", i, got.img.Pix[i], want.img.Pix[i])
		}
	}
	// (8,8) is deep inside the octagon: full coverage must match exactly.
	off := got.img.PixOffset(8, 8)
	if g, w := got.img.Pix[off:off+4], want.img.Pix[off:off+4]; string(g) != string(w) {
		t.Errorf("fully covered pixel: got %v, want %v", g, w)
	}
}

// TestCompositeSampledClipsToBuffer drives compositeSampled with a region
// wider than the buffer. The rasterizer clips to its destination; the typed
// loop must too, not wrap rows or index past Pix.
func TestCompositeSampledClipsToBuffer(t *testing.T) {
	b := newBuffer(4, 4)
	b.clear(gui.RGB(0, 0, 0))
	region := image.Rect(-2, -2, 6, 6)
	mask := b.mask.reset(region)
	for i := range mask.Pix {
		mask.Pix[i] = 0xff
	}
	white := image.NewUniform(color.RGBA{0xff, 0xff, 0xff, 0xff})
	b.compositeSampled(region, white, mask)
	for i, v := range b.img.Pix {
		if v != 0xff {
			t.Fatalf("byte %d = %d, want 255: the in-bounds part must still draw", i, v)
		}
	}
}
