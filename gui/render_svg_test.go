package gui

import (
	"math"
	"testing"
)

func TestRenderSvgNoParser(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeSVG,
		X:         10,
		Y:         20,
		Width:     100,
		Height:    100,
		Resource:  "<svg></svg>",
	}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	// Should not panic; emits error placeholder.
	renderSvg(shape, clip, w)

	hasRect := false
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Color == Magenta {
			hasRect = true
		}
	}
	if !hasRect {
		t.Fatal("expected magenta error placeholder")
	}
}

func TestRenderSvgOutOfClip(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeSVG,
		X:         2000,
		Y:         2000,
		Width:     100,
		Height:    100,
	}
	clip := drawClip{X: 0, Y: 0, Width: 100, Height: 100}
	renderSvg(shape, clip, w)
	if len(w.renderers) != 0 {
		t.Fatal("expected no render commands when out of clip")
	}
}

func TestRenderSvgWithParser(t *testing.T) {
	w := &Window{}
	w.SetSvgParser(&mockSvgParser{width: 64, height: 64})

	shape := &Shape{
		shapeType: shapeSVG,
		X:         0,
		Y:         0,
		Width:     100,
		Height:    100,
		Resource:  "<svg></svg>",
	}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	renderSvg(shape, clip, w)

	hasSvg := false
	hasClip := false
	for _, r := range w.renderers {
		if r.Kind == RenderSvg {
			hasSvg = true
		}
		if r.Kind == RenderClip {
			hasClip = true
		}
	}
	if !hasSvg {
		t.Fatal("expected RenderSvg command")
	}
	if !hasClip {
		t.Fatal("expected RenderClip for viewBox")
	}
}

func TestEmitSvgPathRendererTint(t *testing.T) {
	w := &Window{}
	path := CachedSvgPath{
		Triangles: []float32{0, 0, 10, 0, 5, 10, 5, 10, 10, 0, 10, 10},
		Color:     Color{0, 0, 0, 255, true},
	}
	tint := Color{255, 0, 0, 200, true}
	emitSvgPathRenderer(path, tint, 0, 0, 1.0, nil, w)

	if len(w.renderers) != 1 {
		t.Fatalf("expected 1 renderer, got %d",
			len(w.renderers))
	}
	// Tint should override path color when no vertex colors.
	if w.renderers[0].Color != tint {
		t.Fatalf("expected tint color, got %+v",
			w.renderers[0].Color)
	}
}

func TestEmitSvgPathRendererVertexColors(t *testing.T) {
	w := &Window{}
	path := CachedSvgPath{
		Triangles: []float32{0, 0, 10, 0, 5, 10, 5, 10, 10, 0, 10, 10},
		Color:     Color{0, 0, 0, 255, true},
		VertexColors: []Color{
			{255, 0, 0, 255, true},
			{0, 255, 0, 255, true},
			{0, 0, 255, 255, true},
			{255, 255, 0, 255, true},
			{0, 255, 255, 255, true},
			{255, 0, 255, 255, true},
		},
	}
	// No tint (A=0) → vertex colors used.
	emitSvgPathRenderer(path, Color{}, 0, 0, 1.0, nil, w)

	if len(w.renderers[0].VertexColors) != 6 {
		t.Fatalf("expected 6 vertex colors, got %d",
			len(w.renderers[0].VertexColors))
	}
}

func TestEmitSvgPathRendererAnimatedVertexAlphaNoCopy(t *testing.T) {
	w := &Window{}
	path := CachedSvgPath{
		Triangles: []float32{0, 0, 10, 0, 5, 10, 5, 10, 10, 0, 10, 10},
		Color:     Color{0, 0, 0, 255, true},
		VertexColors: []Color{
			{255, 0, 0, 255, true},
			{0, 255, 0, 255, true},
			{0, 0, 255, 255, true},
			{255, 255, 0, 255, true},
			{0, 255, 255, 255, true},
			{255, 0, 255, 255, true},
		},
		PathID: 1,
	}
	animState := map[uint32]svgAnimState{
		1: {Opacity: 0.5, FillOpacity: 1, StrokeOpacity: 1, Inited: true},
	}
	emitSvgPathRenderer(path, Color{}, 0, 0, 1.0, animState, w)

	r := w.renderers[0]
	if !r.HasVertexAlpha {
		t.Fatal("expected vertex alpha scaling flag")
	}
	if r.VertexAlphaScale != 0.5 {
		t.Fatalf("expected alpha scale 0.5, got %f", r.VertexAlphaScale)
	}
	if &r.VertexColors[0] != &path.VertexColors[0] {
		t.Fatal("expected vertex colors slice to be reused without copy")
	}
}

