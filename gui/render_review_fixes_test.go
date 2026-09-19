package gui

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"testing"
)

// Regression tests for the render* review findings: warn-once SVG
// errors, GradientBorderRects hardening, the render-guard mask,
// SMIL pre-begin phase, float64 animation time, lerpU8 rounding and
// scissor quantization of negative coords.

func TestRenderSvgErrorWarnsOncePerSource(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	w := &Window{}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	shape := &Shape{
		shapeType: shapeSVG,
		X:         10, Y: 20, Width: 100, Height: 100,
		Resource: "<svg></svg>",
	}
	renderSvg(shape, clip, w)
	renderSvg(shape, clip, w)
	if got := strings.Count(buf.String(), "renderSvg:"); got != 1 {
		t.Fatalf("same broken source logged %d times, want 1", got)
	}

	// A different broken source is a new failure and logs once.
	shape.Resource = "<svg broken"
	renderSvg(shape, clip, w)
	renderSvg(shape, clip, w)
	if got := strings.Count(buf.String(), "renderSvg:"); got != 2 {
		t.Fatalf("two broken sources logged %d times, want 2", got)
	}
}

func TestRenderSvgErrorPlaceholderEveryFrame(t *testing.T) {
	log.SetOutput(&bytes.Buffer{})
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	w := &Window{}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	shape := &Shape{
		shapeType: shapeSVG,
		X:         10, Y: 20, Width: 100, Height: 100,
		Resource: "<svg></svg>",
	}
	renderSvg(shape, clip, w)
	renderSvg(shape, clip, w)
	var magentaRects int
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Color == magenta {
			magentaRects++
		}
	}
	if magentaRects != 2 {
		t.Fatalf("magenta placeholders = %d, want 2 (one per frame)",
			magentaRects)
	}
}

func TestGradientBorderRectsNil(t *testing.T) {
	if got := GradientBorderRects(nil); got != [4]GradientBorderRect{} {
		t.Fatalf("nil cmd should return zero rects, got %v", got)
	}
	r := &RenderCmd{X: 1, Y: 2, W: 10, H: 10, Thickness: 2}
	if got := GradientBorderRects(r); got != [4]GradientBorderRect{} {
		t.Fatalf("nil gradient should return zero rects, got %v", got)
	}
}

func TestGradientBorderRectsUnsortedStops(t *testing.T) {
	sorted := &RenderCmd{
		X: 0, Y: 0, W: 100, H: 50, Thickness: 2,
		Gradient: &GradientDef{Stops: []GradientStop{
			{Color: Red, Pos: 0},
			{Color: Blue, Pos: 1},
		}},
	}
	unsorted := &RenderCmd{
		X: 0, Y: 0, W: 100, H: 50, Thickness: 2,
		Gradient: &GradientDef{Stops: []GradientStop{
			{Color: Blue, Pos: 1},
			{Color: Red, Pos: 0},
		}},
	}
	want := GradientBorderRects(sorted)
	got := GradientBorderRects(unsorted)
	if got != want {
		t.Fatalf("unsorted stops sampled %+v, want %+v", got, want)
	}
	if got[0].Color != Red {
		t.Fatalf("top color = %v, want Red (position 0)", got[0].Color)
	}
}

func TestGradientBorderRectsClampsPositions(t *testing.T) {
	r := &RenderCmd{
		X: 0, Y: 0, W: 100, H: 50, Thickness: 2,
		Gradient: &GradientDef{Stops: []GradientStop{
			{Color: Red, Pos: -0.5},
			{Color: Blue, Pos: 1.5},
		}},
	}
	rects := GradientBorderRects(r)
	if rects[0].Color != Red {
		t.Fatalf("top color = %v, want Red (clamped position 0)",
			rects[0].Color)
	}
}

func TestRenderGuardKindFitsMask(t *testing.T) {
	// Runtime twin of the compile-time assertion in
	// render_types.go: the warn-once mask is 32 bits.
	if int(RenderStencilEnd) >= 32 {
		t.Fatalf("kind count %d exceeds the 32-bit guard mask",
			int(RenderStencilEnd)+1)
	}
}

