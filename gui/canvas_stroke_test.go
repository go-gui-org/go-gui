package gui

import (
	"math"
	"slices"
	"testing"
)

// Canvas stroke styles (#905): the zero style draws exactly what the
// unstyled call draws, and a non-zero style tessellates through the
// shared stroker. These tests pin both halves, plus the two stroker
// corrections the shared core carries: the round-join fan sweeps the
// outer wedge (not the inner disc), and the square cap extends along
// the path (not sideways).

// collectStrokeTris draws into a fresh context and flattens every
// batch into one triangle list.
func collectStrokeTris(draw func(dc *DrawContext)) []float32 {
	dc := DrawContext{Width: 200, Height: 200}
	draw(&dc)
	var out []float32
	for _, b := range dc.Batches() {
		out = append(out, b.Triangles...)
	}
	return out
}

// strokeTriContains reports whether (px, py) falls inside the
// triangle. Sign-based: a point is inside when it sits on the same
// side of all three edges.
func strokeTriContains(px, py float32, tri []float32) bool {
	ax, ay := tri[0]-px, tri[1]-py
	bx, by := tri[2]-px, tri[3]-py
	cx, cy := tri[4]-px, tri[5]-py
	d1 := ax*by - ay*bx
	d2 := bx*cy - by*cx
	d3 := cx*ay - cy*ax
	return (d1 >= 0 && d2 >= 0 && d3 >= 0) ||
		(d1 <= 0 && d2 <= 0 && d3 <= 0)
}

// strokeCovers reports whether any triangle in tris covers (px, py).
func strokeCovers(tris []float32, px, py float32) bool {
	for i := 0; i+5 < len(tris); i += 6 {
		if strokeTriContains(px, py, tris[i:i+6]) {
			return true
		}
	}
	return false
}

// strokeBounds returns the bounding box of tris.
func strokeBounds(tris []float32) (float32, float32, float32, float32) {
	minX, maxX := tris[0], tris[0]
	minY, maxY := tris[1], tris[1]
	for i := 2; i+1 < len(tris); i += 2 {
		minX, maxX = min(minX, tris[i]), max(maxX, tris[i])
		minY, maxY = min(minY, tris[i+1]), max(maxY, tris[i+1])
	}
	return minX, maxX, minY, maxY
}

// --- Zero style matches legacy ---

func TestStyledZeroMatchesLegacy(t *testing.T) {
	angled := []float32{0, 0, 50, 30, 100, 10}
	vshape := []float32{0, 0, 50, 50, 100, 0}
	cases := []struct {
		name   string
		legacy func(dc *DrawContext)
		styled func(dc *DrawContext)
	}{
		{
			"Line",
			func(dc *DrawContext) { dc.Line(10, 20, 50, 40, Red, 4) },
			func(dc *DrawContext) {
				dc.LineStyled(10, 20, 50, 40, Red, 4, StrokeStyle{})
			},
		},
		{
			"Polyline",
			func(dc *DrawContext) { dc.Polyline(angled, Green, 4) },
			func(dc *DrawContext) {
				dc.PolylineStyled(angled, Green, 4, StrokeStyle{})
			},
		},
		{
			"PolylineJoined",
			func(dc *DrawContext) { dc.PolylineJoined(vshape, Blue, 4) },
			func(dc *DrawContext) {
				dc.PolylineJoinedStyled(vshape, Blue, 4, StrokeStyle{})
			},
		},
		{
			"Arc",
			func(dc *DrawContext) { dc.Arc(60, 60, 30, 20, 0, 2, Red, 4) },
			func(dc *DrawContext) {
				dc.ArcStyled(60, 60, 30, 20, 0, 2, Red, 4, StrokeStyle{})
			},
		},
		{
			"Circle",
			func(dc *DrawContext) { dc.Circle(60, 60, 25, Green, 4) },
			func(dc *DrawContext) {
				dc.CircleStyled(60, 60, 25, Green, 4, StrokeStyle{})
			},
		},
		{
			"RoundedRect",
			func(dc *DrawContext) { dc.RoundedRect(10, 10, 60, 40, 8, Blue, 4) },
			func(dc *DrawContext) {
				dc.RoundedRectStyled(10, 10, 60, 40, 8, Blue, 4,
					StrokeStyle{})
			},
		},
		{
			"QuadBezier",
			func(dc *DrawContext) { dc.QuadBezier(0, 0, 50, 100, 100, 0, Red, 3) },
			func(dc *DrawContext) {
				dc.QuadBezierStyled(0, 0, 50, 100, 100, 0, Red, 3,
					StrokeStyle{})
			},
		},
		{
			"CubicBezier",
			func(dc *DrawContext) {
				dc.CubicBezier(0, 0, 30, 100, 70, 100, 100, 0, Blue, 3)
			},
			func(dc *DrawContext) {
				dc.CubicBezierStyled(0, 0, 30, 100, 70, 100, 100, 0,
					Blue, 3, StrokeStyle{})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := collectStrokeTris(tc.legacy)
			got := collectStrokeTris(tc.styled)
			if !slices.Equal(want, got) {
				t.Errorf("zero style drew %d floats, legacy drew %d",
					len(got), len(want))
			}
		})
	}
}