func TestEmitCachedSvgTextDraw(t *testing.T) {
	w := &Window{}
	draw := CachedSvgTextDraw{
		Text: "hello",
		TextStyle: TextStyle{
			Family: "sans",
			Size:   12,
			Color:  Color{0, 0, 0, 255, true},
		},
		X: 5,
		Y: 10,
	}
	emitCachedSvgTextDraw(&draw, 100, 200, w)

	if len(w.renderers) != 1 {
		t.Fatalf("expected 1 renderer, got %d",
			len(w.renderers))
	}
	r := w.renderers[0]
	if r.Kind != RenderText {
		t.Fatalf("expected RenderText, got %d", r.Kind)
	}
	if r.X != 105 || r.Y != 210 {
		t.Fatalf("expected (105,210), got (%f,%f)", r.X, r.Y)
	}
	if r.Text != "hello" {
		t.Fatalf("expected 'hello', got %q", r.Text)
	}
	if r.TextStylePtr == nil {
		t.Fatal("expected TextStylePtr to be set")
	}
	if r.TextStylePtr.Family != "sans" {
		t.Fatalf("expected family 'sans', got %q",
			r.TextStylePtr.Family)
	}
}

func TestEmitCachedSvgTextDrawWithStyle(t *testing.T) {
	w := &Window{}
	draw := CachedSvgTextDraw{
		Text: "styled",
		TextStyle: TextStyle{
			Family:        "serif",
			Size:          24,
			Color:         Color{255, 0, 0, 255, true},
			Underline:     true,
			Strikethrough: true,
			StrokeWidth:   2,
			StrokeColor:   Color{0, 0, 255, 255, true},
		},
		X: 10,
		Y: 20,
	}
	emitCachedSvgTextDraw(&draw, 0, 0, w)

	r := w.renderers[0]
	if r.TextStylePtr == nil {
		t.Fatal("expected TextStylePtr")
	}
	ts := r.TextStylePtr
	if !ts.Underline {
		t.Fatal("expected underline")
	}
	if !ts.Strikethrough {
		t.Fatal("expected strikethrough")
	}
	if ts.StrokeWidth != 2 {
		t.Fatalf("expected stroke width 2, got %f",
			ts.StrokeWidth)
	}
}

func TestEmitCachedSvgTextPathDraw(t *testing.T) {
	w := &Window{}
	draw := CachedSvgTextPathDraw{
		Text: "path text",
		TextStyle: TextStyle{
			Family: "sans",
			Size:   12,
			Color:  Color{10, 20, 30, 255, true},
		},
		Path: TextPathData{
			Polyline: []float32{0, 0, 10, 0},
			Table:    []float32{0, 10},
			TotalLen: 10,
		},
	}
	emitCachedSvgTextPathDraw(&draw, 7, 9, w)
	if len(w.renderers) != 1 {
		t.Fatalf("expected 1 renderer, got %d",
			len(w.renderers))
	}
	r := w.renderers[0]
	if r.Kind != RenderTextPath {
		t.Fatalf("expected RenderTextPath, got %d", r.Kind)
	}
	if r.TextStylePtr == nil || r.TextPath == nil {
		t.Fatal("expected text path pointers")
	}
	if r.TextStylePtr.Family != "sans" {
		t.Fatalf("unexpected family: %q", r.TextStylePtr.Family)
	}
	if r.TextPath.TotalLen != 10 {
		t.Fatalf("unexpected total len: %f", r.TextPath.TotalLen)
	}
}

func TestComputeSvgAnimationsOpacityZeroPreserved(t *testing.T) {
	// Animation A sets opacity=0 on group "g1".
	// Animation B is a rotate on the same group.
	// Opacity must stay 0, not reset to 1.
	anims := []SvgAnimation{
		{
			GroupID:       "g1",
			TargetPathIDs: []uint32{1},
			Kind:          SvgAnimOpacity,
			DurSec:        1,
			Values:        []float32{0, 0}, // constant 0
		},
		{
			GroupID:       "g1",
			TargetPathIDs: []uint32{1},
			Kind:          SvgAnimRotate,
			DurSec:        2,
			Values:        []float32{0, 360},
			CenterX:       50,
			CenterY:       50,
		},
	}
	states := computeSvgAnimations(anims, 0.5, nil)
	st, ok := states[1]
	if !ok {
		t.Fatal("expected state for g1")
	}
	if st.Opacity != 0 {
		t.Fatalf("expected opacity 0, got %f", st.Opacity)
	}
	if st.RotAngle == 0 {
		t.Fatal("expected non-zero rotation")
	}
}

func TestEmitErrorPlaceholder(t *testing.T) {
	w := &Window{}
	emitErrorPlaceholder(10, 20, 50, 30, w)

	if len(w.renderers) != 2 {
		t.Fatalf("expected 2 renderers, got %d",
			len(w.renderers))
	}
	if w.renderers[0].Kind != RenderRect {
		t.Fatal("expected RenderRect")
	}
	if w.renderers[0].Color != Magenta {
		t.Fatal("expected Magenta fill")
	}
	if w.renderers[1].Kind != RenderStrokeRect {
		t.Fatal("expected RenderStrokeRect")
	}
	if w.renderers[1].Color != White {
		t.Fatal("expected White stroke")
	}
}