func TestGuardRendererOrSkipIdempotent(t *testing.T) {
	w := &Window{}
	r := RenderCmd{Kind: RenderText}
	if guardRendererOrSkip(r, w) {
		t.Fatal("empty RenderText must be invalid")
	}
	first := w.renderGuardWarned
	if first == 0 {
		t.Fatal("invalid renderer must set a guard bit")
	}
	if guardRendererOrSkip(r, w) {
		t.Fatal("empty RenderText must stay invalid")
	}
	if w.renderGuardWarned != first {
		t.Fatal("guard bitmask must be stable across repeats")
	}
}

func TestSmilPhasePreBeginCycle(t *testing.T) {
	// Fill-backwards before BeginSec with a cycle: the pose is
	// the first keyframe, not the frozen end of an earlier cycle.
	a := &SvgAnimation{DurSec: 2, BeginSec: 5, Cycle: 10, Freeze: true}
	ok, frac, act := smilPhase(a, 1)
	if !ok || frac != 0 || act != 5 {
		t.Fatalf("pre-begin: ok=%v frac=%v act=%v, want true 0 5",
			ok, frac, act)
	}
}

func TestSmilPhaseLongElapsedPrecision(t *testing.T) {
	// At elapsed ~1e7 a float32 second ticks in whole seconds;
	// the phase math must still resolve a quarter-duration offset.
	a := &SvgAnimation{DurSec: 1, BeginSec: 10_000_000}
	ok, frac, _ := smilPhase(a, 10_000_000.25)
	if !ok {
		t.Fatal("expected active phase")
	}
	if !approxEq32(frac, 0.25, 1e-6) {
		t.Fatalf("frac=%v want 0.25 (precision lost)", frac)
	}
}

func TestLerpU8RoundsHalf(t *testing.T) {
	// 127.5 rounds to 128, matching f32ToU8Saturated; truncation
	// gave 127 and drifted tweens from gradient sampling.
	if got := lerpU8(0, 255, 0.5); got != 128 {
		t.Fatalf("lerpU8(0,255,0.5) = %d, want 128", got)
	}
}

func TestWarnSvgOnceStoreStaysBounded(t *testing.T) {
	log.SetOutput(&bytes.Buffer{})
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// A page that mints a new broken source every frame must not
	// grow the warn-once store without limit.
	w := &Window{}
	for i := range capImageCache * 3 {
		warnSvgOnce(w, fmt.Sprintf("<svg id=%d>", i), errors.New("boom"))
	}
	warned := StateMapRead[uint64, bool](w, nsSvgErrWarned)
	if warned == nil {
		t.Fatal("warn-once store was never created")
	}
	if got := warned.Len(); got > capImageCache {
		t.Fatalf("store holds %d entries, want at most %d",
			got, capImageCache)
	}
}

func TestGradientBorderRectsSortedPathNoAlloc(t *testing.T) {
	// Every backend calls this once per gradient-border command
	// per frame. The already-normalized case must not allocate.
	r := &RenderCmd{
		X: 0, Y: 0, W: 100, H: 50, Thickness: 2,
		Gradient: &GradientDef{Stops: []GradientStop{
			{Color: Red, Pos: 0},
			{Color: Blue, Pos: 0.5},
			{Color: Green, Pos: 1},
		}},
	}
	if got := testing.AllocsPerRun(100, func() {
		GradientBorderRects(r)
	}); got != 0 {
		t.Fatalf("sorted stops allocated %v times per call, want 0", got)
	}
}

func TestGradientBorderRectsHugeStopList(t *testing.T) {
	// An untrusted document names the stop count. The sampler must
	// not scan an unbounded list four times per frame.
	stops := make([]GradientStop, gradientBorderMaxStops*2)
	for i := range stops {
		stops[i] = GradientStop{
			Color: Red,
			Pos:   float32(i) / float32(len(stops)-1),
		}
	}
	if gradientStopsNormalized(stops) {
		t.Fatal("an oversized stop list must take the bounded path")
	}
	// Still paints: the bounded path truncates rather than failing.
	if got := GradientBorderRects(&RenderCmd{
		X: 0, Y: 0, W: 100, H: 50, Thickness: 2,
		Gradient: &GradientDef{Stops: stops},
	}); got[0].Color != Red {
		t.Fatalf("top color = %v, want Red", got[0].Color)
	}
}