// --- Caps ---

// A 2-point line is one quad (12 floats). Each round cap fans 8
// triangles (48 floats); each square cap adds one quad (12 floats).
func TestLineStyledRoundCaps(t *testing.T) {
	tris := collectStrokeTris(func(dc *DrawContext) {
		dc.LineStyled(10, 20, 50, 20, Red, 6,
			StrokeStyle{Cap: StrokeRoundCap})
	})
	if len(tris) != 108 {
		t.Fatalf("floats = %d, want 108 (12 quad + 2x48 cap)", len(tris))
	}
	if !f32AllFinite(tris) {
		t.Fatal("non-finite triangle output")
	}
	minX, maxX, minY, maxY := strokeBounds(tris)
	// Half width 3 past each end, half width across.
	if minX != 7 || maxX != 53 || minY != 17 || maxY != 23 {
		t.Errorf("bounds x=[%v,%v] y=[%v,%v], want x=[7,53] y=[17,23]",
			minX, maxX, minY, maxY)
	}
}

func TestLineStyledSquareCaps(t *testing.T) {
	tris := collectStrokeTris(func(dc *DrawContext) {
		dc.LineStyled(10, 20, 50, 20, Red, 6,
			StrokeStyle{Cap: StrokeSquareCap})
	})
	if len(tris) != 36 {
		t.Fatalf("floats = %d, want 36 (12 quad + 2x12 cap)", len(tris))
	}
	minX, maxX, minY, maxY := strokeBounds(tris)
	// The extension runs along the path, not sideways: the y range
	// stays one half width, the x range grows by one each end.
	if minX != 7 || maxX != 53 || minY != 17 || maxY != 23 {
		t.Errorf("bounds x=[%v,%v] y=[%v,%v], want x=[7,53] y=[17,23]",
			minX, maxX, minY, maxY)
	}
}

// --- Joins ---

func TestPolylineJoinedStyledRoundJoinCoversGap(t *testing.T) {
	vshape := []float32{0, 0, 50, 50, 100, 0}
	round := collectStrokeTris(func(dc *DrawContext) {
		dc.PolylineJoinedStyled(vshape, Blue, 4,
			StrokeStyle{Join: StrokeRoundJoin})
	})
	bevel := collectStrokeTris(func(dc *DrawContext) {
		dc.PolylineJoinedStyled(vshape, Blue, 4,
			StrokeStyle{Join: StrokeBevelJoin})
	})
	if len(round) <= len(bevel) {
		t.Errorf("round join floats = %d, want more than bevel %d",
			len(round), len(bevel))
	}
	// Below the vertex, inside the stroke, outside both quads: only
	// the join fan covers this point. A fan sweeping the inner disc
	// leaves it open.
	if !strokeCovers(round, 50, 51.5) {
		t.Error("round join leaves the outer corner open")
	}
	if !f32AllFinite(round) {
		t.Error("non-finite triangle output")
	}
}

func TestPolylineJoinedStyledBevelJoin(t *testing.T) {
	tris := collectStrokeTris(func(dc *DrawContext) {
		dc.PolylineJoinedStyled([]float32{0, 0, 50, 50, 100, 0},
			Blue, 4, StrokeStyle{Join: StrokeBevelJoin})
	})
	// Two quads (24) plus one bevel triangle (6).
	if len(tris) != 30 {
		t.Errorf("floats = %d, want 30", len(tris))
	}
}