func TestEmitErrorPlaceholderZeroSize(t *testing.T) {
	w := &Window{}
	emitErrorPlaceholder(10, 20, 0, 30, w)
	if len(w.renderers) != 0 {
		t.Fatal("expected no renderers for zero width")
	}
}

func TestRenderSvgDispatch(t *testing.T) {
	w := &Window{}
	w.SetSvgParser(&mockSvgParser{width: 64, height: 64})

	shape := &Shape{
		shapeType: shapeSVG,
		X:         10,
		Y:         10,
		Width:     100,
		Height:    100,
		Resource:  "<svg></svg>",
		Opacity:   1.0,
		Color:     ColorTransparent,
	}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}

	// Call through the dispatch.
	renderShapeInner(shape, ColorTransparent, clip, w)

	hasSvg := false
	for _, r := range w.renderers {
		if r.Kind == RenderSvg {
			hasSvg = true
		}
	}
	if !hasSvg {
		t.Fatal("dispatch should call renderSvg")
	}
}

func TestRenderImageDispatch(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeImage,
		X:         10,
		Y:         10,
		Width:     100,
		Height:    100,
		Resource:  "test.png",
		Opacity:   1.0,
	}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	renderShapeInner(shape, ColorTransparent, clip, w)

	hasImage := false
	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			hasImage = true
		}
	}
	if !hasImage {
		t.Fatal("dispatch should call renderImage")
	}
}

// --- isFiniteF ---

func TestFiniteF32_NaNInfFinite(t *testing.T) {
	if !isFiniteF(0) || !isFiniteF(1.5) || !isFiniteF(-1e20) {
		t.Fatal("finite values should report true")
	}
	if isFiniteF(float32(math.NaN())) {
		t.Fatal("NaN must report false")
	}
	if isFiniteF(float32(math.Inf(1))) {
		t.Fatal("+Inf must report false")
	}
	if isFiniteF(float32(math.Inf(-1))) {
		t.Fatal("-Inf must report false")
	}
}

// --- collectAnimContribs timing guards ---

// collectAnimContribs must reject animations whose timing fields
// are NaN or ±Inf. Otherwise downstream Lerp/floor math produces
// NaN values that poison the anim state map.
func TestCollectAnimContribs_RejectsNonFiniteTimings(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	base := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 1},
		DurSec:        1,
	}
	cases := []struct {
		name string
		mut  func(a *SvgAnimation)
	}{
		{"NaN Dur", func(a *SvgAnimation) { a.DurSec = nan }},
		{"Inf Dur", func(a *SvgAnimation) { a.DurSec = inf }},
		{"NaN Begin", func(a *SvgAnimation) { a.BeginSec = nan }},
		{"Inf Begin", func(a *SvgAnimation) { a.BeginSec = inf }},
		{"NaN Cycle", func(a *SvgAnimation) { a.Cycle = nan }},
		{"Inf Cycle", func(a *SvgAnimation) { a.Cycle = inf }},
	}
	for _, tc := range cases {
		a := base
		tc.mut(&a)
		out := collectAnimContribs([]SvgAnimation{a}, 0.5, nil)
		if len(out) != 0 {
			t.Fatalf("%s: expected no contribution, got %d", tc.name, len(out))
		}
	}
	// Sanity: a clean animation still contributes.
	out := collectAnimContribs([]SvgAnimation{base}, 0.5, nil)
	if len(out) != 1 {
		t.Fatalf("clean anim should contribute, got %d", len(out))
	}
}

func TestCollectAnimContribs_RejectsNonFiniteElapsed(t *testing.T) {
	a := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 1},
		DurSec:        1,
	}
	out := collectAnimContribs([]SvgAnimation{a}, float32(math.NaN()), nil)
	if len(out) != 0 {
		t.Fatal("NaN elapsed must produce no contributions")
	}
}

// --- Freeze semantics ---

// fill="freeze" must hold the last keyframe value after dur and
// before the next cycle. Without freeze the animation drops out.
func TestComputeSvgAnimations_FreezeHoldsLastValue(t *testing.T) {
	a := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{1, 0},
		DurSec:        1,
		Freeze:        true,
	}
	// elapsed 5s, dur 1s, no cycle → past end. Freeze must hold
	// frac=1 → value 0.
	st := computeSvgAnimations([]SvgAnimation{a}, 5, nil)
	if st[1].Opacity != 0 {
		t.Fatalf("freeze should hold final value 0, got %f", st[1].Opacity)
	}

	// Without freeze, same animation contributes nothing: state
	// for "g" never gets created.
	a.Freeze = false
	st = computeSvgAnimations([]SvgAnimation{a}, 5, nil)
	if _, ok := st[1]; ok {
		t.Fatal("non-freeze past-dur must not contribute state")
	}
}

