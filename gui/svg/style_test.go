package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// --- Color parsing ---

func TestStyleParseSvgColorNamed(t *testing.T) {
	c, ok := parseSvgColor("red")
	if !ok {
		t.Fatal("expected ok for 'red'")
	}
	if c.R != 255 || c.G != 0 || c.B != 0 || c.A != 255 {
		t.Fatalf("expected red, got %+v", c)
	}
}

func TestStyleParseSvgColorNamedCaseInsensitive(t *testing.T) {
	c, ok := parseSvgColor("RED")
	if !ok {
		t.Fatal("expected ok for 'RED'")
	}
	if c.R != 255 || c.G != 0 || c.B != 0 || c.A != 255 {
		t.Fatalf("expected red for 'RED', got %+v", c)
	}
}

func TestStyleParseSvgColorNone(t *testing.T) {
	c, ok := parseSvgColor("none")
	if !ok || c != colorTransparent {
		t.Fatalf("expected transparent, got %+v ok=%v", c, ok)
	}
}

func TestStyleParseSvgColorInherit(t *testing.T) {
	c, ok := parseSvgColor("inherit")
	if !ok || c != colorCurrent {
		t.Fatalf("expected currentColor sentinel, got %+v ok=%v", c, ok)
	}
}

func TestStyleParseSvgColorCurrentColor(t *testing.T) {
	c, ok := parseSvgColor("currentColor")
	if !ok || c != colorCurrent {
		t.Fatalf("expected currentColor sentinel, got %+v ok=%v", c, ok)
	}
}

func TestStyleParseSvgColorEmpty(t *testing.T) {
	c, ok := parseSvgColor("")
	if ok {
		t.Fatal("empty input must report not-parsed")
	}
	if c != colorInherit {
		t.Fatalf("expected inherit sentinel for empty, got %+v", c)
	}
}

func TestStyleParseSvgColorURL(t *testing.T) {
	c, ok := parseSvgColor("url(#grad1)")
	if !ok || c != colorTransparent {
		t.Fatalf("expected transparent for url(), got %+v ok=%v", c, ok)
	}
}

func TestStyleParseSvgColorUnknown(t *testing.T) {
	c, ok := parseSvgColor("notacolor")
	if ok {
		t.Fatal("unknown keyword must report not-parsed")
	}
	zero := gui.SvgColor{}
	if c != zero {
		t.Fatalf("expected zero color, got %+v", c)
	}
}

func TestStyleParseSvgColorUnknownFunc(t *testing.T) {
	// hsl() is a real CSS color syntax we don't yet support. Must
	// report not-parsed so the cascade keeps inherited fill instead
	// of clobbering it with transparent black.
	_, ok := parseSvgColor("hsl(0, 100%, 50%)")
	if ok {
		t.Fatal("hsl() should report not-parsed until supported")
	}
}

func TestStyleParseHexColorRGB(t *testing.T) {
	c, ok := parseHexColor("#f80")
	if !ok {
		t.Fatal("expected ok for #f80")
	}
	if c.R != 0xff || c.G != 0x88 || c.B != 0x00 || c.A != 255 {
		t.Fatalf("expected #ff8800, got %+v", c)
	}
}

func TestStyleParseHexColorRGBA(t *testing.T) {
	c, _ := parseHexColor("#f80a")
	if c.R != 0xff || c.G != 0x88 || c.B != 0x00 || c.A != 0xaa {
		t.Fatalf("expected #ff8800aa, got %+v", c)
	}
}

func TestStyleParseHexColorRRGGBB(t *testing.T) {
	c, _ := parseHexColor("#1a2b3c")
	if c.R != 0x1a || c.G != 0x2b || c.B != 0x3c || c.A != 255 {
		t.Fatalf("expected #1a2b3c, got %+v", c)
	}
}

func TestStyleParseHexColorRRGGBBAA(t *testing.T) {
	c, _ := parseHexColor("#1a2b3c80")
	if c.R != 0x1a || c.G != 0x2b || c.B != 0x3c || c.A != 0x80 {
		t.Fatalf("expected #1a2b3c80, got %+v", c)
	}
}

