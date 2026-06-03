package svg

import (
	"math"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

const epsilon32 = 1e-4

// firstShapePath returns the first VectorPath whose segments are
// non-empty (i.e. a real shape and not a clip-mask placeholder).
func firstShapePath(t *testing.T, vg *VectorGraphic) VectorPath {
	t.Helper()
	for _, p := range vg.Paths {
		if len(p.Segments) > 0 {
			return p
		}
	}
	t.Fatalf("no shape paths in parsed SVG")
	return VectorPath{}
}

func nearEq32(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= epsilon32
}

func assertTransform(t *testing.T, got [6]float32, want [6]float32) {
	t.Helper()
	for i := range 6 {
		if !nearEq32(got[i], want[i]) {
			t.Fatalf("transform[%d] = %v, want %v (full got=%v want=%v)",
				i, got[i], want[i], got, want)
		}
	}
}

// Regression: prior to nested-<svg> support, descendants of an inner
// <svg> were silently dropped. The inner circle must now render.
func TestNestedSvg_InnerContentRenders(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10" y="10" width="50" height="50" viewBox="0 0 10 10">
			<circle cx="5" cy="5" r="5" fill="red"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("expected inner circle to produce a path; got 0")
	}
}

// Translate-only nested viewport (no viewBox): transform reduces to
// translate(x, y). Multiple paths inherit the same composed matrix.
func TestNestedSvg_TranslateOnly(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10" y="20">
			<rect x="0" y="0" width="5" height="5"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{1, 0, 0, 1, 10, 20})
}

// viewBox on inner svg establishes user-space scaling. inner viewBox
// 0 0 10 10 mapped onto a 50x50 viewport at (10,10) -> uniform scale
// 5, translate (10,10).
func TestNestedSvg_ViewBoxScale(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10" y="10" width="50" height="50" viewBox="0 0 10 10">
			<rect x="0" y="0" width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{5, 0, 0, 5, 10, 10})
}

// preserveAspectRatio="xMidYMid meet" picks min(sx,sy) and centers
// the residual in the major axis. Viewport 50x100, inner viewBox
// 0 0 10 10 -> scale 5, y residual 50, half = 25.
func TestNestedSvg_PreserveAspectMeet(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="0" y="0" width="50" height="100" viewBox="0 0 10 10"
			preserveAspectRatio="xMidYMid meet">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{5, 0, 0, 5, 0, 25})
}

// preserveAspectRatio="none" keeps independent scales.
func TestNestedSvg_PreserveAspectNone(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="0" y="0" width="50" height="100" viewBox="0 0 10 10"
			preserveAspectRatio="none">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{5, 0, 0, 10, 0, 0})
}

// SVG2: `transform=` on a nested <svg> applies AFTER the viewport
// mapping. Composed = parent × elemTransform × viewportTx.
// elemTx = translate(7,3); viewportTx = translate(10,20) (no viewBox).
// Final translate = (17,23); scale untouched.
func TestNestedSvg_TransformAttrComposition(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10" y="20" transform="translate(7,3)">
			<rect x="0" y="0" width="5" height="5"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{1, 0, 0, 1, 17, 23})
}

// Missing width/height on inner svg defaults to 100% of the parent
// viewport. Parent 100x100, inner viewBox 0 0 50 50 -> scale 2.
func TestNestedSvg_DefaultsToParentViewport(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg viewBox="0 0 50 50">
			<rect width="50" height="50"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{2, 0, 0, 2, 0, 0})
}

// Deep-but-legal nesting: parseSvgContent bails at maxGroupDepth and
// never recurses unbounded. Stay below the decoder's +8 slack so the
// XML tree itself decodes cleanly; the assertion is that parsing
// returns without panic, with the rect dropped past the depth cap.
func TestNestedSvg_DepthCapBounded(t *testing.T) {
	depth := maxGroupDepth + 4
	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 100 100">`)
	for range depth {
		b.WriteString(`<svg>`)
	}
	b.WriteString(`<rect x="0" y="0" width="1" height="1"/>`)
	for range depth {
		b.WriteString(`</svg>`)
	}
	b.WriteString(`</svg>`)
	vg, err := parseSvg(b.String())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) != 0 {
		t.Fatalf("expected depth cap to drop deeply nested rect; got %d paths",
			len(vg.Paths))
	}
}

