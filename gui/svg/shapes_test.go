package svg

import (
	"testing"
)

// --- attrFloat ---

func TestShapesAttrFloatPresent(t *testing.T) {
	v := attrFloat(`<rect x="12.5">`, "x", 0)
	if f32Abs(v-12.5) > 1e-5 {
		t.Fatalf("expected 12.5, got %f", v)
	}
}

func TestShapesAttrFloatMissing(t *testing.T) {
	v := attrFloat(`<rect>`, "x", 99)
	if v != 99 {
		t.Fatalf("expected fallback 99, got %f", v)
	}
}

// --- parsePathElement ---

func TestShapesParsePathElementValid(t *testing.T) {
	vp, ok := parsePathElement(`<path d="M 0 0 L 10 10">`)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(vp.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(vp.Segments))
	}
}

func TestShapesParsePathElementMissingD(t *testing.T) {
	_, ok := parsePathElement(`<path fill="red">`)
	if ok {
		t.Fatalf("expected ok=false for missing d")
	}
}

// --- parseRectElement ---

func TestShapesParseRectElement(t *testing.T) {
	vp, ok := parseRectElement(`<rect x="0" y="0" width="10" height="20">`)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	// Simple rect: MoveTo, 3×LineTo, Close = 5 segments
	if len(vp.Segments) != 5 {
		t.Fatalf("expected 5 segments for simple rect, got %d", len(vp.Segments))
	}
}

func TestShapesParseRectElementRounded(t *testing.T) {
	vp, ok := parseRectElement(`<rect x="0" y="0" width="100" height="50" rx="5">`)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	// Rounded rect includes CubicTo segments from arcs
	hasCubic := false
	for _, seg := range vp.Segments {
		if seg.Cmd == cmdCubicTo {
			hasCubic = true
			break
		}
	}
	if !hasCubic {
		t.Fatalf("rounded rect should include CubicTo segments")
	}
}

func TestShapesParseRectElementNoWidth(t *testing.T) {
	_, ok := parseRectElement(`<rect x="0" y="0" height="20">`)
	if ok {
		t.Fatalf("expected ok=false for missing width")
	}
}

// --- parseCircleElement ---

func TestShapesParseCircleElement(t *testing.T) {
	vp, ok := parseCircleElement(`<circle cx="50" cy="50" r="25">`)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	// Ellipse path: MoveTo + 4×CubicTo + Close = 6 segments
	if len(vp.Segments) != 6 {
		t.Fatalf("expected 6 segments for circle, got %d", len(vp.Segments))
	}
}

func TestShapesParseCircleElementMissingR(t *testing.T) {
	_, ok := parseCircleElement(`<circle cx="50" cy="50">`)
	if ok {
		t.Fatalf("expected ok=false for missing r")
	}
}

// --- parseEllipseElement ---

func TestShapesParseEllipseElement(t *testing.T) {
	vp, ok := parseEllipseElement(`<ellipse cx="50" cy="50" rx="30" ry="20">`)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(vp.Segments) != 6 {
		t.Fatalf("expected 6 segments for ellipse, got %d", len(vp.Segments))
	}
}

// --- parsePolygonElement ---

func TestShapesParsePolygonElementClosed(t *testing.T) {
	vp, ok := parsePolygonElement(`<polygon points="0,0 10,0 10,10">`, true)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	// polygon with close: MoveTo + 2×LineTo + Close = 4
	if vp.Segments[len(vp.Segments)-1].Cmd != cmdClose {
		t.Fatalf("polygon should end with Close")
	}
}

func TestShapesParsePolygonElementPolyline(t *testing.T) {
	vp, ok := parsePolygonElement(`<polyline points="0,0 10,0 10,10">`, false)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if vp.Segments[len(vp.Segments)-1].Cmd == cmdClose {
		t.Fatalf("polyline should not end with Close")
	}
}

func TestShapesParsePolygonElementNoPoints(t *testing.T) {
	_, ok := parsePolygonElement(`<polygon>`, true)
	if ok {
		t.Fatalf("expected ok=false for missing points")
	}
}

// --- parseLineElement ---

func TestShapesParseLineElement(t *testing.T) {
	vp, ok := parseLineElement(`<line x1="0" y1="0" x2="10" y2="20">`)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(vp.Segments) != 2 {
		t.Fatalf("expected 2 segments (MoveTo+LineTo), got %d", len(vp.Segments))
	}
	if vp.Segments[0].Cmd != cmdMoveTo || vp.Segments[1].Cmd != cmdLineTo {
		t.Fatalf("expected MoveTo+LineTo, got %d+%d",
			vp.Segments[0].Cmd, vp.Segments[1].Cmd)
	}
}

func TestShapesParseLineElementSamePoint(t *testing.T) {
	_, ok := parseLineElement(`<line x1="5" y1="5" x2="5" y2="5">`)
	if ok {
		t.Fatalf("expected ok=false for zero-length line")
	}
}

// --- ellipseToPath ---

func TestShapesEllipseToPath(t *testing.T) {
	s := parseElementStyle(`<ellipse>`)
	vp := ellipseToPath(50, 50, 30, 20, `<ellipse>`, "", s)
	hasCubic := false
	for _, seg := range vp.Segments {
		if seg.Cmd == cmdCubicTo {
			hasCubic = true
			break
		}
	}
	if !hasCubic {
		t.Fatalf("ellipseToPath should produce CubicTo segments")
	}
}