func TestStyleParseHexColorInvalidLength(t *testing.T) {
	c, ok := parseHexColor("#12")
	if ok {
		t.Fatal("invalid hex length must report not-parsed")
	}
	if c != colorBlack {
		t.Fatalf("expected black for invalid hex, got %+v", c)
	}
}

func TestStyleParseHexColorNonHexDigitsRejected(t *testing.T) {
	// hexDigit silently maps non-hex bytes to 0, which would let
	// "#GGGGGG" / "#zzz" parse as black. parseHexColor must reject
	// so the cascade ignores the declaration.
	cases := []string{"#GGGGGG", "#zzz", "#12X4", "#  ff00"}
	for _, in := range cases {
		if _, ok := parseHexColor(in); ok {
			t.Errorf("%q must be rejected (non-hex digit)", in)
		}
	}
}

func TestStyleParseRGBColorNonNumericRejected(t *testing.T) {
	// Atoi swallows errors as 0, which previously let
	// "rgb(abc,def,ghi)" parse silently as rgb(0,0,0).
	cases := []string{
		"rgb(abc,def,ghi)",
		"rgb(100%,0,0)", // percent channels not yet supported
		"rgb(,,)",
	}
	for _, in := range cases {
		if _, ok := parseRGBColor(in); ok {
			t.Errorf("%q must be rejected", in)
		}
	}
}

func TestStyleParseRGBColorNaNAlphaRejected(t *testing.T) {
	if _, ok := parseRGBColor("rgba(0,0,0,NaN)"); ok {
		t.Fatal("NaN alpha must be rejected, not silently clamped")
	}
}

func TestStyleParseSvgColorWhitespaceOnlyEmpty(t *testing.T) {
	c, ok := parseSvgColor("   \t\n")
	if ok {
		t.Fatal("whitespace-only must report not-parsed")
	}
	if c != colorInherit {
		t.Fatalf("expected inherit sentinel, got %+v", c)
	}
}

func TestStyleParseSvgColorURLCaseInsensitive(t *testing.T) {
	// hasASCIIPrefixFold should match URL(...) case-insensitively.
	c, ok := parseSvgColor("URL(#grad1)")
	if !ok || c != colorTransparent {
		t.Errorf("URL(#grad1) should parse as transparent, got %+v ok=%v",
			c, ok)
	}
}

