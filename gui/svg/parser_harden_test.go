package svg

import (
	"math"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// findAttr must return entity-decoded values. encoding/xml decodes
// attribute entities once; buildOpenTag re-escapes the five chars that
// would let a hostile value smuggle a fake attribute past the
// substring scanner. unescapeAttrEntities reverses that re-escape so
// downstream parsers (id/href cross-refs, color, transform) see the
// original logical value.
func TestFindAttr_UnescapesRoundTrip(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{`<rect id="a&amp;b"/>`, "a&b"},
		{`<rect id="x&lt;y"/>`, "x<y"},
		{`<rect id="x&gt;y"/>`, "x>y"},
		{`<rect id="x&quot;y"/>`, `x"y`},
		{`<rect id="x&#39;y"/>`, "x'y"},
		{`<rect id="plain"/>`, "plain"},
		{`<rect id="a&amp;b&lt;c&gt;d"/>`, "a&b<c>d"},
	}
	for _, c := range cases {
		got, ok := findAttr(c.raw, "id")
		if !ok || got != c.want {
			t.Errorf("findAttr(%q): got %q ok=%v, want %q",
				c.raw, got, ok, c.want)
		}
	}
}

// Round-trip through buildOpenTag + findAttr must restore the original
// decoded value for every char in the re-escape set.
func TestBuildOpenTag_FindAttr_RoundTrip(t *testing.T) {
	for _, v := range []string{`a&b`, `x<y`, `x>y`, `x"y`, `x'y`,
		`mix &<>"' end`, `clean`} {
		tag := buildOpenTag("rect", []xmlAttr{{Name: "id", Value: v}}, true)
		got, ok := findAttr(tag, "id")
		if !ok || got != v {
			t.Errorf("round-trip %q: got %q ok=%v", v, got, ok)
		}
	}
}

// Unknown entities pass through unchanged. Only the five escapes the
// re-encoder produces are reversed.
func TestUnescapeAttrEntities_UnknownPassthrough(t *testing.T) {
	in := "&copy; &#x23; &foo;"
	if got := unescapeAttrEntities(in); got != in {
		t.Errorf("got %q want %q", got, in)
	}
}

// CharData accumulation must be order-preserving when encoding/xml
// fragments a single text node into many small chunks. A hostile asset
// can force chunking by sprinkling numeric entities through the text;
// the decoder emits one CharData per literal run between entity refs.
// Pre-fix, top.Text += s was O(N²); the strings.Builder rewrite must
// still produce the same final string.
func TestDecodeSvgTree_ChunkedCharDataConcat(t *testing.T) {
	// Each &amp; forces a token boundary. 256 chunks of "x" interleaved
	// with literal & gives 511-char body. Verify exact reconstruction.
	var src strings.Builder
	src.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"><title>`)
	var want strings.Builder
	for i := range 256 {
		src.WriteByte('x')
		want.WriteByte('x')
		if i < 255 {
			src.WriteString("&amp;")
			want.WriteByte('&')
		}
	}
	src.WriteString(`</title></svg>`)
	root, err := decodeSvgTree(src.String())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	title := root.findChild("title")
	if title == nil {
		t.Fatal("missing <title>")
	}
	if title.Text != want.String() {
		t.Fatalf("Text mismatch: got %q want %q", title.Text, want.String())
	}
	if title.Leading != want.String() {
		t.Fatalf("Leading mismatch: got %q want %q",
			title.Leading, want.String())
	}
}

// Tail accumulation across multiple chunks between siblings must
// concatenate in document order onto the prior child.
func TestDecodeSvgTree_ChunkedTailConcat(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg">` +
		`<g id="a"/>tail&amp;part&amp;more<g id="b"/></svg>`
	root, err := decodeSvgTree(src)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(root.Children) < 2 {
		t.Fatalf("want >=2 children, got %d", len(root.Children))
	}
	got := root.Children[0].Tail
	want := "tail&part&more"
	if got != want {
		t.Fatalf("Tail mismatch: got %q want %q", got, want)
	}
}