// Additive=true on translate sums the animated value onto the prior
// state (init 0) rather than replacing.
func TestComputeSvgAnimations_AdditiveTranslate(t *testing.T) {
	base := SvgAnimation{
		Kind:          SvgAnimTranslate,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{10, 20, 10, 20}, // constant (10,20)
		DurSec:        1,
		Freeze:        true,
	}
	add := SvgAnimation{
		Kind:          SvgAnimTranslate,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 0, 3, 4}, // by (3,4)
		DurSec:        1,
		Freeze:        true,
		Additive:      true,
	}
	// Same activation time → stable sort keeps input order: base
	// first, additive second sums on top.
	st := computeSvgAnimations([]SvgAnimation{base, add}, 1.0, nil)
	if st[1].TransX != 13 || st[1].TransY != 24 {
		t.Fatalf("additive translate want (13,24), got (%f,%f)",
			st[1].TransX, st[1].TransY)
	}
}

// Additive=true on attr with no base-contrib marks AdditiveMask so
// the override adds to the primitive's parsed value at render.
func TestComputeSvgAnimations_AdditiveAttrMarksAdditiveMask(t *testing.T) {
	a := SvgAnimation{
		Kind:          SvgAnimAttr,
		AttrName:      SvgAttrR,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 5}, // by 5
		DurSec:        1,
		Freeze:        true,
		Additive:      true,
	}
	st := computeSvgAnimations([]SvgAnimation{a}, 1.0, nil)
	ov := st[1].AttrOverride
	if ov.Mask&SvgAnimMaskR == 0 {
		t.Fatal("expected R mask set")
	}
	if ov.AdditiveMask&SvgAnimMaskR == 0 {
		t.Fatal("expected R AdditiveMask set")
	}
	if ov.R != 5 {
		t.Fatalf("expected R=5 delta, got %f", ov.R)
	}
}

// animateMotion samples along a flattened straight-line path by
// arc length and writes TransX/TransY.
func TestComputeSvgAnimations_MotionStraightLine(t *testing.T) {
	// Horizontal line of length 10. At frac=0.5, point = (5, 0).
	a := SvgAnimation{
		Kind:          SvgAnimMotion,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		DurSec:        1,
		Freeze:        true,
		MotionPath:    []float32{0, 0, 10, 0},
		MotionLengths: []float32{0, 10},
	}
	st := computeSvgAnimations([]SvgAnimation{a}, 0.5, nil)
	if st[1].TransX < 4.9 || st[1].TransX > 5.1 ||
		st[1].TransY != 0 {
		t.Fatalf("expected (5,0), got (%f,%f)",
			st[1].TransX, st[1].TransY)
	}
	if !st[1].HasXform {
		t.Fatal("expected HasXform=true")
	}
}

// rotate=auto writes the tangent angle into RotAngle.
func TestComputeSvgAnimations_MotionRotateAuto(t *testing.T) {
	// Vertical line: tangent = (0, 1) → atan2(1,0) = 90°.
	a := SvgAnimation{
		Kind:          SvgAnimMotion,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		DurSec:        1,
		Freeze:        true,
		MotionPath:    []float32{0, 0, 0, 10},
		MotionLengths: []float32{0, 10},
		MotionRotate:  SvgAnimMotionRotateAuto,
	}
	st := computeSvgAnimations([]SvgAnimation{a}, 0.5, nil)
	if st[1].RotAngle < 89 || st[1].RotAngle > 91 {
		t.Fatalf("expected ~90°, got %f", st[1].RotAngle)
	}
}

// Accumulate=sum stacks each cycle's delta onto the value.
func TestComputeSvgAnimations_AccumulateSum(t *testing.T) {
	a := SvgAnimation{
		Kind:          SvgAnimRotate,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 360}, // one full turn per cycle
		DurSec:        1,
		Cycle:         1, // re-fire each second
		Freeze:        true,
		Accumulate:    true,
	}
	// At t=3.5s: 3 completed prior cycles → accum offset = 3*360 = 1080.
	// In the current cycle, phase=0.5 → Lerp → 180. Total = 1260.
	st := computeSvgAnimations([]SvgAnimation{a}, 3.5, nil)
	got := st[1].RotAngle
	if got < 1259 || got > 1261 {
		t.Fatalf("accumulate sum want ≈1260 deg, got %f", got)
	}
}