func TestStyleParseIntStrict_EmptyAndGarbage(t *testing.T) {
	cases := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"", 0, false},
		{"   ", 0, false},
		{"42", 42, true},
		{"  42  ", 42, true},
		{"42px", 0, false},
		{"abc", 0, false},
		{"-7", -7, true},
		{"+12", 12, true},
	}
	for _, tc := range cases {
		got, ok := parseIntStrict(tc.in)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("parseIntStrict(%q) = (%d,%v), want (%d,%v)",
				tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestStyleParseFloatStrict_NaNInfReject(t *testing.T) {
	rejected := []string{"", "   ", "NaN", "+Inf", "-Inf", "abc",
		"1.0e500"} // 1e500 overflows float64 → ParseFloat err
	for _, in := range rejected {
		if _, ok := parseFloatStrict(in); ok {
			t.Errorf("parseFloatStrict(%q) must reject", in)
		}
	}
	good := []struct {
		in   string
		want float32
	}{
		{"0.5", 0.5},
		{"  3.14  ", 3.14},
		{"-2", -2},
	}
	for _, tc := range good {
		got, ok := parseFloatStrict(tc.in)
		if !ok {
			t.Errorf("parseFloatStrict(%q) must accept", tc.in)
			continue
		}
		if got < tc.want-1e-5 || got > tc.want+1e-5 {
			t.Errorf("parseFloatStrict(%q) = %f, want %f",
				tc.in, got, tc.want)
		}
	}
}

func TestHasASCIIPrefixFold_CaseAndLength(t *testing.T) {
	cases := []struct {
		s, prefix string
		want      bool
	}{
		{"rgb(1,2,3)", "rgb", true},
		{"RGB(1,2,3)", "rgb", true},
		{"Rgba(1,2,3,0.5)", "rgb", true},
		{"url(#x)", "url(", true},
		{"URL(#x)", "url(", true},
		{"rg", "rgb", false}, // shorter than prefix
		{"", "rgb", false},
		{"red", "rgb", false},
		{"  rgb(", "rgb", false}, // leading space — caller trims
	}
	for _, tc := range cases {
		got := hasASCIIPrefixFold(tc.s, tc.prefix)
		if got != tc.want {
			t.Errorf("hasASCIIPrefixFold(%q,%q) = %v, want %v",
				tc.s, tc.prefix, got, tc.want)
		}
	}
}

func TestStyleIsHexDigit_Boundaries(t *testing.T) {
	accept := "0123456789abcdefABCDEF"
	for i := 0; i < len(accept); i++ {
		if !isHexDigit(accept[i]) {
			t.Errorf("%q must be hex digit", accept[i])
		}
	}
	reject := "gGzZ /\x00\xff:" + "@`"
	for i := 0; i < len(reject); i++ {
		if isHexDigit(reject[i]) {
			t.Errorf("%q must NOT be hex digit", reject[i])
		}
	}
}

func TestStyleHexDigit(t *testing.T) {
	if hexDigit('0') != 0 {
		t.Fatalf("'0' should be 0")
	}
	if hexDigit('9') != 9 {
		t.Fatalf("'9' should be 9")
	}
	if hexDigit('a') != 10 {
		t.Fatalf("'a' should be 10")
	}
	if hexDigit('F') != 15 {
		t.Fatalf("'F' should be 15")
	}
	if hexDigit('g') != 0 {
		t.Fatalf("'g' should be 0")
	}
}

func TestStyleParseRGBColor(t *testing.T) {
	c, ok := parseRGBColor("rgb(128,64,32)")
	if !ok {
		t.Fatal("expected ok for rgb(...)")
	}
	if c.R != 128 || c.G != 64 || c.B != 32 || c.A != 255 {
		t.Fatalf("expected rgb(128,64,32), got %+v", c)
	}
}

func TestStyleParseRGBAColor(t *testing.T) {
	c, _ := parseRGBColor("rgba(100,200,50,0.5)")
	if c.R != 100 || c.G != 200 || c.B != 50 {
		t.Fatalf("expected rgb(100,200,50), got %+v", c)
	}
	if c.A != 127 {
		t.Fatalf("expected alpha ~127, got %d", c.A)
	}
}

func TestStyleParseRGBColorClamping(t *testing.T) {
	c, _ := parseRGBColor("rgb(300,-10,128)")
	if c.R != 255 || c.G != 0 || c.B != 128 {
		t.Fatalf("expected clamped (255,0,128), got %+v", c)
	}
}

func TestStyleClampByte(t *testing.T) {
	if clampByte(-1) != 0 {
		t.Fatalf("-1 should clamp to 0")
	}
	if clampByte(128) != 128 {
		t.Fatalf("128 should stay 128")
	}
	if clampByte(256) != 255 {
		t.Fatalf("256 should clamp to 255")
	}
}

// --- Attribute extraction ---

func TestStyleFindAttrPresent(t *testing.T) {
	val, ok := findAttr(`<rect width="100" height="50">`, "width")
	if !ok || val != "100" {
		t.Fatalf("expected '100', got %q ok=%v", val, ok)
	}
}

func TestStyleFindAttrMissing(t *testing.T) {
	_, ok := findAttr(`<rect width="100">`, "height")
	if ok {
		t.Fatalf("expected not found")
	}
}

func TestStyleFindAttrSingleQuotes(t *testing.T) {
	val, ok := findAttr(`<rect width='200'>`, "width")
	if !ok || val != "200" {
		t.Fatalf("expected '200', got %q ok=%v", val, ok)
	}
}

func TestStyleFindAttrNoPartialMatch(t *testing.T) {
	// "width" must not match inside "stroke-width"
	elem := `<path stroke-width="3">`
	_, ok := findAttr(elem, "width")
	if ok {
		t.Fatalf("should not match 'width' inside 'stroke-width'")
	}
}

func TestStyleFindStylePropertyPresent(t *testing.T) {
	val, ok := findStyleProperty("fill:red;stroke:blue", "fill")
	if !ok || val != "red" {
		t.Fatalf("expected 'red', got %q ok=%v", val, ok)
	}
}

func TestStyleFindStylePropertyMissing(t *testing.T) {
	_, ok := findStyleProperty("fill:red", "stroke")
	if ok {
		t.Fatalf("expected not found")
	}
}

func TestStyleFindAttrOrStyleStyleWins(t *testing.T) {
	elem := `<rect fill="blue" style="fill:green">`
	val, ok := findAttrOrStyle(elem, "fill")
	if !ok || val != "green" {
		t.Fatalf("style should win, got %q", val)
	}
}

func TestStyleParseFillURLValid(t *testing.T) {
	id, ok := parseFillURL("url(#grad1)")
	if !ok || id != "grad1" {
		t.Fatalf("expected 'grad1', got %q ok=%v", id, ok)
	}
}

func TestStyleParseFillURLNoHash(t *testing.T) {
	_, ok := parseFillURL("url(grad1)")
	if ok {
		t.Fatalf("expected false for url without #")
	}
}

func TestStyleParseFillURLNotURL(t *testing.T) {
	_, ok := parseFillURL("red")
	if ok {
		t.Fatalf("expected false for non-url")
	}
}

// --- Transform parsing ---

func TestStyleMatrixMultiplyIdentity(t *testing.T) {
	id := identityTransform
	r := matrixMultiply(id, id)
	if r != id {
		t.Fatalf("identity * identity should be identity, got %v", r)
	}
}

func TestStyleMatrixMultiplyTranslate(t *testing.T) {
	id := identityTransform
	tr := [6]float32{1, 0, 0, 1, 10, 20}
	r := matrixMultiply(id, tr)
	if r != tr {
		t.Fatalf("identity * translate should be translate, got %v", r)
	}
}

func TestStyleApplyTransformPtIdentity(t *testing.T) {
	rx, ry := applyTransformPt(5, 7, identityTransform)
	if rx != 5 || ry != 7 {
		t.Fatalf("identity transform should not change point, got (%f,%f)", rx, ry)
	}
}

func TestStyleApplyTransformPtScale(t *testing.T) {
	m := [6]float32{2, 0, 0, 3, 0, 0}
	rx, ry := applyTransformPt(4, 5, m)
	if rx != 8 || ry != 15 {
		t.Fatalf("scale(2,3) on (4,5) should be (8,15), got (%f,%f)", rx, ry)
	}
}

func TestStyleParseTransformTranslate(t *testing.T) {
	m := parseTransform("translate(10,20)")
	if f32Abs(m[4]-10) > 1e-5 || f32Abs(m[5]-20) > 1e-5 {
		t.Fatalf("expected translate(10,20), got %v", m)
	}
}

func TestStyleParseTransformScale(t *testing.T) {
	m := parseTransform("scale(2,3)")
	if f32Abs(m[0]-2) > 1e-5 || f32Abs(m[3]-3) > 1e-5 {
		t.Fatalf("expected scale(2,3), got %v", m)
	}
}

func TestStyleParseTransformRotate90(t *testing.T) {
	m := parseTransform("rotate(90)")
	// cos(90°)=0, sin(90°)=1
	if f32Abs(m[0]) > 1e-5 || f32Abs(m[1]-1) > 1e-5 {
		t.Fatalf("expected rotate(90), got %v", m)
	}
}

func TestStyleParseTransformRotateWithCenter(t *testing.T) {
	m := parseTransform("rotate(90,50,50)")
	// Transform point (50,50) through rotation about itself → (50,50)
	rx, ry := applyTransformPt(50, 50, m)
	if f32Abs(rx-50) > 1e-3 || f32Abs(ry-50) > 1e-3 {
		t.Fatalf("center should be invariant, got (%f,%f)", rx, ry)
	}
}

func TestStyleParseTransformMatrix(t *testing.T) {
	m := parseTransform("matrix(1,0,0,1,5,10)")
	if m[4] != 5 || m[5] != 10 {
		t.Fatalf("expected matrix translate (5,10), got %v", m)
	}
}

func TestStyleParseTransformChained(t *testing.T) {
	m := parseTransform("translate(10,0) scale(2)")
	// translate then scale: point (0,0) → (10,0) → (20,0)?
	// No: compose left-to-right means result = I * translate * scale
	// So [2,0,0,2,10,0] → applying to (1,0): 2*1+10=12, 0
	rx, ry := applyTransformPt(1, 0, m)
	if f32Abs(rx-12) > 1e-4 || f32Abs(ry) > 1e-4 {
		t.Fatalf("chained transform on (1,0) should be (12,0), got (%f,%f)", rx, ry)
	}
}

func TestStyleParseTransformEmpty(t *testing.T) {
	m := parseTransform("")
	if m != identityTransform {
		t.Fatalf("empty transform should be identity, got %v", m)
	}
}

func TestStyleIsIdentityTransformTrue(t *testing.T) {
	if !isIdentityTransform(identityTransform) {
		t.Fatalf("identity should be identity")
	}
}

func TestStyleIsIdentityTransformFalse(t *testing.T) {
	m := [6]float32{2, 0, 0, 1, 0, 0}
	if isIdentityTransform(m) {
		t.Fatalf("scaled matrix should not be identity")
	}
}

// --- Misc helpers ---

func TestStyleParseLength(t *testing.T) {
	if f32Abs(parseLength("12.5px")-12.5) > 1e-5 {
		t.Fatalf("expected 12.5, got %f", parseLength("12.5px"))
	}
}

func TestStyleParseLengthEmpty(t *testing.T) {
	if parseLength("") != 0 {
		t.Fatalf("empty should be 0")
	}
}

func TestStyleClampViewBoxDimNegative(t *testing.T) {
	if clampViewBoxDim(-5) != 0 {
		t.Fatalf("negative should clamp to 0")
	}
}

func TestStyleClampViewBoxDimInRange(t *testing.T) {
	if clampViewBoxDim(500) != 500 {
		t.Fatalf("in-range should be unchanged")
	}
}

func TestStyleClampViewBoxDimOverMax(t *testing.T) {
	if clampViewBoxDim(20000) != maxViewBoxDim {
		t.Fatalf("over max should clamp to %f", maxViewBoxDim)
	}
}

func TestStyleApplyOpacityFull(t *testing.T) {
	c := gui.SvgColor{R: 100, G: 100, B: 100, A: 200}
	r := applyOpacity(c, 1.0)
	if r != c {
		t.Fatalf("opacity 1.0 should not change color")
	}
}

func TestStyleApplyOpacityHalf(t *testing.T) {
	c := gui.SvgColor{R: 100, G: 100, B: 100, A: 200}
	r := applyOpacity(c, 0.5)
	if r.A != 100 {
		t.Fatalf("expected alpha 100, got %d", r.A)
	}
}

// --- Stroke attribute extraction ---

func TestStyleGetStrokeLinecapRound(t *testing.T) {
	lineCap := getStrokeLinecap(`<path stroke-linecap="round">`)
	if lineCap != gui.SvgRoundCap {
		t.Fatalf("expected RoundCap, got %d", lineCap)
	}
}

func TestStyleGetStrokeLinejoinBevel(t *testing.T) {
	join := getStrokeLinejoin(`<path stroke-linejoin="bevel">`)
	if join != gui.SvgBevelJoin {
		t.Fatalf("expected BevelJoin, got %d", join)
	}
}

func TestStyleGetStrokeWidth(t *testing.T) {
	w := getStrokeWidth(`<path stroke-width="3">`)
	if f32Abs(w-3) > 1e-5 {
		t.Fatalf("expected 3, got %f", w)
	}
}

func TestStyleGetStrokeDasharrayValid(t *testing.T) {
	da := getStrokeDasharray(`<path stroke-dasharray="5 3">`)
	if len(da) != 2 || da[0] != 5 || da[1] != 3 {
		t.Fatalf("expected [5,3], got %v", da)
	}
}

func TestStyleGetStrokeDasharrayNone(t *testing.T) {
	da := getStrokeDasharray(`<path stroke-dasharray="none">`)
	if da != nil {
		t.Fatalf("expected nil for 'none', got %v", da)
	}
}

func TestStyleGetStrokeDasharrayOddDoubled(t *testing.T) {
	da := getStrokeDasharray(`<path stroke-dasharray="5 3 2">`)
	if len(da) != 6 {
		t.Fatalf("odd-count should double, got len=%d", len(da))
	}
	if da[0] != 5 || da[3] != 5 {
		t.Fatalf("doubled array mismatch: %v", da)
	}
}

// --- parseTransform: skewX ---

func TestStyleParseTransformSkewX(t *testing.T) {
	m := parseTransform("skewX(45)")
	expected := float32(math.Tan(45.0 * math.Pi / 180.0))
	if f32Abs(m[2]-expected) > 1e-4 {
		t.Fatalf("expected m[2]=%f, got %f", expected, m[2])
	}
}

// --- applyInheritedTransformPt ---

func TestApplyInheritedTransformPt_ZeroAndIdentityNoop(t *testing.T) {
	var zero [6]float32
	x, y := applyInheritedTransformPt(3, 4, zero)
	if x != 3 || y != 4 {
		t.Fatalf("zero matrix should be no-op, got (%f,%f)", x, y)
	}
	x, y = applyInheritedTransformPt(3, 4, identityTransform)
	if x != 3 || y != 4 {
		t.Fatalf("identity should be no-op, got (%f,%f)", x, y)
	}
	// Real matrix: scale(2) + translate(10,20).
	m := [6]float32{2, 0, 0, 2, 10, 20}
	x, y = applyInheritedTransformPt(3, 4, m)
	if x != 16 || y != 28 {
		t.Fatalf("expected (16,28), got (%f,%f)", x, y)
	}
}

// --- clampOpacity01 ---

func TestClampOpacity01_EdgeInputs(t *testing.T) {
	if clampOpacity01(0.5) != 0.5 {
		t.Fatal("in-range unchanged")
	}
	if clampOpacity01(-1) != 0 {
		t.Fatal("negative must clamp to 0")
	}
	if clampOpacity01(2) != 1 {
		t.Fatal(">1 must clamp to 1")
	}
	if clampOpacity01(float32(math.NaN())) != 0 {
		t.Fatal("NaN must clamp to 0")
	}
	if clampOpacity01(float32(math.Inf(1))) != 1 {
		t.Fatal("+Inf must clamp to 1")
	}
	if clampOpacity01(float32(math.Inf(-1))) != 0 {
		t.Fatal("-Inf must clamp to 0")
	}
}

func TestSanitizeStrokeWidth_NaNNegativeZero(t *testing.T) {
	if sanitizeStrokeWidth(2.5) != 2.5 {
		t.Fatal("positive unchanged")
	}
	if sanitizeStrokeWidth(0) != 0 {
		t.Fatal("zero unchanged")
	}
	if sanitizeStrokeWidth(-3) != 0 {
		t.Fatal("negative must clamp to 0")
	}
	if sanitizeStrokeWidth(float32(math.NaN())) != 0 {
		t.Fatal("NaN must clamp to 0")
	}
}

// --- bakePathOpacity ---

func TestBakePathOpacity_SkipFlagsForceUnity(t *testing.T) {
	// Static fill-opacity="0" would normally bake fill alpha to 0
	// and cause tessellate to drop the geometry. SkipFillOpacity
	// forces the multiplier to 1 so an inline animation can supply
	// the value at render time.
	path := &VectorPath{
		FillColor:     gui.SvgColor{R: 10, G: 20, B: 30, A: 255},
		StrokeColor:   gui.SvgColor{R: 40, G: 50, B: 60, A: 255},
		Opacity:       1,
		FillOpacity:   0,
		StrokeOpacity: 0,
	}
	inh := defaultComputedStyle(identityTransform)
	inh.SkipFillOpacity = true
	inh.SkipStrokeOpacity = true
	bakePathOpacity(path, inh)
	if path.FillColor.A != 255 {
		t.Fatalf("SkipFillOpacity should preserve alpha, got %d",
			path.FillColor.A)
	}
	if path.StrokeColor.A != 255 {
		t.Fatalf("SkipStrokeOpacity should preserve alpha, got %d",
			path.StrokeColor.A)
	}

	// SkipOpacity forces combined to 1 even if path.Opacity=0.
	// FillOpacity=1 triggers use of inherited (which is 1).
	path = &VectorPath{
		FillColor:   gui.SvgColor{R: 0, G: 0, B: 0, A: 255},
		Opacity:     0,
		FillOpacity: 1,
	}
	inh = defaultComputedStyle(identityTransform)
	inh.SkipOpacity = true
	bakePathOpacity(path, inh)
	if path.FillColor.A != 255 {
		t.Fatalf("SkipOpacity should preserve alpha, got %d",
			path.FillColor.A)
	}
}

func TestBakePathOpacity_SentinelAlphaPromoted(t *testing.T) {
	// Sentinel colors carry tiny marker alphas (1 or 2). With a
	// nontrivial inherited opacity, naive multiplication would
	// collapse to 0 and drop the path. bakePathOpacity must lift
	// sentinel A to 255 before baking so small opacities survive.
	path := &VectorPath{
		FillColor:     colorCurrent,
		StrokeColor:   colorInherit,
		Opacity:       0.5,
		FillOpacity:   1,
		StrokeOpacity: 1,
	}
	inh := defaultComputedStyle(identityTransform)
	bakePathOpacity(path, inh)
	// Combined opacity 0.5 * 1 = 0.5. A promoted from 2 → 255,
	// then applyOpacity yields ~127. Without promotion A would be
	// uint8(2 * 0.5) = 1 and likely cause drop.
	if path.FillColor.A < 100 {
		t.Fatalf("expected sentinel promoted then scaled ~127, got %d",
			path.FillColor.A)
	}
	if path.StrokeColor.A < 100 {
		t.Fatalf("expected sentinel stroke promoted then scaled, got %d",
			path.StrokeColor.A)
	}
}

// isValidClipOrFilterValue is the cascade gate that decides whether
// a clip-path/filter declaration counts as authored. Direct unit
// coverage for edges that parseSvg-level tests reach only indirectly.
func TestIsValidClipOrFilterValue_EdgeCases(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"   ", false},
		{"none", true},
		{"NONE", true},
		{"  None  ", true},
		{"url(#a)", true},
		{"  url(#abc) ", true},
		{"url()", false},
		{"url(#)", false},
		{"url(#a", false},
		{"bogus", false},
		{"inherit", false},
		{"initial", false},
	}
	for _, c := range cases {
		if got := isValidClipOrFilterValue(c.in); got != c.want {
			t.Errorf("isValidClipOrFilterValue(%q)=%v; want %v",
				c.in, got, c.want)
		}
	}
}

func TestBakePathOpacity_NaNInputClampsSafely(t *testing.T) {
	// Hostile NaN opacity must not reach applyOpacity's uint8 cast.
	// Phase C: opacity is now sourced from the cascade-resolved
	// inh.Opacity (path.Opacity is mirror-only); inject NaN there.
	path := &VectorPath{
		FillColor: gui.SvgColor{R: 10, G: 20, B: 30, A: 200},
	}
	inh := defaultComputedStyle(identityTransform)
	inh.Opacity = float32(math.NaN())
	bakePathOpacity(path, inh)
	if path.FillColor.A != 0 {
		t.Fatalf("NaN opacity must clamp alpha to 0, got %d",
			path.FillColor.A)
	}
}