func TestSmilPhaseDenormalCycleStaysFinite(t *testing.T) {
	// A denormal Cycle overflowed float32 activation to +Inf and
	// returned an infinite frac that poisoned every lerp.
	a := &SvgAnimation{
		DurSec: 1e-44, Cycle: 1e-44, BeginSec: 0, Freeze: true,
	}
	ok, frac, act := smilPhase(a, 1000)
	if !ok {
		t.Fatal("freeze animation must stay active")
	}
	if !f32IsFinite(frac) || frac < 0 || frac > 1 {
		t.Fatalf("frac = %v, want a finite value in [0,1]", frac)
	}
	if !f32IsFinite(act) {
		t.Fatalf("activation = %v, want finite", act)
	}
}

func TestCssIterPhaseDenormalDurStaysFinite(t *testing.T) {
	// A denormal DurSec drove the iteration index past int range,
	// where the float-to-int conversion is implementation defined.
	a := &SvgAnimation{
		DurSec: 1e-44, BeginSec: 0, Iterations: SvgAnimIterInfinite,
	}
	ok, frac, _ := cssIterPhase(a, 1000)
	if !ok {
		t.Fatal("infinite animation must stay active")
	}
	if !f32IsFinite(frac) || frac < 0 || frac > 1 {
		t.Fatalf("frac = %v, want a finite value in [0,1]", frac)
	}
}

func TestGradientBorderRectsUnsortedPathNoAlloc(t *testing.T) {
	// The misordered path must be allocation free too: an app whose
	// BorderGradient stops are out of order reaches this on every
	// frame of every backend.
	r := &RenderCmd{
		X: 0, Y: 0, W: 100, H: 50, Thickness: 2,
		Gradient: &GradientDef{Stops: []GradientStop{
			{Color: Blue, Pos: 1.5},
			{Color: Green, Pos: 0.5},
			{Color: Red, Pos: -0.5},
		}},
	}
	if got := testing.AllocsPerRun(100, func() {
		GradientBorderRects(r)
	}); got != 0 {
		t.Fatalf("unsorted stops allocated %v times per call, want 0", got)
	}
}

func TestNormalizeStopsInlineTruncates(t *testing.T) {
	stops := make([]GradientStop, gradientBorderInlineStops+4)
	for i := range stops {
		// Descending, so the sort has work to do and truncation
		// is visible in the result.
		stops[i] = GradientStop{
			Color: Red,
			Pos:   1 - float32(i)/float32(len(stops)),
		}
	}
	var buf [gradientBorderInlineStops]GradientStop
	got := normalizeStopsInline(stops, &buf)
	if len(got) != gradientBorderInlineStops {
		t.Fatalf("len = %d, want %d", len(got), gradientBorderInlineStops)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Pos < got[i-1].Pos {
			t.Fatalf("not sorted at %d: %v", i, got)
		}
	}
}

