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

func TestShapesParseRectElementNegative(t *testing.T) {
	// Negative sizes are errors that disable rendering. Zero is
	// degenerate but preserved for animated placeholders, so it
	// still parses here; the tessellator drops it when static.
	for _, elem := range []string{
		`<rect x="0" y="0" width="-10" height="20">`,
		`<rect x="0" y="0" width="10" height="-5">`,
	} {
		if _, ok := parseRectElement(elem); ok {
			t.Fatalf("expected ok=false for %s", elem)
		}
	}
	if _, ok := parseRectElement(`<rect x="0" y="0" width="0" height="20">`); !ok {
		t.Fatalf("expected ok=true for zero width (animated placeholder)")
	}
}

func TestShapesParseRectElementNegativeRadius(t *testing.T) {
	// SVG 2: a negative rx/ry is an invalid value, ignored as auto. The
	// rect still renders; only negative width/height disables it.
	vp, ok := parseRectElement(`<rect x="0" y="0" width="10" height="20" rx="-2">`)
	if !ok {
		t.Fatalf("negative rx must not drop the rect")
	}
	for _, seg := range vp.Segments {
		if seg.Cmd != cmdMoveTo && seg.Cmd != cmdLineTo && seg.Cmd != cmdClose {
			t.Fatalf("both radii auto: want square corners, got cmd %v", seg.Cmd)
		}
	}
	// Negative ry is auto, so it takes rx and the corners round.
	vp, ok = parseRectElement(`<rect x="0" y="0" width="10" height="20" rx="3" ry="-1">`)
	if !ok {
		t.Fatalf("negative ry must not drop the rect")
	}
	rounded := false
	for _, seg := range vp.Segments {
		if seg.Cmd != cmdMoveTo && seg.Cmd != cmdLineTo && seg.Cmd != cmdClose {
			rounded = true
		}
	}
	if !rounded {
		t.Fatalf("ry auto should copy rx=3 and round the corners")
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

func TestShapesParseCircleElementNegativeR(t *testing.T) {
	if _, ok := parseCircleElement(`<circle cx="50" cy="50" r="-3">`); ok {
		t.Fatalf("expected ok=false for negative r")
	}
	// Zero is degenerate but preserved for animated placeholders.
	if _, ok := parseCircleElement(`<circle cx="50" cy="50" r="0">`); !ok {
		t.Fatalf("expected ok=true for zero r (animated placeholder)")
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

func TestShapesParseEllipseElementNegativeRadius(t *testing.T) {
	// SVG 2 §10.4: a negative rx/ry is ignored as a parse error, which
	// leaves it auto, and auto takes the other radius: a circle.
	vp, ok := parseEllipseElement(`<ellipse cx="50" cy="50" rx="-1" ry="20">`)
	if !ok {
		t.Fatalf("negative rx must read as auto, not drop the ellipse")
	}
	if vp.Primitive.RX != 20 || vp.Primitive.RY != 20 {
		t.Fatalf("rx auto: got RX=%v RY=%v, want 20/20", vp.Primitive.RX, vp.Primitive.RY)
	}
	vp, ok = parseEllipseElement(`<ellipse cx="50" cy="50" rx="7" ry="-3">`)
	if !ok || vp.Primitive.RY != 7 {
		t.Fatalf("ry auto: ok=%v RY=%v, want true/7", ok, vp.Primitive.RY)
	}
	// Both auto disables rendering.
	if _, ok = parseEllipseElement(`<ellipse cx="50" cy="50" rx="-1" ry="-2">`); ok {
		t.Fatalf("both radii negative must not render")
	}
	// Zero is degenerate but preserved for animated placeholders.
	if _, ok := parseEllipseElement(`<ellipse cx="50" cy="50" rx="0" ry="20">`); !ok {
		t.Fatalf("expected ok=true for zero rx (animated placeholder)")
	}
}

func TestShapesParseEllipseElementMissingRadius(t *testing.T) {
	// SVG 2 §10.4: a missing rx/ry is auto, and auto takes the other
	// radius. SVG 1.1 dropped the element instead.
	vp, ok := parseEllipseElement(`<ellipse cx="50" cy="50" ry="20">`)
	if !ok || vp.Primitive.RX != 20 || vp.Primitive.RY != 20 {
		t.Fatalf("missing rx: ok=%v RX=%v RY=%v, want true/20/20",
			ok, vp.Primitive.RX, vp.Primitive.RY)
	}
	vp, ok = parseEllipseElement(`<ellipse cx="50" cy="50" rx="8">`)
	if !ok || vp.Primitive.RY != 8 {
		t.Fatalf("missing ry: ok=%v RY=%v, want true/8", ok, vp.Primitive.RY)
	}
	// A missing radius and a negative one are both auto: both auto
	// disables rendering.
	for _, elem := range []string{
		`<ellipse cx="50" cy="50">`,
		`<ellipse cx="50" cy="50" rx="-4">`,
	} {
		if _, ok = parseEllipseElement(elem); ok {
			t.Fatalf("both radii auto must not render: %s", elem)
		}
	}
}

func TestShapesParseEllipseElementUnparseableRadius(t *testing.T) {
	// SVG 2: an invalid radius is ignored as a parse error, leaving it
	// auto, so it takes the other radius instead of reading as zero.
	for _, elem := range []string{
		`<ellipse cx="50" cy="50" rx="abc" ry="20">`,
		`<ellipse cx="50" cy="50" rx="" ry="20">`,
		`<ellipse cx="50" cy="50" rx="NaN" ry="20">`,
		`<ellipse cx="50" cy="50" rx="1e500" ry="20">`,
	} {
		vp, ok := parseEllipseElement(elem)
		if !ok || vp.Primitive.RX != 20 {
			t.Fatalf("%s: ok=%v RX=%v, want true/20", elem, ok, vp.Primitive.RX)
		}
	}
	// A px suffix is a valid user-unit length, not a parse error.
	vp, ok := parseEllipseElement(`<ellipse cx="50" cy="50" rx="12px" ry="20">`)
	if !ok || vp.Primitive.RX != 12 {
		t.Fatalf("px suffix: ok=%v RX=%v, want true/12", ok, vp.Primitive.RX)
	}
	if _, ok = parseEllipseElement(`<ellipse cx="50" cy="50" rx="x" ry="y">`); ok {
		t.Fatalf("both radii unparseable must not render")
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
	vp := ellipseToPath(50, 50, 30, 20, "", s)
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

func TestShapesInvalidFillFallsBackToInherit(t *testing.T) {
	vp, ok := parseRectElement(`<rect x="0" y="0" width="10" height="10" fill="bogus">`)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if vp.FillColor != colorInherit {
		t.Fatalf("invalid fill should fall back to inherit, got %+v", vp.FillColor)
	}
}