// A miter join through the shared stroker adds the same wedge the
// legacy path draws, plus the caps the style asks for: two quads
// (24), one two-triangle miter fill (12) and two square caps (24).
func TestPolylineJoinedStyledMiterJoin(t *testing.T) {
	tris := collectStrokeTris(func(dc *DrawContext) {
		dc.PolylineJoinedStyled([]float32{0, 0, 50, 50, 100, 0},
			Blue, 4, StrokeStyle{Cap: StrokeSquareCap, Join: StrokeMiterJoin})
	})
	if len(tris) != 60 {
		t.Errorf("floats = %d, want 60", len(tris))
	}
	if !f32AllFinite(tris) {
		t.Error("non-finite triangle output")
	}
}

// --- Arcs and closed paths ---

func TestArcStyledCapDifference(t *testing.T) {
	round := collectStrokeTris(func(dc *DrawContext) {
		dc.ArcStyled(60, 60, 30, 20, 0, 2, Red, 4,
			StrokeStyle{Cap: StrokeRoundCap})
	})
	square := collectStrokeTris(func(dc *DrawContext) {
		dc.ArcStyled(60, 60, 30, 20, 0, 2, Red, 4,
			StrokeStyle{Cap: StrokeSquareCap})
	})
	// Same quads and joins; only the caps differ: 2x48 round fan
	// against 2x12 square quad.
	if len(round)-len(square) != 72 {
		t.Errorf("round-square = %d, want 72", len(round)-len(square))
	}
}

// A full sweep closes the path, so caps must not add geometry.
func TestCircleStyledIgnoresCaps(t *testing.T) {
	round := collectStrokeTris(func(dc *DrawContext) {
		dc.CircleStyled(60, 60, 25, Green, 4,
			StrokeStyle{Cap: StrokeRoundCap})
	})
	square := collectStrokeTris(func(dc *DrawContext) {
		dc.CircleStyled(60, 60, 25, Green, 4,
			StrokeStyle{Cap: StrokeSquareCap})
	})
	if !slices.Equal(round, square) {
		t.Errorf("closed path: round drew %d floats, square drew %d",
			len(round), len(square))
	}
}

// --- Hardening ---

func TestStyledRejectsBadInput(t *testing.T) {
	nan := float32(math.NaN())
	style := StrokeStyle{Cap: StrokeRoundCap, Join: StrokeRoundJoin}
	calls := []struct {
		name string
		draw func(dc *DrawContext)
	}{
		{"Line zero width", func(dc *DrawContext) {
			dc.LineStyled(0, 0, 10, 10, Red, 0, style)
		}},
		{"Line NaN", func(dc *DrawContext) {
			dc.LineStyled(nan, 0, 10, 10, Red, 2, style)
		}},
		{"Polyline short", func(dc *DrawContext) {
			dc.PolylineStyled([]float32{0, 0}, Red, 2, style)
		}},
		{"Polyline NaN", func(dc *DrawContext) {
			dc.PolylineStyled([]float32{0, 0, nan, nan}, Red, 2, style)
		}},
		{"PolylineJoined NaN", func(dc *DrawContext) {
			dc.PolylineJoinedStyled([]float32{0, 0, 1, 1, nan, 0},
				Red, 2, style)
		}},
		{"Arc zero width", func(dc *DrawContext) {
			dc.ArcStyled(5, 5, 4, 4, 0, 1, Red, 0, style)
		}},
		{"Circle NaN", func(dc *DrawContext) {
			dc.CircleStyled(nan, 5, 4, Red, 2, style)
		}},
		{"QuadBezier NaN", func(dc *DrawContext) {
			dc.QuadBezierStyled(0, 0, nan, 1, 2, 2, Red, 2, style)
		}},
		{"CubicBezier NaN", func(dc *DrawContext) {
			dc.CubicBezierStyled(0, 0, 1, 1, 2, 2, nan, 2, Red, 2, style)
		}},
		{"RoundedRect zero width", func(dc *DrawContext) {
			dc.RoundedRectStyled(0, 0, 10, 10, 2, Red, 0, style)
		}},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			dc := DrawContext{Width: 100, Height: 100}
			tc.draw(&dc)
			if len(dc.Batches()) != 0 {
				t.Errorf("batches = %d, want 0", len(dc.Batches()))
			}
		})
	}
}