// preserveAspectRatio="xMidYMid slice" picks max(sx,sy) and centers
// the residual in the major axis. Viewport 50x100, inner viewBox
// 0 0 10 10 -> scale 10 (uniform max), x residual -50, half = -25.
func TestNestedSvg_PreserveAspectSlice(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="0" y="0" width="50" height="100" viewBox="0 0 10 10"
			preserveAspectRatio="xMidYMid slice">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{10, 0, 0, 10, -25, 0})
}

// Negative viewBox origin: vbX=-5, vbY=-5 shifts user-space so (0,0)
// in descendant coords lands at +5,+5 in outer. Viewport 100x100,
// scale 10. tx = 0 + 0*xFrac - 10*(-5) = 50.
func TestNestedSvg_NegativeViewBoxOrigin(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="0" y="0" width="100" height="100"
			viewBox="-5 -5 10 10" preserveAspectRatio="none">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{10, 0, 0, 10, 50, 50})
}

// Sibling nested <svg> elements at the same depth must each see the
// outer viewport, not the previous sibling's. Test: two siblings,
// each width="50%" of parent 100. Both should scale identically.
func TestNestedSvg_SiblingViewportRestored(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="0" y="0" width="50%" height="50%" viewBox="0 0 10 10"
			preserveAspectRatio="none">
			<rect width="10" height="10"/>
		</svg>
		<svg x="50" y="50" width="50%" height="50%" viewBox="0 0 10 10"
			preserveAspectRatio="none">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var shapes []VectorPath
	for _, p := range vg.Paths {
		if len(p.Segments) > 0 {
			shapes = append(shapes, p)
		}
	}
	if len(shapes) != 2 {
		t.Fatalf("expected 2 shape paths; got %d", len(shapes))
	}
	assertTransform(t, shapes[0].Transform, [6]float32{5, 0, 0, 5, 0, 0})
	assertTransform(t, shapes[1].Transform, [6]float32{5, 0, 0, 5, 50, 50})
}

// Adversarial: NaN/Inf/giant attribute values must not poison the
// transform. computeNestedSvgViewport returns a finite matrix.
func TestNestedSvg_HardenNaNInfInputs(t *testing.T) {
	cases := []map[string]string{
		{"x": "NaN", "y": "Inf", "width": "50", "height": "50"},
		{"width": "-Inf", "height": "NaN", "viewBox": "0 0 10 10"},
		{"x": "1e30%", "y": "1e30%", "width": "100", "height": "100"},
		{"viewBox": "NaN NaN 10 10", "width": "100", "height": "100"},
		{"viewBox": "0 0 Inf Inf", "width": "100", "height": "100"},
		{"viewBox": "0 0 0 0", "width": "100", "height": "100"},
		{"viewBox": "0 0 -5 -5", "width": "100", "height": "100"},
	}
	parent := viewportRect{X: 0, Y: 0, W: 100, H: 100}
	for i, attrs := range cases {
		_, _, m := computeNestedSvgViewport(attrs, parent)
		for j, v := range m {
			f := float64(v)
			if math.IsNaN(f) || math.IsInf(f, 0) {
				t.Fatalf("case %d: m[%d] non-finite: %v (full=%v)",
					i, j, v, m)
			}
		}
	}
}

// Adversarial: poisoned parent viewport must be sanitized before use.
func TestNestedSvg_HardenPoisonedParent(t *testing.T) {
	parent := viewportRect{
		X: float32(math.NaN()),
		Y: float32(math.Inf(1)),
		W: float32(math.NaN()),
		H: float32(math.Inf(-1)),
	}
	_, _, m := computeNestedSvgViewport(map[string]string{
		"width": "50%", "height": "50%",
	}, parent)
	for i, v := range m {
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			t.Fatalf("m[%d] non-finite: %v (full=%v)", i, v, m)
		}
	}
}

