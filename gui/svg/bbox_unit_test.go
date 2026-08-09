package svg

import "testing"

func TestBBoxFromSegments_EmptyReturnsUnset(t *testing.T) {
	b := bboxFromSegments(nil)
	if b.Set {
		t.Error("empty segs should yield unset bbox")
	}
}

func TestBBoxFromSegments_SingleMoveTo(t *testing.T) {
	b := bboxFromSegments([]pathSegment{
		{Cmd: cmdMoveTo, Points: []float32{3, 4}},
	})
	if !b.Set || b.MinX != 3 || b.MinY != 4 || b.MaxX != 3 || b.MaxY != 4 {
		t.Errorf("got %+v want zero-extent at (3,4)", b)
	}
}

func TestBBoxFromSegments_MalformedPointsIgnored(t *testing.T) {
	// Truncated point lists must not panic and must not extend bbox.
	b := bboxFromSegments([]pathSegment{
		{Cmd: cmdMoveTo, Points: []float32{0, 0}},
		{Cmd: cmdLineTo, Points: []float32{1}},           // truncated
		{Cmd: cmdQuadTo, Points: []float32{1, 2, 3}},     // truncated
		{Cmd: cmdCubicTo, Points: []float32{1, 2, 3, 4}}, // truncated
		{Cmd: cmdClose, Points: nil},
	})
	if !b.Set || b.MaxX != 0 || b.MaxY != 0 {
		t.Errorf("got %+v want bbox unchanged from origin", b)
	}
}

func TestBBoxFromSegments_QuadIncludesControlPoint(t *testing.T) {
	// Control (10,20) lies outside the line; bbox must enclose it.
	b := bboxFromSegments([]pathSegment{
		{Cmd: cmdMoveTo, Points: []float32{0, 0}},
		{Cmd: cmdQuadTo, Points: []float32{10, 20, 5, 0}},
	})
	if b.MaxX != 10 || b.MaxY != 20 {
		t.Errorf("got %+v want max (10,20)", b)
	}
}

func TestBBoxFromSegments_NegativeCoords(t *testing.T) {
	b := bboxFromSegments([]pathSegment{
		{Cmd: cmdMoveTo, Points: []float32{-5, -7}},
		{Cmd: cmdLineTo, Points: []float32{2, 3}},
	})
	if b.MinX != -5 || b.MinY != -7 || b.MaxX != 2 || b.MaxY != 3 {
		t.Errorf("got %+v want negative-bounded box", b)
	}
}

func TestExtendBBox_FirstPointInitializes(t *testing.T) {
	b := extendBBox(bbox{}, 7, 9)
	if !b.Set || b.MinX != 7 || b.MaxX != 7 || b.MinY != 9 || b.MaxY != 9 {
		t.Errorf("got %+v want point bbox at (7,9)", b)
	}
}

func TestUnionBbox_BothUnsetYieldsUnset(t *testing.T) {
	out := unionBbox(bbox{}, bbox{})
	if out.Set {
		t.Errorf("got %+v want unset", out)
	}
}

func TestUnionBbox_UnsetSideYieldsOther(t *testing.T) {
	a := bbox{MinX: 1, MinY: 2, MaxX: 3, MaxY: 4, Set: true}
	if got := unionBbox(bbox{}, a); got != a {
		t.Errorf("unset+a got %+v want %+v", got, a)
	}
	if got := unionBbox(a, bbox{}); got != a {
		t.Errorf("a+unset got %+v want %+v", got, a)
	}
}

func TestUnionBbox_EnclosesBoth(t *testing.T) {
	a := bbox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10, Set: true}
	b := bbox{MinX: -5, MinY: 5, MaxX: 7, MaxY: 20, Set: true}
	out := unionBbox(a, b)
	want := bbox{MinX: -5, MinY: 0, MaxX: 10, MaxY: 20, Set: true}
	if out != want {
		t.Errorf("got %+v want %+v", out, want)
	}
}

func TestUnionBbox_OrderingInvariant(t *testing.T) {
	a := bbox{MinX: 1, MinY: 2, MaxX: 5, MaxY: 6, Set: true}
	b := bbox{MinX: -3, MinY: 4, MaxX: 8, MaxY: 9, Set: true}
	if unionBbox(a, b) != unionBbox(b, a) {
		t.Error("union not commutative")
	}
}

func TestUnionPathBboxes_EmptySliceUnset(t *testing.T) {
	if got := unionPathBboxes(nil); got.Set {
		t.Errorf("nil slice got %+v want unset", got)
	}
	if got := unionPathBboxes([]vectorPath{}); got.Set {
		t.Errorf("empty slice got %+v want unset", got)
	}
}

func TestUnionPathBboxes_AllUnsetYieldsUnset(t *testing.T) {
	paths := []vectorPath{{}, {}, {}}
	if got := unionPathBboxes(paths); got.Set {
		t.Errorf("all unset got %+v want unset", got)
	}
}

func TestUnionPathBboxes_SkipsUnsetMembers(t *testing.T) {
	paths := []vectorPath{
		{},
		{bbox: bbox{MinX: 1, MinY: 2, MaxX: 3, MaxY: 4, Set: true}},
		{},
		{bbox: bbox{MinX: -1, MinY: 0, MaxX: 5, MaxY: 6, Set: true}},
	}
	got := unionPathBboxes(paths)
	want := bbox{MinX: -1, MinY: 0, MaxX: 5, MaxY: 6, Set: true}
	if got != want {
		t.Errorf("got %+v want %+v", got, want)
	}
}