// Accumulate and Additive are orthogonal per SMIL: accumulate stacks
// prior-iteration deltas onto the current value; additive then sums
// onto the base (existing sandwich state). Both active must apply.
func TestComputeSvgAnimations_AccumulateWithAdditive(t *testing.T) {
	// Base rotate holds at 10°. Second animation: rotate 0→90 with
	// repeatCount=3 (via Cycle=1), accumulate="sum", additive="sum".
	// At t=2.5s, iteration 2, localFrac=0.5 → local=45. Accum delta
	// from 2 prior iterations = 2*90 = 180. animValue = 45+180 = 225.
	// Additive sums onto base 10 → effective 235.
	base := SvgAnimation{
		Kind:          SvgAnimRotate,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{10, 10},
		DurSec:        1,
		Freeze:        true,
	}
	acc := SvgAnimation{
		Kind:          SvgAnimRotate,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 90},
		DurSec:        1,
		Cycle:         1,
		Freeze:        true,
		Accumulate:    true,
		Additive:      true,
	}
	st := computeSvgAnimations([]SvgAnimation{base, acc}, 2.5, nil)
	got := st[1].RotAngle
	if got < 234 || got > 236 {
		t.Fatalf("accumulate+additive want ≈235, got %f", got)
	}
}

// Restart=never clamps activation to BeginSec even when Cycle>0.
func TestComputeSvgAnimations_RestartNever(t *testing.T) {
	a := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{1, 0},
		DurSec:        1,
		Cycle:         1, // would normally re-fire
		Freeze:        true,
		Restart:       SvgAnimRestartNever,
	}
	// At t=5 (5 cycles in): without never, activation would be 5.0
	// and phase=0 → value=1. With never, activation stays at 0 and
	// phase=5 → past dur, freeze holds frac=1 → value=0.
	st := computeSvgAnimations([]SvgAnimation{a}, 5, nil)
	if st[1].Opacity != 0 {
		t.Fatalf("restart=never should freeze at 0, got %f",
			st[1].Opacity)
	}
}

// Restart=whenNotActive suppresses re-trigger while previous activation
// still within dur.
func TestComputeSvgAnimations_RestartWhenNotActive(t *testing.T) {
	a := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{1, 0},
		DurSec:        2, // longer than cycle
		Cycle:         1, // re-fires at t=1, 2, 3...
		Freeze:        true,
		Restart:       SvgAnimRestartWhenNotActive,
	}
	// At t=1.5: always-mode would jump to activation=1 (phase=0.5).
	// whenNotActive: prev activation 0 is still active (0<dur=2), so
	// suppress → activation stays 0, phase=1.5, frac=0.75 → 0.25.
	st := computeSvgAnimations([]SvgAnimation{a}, 1.5, nil)
	if st[1].Opacity < 0.2 || st[1].Opacity > 0.3 {
		t.Fatalf("whenNotActive should keep prev activation, "+
			"got opacity=%f", st[1].Opacity)
	}
}

// <set> zero-duration animation: inactive before BeginSec, contributes
// to-value after BeginSec regardless of dur.
func TestComputeSvgAnimations_SetZeroDuration(t *testing.T) {
	a := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 0}, // to=0
		BeginSec:      1,
		IsSet:         true,
		Freeze:        true,
	}
	// Before begin: no contribution.
	st := computeSvgAnimations([]SvgAnimation{a}, 0.5, nil)
	if _, ok := st[1]; ok {
		t.Fatal("before BeginSec must not contribute")
	}
	// At begin and beyond: opacity is forced to 0.
	st = computeSvgAnimations([]SvgAnimation{a}, 1.0, nil)
	if st[1].Opacity != 0 {
		t.Fatalf("expected opacity=0 at begin, got %f", st[1].Opacity)
	}
	st = computeSvgAnimations([]SvgAnimation{a}, 10.0, nil)
	if st[1].Opacity != 0 {
		t.Fatalf("expected opacity=0 long after begin, got %f",
			st[1].Opacity)
	}
}

// --- Cycle restart ---

// Cycle>0 must re-fire the animation every cycle seconds. At
// elapsed = Cycle+epsilon the phase is effectively 0 so the Lerp
// returns the first keyframe value again.
func TestComputeSvgAnimations_CycleRestart(t *testing.T) {
	a := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{1, 0}, // 1→0 over dur
		DurSec:        1,
		Cycle:         2, // repeats every 2s
	}
	// At elapsed=2 (cycle boundary) phase ≈ 0 → opacity ≈ 1.
	st := computeSvgAnimations([]SvgAnimation{a}, 2, nil)
	if math.Abs(float64(st[1].Opacity-1)) > 1e-3 {
		t.Fatalf("expected opacity ~1 at cycle restart, got %f",
			st[1].Opacity)
	}
}

// --- Sandwich priority ---

// When two animations target the same attribute on the same
// group, the one whose last activation is later must win under
// SMIL sandwich semantics.
func TestComputeSvgAnimations_SandwichLastActivationWins(t *testing.T) {
	// a1 activates at t=0, a2 at t=1. At elapsed=2 both are past
	// their dur (each is 1s). With Freeze both contribute; a2's
	// activation (1) > a1's (0), so a2's final value wins.
	a1 := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 0.25},
		DurSec:        1,
		BeginSec:      0,
		Freeze:        true,
	}
	a2 := SvgAnimation{
		Kind:          SvgAnimOpacity,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		Values:        []float32{0, 0.75},
		DurSec:        1,
		BeginSec:      1,
		Freeze:        true,
	}
	st := computeSvgAnimations([]SvgAnimation{a1, a2}, 2, nil)
	if math.Abs(float64(st[1].Opacity-0.75)) > 1e-3 {
		t.Fatalf("expected a2's value 0.75 to win, got %f", st[1].Opacity)
	}
}