// Defaults & percent: width="100%", height="50%" on inner svg should
// resolve against the parent viewport's user-space dimensions
// (100x100 here -> 100x50).
func TestNestedSvg_PercentWidthHeight(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg width="100%" height="50%" viewBox="0 0 10 10"
			preserveAspectRatio="none">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{10, 0, 0, 5, 0, 0})
}

// All 9 align modes (excluding "none") drive xFrac/yFrac via
// nestedAlignFractions. Inner viewBox is square (10x10) but outer
// viewport is 100x50; meet picks scale 5, leaves 50px x-residual.
// xFrac dictates where the residual lands.
func TestNestedSvg_AlignFractionsAllModes(t *testing.T) {
	cases := []struct {
		mode   string
		wantTx float32
		wantTy float32
	}{
		{"xMinYMin meet", 0, 0},
		{"xMidYMin meet", 25, 0},
		{"xMaxYMin meet", 50, 0},
		{"xMinYMid meet", 0, 0},
		{"xMidYMid meet", 25, 0},
		{"xMaxYMid meet", 50, 0},
		{"xMinYMax meet", 0, 0},
		{"xMidYMax meet", 25, 0},
		{"xMaxYMax meet", 50, 0},
	}
	for _, tc := range cases {
		svg := `<svg viewBox="0 0 200 100">
			<svg width="100" height="50" viewBox="0 0 10 10"
				preserveAspectRatio="` + tc.mode + `">
				<rect width="10" height="10"/>
			</svg>
		</svg>`
		vg, err := parseSvg(svg)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.mode, err)
		}
		p := firstShapePath(t, vg)
		want := [6]float32{5, 0, 0, 5, tc.wantTx, tc.wantTy}
		if !nearEq32(p.Transform[4], want[4]) ||
			!nearEq32(p.Transform[5], want[5]) {
			t.Fatalf("%s: tx,ty = (%v,%v); want (%v,%v); full=%v",
				tc.mode, p.Transform[4], p.Transform[5],
				want[4], want[5], p.Transform)
		}
	}
}

// Vertical residual coverage: y-axis residual lands per yFrac. Inner
// viewport 50x100, viewBox 10x10 -> meet picks min(5,10)=5; y residual
// = 100 - 5*10 = 50. yFrac × 50 yields the ty.
func TestNestedSvg_AlignFractionsYAxis(t *testing.T) {
	cases := []struct {
		mode   string
		wantTy float32
	}{
		{"xMinYMin meet", 0},
		{"xMinYMid meet", 25},
		{"xMinYMax meet", 50},
	}
	for _, tc := range cases {
		svg := `<svg viewBox="0 0 100 200">
			<svg width="50" height="100" viewBox="0 0 10 10"
				preserveAspectRatio="` + tc.mode + `">
				<rect width="10" height="10"/>
			</svg>
		</svg>`
		vg, err := parseSvg(svg)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.mode, err)
		}
		p := firstShapePath(t, vg)
		if !nearEq32(p.Transform[5], tc.wantTy) {
			t.Fatalf("%s: ty=%v want %v (full=%v)",
				tc.mode, p.Transform[5], tc.wantTy, p.Transform)
		}
	}
}

// display:none on a nested <svg> drops the entire subtree.
func TestNestedSvg_DisplayNoneSkipsSubtree(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<rect width="10" height="10"/>
		<svg display="none">
			<rect width="20" height="20"/>
			<circle r="5"/>
		</svg>
		<rect width="30" height="30"/>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var shapes int
	for _, p := range vg.Paths {
		if len(p.Segments) > 0 {
			shapes++
		}
	}
	if shapes != 2 {
		t.Fatalf("expected 2 shape paths (siblings of hidden svg); got %d",
			shapes)
	}
}

