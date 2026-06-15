package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// --- applyOpacity ---

func TestApplyOpacity_Normal(t *testing.T) {
	c := gui.SvgColor{R: 200, G: 100, B: 50, A: 200}
	got := applyOpacity(c, 0.5)
	want := gui.SvgColor{R: 200, G: 100, B: 50, A: 100}
	if got != want {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestApplyOpacity_FullPreservesAlpha(t *testing.T) {
	c := gui.SvgColor{R: 10, G: 20, B: 30, A: 128}
	if applyOpacity(c, 1.0) != c {
		t.Error("opacity >= 1 must return color unchanged")
	}
}

func TestApplyOpacity_ZeroOrNegativeClearsAlpha(t *testing.T) {
	c := gui.SvgColor{R: 100, G: 100, B: 100, A: 255}
	got := applyOpacity(c, 0)
	if got.R != 100 || got.G != 100 || got.B != 100 || got.A != 0 {
		t.Errorf("zero opacity: got %+v want A=0", got)
	}
	got2 := applyOpacity(c, -0.5)
	if got2.A != 0 {
		t.Errorf("negative opacity: got A=%d want 0", got2.A)
	}
}

func TestApplyOpacity_NaNClampsToZero(t *testing.T) {
	c := gui.SvgColor{R: 100, G: 100, B: 100, A: 255}
	got := applyOpacity(c, float32(math.NaN()))
	if got.A != 0 {
		t.Errorf("NaN opacity: got A=%d want 0", got.A)
	}
}

// --- parseOpacityAttr ---

func TestParseOpacityAttr_FallbackWhenMissing(t *testing.T) {
	elem := `<rect fill="red"/>`
	v := parseOpacityAttr(elem, "opacity", 1.0)
	if v != 1.0 {
		t.Errorf("got %v want 1.0", v)
	}
}

func TestParseOpacityAttr_ClampOutOfRange(t *testing.T) {
	elem := `<rect opacity="2.5"/>`
	v := parseOpacityAttr(elem, "opacity", 1.0)
	if v != 1.0 {
		t.Errorf("clamp >1: got %v want 1.0", v)
	}
	elem2 := `<rect opacity="-3"/>`
	v2 := parseOpacityAttr(elem2, "opacity", 1.0)
	if v2 != 0 {
		t.Errorf("clamp <0: got %v want 0", v2)
	}
}

func TestParseOpacityAttr_ValidRange(t *testing.T) {
	elem := `<rect opacity="0.4"/>`
	v := parseOpacityAttr(elem, "opacity", 1.0)
	if v < 0.399 || v > 0.401 {
		t.Errorf("got %v want ~0.4", v)
	}
}

// --- parseElementStyle ---

func TestParseElementStyle_BasicAttributes(t *testing.T) {
	elem := `<rect fill="red" stroke="blue" stroke-width="2"/>`
	s := parseElementStyle(elem)
	if s.Opacity != 1.0 {
		t.Errorf("default opacity: got %v want 1", s.Opacity)
	}
}

// --- collectVars ---

func TestCollectVars_NoCustomPropsSharesParent(t *testing.T) {
	parent := map[string]string{"--x": "10"}
	decls := []css.MatchedDecl{
		{Decl: css.Decl{Name: "fill", Value: "red"}},
	}
	got := collectVars(decls, parent)
	// Parent returned when no custom props are declared.
	if len(got) != 1 || got["--x"] != "10" {
		t.Errorf("got %v want parent map unchanged", got)
	}
}

func TestCollectVars_CustomPropsAdded(t *testing.T) {
	parent := map[string]string{"--parent-x": "10"}
	decls := []css.MatchedDecl{
		{Decl: css.Decl{Name: "--my-var", Value: "20", CustomProp: true}},
		{Decl: css.Decl{Name: "fill", Value: "red"}},
	}
	got := collectVars(decls, parent)
	if len(got) != 2 {
		t.Fatalf("got %d entries want 2", len(got))
	}
	if got["--my-var"] != "20" {
		t.Errorf("--my-var=%q want 20", got["--my-var"])
	}
	if got["--parent-x"] != "10" {
		t.Errorf("--parent-x=%q want 10 (inherited)", got["--parent-x"])
	}
}

func TestCollectVars_OverrideParentVar(t *testing.T) {
	parent := map[string]string{"--color": "red"}
	decls := []css.MatchedDecl{
		{Decl: css.Decl{Name: "--color", Value: "blue", CustomProp: true}},
	}
	got := collectVars(decls, parent)
	if got["--color"] != "blue" {
		t.Errorf("got %q want blue", got["--color"])
	}
}

// --- makeElementInfo ---

func TestMakeElementInfo_TagAndID(t *testing.T) {
	openTag := `<rect id="r1" fill="red"/>`
	info := makeElementInfo("rect", openTag, 3, false, nil)
	if info.Tag != "rect" {
		t.Errorf("Tag=%q want rect", info.Tag)
	}
	if info.ID != "r1" {
		t.Errorf("ID=%q want r1", info.ID)
	}
	if info.Index != 3 {
		t.Errorf("Index=%d want 3", info.Index)
	}
	if info.IsRoot {
		t.Error("IsRoot must be false")
	}
}

func TestMakeElementInfo_WithClassAndAttrs(t *testing.T) {
	openTag := `<circle id="c1" class="big blue" cx="10"/>`
	attrs := map[string]string{"cx": "10"}
	info := makeElementInfo("circle", openTag, 1, false, attrs)
	if len(info.Classes) != 2 {
		t.Fatalf("Classes=%v want 2 entries", info.Classes)
	}
	if info.Classes[0] != "big" || info.Classes[1] != "blue" {
		t.Errorf("Classes=%v want [big blue]", info.Classes)
	}
	if info.Attrs["cx"] != "10" {
		t.Errorf("Attrs[cx]=%q want 10", info.Attrs["cx"])
	}
}

func TestMakeElementInfo_IsRoot(t *testing.T) {
	openTag := `<svg id="root" xmlns="..."/>`
	info := makeElementInfo("svg", openTag, 0, true, nil)
	if !info.IsRoot {
		t.Error("root element must have IsRoot=true")
	}
}

// --- applyPseudoState ---

func TestApplyPseudoState_HoverMatch(t *testing.T) {
	info := css.ElementInfo{ID: "btn1"}
	state := &parseState{hoveredID: "btn1"}
	applyPseudoState(&info, state)
	if !info.State.Hover {
		t.Error("Hover must be true when id matches hoveredID")
	}
	if info.State.Focus {
		t.Error("Focus must be false when id does not match focusedID")
	}
}

func TestApplyPseudoState_FocusMatch(t *testing.T) {
	info := css.ElementInfo{ID: "input1"}
	state := &parseState{focusedID: "input1"}
	applyPseudoState(&info, state)
	if !info.State.Focus {
		t.Error("Focus must be true when id matches focusedID")
	}
}

func TestApplyPseudoState_BothMatch(t *testing.T) {
	info := css.ElementInfo{ID: "x"}
	state := &parseState{hoveredID: "x", focusedID: "x"}
	applyPseudoState(&info, state)
	if !info.State.Hover || !info.State.Focus {
		t.Errorf("both states true: Hover=%v Focus=%v", info.State.Hover, info.State.Focus)
	}
}

func TestApplyPseudoState_NilState(t *testing.T) {
	info := css.ElementInfo{ID: "y"}
	applyPseudoState(&info, nil)
	if info.State.Hover || info.State.Focus {
		t.Error("nil state must not set any pseudo state")
	}
}

func TestApplyPseudoState_EmptyIDNoMatch(t *testing.T) {
	info := css.ElementInfo{ID: ""}
	state := &parseState{hoveredID: "", focusedID: ""}
	applyPseudoState(&info, state)
	if info.State.Hover || info.State.Focus {
		t.Error("empty IDs must not match")
	}
}

// --- resolveFillRule ---

func TestResolveFillRule_EvenOdd(t *testing.T) {
	elem := `<path fill-rule="evenodd"/>`
	parent := ComputedStyle{FillRule: FillRuleNonzero}
	got := resolveFillRule(elem, parent)
	if got != FillRuleEvenOdd {
		t.Errorf("got %v want FillRuleEvenOdd", got)
	}
}

func TestResolveFillRule_NonzeroDefault(t *testing.T) {
	elem := `<path fill-rule="bogus"/>`
	parent := ComputedStyle{FillRule: FillRuleEvenOdd}
	got := resolveFillRule(elem, parent)
	// bogus value defaults to nonzero, not inherited.
	if got != FillRuleNonzero {
		t.Errorf("got %v want FillRuleNonzero", got)
	}
}

func TestResolveFillRule_InheritsFromParent(t *testing.T) {
	elem := `<path/>`
	parent := ComputedStyle{FillRule: FillRuleEvenOdd}
	got := resolveFillRule(elem, parent)
	if got != FillRuleEvenOdd {
		t.Errorf("got %v want FillRuleEvenOdd (inherited)", got)
	}
}

// --- computeStyle ---

func TestComputeStyle_BasicInheritance(t *testing.T) {
	parent := ComputedStyle{
		Opacity:       0.8,
		FillOpacity:   0.9,
		StrokeOpacity: 0.7,
		Fill:          gui.SvgColor{R: 255, G: 0, B: 0, A: 255},
		FillSet:       true,
		Transform:     [6]float32{1, 0, 0, 1, 10, 0},
	}
	elem := `<rect fill="none"/>`
	info := css.ElementInfo{Tag: "rect"}
	out := computeStyle(elem, parent, nil, info, nil, nil)
	if out.Opacity != 0.8 {
		t.Errorf("Opacity=%v want 0.8", out.Opacity)
	}
	if out.FillOpacity != 0.9 {
		t.Errorf("FillOpacity=%v want 0.9", out.FillOpacity)
	}
	// fill="none" → display:none check not in computeStyle (that's in
	// parseSvgContent). The cascade fold should still run but with no
	// CSS rules or inline style, only pres-attrs.
	if out.FillSet {
		t.Log("FillSet may be set from pres-attr 'none' — cascade-ok")
	}
}

func TestComputeStyle_NilStateNoCSS(t *testing.T) {
	parent := ComputedStyle{
		Opacity: 1.0,
		GroupID: "parent-group",
	}
	elem := `<circle id="c1" fill="blue"/>`
	info := css.ElementInfo{Tag: "circle"}
	out := computeStyle(elem, parent, nil, info, nil, nil)
	if out.GroupID != "c1" {
		t.Errorf("GroupID=%q want c1 (from id attr)", out.GroupID)
	}
}

func TestComputeStyle_GroupIDInheritedWhenNoID(t *testing.T) {
	parent := ComputedStyle{GroupID: "g1"}
	elem := `<rect fill="red"/>`
	info := css.ElementInfo{Tag: "rect"}
	out := computeStyle(elem, parent, nil, info, nil, nil)
	if out.GroupID != "g1" {
		t.Errorf("GroupID=%q want g1 (inherited)", out.GroupID)
	}
}

func TestComputeStyle_OpacityMultipliesThroughAncestors(t *testing.T) {
	parent := ComputedStyle{Opacity: 0.5}
	elem := `<rect opacity="0.5"/>`
	info := css.ElementInfo{Tag: "rect"}
	out := computeStyle(elem, parent, nil, info, nil, nil)
	// parent 0.5 × element 0.5 = 0.25
	if out.Opacity < 0.24 || out.Opacity > 0.26 {
		t.Errorf("Opacity=%v want ~0.25", out.Opacity)
	}
}

func TestComputeStyle_DisplayReset(t *testing.T) {
	parent := ComputedStyle{Display: DisplayNone}
	elem := `<rect/>`
	info := css.ElementInfo{Tag: "rect"}
	out := computeStyle(elem, parent, nil, info, nil, nil)
	// display is NOT inherited; computeStyle resets to DisplayInline.
	if out.Display != DisplayInline {
		t.Errorf("Display=%v want DisplayInline (reset)", out.Display)
	}
}

// --- applyComputedStyle ---

func TestApplyComputedStyle_TransformApplied(t *testing.T) {
	path := &VectorPath{
		FillColor:  gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
		StrokeCap:  strokeCapInherit,
		StrokeJoin: strokeJoinInherit,
	}
	inh := ComputedStyle{
		Transform:   [6]float32{2, 0, 0, 2, 5, 5},
		Opacity:     1,
		FillOpacity: 1, StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.Transform != inh.Transform {
		t.Errorf("Transform=%v want %v", path.Transform, inh.Transform)
	}
}

func TestApplyComputedStyle_FillInheritDefaultsToBlack(t *testing.T) {
	path := &VectorPath{
		FillColor:      colorInherit,
		FillGradientID: "",
		StrokeColor:    colorTransparent,
	}
	inh := ComputedStyle{
		Opacity:     1,
		FillOpacity: 1, StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.FillColor != colorBlack {
		t.Errorf("FillColor=%+v want black (from inherit)", path.FillColor)
	}
}

func TestApplyComputedStyle_StrokeInheritDefaultsToTransparent(t *testing.T) {
	path := &VectorPath{
		StrokeColor: colorInherit,
		FillColor:   gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
	}
	inh := ComputedStyle{
		Opacity:     1,
		FillOpacity: 1, StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.StrokeColor != colorTransparent {
		t.Errorf("StrokeColor=%+v want transparent (from inherit)", path.StrokeColor)
	}
}

func TestApplyComputedStyle_UndefinedStrokeWidthDefaultsToOne(t *testing.T) {
	path := &VectorPath{
		StrokeWidth: -1, // sentinel for "unset"
		FillColor:   gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
		StrokeColor: colorTransparent,
	}
	// StrokeWidth < 0 on inh means "unset" so the path keeps its own.
	inh := ComputedStyle{
		StrokeWidth:   -1,
		Opacity:       1,
		FillOpacity:   1,
		StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.StrokeWidth != 1.0 {
		t.Errorf("StrokeWidth=%v want 1.0", path.StrokeWidth)
	}
}

func TestApplyComputedStyle_UndefinedStrokeCapDefaultsToButt(t *testing.T) {
	path := &VectorPath{
		StrokeCap:   strokeCapInherit,
		FillColor:   gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
		StrokeColor: colorTransparent,
	}
	inh := ComputedStyle{
		Opacity:     1,
		FillOpacity: 1, StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.StrokeCap != gui.SvgButtCap {
		t.Errorf("StrokeCap=%v want SvgButtCap", path.StrokeCap)
	}
}

func TestApplyComputedStyle_UndefinedStrokeJoinDefaultsToMiter(t *testing.T) {
	path := &VectorPath{
		StrokeJoin:  strokeJoinInherit,
		FillColor:   gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
		StrokeColor: colorTransparent,
	}
	inh := ComputedStyle{
		Opacity:     1,
		FillOpacity: 1, StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.StrokeJoin != gui.SvgMiterJoin {
		t.Errorf("StrokeJoin=%v want SvgMiterJoin", path.StrokeJoin)
	}
}

func TestApplyComputedStyle_GradientOverridesColor(t *testing.T) {
	path := &VectorPath{
		FillColor:   gui.SvgColor{R: 255, G: 0, B: 0, A: 255},
		StrokeColor: colorTransparent,
	}
	inh := ComputedStyle{
		FillGradient: "url(#g1)",
		FillSet:      true,
		Opacity:      1,
		FillOpacity:  1, StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.FillGradientID != "url(#g1)" {
		t.Errorf("FillGradientID=%q want url(#g1)", path.FillGradientID)
	}
	if path.FillColor != colorTransparent {
		t.Errorf("FillColor must be transparent when gradient set; got %+v", path.FillColor)
	}
}

func TestApplyComputedStyle_ClipPathInheritedWhenUnset(t *testing.T) {
	path := &VectorPath{
		FillColor:   gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
		StrokeColor: colorTransparent,
	}
	inh := ComputedStyle{
		ClipPathID:  "clip1",
		Opacity:     1,
		FillOpacity: 1, StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.ClipPathID != "clip1" {
		t.Errorf("ClipPathID=%q want clip1", path.ClipPathID)
	}
}

func TestApplyComputedStyle_GroupIDInheritedWhenUnset(t *testing.T) {
	path := &VectorPath{
		FillColor:   gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
		StrokeColor: colorTransparent,
	}
	inh := ComputedStyle{
		GroupID:     "g1",
		Opacity:     1,
		FillOpacity: 1, StrokeOpacity: 1,
	}
	applyComputedStyle(path, inh)
	if path.GroupID != "g1" {
		t.Errorf("GroupID=%q want g1", path.GroupID)
	}
}

func TestApplyComputedStyle_OpacityMirroredToPath(t *testing.T) {
	path := &VectorPath{
		FillColor:   gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
		StrokeColor: colorTransparent,
	}
	inh := ComputedStyle{
		Opacity:       0.5,
		FillOpacity:   0.8,
		StrokeOpacity: 0.3,
	}
	applyComputedStyle(path, inh)
	if path.Opacity != 0.5 {
		t.Errorf("path.Opacity=%v want 0.5", path.Opacity)
	}
	if path.FillOpacity != 0.8 {
		t.Errorf("path.FillOpacity=%v want 0.8", path.FillOpacity)
	}
	if path.StrokeOpacity != 0.3 {
		t.Errorf("path.StrokeOpacity=%v want 0.3", path.StrokeOpacity)
	}
}

// --- bakePathOpacity ---

func TestBakePathOpacity_VisibilityHiddenZeroesAlpha(t *testing.T) {
	path := &VectorPath{
		FillColor:   gui.SvgColor{R: 100, G: 100, B: 100, A: 200},
		StrokeColor: gui.SvgColor{R: 50, G: 50, B: 50, A: 128},
	}
	inh := ComputedStyle{
		Visibility:    VisibilityHidden,
		Opacity:       1,
		FillOpacity:   1,
		StrokeOpacity: 1,
	}
	bakePathOpacity(path, inh)
	if path.FillColor.A != 0 || path.StrokeColor.A != 0 {
		t.Errorf("hidden: Fill.A=%d Stroke.A=%d want both 0",
			path.FillColor.A, path.StrokeColor.A)
	}
	if path.Opacity != 0 || path.FillOpacity != 0 || path.StrokeOpacity != 0 {
		t.Error("hidden: all opacity fields must be zero")
	}
}

func TestBakePathOpacity_Normal(t *testing.T) {
	path := &VectorPath{
		FillColor:   gui.SvgColor{R: 100, G: 100, B: 100, A: 200},
		StrokeColor: gui.SvgColor{R: 50, G: 50, B: 50, A: 128},
	}
	inh := ComputedStyle{
		Opacity:       0.5,
		FillOpacity:   1,
		StrokeOpacity: 1,
	}
	bakePathOpacity(path, inh)
	// combinedOpacity = 0.5, fillOpacity = 1 → fill alpha = 200 * 0.5 = 100
	if path.FillColor.A != 100 {
		t.Errorf("FillColor.A=%d want 100", path.FillColor.A)
	}
	if path.StrokeColor.A != 64 {
		t.Errorf("StrokeColor.A=%d want 64", path.StrokeColor.A)
	}
}