// --- Opacity clamp on render ---

// An opacity value outside [0,1] (reached via a hostile anim
// values list) must not drive a negative or >1 multiplier into
// the uint8 alpha cast. clampFrac in emitSvgPathRenderer guards
// this; verify NaN opacity → alpha 0.
func TestEmitSvgPathRenderer_OpacityNaNClampedToZero(t *testing.T) {
	w := &Window{}
	path := CachedSvgPath{
		Triangles: []float32{0, 0, 10, 0, 5, 10, 5, 10, 10, 0, 10, 10},
		Color:     Color{10, 20, 30, 200, true},
		PathID:    1,
	}
	animState := map[uint32]svgAnimState{
		1: {
			Opacity:       float32(math.NaN()),
			FillOpacity:   1,
			StrokeOpacity: 1,
			Inited:        true,
		},
	}
	emitSvgPathRenderer(path, Color{}, 0, 0, 1.0, animState, w)
	if len(w.renderers) != 1 {
		t.Fatalf("expected 1 renderer, got %d", len(w.renderers))
	}
	if w.renderers[0].Color.A != 0 {
		t.Fatalf("NaN opacity must clamp alpha to 0, got %d",
			w.renderers[0].Color.A)
	}
}

// Fill-opacity animation targeting a stroke path must NOT dim
// the stroke (and vice-versa). emitSvgPathRenderer picks
// FillOpacity vs StrokeOpacity based on path.IsStroke.
func TestSvgRender_FillOpacityAnimDoesNotDimStroke(t *testing.T) {
	w := &Window{}
	strokePath := CachedSvgPath{
		Triangles: []float32{0, 0, 10, 0, 5, 10, 5, 10, 10, 0, 10, 10},
		Color:     Color{0, 0, 0, 255, true},
		PathID:    1,
		IsStroke:  true,
	}
	// FillOpacity drops to 0; StrokeOpacity stays 1. The stroke
	// path's alpha must remain 255.
	animState := map[uint32]svgAnimState{
		1: {
			Opacity:       1,
			FillOpacity:   0,
			StrokeOpacity: 1,
			Inited:        true,
		},
	}
	emitSvgPathRenderer(strokePath, Color{}, 0, 0, 1.0, animState, w)
	if w.renderers[0].Color.A != 255 {
		t.Fatalf("stroke should keep alpha 255 when only fill-opacity "+
			"is animated, got %d", w.renderers[0].Color.A)
	}

	// Mirror: fill path under stroke-opacity=0 stays opaque.
	w = &Window{}
	fillPath := strokePath
	fillPath.IsStroke = false
	animState = map[uint32]svgAnimState{
		1: {
			Opacity:       1,
			FillOpacity:   1,
			StrokeOpacity: 0,
			Inited:        true,
		},
	}
	emitSvgPathRenderer(fillPath, Color{}, 0, 0, 1.0, animState, w)
	if w.renderers[0].Color.A != 255 {
		t.Fatalf("fill should keep alpha 255 when only stroke-opacity "+
			"is animated, got %d", w.renderers[0].Color.A)
	}
}

// motionSample returns zeros when MotionPath holds fewer vertices
// than MotionLengths claims — without the guard, poly[(idx+1)*2]
// would panic on out-of-bounds.
func TestMotionSample_InconsistentPolyLengthReturnsZero(t *testing.T) {
	a := &SvgAnimation{
		MotionLengths: []float32{0, 10, 20, 30},
		MotionPath:    []float32{0, 0, 10, 0}, // only 2 vertices
	}
	x, y, ang := motionSample(a, 0.5)
	if x != 0 || y != 0 || ang != 0 {
		t.Fatalf("inconsistent lens/poly: want (0,0,0), got (%f,%f,%f)",
			x, y, ang)
	}
}

// motionSample returns zeros when the total arc length is non-
// finite so NaN/Inf cannot poison TransX/TransY.
func TestMotionSample_NonFiniteTotalReturnsZero(t *testing.T) {
	inf := float32(math.Inf(1))
	a := &SvgAnimation{
		MotionLengths: []float32{0, inf},
		MotionPath:    []float32{0, 0, 1, 0},
	}
	x, y, ang := motionSample(a, 0.5)
	if x != 0 || y != 0 || ang != 0 {
		t.Fatalf("non-finite total: want (0,0,0), got (%f,%f,%f)",
			x, y, ang)
	}

	nan := float32(math.NaN())
	a.MotionLengths = []float32{0, nan}
	x, y, ang = motionSample(a, 0.5)
	if x != 0 || y != 0 || ang != 0 {
		t.Fatalf("NaN total: want (0,0,0), got (%f,%f,%f)", x, y, ang)
	}
}