// transform= on nested <svg> composes BEFORE viewport mapping:
// final = parent · elemTransform · viewportTx. elemTx scale(2) and
// viewportTx scale(5) (10x10 viewBox into 50x50 rect) → diag scale 10,
// translate from rect origin (10,20).
func TestNestedSvg_TransformWithViewBoxCompose(t *testing.T) {
	svg := `<svg viewBox="0 0 200 200">
		<svg x="10" y="20" width="50" height="50" viewBox="0 0 10 10"
			transform="scale(2)" preserveAspectRatio="none">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	// scale(2) applied first; then viewportTx places the 10x10 viewBox
	// into the 50x50 rect at (10,20). Compose: outer point =
	// scale(2) · translate(10,20)·scale(5) · innerPoint
	//        = [10,0,0,10, 20, 40] · innerPoint.
	assertTransform(t, p.Transform, [6]float32{10, 0, 0, 10, 20, 40})
}

// Three levels of nested <svg>, each scaling 2x via viewBox, compose
// into a final scale of 8. Verifies state.curViewport tracks across
// recursion correctly.
func TestNestedSvg_ThreeLevelComposition(t *testing.T) {
	svg := `<svg viewBox="0 0 80 80">
		<svg width="80" height="80" viewBox="0 0 40 40">
			<svg width="40" height="40" viewBox="0 0 20 20">
				<svg width="20" height="20" viewBox="0 0 10 10">
					<rect width="10" height="10"/>
				</svg>
			</svg>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{8, 0, 0, 8, 0, 0})
}

// fill on outer <svg> must inherit through a nested <svg> wrapper to
// descendant shapes. Regression: nested-svg cascade should propagate
// presentation attrs the same as <g>.
func TestNestedSvg_FillInheritsThroughViewport(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100" fill="red">
		<svg width="50" height="50" viewBox="0 0 10 10">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	want := gui.SvgColor{R: 255, G: 0, B: 0, A: 255}
	if p.FillColor != want {
		t.Fatalf("FillColor=%+v; want %+v", p.FillColor, want)
	}
}

// Step 5: descendants of a nested <svg> inherit a synthesized clip
// rectangle so out-of-viewport geometry is masked at tessellation
// time. The clip id "__nested_svg_clip_1" is minted for the first
// nested viewport encountered.
func TestNestedSvg_ClipsOutsideContent(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10" y="10" width="20" height="20">
			<circle cx="100" cy="100" r="5" fill="red"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	const want = "__nested_svg_clip_1"
	if p.ClipPathID != want {
		t.Fatalf("ClipPathID=%q; want %q", p.ClipPathID, want)
	}
	clip, ok := vg.ClipPaths[want]
	if !ok {
		t.Fatalf("vg.ClipPaths[%q] missing", want)
	}
	if len(clip) != 1 {
		t.Fatalf("clip path count=%d; want 1", len(clip))
	}
	if len(clip[0].Segments) == 0 {
		t.Fatal("clip path has no segments")
	}
}

// Inside-viewport descendants get the same inherited clip id.
func TestNestedSvg_ClipsInsideContent(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10" y="10" width="20" height="20">
			<circle cx="5" cy="5" r="2"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	if p.ClipPathID != "__nested_svg_clip_1" {
		t.Fatalf("ClipPathID=%q", p.ClipPathID)
	}
	if _, ok := vg.ClipPaths["__nested_svg_clip_1"]; !ok {
		t.Fatal("clip registry entry missing")
	}
}

// Empty <svg/> must not register a viewport clip — nothing to mask.
func TestNestedSvg_EmptyNoClip(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10" y="10" width="20" height="20"/>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for k := range vg.ClipPaths {
		if strings.HasPrefix(k, "__nested_svg_clip_") {
			t.Fatalf("unexpected synth clip %q for empty <svg/>", k)
		}
	}
}