// --- Recorders ---

// strokeCapture is an xformCapture that also speaks the stroke
// extension, so the styled path and its transform baking can be
// checked call by call.
type strokeCapture struct {
	xformCapture
	styledPoly   []float32
	styledJoined []float32
	styledWidth  float32
	styledStyle  StrokeStyle
	styledCalls  int
	circleCalls  int
}

func (r *strokeCapture) PolylineStyled(p []float32, _ Color, w float32,
	s StrokeStyle) {
	r.styledPoly = append(r.styledPoly[:0], p...)
	r.styledWidth = w
	r.styledStyle = s
	r.styledCalls++
}

func (r *strokeCapture) PolylineJoinedStyled(p []float32, _ Color, w float32,
	s StrokeStyle) {
	r.styledJoined = append(r.styledJoined[:0], p...)
	r.styledWidth = w
	r.styledStyle = s
	r.styledCalls++
}

// The remaining extension methods only need to exist so the stub
// satisfies the interface; the tests below never call them.
func (r *strokeCapture) LineStyled(_, _, _, _ float32, _ Color, _ float32,
	s StrokeStyle) {
	r.styledStyle = s
	r.styledCalls++
}

func (r *strokeCapture) ArcStyled(_, _, _, _, _, _ float32, _ Color,
	_ float32, s StrokeStyle) {
	r.styledStyle = s
	r.styledCalls++
}

func (r *strokeCapture) CircleStyled(_, _, _ float32, _ Color, _ float32,
	s StrokeStyle) {
	r.styledStyle = s
	r.styledCalls++
	r.circleCalls++
}

func (r *strokeCapture) RoundedRectStyled(_, _, _, _, _ float32, _ Color,
	_ float32, s StrokeStyle) {
	r.styledStyle = s
	r.styledCalls++
}

func (r *strokeCapture) QuadBezierStyled(_, _, _, _, _, _ float32, _ Color,
	_ float32, s StrokeStyle) {
	r.styledStyle = s
	r.styledCalls++
}

func (r *strokeCapture) CubicBezierStyled(_, _, _, _, _, _, _, _ float32,
	_ Color, _ float32, s StrokeStyle) {
	r.styledStyle = s
	r.styledCalls++
}

// A recorder without the extension still gets the stroke, as the
// unstyled primitive. xformCapture has no Styled methods, so every
// Styled call below must arrive as its legacy twin.
func TestStyledRecorderFallback(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	style := StrokeStyle{Cap: StrokeRoundCap, Join: StrokeRoundJoin}

	dc.LineStyled(0, 0, 5, 5, Blue, 3, style)
	// LineStyled delegates to PolylineStyled, so the fallback
	// arrives as a two-point Polyline.
	if rec.poly == nil {
		t.Error("LineStyled did not fall back to Polyline")
	}
	pts := []float32{0, 0, 10, 0, 10, 10}
	dc.PolylineStyled(pts, Blue, 2, style)
	if rec.poly == nil {
		t.Error("PolylineStyled did not fall back to Polyline")
	}
	dc.PolylineJoinedStyled(pts, Blue, 2, style)
	if rec.joined == nil {
		t.Error("PolylineJoinedStyled did not fall back to PolylineJoined")
	}
	if len(dc.Batches()) != 0 {
		t.Errorf("batches = %d, want 0 (recorder owns it)",
			len(dc.Batches()))
	}
}

// The extension path bakes the transform and keeps the style: the
// style holds no lengths, so it passes through untouched.
func TestStyledRecorderExtension(t *testing.T) {
	rec := &strokeCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Translate(10, 20)
	dc.ScaleBy(2, 2)

	style := StrokeStyle{Cap: StrokeRoundCap, Join: StrokeRoundJoin}
	pts := []float32{0, 0, 5, 5}
	dc.PolylineStyled(pts, Blue, 3, style)
	if rec.styledCalls != 1 {
		t.Fatalf("styled calls = %d, want 1", rec.styledCalls)
	}
	if !slices.Equal(rec.styledPoly, []float32{10, 20, 20, 30}) {
		t.Errorf("PolylineStyled = %v, want [10 20 20 30]", rec.styledPoly)
	}
	if rec.styledWidth != 6 {
		t.Errorf("width = %v, want 6", rec.styledWidth)
	}
	if rec.styledStyle != style {
		t.Errorf("style = %+v, want %+v", rec.styledStyle, style)
	}
	if !slices.Equal(pts, []float32{0, 0, 5, 5}) {
		t.Errorf("caller slice mutated: %v", pts)
	}
}