func TestSvgFilterBeginBalancedWithZeroBlurLayers(t *testing.T) {
	// A filter with BlurLayers 0 used to emit a RenderFilterBegin
	// that validFilterBeginCmd rejected (Layers < 1), dropping the
	// Begin while the unconditional RenderFilterEnd stayed and
	// unbalancing the bracket.
	w := &Window{}
	const src = "<svg/>"
	cached := &CachedSvg{
		Scale:  1,
		Width:  10,
		Height: 10,
		FilteredGroups: []cachedFilteredGroup{{
			Filter: SvgFilter{StdDev: 2, BlurLayers: 0},
			bBox:   [4]float32{0, 0, 10, 10},
		}},
	}
	sm := StateMap[svgCacheKey, *CachedSvg](w, nsSvgCache, capImageCache)
	sm.Set(buildSvgCacheLookupKey(hashString(src), 0, 10, 10,
		w.svgParseOpts()), cached)

	shape := &Shape{
		shapeType: shapeSVG,
		X:         0, Y: 0, Width: 10, Height: 10,
		Resource: src,
	}
	renderSvg(shape, drawClip{X: 0, Y: 0, Width: 100, Height: 100}, w)

	var begins, ends, layers int
	for _, r := range w.renderers {
		switch r.Kind {
		case RenderFilterBegin:
			begins++
			layers = r.Layers
		case RenderFilterEnd:
			ends++
		}
	}
	if begins != 1 || ends != 1 {
		t.Fatalf("filter bracket = %d Begin / %d End, want 1 / 1",
			begins, ends)
	}
	if layers != 1 {
		t.Fatalf("Layers = %d, want the floor of 1", layers)
	}
}

func TestSanitizeFilterBlurFoldsNonFinite(t *testing.T) {
	stdDev := float32(1e38)
	big := stdDev * float32(34) // +Inf
	cases := []struct {
		name string
		in   float32
		want float32
	}{
		{"overflow", big, maxFilterBlur},
		{"nan", float32(math.NaN()), 0},
		{"negative", -4, 0},
		{"zero", 0, 0},
		{"kept", 12, 12},
		{"capped", maxFilterBlur * 3, maxFilterBlur},
	}
	for _, c := range cases {
		if got := sanitizeFilterBlur(c.in); got != c.want {
			t.Errorf("%s: sanitizeFilterBlur(%v) = %v, want %v",
				c.name, c.in, got, c.want)
		}
	}
}

func TestSvgFilterBlurOverflowStaysBalanced(t *testing.T) {
	// stdDeviation is only checked for NaN/Inf and > 0 at parse time,
	// so a six-digit stdDev times an ordinary tessellation scale
	// overflowed to +Inf. validFilterBeginCmd then dropped the Begin
	// while the unconditional RenderFilterEnd stayed, and the GPU
	// backends composite a stale filter layer on an unmatched End.
	w := &Window{}
	const src = "<svg id=\"blur-overflow\"/>"
	cached := &CachedSvg{
		Scale:  34,
		Width:  10,
		Height: 10,
		FilteredGroups: []cachedFilteredGroup{{
			Filter: SvgFilter{StdDev: 1e38, BlurLayers: 2},
			bBox:   [4]float32{0, 0, 10, 10},
		}},
	}
	sm := StateMap[svgCacheKey, *CachedSvg](w, nsSvgCache, capImageCache)
	sm.Set(buildSvgCacheLookupKey(hashString(src), 0, 340, 340,
		w.svgParseOpts()), cached)

	shape := &Shape{
		shapeType: shapeSVG,
		Width:     340,
		Height:    340,
		Resource:  src,
	}
	renderSvg(shape, drawClip{X: 0, Y: 0, Width: 400, Height: 400}, w)

	var begins, ends int
	var blur float32
	for _, r := range w.renderers {
		switch r.Kind {
		case RenderFilterBegin:
			begins++
			blur = r.BlurRadius
		case RenderFilterEnd:
			ends++
		}
	}
	if begins != 1 || ends != 1 {
		t.Fatalf("filter bracket = %d Begin / %d End, want 1 / 1",
			begins, ends)
	}
	if !f32IsFinite(blur) || blur > maxFilterBlur {
		t.Fatalf("BlurRadius = %v, want finite and <= %v",
			blur, maxFilterBlur)
	}
}

