package svg

import (
	"os"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// --- Small pure functions ---

func TestXmlParseFontWeightBold(t *testing.T) {
	if parseFontWeight("bold") != 700 {
		t.Fatalf("'bold' should be 700")
	}
}

func TestXmlParseFontWeightNormal(t *testing.T) {
	if parseFontWeight("normal") != 400 {
		t.Fatalf("'normal' should be 400")
	}
}

func TestXmlParseFontWeightNumeric(t *testing.T) {
	if parseFontWeight("600") != 600 {
		t.Fatalf("'600' should be 600")
	}
}

func TestXmlParseFontWeightEmpty(t *testing.T) {
	if parseFontWeight("") != 0 {
		t.Fatalf("empty should be 0")
	}
}

func TestXmlParseFontWeightBolder(t *testing.T) {
	if parseFontWeight("bolder") != 700 {
		t.Fatalf("'bolder' should be 700")
	}
}

func TestXmlParseFontWeightLighter(t *testing.T) {
	if parseFontWeight("lighter") != 400 {
		t.Fatalf("'lighter' should be 400")
	}
}

func TestXmlParseFontWeightOutOfRange(t *testing.T) {
	if parseFontWeight("50") != 0 {
		t.Fatalf("'50' out of range should be 0")
	}
}

func TestXmlCleanFontFamilyMultiple(t *testing.T) {
	r := cleanFontFamily("Courier New, monospace")
	if r != "Courier New" {
		t.Fatalf("expected 'Courier New', got %q", r)
	}
}

func TestXmlCleanFontFamilySingle(t *testing.T) {
	r := cleanFontFamily("Arial")
	if r != "Arial" {
		t.Fatalf("expected 'Arial', got %q", r)
	}
}

// --- Gradient parsing ---

func TestXmlParseGradientCoordOBBPercent(t *testing.T) {
	v := parseGradientCoord("50%", true)
	if f32Abs(v-0.5) > 1e-5 {
		t.Fatalf("expected 0.5, got %f", v)
	}
}

func TestXmlParseGradientCoordOBBBare(t *testing.T) {
	v := parseGradientCoord("0.75", true)
	if f32Abs(v-0.75) > 1e-5 {
		t.Fatalf("expected 0.75, got %f", v)
	}
}

func TestXmlParseGradientCoordUserSpace(t *testing.T) {
	v := parseGradientCoord("100", false)
	if f32Abs(v-100) > 1e-5 {
		t.Fatalf("expected 100, got %f", v)
	}
}

func TestXmlParseGradientStops(t *testing.T) {
	root, err := decodeSvgTree(`<linearGradient>` +
		`<stop offset="0" stop-color="red"/>` +
		`<stop offset="1" stop-color="blue"/>` +
		`</linearGradient>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	stops := parseGradientStops(root)
	if len(stops) != 2 {
		t.Fatalf("expected 2 stops, got %d", len(stops))
	}
	if stops[0].Offset != 0 || stops[1].Offset != 1 {
		t.Fatalf("expected offsets 0 and 1, got %f and %f",
			stops[0].Offset, stops[1].Offset)
	}
	// Red
	if stops[0].Color.R != 255 || stops[0].Color.G != 0 {
		t.Fatalf("first stop should be red, got %+v", stops[0].Color)
	}
}

// --- parseSvgDimensions ---

func TestXmlParseSvgDimensionsViewBox(t *testing.T) {
	svg := `<svg viewBox="0 0 100 50">`
	w, h := parseSvgDimensions(svg)
	if w != 100 || h != 50 {
		t.Fatalf("expected (100,50), got (%f,%f)", w, h)
	}
}

func TestXmlParseSvgDimensionsWidthHeight(t *testing.T) {
	svg := `<svg width="200" height="100">`
	w, h := parseSvgDimensions(svg)
	if w != 200 || h != 100 {
		t.Fatalf("expected (200,100), got (%f,%f)", w, h)
	}
}

func TestXmlParseSvgDimensionsIgnoresChildWidthHeight(t *testing.T) {
	svg := `<svg><rect width="200" height="100"/></svg>`
	w, h := parseSvgDimensions(svg)
	if w != defaultIconSize || h != defaultIconSize {
		t.Fatalf("expected defaults (%d,%d), got (%f,%f)",
			defaultIconSize, defaultIconSize, w, h)
	}
}

// --- Integration ---

func TestXmlParseSvgMinimalRect(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100"><rect x="0" y="0" width="50" height="50" fill="red"/></svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vg.Paths) < 1 {
		t.Fatalf("expected at least 1 path, got %d", len(vg.Paths))
	}
}

func TestXmlParseSvgWithGradient(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100">
		<defs>
			<linearGradient id="g1" x1="0" y1="0" x2="1" y2="0">
				<stop offset="0" stop-color="red"/>
				<stop offset="1" stop-color="blue"/>
			</linearGradient>
		</defs>
		<rect x="0" y="0" width="100" height="100" fill="url(#g1)"/>
	</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vg.Gradients) == 0 {
		t.Fatalf("expected gradients, got none")
	}
	if _, ok := vg.Gradients["g1"]; !ok {
		t.Fatalf("expected gradient 'g1' in map")
	}
}

func TestXmlParseSvgViewBoxOffset(t *testing.T) {
	svg := `<svg viewBox="10 20 100 100"><rect x="10" y="20" width="50" height="50" fill="red"/></svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vg.ViewBoxX != 10 || vg.ViewBoxY != 20 {
		t.Fatalf("expected viewBox offset (10,20), got (%f,%f)",
			vg.ViewBoxX, vg.ViewBoxY)
	}
}

func TestXmlParseSvgEmpty(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100"></svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vg.Paths) != 0 {
		t.Fatalf("expected 0 paths, got %d", len(vg.Paths))
	}
}

func TestXmlParseSvgFileTooLarge(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/huge.svg"
	// Create file just over the 4MB limit.
	data := make([]byte, maxSvgFileSize+1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := parseSvgFile(path)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
}

func TestParseSvgRejectsMalformedXML(t *testing.T) {
	cases := []string{
		`<svg><g></svg>`,
		`<svg><rect></svg>`,
		`<svg><rect/></svg><`,
	}
	for _, svg := range cases {
		if _, err := parseSvg(svg); err == nil {
			t.Fatalf("expected malformed SVG to fail: %q", svg)
		}
	}
}

// suppress unused import
var _ = gui.SvgColor{}

// --- scanOpacityAnimTargets ---

// scanOpacityAnimNode builds an xmlNode wrapping the given animation
// children, for scanOpacityAnimTargets testing.
func scanOpacityAnimNode(t *testing.T, children string) *xmlNode {
	t.Helper()
	root, err := decodeSvgTree(`<shape>` + children + `</shape>`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return root
}

func TestScanOpacityAnimTargets_DetectsEachTarget(t *testing.T) {
	n := scanOpacityAnimNode(t,
		`<animate attributeName="opacity" values="1;0" dur="1s"/>`+
			`<animate attributeName="fill-opacity" values="1;0" dur="1s"/>`+
			`<animate attributeName="stroke-opacity" values="1;0" dur="1s"/>`)
	all, fill, stroke := scanOpacityAnimTargets(n)
	if !all {
		t.Fatal("expected opacity → all=true")
	}
	if !fill {
		t.Fatal("expected fill-opacity → fill=true")
	}
	if !stroke {
		t.Fatal("expected stroke-opacity → stroke=true")
	}
}

func TestScanOpacityAnimTargets_IgnoresAnimateTransform(t *testing.T) {
	// <animateTransform> never targets opacity; must be skipped so
	// it cannot trip the bake-suppression heuristics.
	n := scanOpacityAnimNode(t,
		`<animateTransform type="rotate" attributeName="transform" `+
			`values="0 5 5;360 5 5" dur="1s"/>`)
	all, fill, stroke := scanOpacityAnimTargets(n)
	if all || fill || stroke {
		t.Fatalf("animateTransform must not set any flag; "+
			"got all=%v fill=%v stroke=%v", all, fill, stroke)
	}
}

func TestScanOpacityAnimTargets_EmptyBody(t *testing.T) {
	n := scanOpacityAnimNode(t, "")
	all, fill, stroke := scanOpacityAnimTargets(n)
	if all || fill || stroke {
		t.Fatal("empty body must report no targets")
	}
}

// --- Root-level presentation attribute inheritance ---

// `fill="currentColor"` on the root <svg> element must propagate
// to child shapes so render-time tint can replace the placeholder.
func TestParseSvg_RootFillInheritedByShapes(t *testing.T) {
	svg := `<svg viewBox="0 0 10 10" fill="currentColor" ` +
		`xmlns="http://www.w3.org/2000/svg">` +
		`<rect x="0" y="0" width="10" height="10"/>` +
		`</svg>`
	vg, err := parseSvg(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) == 0 {
		t.Fatal("expected a path from the rect")
	}
	// The rect must inherit the currentColor sentinel (magenta RGB)
	// from the root; bakePathOpacity then promotes A to 255.
	// Sentinel RGB survives so render-side tint can substitute.
	if vg.Paths[0].FillColor.R != 255 || vg.Paths[0].FillColor.B != 255 {
		t.Fatalf("expected sentinel magenta RGB on fill, got %+v",
			vg.Paths[0].FillColor)
	}
	if vg.Paths[0].FillColor.A == 0 {
		t.Fatal("fill alpha collapsed to 0 — sentinel bump failed")
	}
}

// parseAnimateForDispatch routes by attributeName: opacity → opacity
// parser, stroke-dasharray → dasharray parser, stroke-dashoffset →
// dashoffset parser, anything else → primitive-attribute parser.
func TestParseAnimateForDispatch_OpacityKind(t *testing.T) {
	elem := `<animate attributeName="opacity" dur="1s" values="0;1"/>`
	a, ok := parseAnimateForDispatch(elem, ComputedStyle{GroupID: "g"})
	if !ok || a.Kind != gui.SvgAnimOpacity {
		t.Fatalf("kind=%v ok=%v", a.Kind, ok)
	}
}

func TestParseAnimateForDispatch_DashArrayKind(t *testing.T) {
	elem := `<animate attributeName="stroke-dasharray" dur="1s" ` +
		`values="0 5;5 5"/>`
	a, ok := parseAnimateForDispatch(elem, ComputedStyle{GroupID: "g"})
	if !ok || a.Kind != gui.SvgAnimDashArray {
		t.Fatalf("kind=%v ok=%v", a.Kind, ok)
	}
}

func TestParseAnimateForDispatch_DashOffsetKind(t *testing.T) {
	elem := `<animate attributeName="stroke-dashoffset" dur="1s" ` +
		`values="0;-10"/>`
	a, ok := parseAnimateForDispatch(elem, ComputedStyle{GroupID: "g"})
	if !ok || a.Kind != gui.SvgAnimDashOffset {
		t.Fatalf("kind=%v ok=%v", a.Kind, ok)
	}
}

func TestParseAnimateForDispatch_PrimitiveAttrFallback(t *testing.T) {
	elem := `<animate attributeName="r" dur="1s" values="0;5"/>`
	a, ok := parseAnimateForDispatch(elem, ComputedStyle{GroupID: "g"})
	if !ok || a.Kind != gui.SvgAnimAttr {
		t.Fatalf("kind=%v ok=%v", a.Kind, ok)
	}
}

func TestParseAnimateForDispatch_MissingAttrNameRejects(t *testing.T) {
	elem := `<animate dur="1s" values="0;1"/>`
	if _, ok := parseAnimateForDispatch(elem,
		ComputedStyle{GroupID: "g"}); ok {
		t.Fatal("missing attributeName must reject")
	}
}