// Additive=true on animateMotion sums its (x,y) onto the prior
// sandwich state rather than replacing.
func TestComputeSvgAnimations_MotionAdditiveStacks(t *testing.T) {
	base := SvgAnimation{
		Kind:          SvgAnimMotion,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		DurSec:        1,
		Freeze:        true,
		MotionPath:    []float32{100, 200, 100, 200},
		MotionLengths: []float32{0, 0}, // total=0 → stays at (100,200)
	}
	add := SvgAnimation{
		Kind:          SvgAnimMotion,
		GroupID:       "g",
		TargetPathIDs: []uint32{1},
		DurSec:        1,
		Freeze:        true,
		Additive:      true,
		MotionPath:    []float32{0, 0, 6, 8},
		MotionLengths: []float32{0, 10},
	}
	// At frac=1, additive sample = (6,8). Expect (100+6, 200+8).
	st := computeSvgAnimations([]SvgAnimation{base, add}, 1.0, nil)
	if st[1].TransX != 106 || st[1].TransY != 208 {
		t.Fatalf("additive motion want (106,208), got (%f,%f)",
			st[1].TransX, st[1].TransY)
	}
}

// applyDashArrayContrib: linear Lerp between stride-2 keyframes.
// Values [0,150 ; 42,150] at frac=0.5 → [21, 150].
func TestApplyDashArrayContrib_LinearMidpoint(t *testing.T) {
	ov := &SvgAnimAttrOverride{}
	a := &SvgAnimation{
		Kind:            SvgAnimDashArray,
		Values:          []float32{0, 150, 42, 150},
		DashKeyframeLen: 2,
	}
	applyDashArrayContrib(ov, a, 0.5)
	if ov.StrokeDashArrayLen != 2 {
		t.Fatalf("len=%d want 2", ov.StrokeDashArrayLen)
	}
	if ov.StrokeDashArray[0] != 21 || ov.StrokeDashArray[1] != 150 {
		t.Fatalf("got [%v %v] want [21 150]",
			ov.StrokeDashArray[0], ov.StrokeDashArray[1])
	}
	if ov.Mask&SvgAnimMaskStrokeDashArray == 0 {
		t.Fatal("mask bit not set")
	}
}

// applyDashArrayContrib: discrete picks the left keyframe value.
func TestApplyDashArrayContrib_DiscreteSelectsLeft(t *testing.T) {
	ov := &SvgAnimAttrOverride{}
	a := &SvgAnimation{
		Kind:            SvgAnimDashArray,
		Values:          []float32{0, 150, 42, 100},
		DashKeyframeLen: 2,
		CalcMode:        SvgAnimCalcDiscrete,
	}
	// frac in first half of two keyframes → idx=0 → values[0:2].
	applyDashArrayContrib(ov, a, 0.25)
	if ov.StrokeDashArray[0] != 0 || ov.StrokeDashArray[1] != 150 {
		t.Fatalf("got [%v %v] want [0 150]",
			ov.StrokeDashArray[0], ov.StrokeDashArray[1])
	}
}

// applyDashArrayContrib: frac>=1 picks the last keyframe regardless
// of mode (atEnd branch).
func TestApplyDashArrayContrib_AtEndPicksLast(t *testing.T) {
	ov := &SvgAnimAttrOverride{}
	a := &SvgAnimation{
		Kind:            SvgAnimDashArray,
		Values:          []float32{1, 2, 3, 4, 5, 6},
		DashKeyframeLen: 2,
	}
	applyDashArrayContrib(ov, a, 1.0)
	if ov.StrokeDashArray[0] != 5 || ov.StrokeDashArray[1] != 6 {
		t.Fatalf("got [%v %v] want [5 6]",
			ov.StrokeDashArray[0], ov.StrokeDashArray[1])
	}
}

// applyDashArrayContrib: stride=1 (single-value pattern) writes one
// slot and updates Len accordingly.
func TestApplyDashArrayContrib_Stride1(t *testing.T) {
	ov := &SvgAnimAttrOverride{}
	a := &SvgAnimation{
		Kind:            SvgAnimDashArray,
		Values:          []float32{10, 20},
		DashKeyframeLen: 1,
	}
	applyDashArrayContrib(ov, a, 0.5)
	if ov.StrokeDashArrayLen != 1 || ov.StrokeDashArray[0] != 15 {
		t.Fatalf("got len=%d val=%v want len=1 val=15",
			ov.StrokeDashArrayLen, ov.StrokeDashArray[0])
	}
}