// A rotated arc has no axis-aligned form, so it lowers to a styled
// join: the style must survive the lowering, not fall back to the
// unstyled primitive.
func TestStyledArcRotatedLowersToStyledJoin(t *testing.T) {
	rec := &strokeCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Translate(10, 0)
	dc.Rotate(0.5)

	style := StrokeStyle{Cap: StrokeRoundCap, Join: StrokeRoundJoin}
	dc.ArcStyled(0, 0, 20, 20, 0, 1, Blue, 2, style)
	if rec.styledCalls != 1 {
		t.Fatalf("styled calls = %d, want 1", rec.styledCalls)
	}
	if len(rec.styledJoined) < 4 {
		t.Errorf("lowered arc points = %d floats, want a polyline",
			len(rec.styledJoined))
	}
	if rec.styledStyle != style {
		t.Errorf("style = %+v, want %+v", rec.styledStyle, style)
	}
}

// The decorator implements every extension, so strokeRecorder must
// assert against the inner recorder — otherwise a plain recorder
// would look styled and lose the unstyled degradation.
func TestStrokeRecorderUnwrapKeepsDegradation(t *testing.T) {
	plain := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(plain)
	dc.ScaleBy(2, 2)
	if _, ok := dc.strokeRecorder(); ok {
		t.Fatal("a plain recorder was reported as a DrawStrokeRecorder")
	}
}

// --- Core guards ---

func TestAppendStrokeTrisGuards(t *testing.T) {
	var tris []float32
	scratch := make([]float32, 0, 16)
	scratch = AppendStrokeTris(&tris, nil, 1,
		StrokeRoundCap, StrokeRoundJoin, scratch)
	scratch = AppendStrokeTris(&tris, []float32{0, 0}, 1,
		StrokeRoundCap, StrokeRoundJoin, scratch)
	scratch = AppendStrokeTris(&tris, []float32{0, 0, 1}, 1,
		StrokeRoundCap, StrokeRoundJoin, scratch)
	if len(tris) != 0 {
		t.Errorf("degenerate input drew %d floats, want 0", len(tris))
	}
	if scratch == nil {
		t.Error("scratch was not threaded back")
	}
}

// circleCapture is a plain recorder that notes a stroked Circle, so
// the CircleStyled fallback can be told apart from an Arc.
type circleCapture struct {
	xformCapture
	circles int
}

func (r *circleCapture) Circle(_, _, _ float32, _ Color, _ float32) {
	r.circles++
}

// A recorder sees CircleStyled as a circle, as Circle does: through
// the extension when it has one, else as the unstyled Circle. It must
// not see the ArcStyled the batch path lowers it to.
func TestCircleStyledRecorderKeepsCircle(t *testing.T) {
	style := StrokeStyle{Join: StrokeRoundJoin}

	styled := &strokeCapture{}
	var dc DrawContext
	dc.SetRecorder(styled)
	dc.CircleStyled(5, 5, 4, Blue, 2, style)
	if styled.circleCalls != 1 || styled.styledCalls != 1 {
		t.Errorf("circle, styled calls = %d, %d, want 1, 1",
			styled.circleCalls, styled.styledCalls)
	}

	plain := &circleCapture{}
	var dc2 DrawContext
	dc2.SetRecorder(plain)
	dc2.CircleStyled(5, 5, 4, Blue, 2, style)
	if plain.circles != 1 {
		t.Errorf("fallback Circle calls = %d, want 1", plain.circles)
	}
}

// AppendStrokeTris is exported, so it screens its own half width.
func TestAppendStrokeTrisRejectsBadHalfWidth(t *testing.T) {
	pts := []float32{0, 0, 10, 0, 10, 10}
	for _, hw := range []float32{0, -1, float32(math.NaN()),
		float32(math.Inf(1))} {
		var tris []float32
		AppendStrokeTris(&tris, pts, hw, StrokeRoundCap,
			StrokeRoundJoin, nil)
		if len(tris) != 0 {
			t.Errorf("halfW %v drew %d floats, want 0", hw, len(tris))
		}
	}
}