// nonNegF32 folds NaN and negative inputs to 0 so oversized / reversed-
// winding primitives from pathological spline overshoot never reach
// segmentsForRect / segmentsForEllipse.
func TestNonNegF32_NaNAndNegativeReturnZero(t *testing.T) {
	cases := []struct {
		in, want float32
	}{
		{0, 0},
		{1, 1},
		{-1, 0},
		{float32(math.NaN()), 0},
		{float32(math.Inf(-1)), 0},
		{float32(math.Inf(1)), float32(math.Inf(1))},
	}
	for _, c := range cases {
		got := nonNegF32(c.in)
		// NaN != NaN — compare bit-pattern via math.IsNaN.
		if math.IsNaN(float64(got)) != math.IsNaN(float64(c.want)) ||
			(!math.IsNaN(float64(got)) && got != c.want) {
			t.Errorf("nonNegF32(%v)=%v want %v", c.in, got, c.want)
		}
	}
}

// overrideScalar replaces non-finite override values with the parsed
// base so NaN/Inf keyframes cannot contaminate primitive geometry.
func TestOverrideScalar_NaNFallsBackToBase(t *testing.T) {
	ov := gui.SvgAnimAttrOverride{Mask: gui.SvgAnimMaskCX}
	nan := float32(math.NaN())
	if got := overrideScalar(5, nan, &ov, gui.SvgAnimMaskCX); got != 5 {
		t.Fatalf("NaN v: want base=5, got %v", got)
	}
	inf := float32(math.Inf(1))
	if got := overrideScalar(5, inf, &ov, gui.SvgAnimMaskCX); got != 5 {
		t.Fatalf("+Inf v: want base=5, got %v", got)
	}
	// Finite replace still wins.
	if got := overrideScalar(5, 9, &ov, gui.SvgAnimMaskCX); got != 9 {
		t.Fatalf("finite replace: want 9, got %v", got)
	}
	// Unset mask bit returns base unconditionally.
	ovEmpty := gui.SvgAnimAttrOverride{}
	if got := overrideScalar(5, 9, &ovEmpty, gui.SvgAnimMaskCX); got != 5 {
		t.Fatalf("unset mask: want 5, got %v", got)
	}
}

// buildParsed must propagate the viewBox origin onto SvgParsed so the
// render path can apply it as an outer translate. Animation fields
// and path coords stay in raw viewBox space so SMIL animateTransform
// in replace mode cannot erase the mapping.
func TestBuildParsed_PropagatesViewBoxOrigin(t *testing.T) {
	p := New()
	vg := &VectorGraphic{
		Width: 32, Height: 32,
		ViewBoxX: 10, ViewBoxY: 20,
		Animations: []gui.SvgAnimation{
			{
				Kind:       gui.SvgAnimMotion,
				GroupID:    "g1",
				MotionPath: []float32{100, 200, 110, 210},
			},
			{
				Kind:    gui.SvgAnimRotate,
				GroupID: "g1",
				CenterX: 50, CenterY: 60,
			},
		},
	}
	parsed := p.buildParsed(1, "", vg, 1)
	if parsed.ViewBoxX != 10 || parsed.ViewBoxY != 20 {
		t.Fatalf("viewBox origin not propagated: x=%v y=%v",
			parsed.ViewBoxX, parsed.ViewBoxY)
	}
	// MotionPath and rotate center must stay in authored viewBox
	// coords — render applies the outer shift.
	if parsed.Animations[0].MotionPath[0] != 100 ||
		parsed.Animations[0].MotionPath[1] != 200 {
		t.Fatalf("motion path shifted at parse: %v",
			parsed.Animations[0].MotionPath[:2])
	}
	if parsed.Animations[1].CenterX != 50 ||
		parsed.Animations[1].CenterY != 60 {
		t.Fatalf("rotate center shifted at parse: cx=%v cy=%v",
			parsed.Animations[1].CenterX, parsed.Animations[1].CenterY)
	}
}