func TestColorFilterBracketSkippedWhenBeginDropped(t *testing.T) {
	// A non-finite color matrix drops the RenderFilterBegin. The End
	// is always valid, so emitting it regardless left the bracket
	// unbalanced for the whole frame.
	w := &Window{}
	cf := &ColorFilter{}
	cf.matrix[0] = float32(math.NaN())
	layout := Layout{Shape: &Shape{
		shapeType: shapeRectangle,
		Width:     10,
		Height:    10,
		fx:        &shapeEffects{ColorFilter: cf},
	}}
	renderLayout(&layout, ColorTransparent,
		drawClip{X: 0, Y: 0, Width: 100, Height: 100}, w)

	for _, r := range w.renderers {
		if r.Kind == RenderFilterBegin || r.Kind == RenderFilterEnd {
			t.Fatalf("emitted %v for a dropped filter begin", r.Kind)
		}
	}
	if w.inFilter {
		t.Fatal("inFilter left set by a bracket that never opened")
	}
}

func TestColorFilterDescendantOpensWhenOuterDropped(t *testing.T) {
	// A dropped outer Begin must not suppress a descendant's own
	// filter: inFilter is set only when the Begin lands, so the
	// child's bracket still opens and the frame holds one balanced
	// pair.
	w := &Window{}
	outer := &ColorFilter{}
	outer.matrix[0] = float32(math.NaN())
	layout := Layout{Shape: &Shape{
		shapeType: shapeRectangle,
		Width:     10,
		Height:    10,
		fx:        &shapeEffects{ColorFilter: outer},
	}, Children: []Layout{{Shape: &Shape{
		shapeType: shapeRectangle,
		Width:     10,
		Height:    10,
		fx:        &shapeEffects{ColorFilter: &ColorFilter{}},
	}}}}
	renderLayout(&layout, ColorTransparent,
		drawClip{X: 0, Y: 0, Width: 100, Height: 100}, w)

	var begins, ends int
	for _, r := range w.renderers {
		switch r.Kind {
		case RenderFilterBegin:
			begins++
		case RenderFilterEnd:
			ends++
		}
	}
	if begins != 1 || ends != 1 {
		t.Fatalf("filter bracket = %d Begin / %d End, want 1 / 1",
			begins, ends)
	}
	if w.inFilter {
		t.Fatal("inFilter left set after the descendant bracket closed")
	}
}

func TestStencilBracketSkippedAtDepthCap(t *testing.T) {
	// StencilDepth is a uint8 and the GPU stencil buffer saturates at
	// 255, so a bracket emitted at the parent's depth would decrement
	// the coverage the parent still needs on its End.
	w := &Window{}
	w.stencilDepth = 255
	layout := Layout{Shape: &Shape{
		shapeType:    shapeRectangle,
		Width:        10,
		Height:       10,
		clipContents: true,
		shapeClip:    drawClip{X: 0, Y: 0, Width: 10, Height: 10},
	}}
	renderLayout(&layout, ColorTransparent,
		drawClip{X: 0, Y: 0, Width: 100, Height: 100}, w)

	for _, r := range w.renderers {
		if r.Kind == RenderStencilBegin || r.Kind == RenderStencilEnd {
			t.Fatalf("emitted %v at the depth cap", r.Kind)
		}
	}
	if w.stencilDepth != 255 {
		t.Fatalf("stencilDepth = %d, want 255 unchanged", w.stencilDepth)
	}
}

func TestApplyDashArrayContribRejectsOversizeStride(t *testing.T) {
	// The stride bound lives in evalAnimContrib, one call frame away.
	// The slots here are a fixed-size array, so the check is repeated
	// at the write site.
	var ov SvgAnimAttrOverride
	a := &SvgAnimation{
		Kind:            SvgAnimDashArray,
		DashKeyframeLen: SvgAnimDashArrayCap + 1,
		Values:          make([]float32, 4*(SvgAnimDashArrayCap+1)),
	}
	applyDashArrayContrib(&ov, a, 0.5)
	if ov.Mask&SvgAnimMaskStrokeDashArray != 0 {
		t.Fatal("oversize stride was applied")
	}
	// A stride within the cap but with too few values must also be a
	// no-op rather than an out-of-range read.
	a.DashKeyframeLen = 4
	a.Values = make([]float32, 4)
	applyDashArrayContrib(&ov, a, 0.5)
	if ov.Mask&SvgAnimMaskStrokeDashArray != 0 {
		t.Fatal("single-keyframe stream was applied")
	}
}