// Sibling nested <svg>s mint distinct clip ids; descendants of each
// inherit the matching one.
func TestNestedSvg_SiblingsDistinctClips(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="0" y="0" width="40" height="40">
			<rect width="10" height="10"/>
		</svg>
		<svg x="50" y="50" width="40" height="40">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var shapes []VectorPath
	for _, p := range vg.Paths {
		if len(p.Segments) > 0 {
			shapes = append(shapes, p)
		}
	}
	if len(shapes) != 2 {
		t.Fatalf("shapes=%d want 2", len(shapes))
	}
	if shapes[0].ClipPathID != "__nested_svg_clip_1" {
		t.Fatalf("shape0 clip=%q", shapes[0].ClipPathID)
	}
	if shapes[1].ClipPathID != "__nested_svg_clip_2" {
		t.Fatalf("shape1 clip=%q", shapes[1].ClipPathID)
	}
	if _, ok := vg.ClipPaths["__nested_svg_clip_1"]; !ok {
		t.Fatal("clip 1 not registered")
	}
	if _, ok := vg.ClipPaths["__nested_svg_clip_2"]; !ok {
		t.Fatal("clip 2 not registered")
	}
}

// Doubly nested <svg>: innermost descendant inherits the innermost
// viewport clip. Outer clips are overridden by cascade — v1 limitation
// (no clip-path intersection composition).
func TestNestedSvg_DoublyNestedInheritsInnermost(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="0" y="0" width="50" height="50">
			<svg x="0" y="0" width="25" height="25">
				<rect width="5" height="5"/>
			</svg>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	if p.ClipPathID != "__nested_svg_clip_2" {
		t.Fatalf("ClipPathID=%q want __nested_svg_clip_2", p.ClipPathID)
	}
}

// Step 6: <clipPath> defined inside a nested <svg>'s <defs> reaches
// the global registry. parseDefsClipPaths uses findAllByName which
// already recurses; this test locks the behavior in.
func TestNestedSvg_DefsClipPathReachable(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg width="50" height="50">
			<defs>
				<clipPath id="cp1">
					<rect x="0" y="0" width="10" height="10"/>
				</clipPath>
			</defs>
			<rect width="10" height="10" clip-path="url(#cp1)"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if paths, ok := vg.ClipPaths["cp1"]; !ok || len(paths) == 0 {
		t.Fatalf("cp1 not reachable from nested-svg defs (paths=%v)", paths)
	}
}

// Step 6: <linearGradient> inside nested-svg defs reachable.
func TestNestedSvg_DefsGradientReachable(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg width="50" height="50">
			<defs>
				<linearGradient id="g1">
					<stop offset="0" stop-color="red"/>
					<stop offset="1" stop-color="blue"/>
				</linearGradient>
			</defs>
			<rect width="10" height="10" fill="url(#g1)"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := vg.Gradients["g1"]; !ok {
		t.Fatal("g1 not reachable from nested-svg defs")
	}
}

// Step 6: <filter> inside nested-svg defs reachable.
func TestNestedSvg_DefsFilterReachable(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg width="50" height="50">
			<defs>
				<filter id="f1">
					<feGaussianBlur stdDeviation="2"/>
				</filter>
			</defs>
			<rect width="10" height="10" filter="url(#f1)"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := vg.Filters["f1"]; !ok {
		t.Fatal("f1 not reachable from nested-svg defs")
	}
}