// SvgAnimDashOffset replace: writes value, clears AdditiveMask bit.
func TestApplyAnimContrib_DashOffsetReplace(t *testing.T) {
	states := map[uint32]svgAnimState{}
	a := &SvgAnimation{
		Kind: SvgAnimDashOffset, GroupID: "g", TargetPathIDs: []uint32{1},
		Values: []float32{0, -16}, DurSec: 1, Cycle: 1,
	}
	c := &animContrib{anim: a, value: -8}
	applyAnimContrib(c, states, nil)
	st := states[1]
	if st.AttrOverride.Mask&SvgAnimMaskStrokeDashOffset == 0 {
		t.Fatal("mask bit not set")
	}
	if st.AttrOverride.StrokeDashOffset != -8 {
		t.Fatalf("offset=%v want -8", st.AttrOverride.StrokeDashOffset)
	}
	if st.AttrOverride.AdditiveMask&SvgAnimMaskStrokeDashOffset != 0 {
		t.Fatal("AdditiveMask bit must be clear on replace")
	}
}

// SvgAnimDashOffset additive on first touch: stores value AND marks
// AdditiveMask so subsequent writes accumulate, not stomp.
func TestApplyAnimContrib_DashOffsetAdditiveFirst(t *testing.T) {
	states := map[uint32]svgAnimState{}
	a := &SvgAnimation{
		Kind: SvgAnimDashOffset, GroupID: "g", TargetPathIDs: []uint32{1},
		Additive: true,
	}
	c := &animContrib{anim: a, value: 5}
	applyAnimContrib(c, states, nil)
	st := states[1]
	if st.AttrOverride.StrokeDashOffset != 5 {
		t.Fatalf("first additive: offset=%v want 5",
			st.AttrOverride.StrokeDashOffset)
	}
	if st.AttrOverride.AdditiveMask&SvgAnimMaskStrokeDashOffset == 0 {
		t.Fatal("AdditiveMask bit must be set after additive write")
	}
}

// SvgAnimDashOffset additive subsequent: accumulates onto prior.
func TestApplyAnimContrib_DashOffsetAdditiveAccumulates(t *testing.T) {
	states := map[uint32]svgAnimState{}
	a1 := &SvgAnimation{Kind: SvgAnimDashOffset, GroupID: "g",
		TargetPathIDs: []uint32{1}, Additive: true}
	a2 := &SvgAnimation{Kind: SvgAnimDashOffset, GroupID: "g",
		TargetPathIDs: []uint32{1}, Additive: true}
	applyAnimContrib(&animContrib{anim: a1, value: 3}, states, nil)
	applyAnimContrib(&animContrib{anim: a2, value: 4}, states, nil)
	if states[1].AttrOverride.StrokeDashOffset != 7 {
		t.Fatalf("offset=%v want 7",
			states[1].AttrOverride.StrokeDashOffset)
	}
}

// computeSvgAnimationsReuse seeds svgAnimState from baseByGroup so
// non-transform animations leave the author's base xform intact.
func TestComputeSvgAnimationsReuse_SeedsBaseFromGroup(t *testing.T) {
	anims := []SvgAnimation{{
		Kind: SvgAnimAttr, GroupID: "g", TargetPathIDs: []uint32{1},
		AttrName: SvgAttrR,
		Values:   []float32{0, 5}, DurSec: 1, Cycle: 1,
	}}
	base := map[uint32]svgBaseXform{
		1: {TransX: 12, TransY: 12, ScaleX: 2, ScaleY: 2,
			RotAngle: 45},
	}
	st := computeSvgAnimationsReuse(anims, 0.5, nil, nil, base)
	got := st[1]
	if !got.HasXform {
		t.Fatal("HasXform must be true after seeding")
	}
	if got.TransX != 12 || got.TransY != 12 ||
		got.ScaleX != 2 || got.ScaleY != 2 || got.RotAngle != 45 {
		t.Fatalf("not seeded: %+v", got)
	}
	// AttrR animation still applied.
	if got.AttrOverride.Mask&SvgAnimMaskR == 0 {
		t.Fatal("attr animation lost when seeding base")
	}
}

// Without a baseByGroup entry, init falls back to identity scale and
// HasXform stays false (unless a transform anim sets it).
func TestComputeSvgAnimationsReuse_NoBaseLeavesIdentity(t *testing.T) {
	anims := []SvgAnimation{{
		Kind: SvgAnimAttr, GroupID: "g", TargetPathIDs: []uint32{1},
		AttrName: SvgAttrR,
		Values:   []float32{0, 5}, DurSec: 1, Cycle: 1,
	}}
	st := computeSvgAnimationsReuse(anims, 0.5, nil, nil, nil)
	got := st[1]
	if got.HasXform {
		t.Fatal("HasXform should be false when no base + no xform anim")
	}
	if got.ScaleX != 1 || got.ScaleY != 1 {
		t.Fatalf("scale not identity: (%v,%v)", got.ScaleX, got.ScaleY)
	}
}