// Negative percent on width/height: SVG spec says length must be
// non-negative. resolveViewportLength yields a negative pre-clamp;
// clampViewBoxDim then floors to 0, giving a degenerate inner
// viewport. Inner viewBox path must be skipped (vbW > 0 guards it),
// falling through to translate-only (1,0,0,1,x,y) with a 0-dim rect.
// Document the actual contract via assertion.
func TestResolveViewportLength_NegativePercent(t *testing.T) {
	got := resolveViewportLength("-50%", 100, 0)
	want := float32(-50)
	if !nearEq32(got, want) {
		t.Fatalf("resolveViewportLength(-50%%,100,0)=%v want %v", got, want)
	}
	// End-to-end: negative width zeroes the viewport. Inner has a
	// viewBox; nums[2]>0 still passes (10) but w=0 → sx=0. Inner
	// content collapses to a point at (0,0).
	svg := `<svg viewBox="0 0 100 100">
		<svg width="-50%" height="50" viewBox="0 0 10 10"
			preserveAspectRatio="none">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	if !nearEq32(p.Transform[0], 0) {
		t.Fatalf("expected sx=0 from clamped negative width; got %v",
			p.Transform[0])
	}
}

// Degenerate inner viewport (zero or negative dim post-clamp) must
// skip synth clip emission so ClipPaths can't be flooded with empty
// entries under adversarial inputs.
func TestNestedSvg_DegenerateViewportNoClip(t *testing.T) {
	cases := []string{
		`<svg viewBox="0 0 100 100">
			<svg x="0" y="0" width="0" height="50">
				<rect width="10" height="10"/>
			</svg>
		</svg>`,
		`<svg viewBox="0 0 100 100">
			<svg x="0" y="0" width="50" height="0">
				<rect width="10" height="10"/>
			</svg>
		</svg>`,
		`<svg viewBox="0 0 100 100">
			<svg width="-50%" height="50">
				<rect width="10" height="10"/>
			</svg>
		</svg>`,
	}
	for i, svg := range cases {
		vg, err := parseSvg(svg)
		if err != nil {
			t.Fatalf("case %d parse: %v", i, err)
		}
		for k := range vg.ClipPaths {
			if strings.HasPrefix(k, "__nested_svg_clip_") {
				t.Fatalf("case %d: unexpected synth clip %q for "+
					"degenerate viewport", i, k)
			}
		}
	}
}

// KNOWN GAP — to fix when the clip-mask renderer lands.
// Spec: an inner <svg> with an author clip-path AND default
// overflow:hidden must clip by the INTERSECTION of the author clip
// and the viewport rect. The renderer applies one mask per shape (no
// two-pass clip), so true intersection is unimplemented. The parser
// preserves the authored clip — losing overflow clipping rather than
// the asset's explicit semantic. When the renderer learns multi-clip
// composition, the data model must carry both (e.g. []string with
// stencil-AND); that's when this test should flip.
func TestNestedSvg_AuthorClipPathOnSvgPreserved(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="author">
				<rect x="0" y="0" width="5" height="5"/>
			</clipPath>
		</defs>
		<svg x="10" y="10" width="20" height="20" clip-path="url(#author)">
			<rect width="10" height="10"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	if p.ClipPathID != "author" {
		t.Fatalf("ClipPathID=%q; want author (authored clip wins, viewport clip skipped)",
			p.ClipPathID)
	}
	// Synth clip must NOT have been allocated for this <svg>.
	if _, ok := vg.ClipPaths["__nested_svg_clip_1"]; ok {
		t.Fatal("synth viewport clip allocated despite authored clip")
	}
}

// Adversarial: outer rect returned by computeNestedSvgViewport must
// be finite and non-negative under NaN/Inf attrs and poisoned parent.
// segmentsForRect downstream would propagate non-finite coords into
// tessellation otherwise.
func TestNestedSvg_HardenOuterRectFinite(t *testing.T) {
	cases := []struct {
		attrs  map[string]string
		parent viewportRect
	}{
		{map[string]string{"x": "NaN", "y": "Inf",
			"width": "50", "height": "50"},
			viewportRect{W: 100, H: 100}},
		{map[string]string{"width": "-Inf", "height": "NaN",
			"viewBox": "0 0 10 10"}, viewportRect{W: 100, H: 100}},
		{map[string]string{"x": "1e30%", "y": "1e30%",
			"width": "100", "height": "100"},
			viewportRect{W: 100, H: 100}},
		{map[string]string{"width": "50%", "height": "50%"},
			viewportRect{
				X: float32(math.NaN()), Y: float32(math.Inf(1)),
				W: float32(math.NaN()), H: float32(math.Inf(-1)),
			}},
	}
	for i, tc := range cases {
		_, outer, _ := computeNestedSvgViewport(tc.attrs, tc.parent)
		fields := [4]float32{outer.X, outer.Y, outer.W, outer.H}
		for j, v := range fields {
			f := float64(v)
			if math.IsNaN(f) || math.IsInf(f, 0) {
				t.Fatalf("case %d: outer[%d]=%v non-finite", i, j, v)
			}
		}
		if outer.W < 0 || outer.H < 0 {
			t.Fatalf("case %d: outer W,H must be non-negative; got %v,%v",
				i, outer.W, outer.H)
		}
	}
}

// Direct unit test for the outer rect return. Locks the contract so
// the synth-clip rect can't drift from the transform translation.
func TestComputeNestedSvgViewport_OuterRect(t *testing.T) {
	parent := viewportRect{X: 0, Y: 0, W: 200, H: 100}
	cases := []struct {
		name  string
		attrs map[string]string
		want  viewportRect
	}{
		{"defaults", map[string]string{},
			viewportRect{X: 0, Y: 0, W: 200, H: 100}},
		{"explicit_xywh", map[string]string{
			"x": "10", "y": "20", "width": "50", "height": "30"},
			viewportRect{X: 10, Y: 20, W: 50, H: 30}},
		{"percent_wh", map[string]string{
			"width": "50%", "height": "50%"},
			viewportRect{X: 0, Y: 0, W: 100, H: 50}},
		{"percent_xy", map[string]string{
			"x": "10%", "y": "20%", "width": "50", "height": "30"},
			viewportRect{X: 20, Y: 20, W: 50, H: 30}},
		{"viewbox_does_not_affect_outer", map[string]string{
			"x": "5", "y": "5", "width": "40", "height": "40",
			"viewBox": "0 0 10 10"},
			viewportRect{X: 5, Y: 5, W: 40, H: 40}},
	}
	for _, tc := range cases {
		_, outer, _ := computeNestedSvgViewport(tc.attrs, parent)
		if !nearEq32(outer.X, tc.want.X) || !nearEq32(outer.Y, tc.want.Y) ||
			!nearEq32(outer.W, tc.want.W) || !nearEq32(outer.H, tc.want.H) {
			t.Fatalf("%s: outer=%+v want %+v", tc.name, outer, tc.want)
		}
	}
}

// Synth clip rect coordinates must equal the authored viewport rect
// in outer coords. Walks the path segments emitted by segmentsForRect
// (5 commands: MoveTo, 3×LineTo, Close) and checks the four corner
// points.
func TestNestedSvg_ClipRectMatchesViewport(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10" y="20" width="30" height="40">
			<rect width="5" height="5"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	clip, ok := vg.ClipPaths["__nested_svg_clip_1"]
	if !ok || len(clip) != 1 {
		t.Fatalf("clip missing or wrong count: %v", clip)
	}
	segs := clip[0].Segments
	if len(segs) != 5 {
		t.Fatalf("segs len=%d want 5; full=%v", len(segs), segs)
	}
	wantPts := [4][2]float32{
		{10, 20}, {40, 20}, {40, 60}, {10, 60},
	}
	for i, w := range wantPts {
		got := segs[i].Points
		if len(got) < 2 ||
			!nearEq32(got[0], w[0]) || !nearEq32(got[1], w[1]) {
			t.Fatalf("seg[%d]=%v want corner %v", i, got, w)
		}
	}
}

// Cascade: a descendant inside a nested <svg> with its own
// clip-path=url(#…) overrides the inherited synth viewport clip on
// itself (spec cascade behavior; v1 does not intersect them).
func TestNestedSvg_DescendantAuthorClipOverridesSynth(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="author">
				<rect x="0" y="0" width="3" height="3"/>
			</clipPath>
		</defs>
		<svg x="10" y="10" width="50" height="50">
			<rect width="10" height="10" clip-path="url(#author)"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	if p.ClipPathID != "author" {
		t.Fatalf("ClipPathID=%q; want \"author\" (descendant clip overrides synth)",
			p.ClipPathID)
	}
}

// Percent x/y on inner <svg> resolves against parent W,H (not the
// inner viewport). Parent 100x100, x=10% → tx=10, y=20% → ty=20.
func TestNestedSvg_PercentXY(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<svg x="10%" y="20%" width="50" height="50">
			<rect x="0" y="0" width="5" height="5"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	assertTransform(t, p.Transform, [6]float32{1, 0, 0, 1, 10, 20})
}

// Adversarial inner viewBox + grandchild percent dim: even with
// poisoned viewBox, grandchild geometry must be finite. Pins that
// state.curViewport handed to deeper recursion is sane.
func TestNestedSvg_HardenInnerViewportSane(t *testing.T) {
	cases := []string{
		`<svg viewBox="0 0 100 100">
			<svg width="50" height="50" viewBox="NaN NaN 10 10">
				<rect width="50%" height="50%"/>
			</svg>
		</svg>`,
		`<svg viewBox="0 0 100 100">
			<svg width="50" height="50" viewBox="0 0 Inf Inf">
				<rect width="50%" height="50%"/>
			</svg>
		</svg>`,
		`<svg viewBox="0 0 100 100">
			<svg width="50" height="50" viewBox="0 0 0 0">
				<rect width="50%" height="50%"/>
			</svg>
		</svg>`,
	}
	for i, svg := range cases {
		vg, err := parseSvg(svg)
		if err != nil {
			t.Fatalf("case %d parse: %v", i, err)
		}
		for _, p := range vg.Paths {
			for j, v := range p.Transform {
				f := float64(v)
				if math.IsNaN(f) || math.IsInf(f, 0) {
					t.Fatalf("case %d: path Transform[%d]=%v non-finite",
						i, j, v)
				}
			}
		}
	}
}

// Inner <svg> redeclaring the SAME clip-path id its parent already
// carries must still suppress the synth viewport clip. Pure value
// comparison would miss this (parent.ClipPathID == child.ClipPathID);
// suppression keys off AuthoredClipPath instead.
func TestNestedSvg_RedeclaredSameClipPathSuppressesSynthClip(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100" clip-path="url(#a)">
		<defs>
			<clipPath id="a"><rect width="50" height="50"/></clipPath>
		</defs>
		<svg viewBox="0 0 100 100" width="100" height="100"
			clip-path="url(#a)">
			<rect width="100" height="100"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	if p.ClipPathID != "a" {
		t.Fatalf("ClipPathID=%q; redeclared author clip should win → want a",
			p.ClipPathID)
	}
	if _, ok := vg.ClipPaths["__nested_svg_clip_1"]; ok {
		t.Fatal("synth viewport clip allocated despite redeclared authored clip")
	}
}

// Inner <svg> author clip declared via inline style="" must also
// suppress the synth viewport clip (cascade origin parity with the
// bare attr / CSS rule cases).
func TestNestedSvg_InnerSvgInlineStyleClipPathSuppressesSynthClip(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<defs>
			<clipPath id="author"><rect width="50" height="50"/></clipPath>
		</defs>
		<svg viewBox="0 0 100 100" width="100" height="100"
			style="clip-path:url(#author)">
			<rect width="100" height="100"/>
		</svg>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p := firstShapePath(t, vg)
	if p.ClipPathID != "author" {
		t.Fatalf("ClipPathID=%q; inline-style author clip should win → want author",
			p.ClipPathID)
	}
	if _, ok := vg.ClipPaths["__nested_svg_clip_1"]; ok {
		t.Fatal("synth viewport clip allocated despite inline-style author clip")
	}
}
