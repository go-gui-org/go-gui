package svg

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// parsePathWithStyle parses a <path> element with inherited style.
func parsePathWithStyle(elem string, inherited computedStyle) (vectorPath, bool) {
	path, ok := parsePathElement(elem)
	if !ok {
		return vectorPath{}, false
	}
	path.fillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parseRectWithStyle(elem string, inherited computedStyle) (vectorPath, bool) {
	path, ok := parseRectElementResolved(elem, inherited)
	if !ok {
		return vectorPath{}, false
	}
	path.fillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parseCircleWithStyle(elem string, inherited computedStyle) (vectorPath, bool) {
	path, ok := parseCircleElementResolved(elem, inherited)
	if !ok {
		return vectorPath{}, false
	}
	path.fillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parseEllipseWithStyle(elem string, inherited computedStyle) (vectorPath, bool) {
	path, ok := parseEllipseElementResolved(elem, inherited)
	if !ok {
		return vectorPath{}, false
	}
	path.fillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parsePolygonWithStyle(elem string, inherited computedStyle, closed bool) (vectorPath, bool) {
	path, ok := parsePolygonElement(elem, closed)
	if !ok {
		return vectorPath{}, false
	}
	path.fillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parseLineWithStyle(elem string, inherited computedStyle) (vectorPath, bool) {
	path, ok := parseLineElementResolved(elem, inherited)
	if !ok {
		return vectorPath{}, false
	}
	path.fillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

// geomFloat returns the effective value of an optional geometry
// property: the cascade-resolved CSS value when the cascade wrote
// the property, else the presentation attribute (or fallback).
func geomFloat(set bool, v float32, elem, name string, fallback float32) float32 {
	if set {
		return v
	}
	return attrFloat(elem, name, fallback)
}

// geomRequired returns the effective value of a required geometry
// property (r, width, height): the cascade-resolved CSS value when
// set, else the presentation attribute. ok is false when neither
// provides one, which disables rendering.
func geomRequired(set bool, v float32, elem, name string) (float32, bool) {
	if set {
		return v, true
	}
	s, ok := findAttr(elem, name)
	if !ok {
		return 0, false
	}
	return parseF32(s), true
}

// parsePathElement parses a <path> element.
func parsePathElement(elem string) (vectorPath, bool) {
	d, ok := findAttr(elem, "d")
	if !ok {
		return vectorPath{}, false
	}
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)
	// An unparseable fill means "ignore the declaration", not
	// "paint nothing": fall back to the inherit sentinel so the
	// cascade resolves it, instead of the transparent zero value.
	fillColor := svgFillColor(fill)

	path := vectorPath{
		FillColor:        fillColor,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		strokeCap:        s.strokeCap,
		strokeJoin:       s.strokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		strokeGradientID: s.strokeGradientID,
		strokeDasharray:  s.strokeDasharray,
	}
	if gid, found := parseFillURL(fill); found {
		path.FillGradientID = gid
	}
	path.Segments = parsePathD(d)
	if len(path.Segments) == 0 {
		return vectorPath{}, false
	}
	path.bbox = bboxFromSegments(path.Segments)
	return path, true
}

// segmentsForRect returns path segments for a <rect> primitive with
// the given attributes. Shared between parse time and animated
// re-tessellation.
func segmentsForRect(x, y, rw, rh, rx, ry float32) []pathSegment {
	if rx == 0 && ry > 0 {
		rx = ry
	}
	if ry == 0 && rx > 0 {
		ry = rx
	}
	if rx == 0 && ry == 0 {
		return []pathSegment{
			{cmdMoveTo, []float32{x, y}},
			{cmdLineTo, []float32{x + rw, y}},
			{cmdLineTo, []float32{x + rw, y + rh}},
			{cmdLineTo, []float32{x, y + rh}},
			{cmdClose, nil},
		}
	}
	if rx > rw/2 {
		rx = rw / 2
	}
	if ry > rh/2 {
		ry = rh / 2
	}
	segments := make([]pathSegment, 0, 16)
	segments = append(segments, pathSegment{cmdMoveTo, []float32{x + rx, y}})
	segments = append(segments, pathSegment{cmdLineTo, []float32{x + rw - rx, y}})
	segments = append(segments, arcToCubic(x+rw-rx, y, rx, ry, 0, false, true, x+rw, y+ry)...)
	segments = append(segments, pathSegment{cmdLineTo, []float32{x + rw, y + rh - ry}})
	segments = append(segments, arcToCubic(x+rw, y+rh-ry, rx, ry, 0, false, true, x+rw-rx, y+rh)...)
	segments = append(segments, pathSegment{cmdLineTo, []float32{x + rx, y + rh}})
	segments = append(segments, arcToCubic(x+rx, y+rh, rx, ry, 0, false, true, x, y+rh-ry)...)
	segments = append(segments, pathSegment{cmdLineTo, []float32{x, y + ry}})
	segments = append(segments, arcToCubic(x, y+ry, rx, ry, 0, false, true, x+rx, y)...)
	segments = append(segments, pathSegment{cmdClose, nil})
	return segments
}

// segmentsForEllipse returns path segments for a <circle> or
// <ellipse> primitive. A circle passes r for both rx and ry.
func segmentsForEllipse(cx, cy, rx, ry float32) []pathSegment {
	const k = float32(0.5522847498)
	kx := rx * k
	ky := ry * k
	return []pathSegment{
		{cmdMoveTo, []float32{cx, cy - ry}},
		{cmdCubicTo, []float32{cx + kx, cy - ry, cx + rx, cy - ky, cx + rx, cy}},
		{cmdCubicTo, []float32{cx + rx, cy + ky, cx + kx, cy + ry, cx, cy + ry}},
		{cmdCubicTo, []float32{cx - kx, cy + ry, cx - rx, cy + ky, cx - rx, cy}},
		{cmdCubicTo, []float32{cx - rx, cy - ky, cx - kx, cy - ry, cx, cy - ry}},
		{cmdClose, nil},
	}
}

// segmentsForLine returns path segments for a <line> primitive.
func segmentsForLine(x1, y1, x2, y2 float32) []pathSegment {
	return []pathSegment{
		{cmdMoveTo, []float32{x1, y1}},
		{cmdLineTo, []float32{x2, y2}},
	}
}

// parseRectElement converts <rect> to path.
func parseRectElement(elem string) (vectorPath, bool) {
	x := attrFloat(elem, "x", 0)
	y := attrFloat(elem, "y", 0)
	w, wok := findAttr(elem, "width")
	h, hok := findAttr(elem, "height")
	if !wok || !hok {
		return vectorPath{}, false
	}
	rw := parseF32(w)
	rh := parseF32(h)
	// A negative size is an error that disables rendering. Zero is
	// degenerate but preserved: the tessellator drops it when static
	// and keeps it as an animated placeholder when an <animate>
	// child drives the size (phase 8).
	if rw < 0 || rh < 0 {
		return vectorPath{}, false
	}

	rx := attrFloat(elem, "rx", 0)
	ry := attrFloat(elem, "ry", 0)
	// SVG 2: a negative rx/ry is an invalid value, ignored as auto; it
	// does not disable rendering. Zero is how auto reads here:
	// segmentsForRect copies the other radius into a zero one, and
	// both zero draws square corners.
	rx = max(rx, 0)
	ry = max(ry, 0)
	return rectPath(elem, x, y, rw, rh, rx, ry), true
}

// parseRectElementResolved converts <rect> to path using
// cascade-resolved geometry: author CSS rules and inline style (with
// var()/calc() already substituted) override the presentation
// attributes per SVG2.
func parseRectElementResolved(elem string, gs computedStyle) (vectorPath, bool) {
	x := geomFloat(gs.geomXSet, gs.geomX, elem, "x", 0)
	y := geomFloat(gs.geomYSet, gs.geomY, elem, "y", 0)
	rw, wok := geomRequired(gs.geomWSet, gs.geomW, elem, "width")
	rh, hok := geomRequired(gs.geomHSet, gs.geomH, elem, "height")
	if !wok || !hok {
		return vectorPath{}, false
	}
	// A negative size is an error that disables rendering. Zero is
	// degenerate but preserved: the tessellator drops it when static
	// and keeps it as an animated placeholder when an <animate>
	// child drives the size (phase 8).
	if rw < 0 || rh < 0 {
		return vectorPath{}, false
	}
	rx := max(geomFloat(gs.geomRXSet, gs.geomRX, elem, "rx", 0), 0)
	ry := max(geomFloat(gs.geomRYSet, gs.geomRY, elem, "ry", 0), 0)
	return rectPath(elem, x, y, rw, rh, rx, ry), true
}

// rectPath builds the vectorPath for resolved rect geometry.
func rectPath(elem string, x, y, rw, rh, rx, ry float32) vectorPath {
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)
	fillColor := svgFillColor(fill)

	segments := segmentsForRect(x, y, rw, rh, rx, ry)

	vp := vectorPath{
		Segments:         segments,
		FillColor:        fillColor,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		strokeCap:        s.strokeCap,
		strokeJoin:       s.strokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		strokeGradientID: s.strokeGradientID,
		strokeDasharray:  s.strokeDasharray,
		Primitive: gui.SvgPrimitive{
			Kind: gui.SvgPrimRect,
			X:    x,
			Y:    y,
			W:    rw,
			H:    rh,
			RX:   rx,
			RY:   ry,
		},
		bbox: bboxFromRect(x, y, rw, rh),
	}
	if gid, found := parseFillURL(fill); found {
		vp.FillGradientID = gid
	}
	return vp
}

// parseCircleElement converts <circle> to path.
func parseCircleElement(elem string) (vectorPath, bool) {
	cx := attrFloat(elem, "cx", 0)
	cy := attrFloat(elem, "cy", 0)
	_, rok := findAttr(elem, "r")
	if !rok {
		return vectorPath{}, false
	}
	r := attrFloat(elem, "r", 0)
	// A negative radius is an error that disables rendering. Zero is
	// degenerate but preserved for the animated placeholder path.
	if r < 0 {
		return vectorPath{}, false
	}
	return circlePath(elem, cx, cy, r), true
}

// parseCircleElementResolved converts <circle> to path using
// cascade-resolved geometry: author CSS rules and inline style (with
// var()/calc() already substituted) override the presentation
// attributes per SVG2.
func parseCircleElementResolved(elem string, gs computedStyle) (vectorPath, bool) {
	cx := geomFloat(gs.geomCXSet, gs.geomCX, elem, "cx", 0)
	cy := geomFloat(gs.geomCYSet, gs.geomCY, elem, "cy", 0)
	r, rok := geomRequired(gs.geomRSet, gs.geomR, elem, "r")
	if !rok {
		return vectorPath{}, false
	}
	// A negative radius is an error that disables rendering. Zero is
	// degenerate but preserved for the animated placeholder path.
	if r < 0 {
		return vectorPath{}, false
	}
	return circlePath(elem, cx, cy, r), true
}

// circlePath builds the vectorPath for resolved circle geometry.
func circlePath(elem string, cx, cy, r float32) vectorPath {
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)
	vp := ellipseToPath(cx, cy, r, r, fill, s)
	vp.Primitive = gui.SvgPrimitive{
		Kind: gui.SvgPrimCircle,
		CX:   cx,
		CY:   cy,
		R:    r,
	}
	vp.bbox = bboxFromEllipse(cx, cy, r, r)
	return vp
}

// parseEllipseElement converts <ellipse> to path.
func parseEllipseElement(elem string) (vectorPath, bool) {
	cx := attrFloat(elem, "cx", 0)
	cy := attrFloat(elem, "cy", 0)
	// SVG 2 §10.4: a missing, unparseable or negative radius is auto.
	// Auto takes the other radius; both auto disables rendering. An
	// explicit zero is degenerate but preserved for the animated
	// placeholder path.
	rx, rxok := attrRadius(elem, "rx")
	ry, ryok := attrRadius(elem, "ry")
	if !rxok && !ryok {
		return vectorPath{}, false
	}
	if !rxok {
		rx = ry
	}
	if !ryok {
		ry = rx
	}
	return ellipsePath(elem, cx, cy, rx, ry), true
}

// parseEllipseElementResolved converts <ellipse> to path using
// cascade-resolved geometry: author CSS rules and inline style (with
// var()/calc() already substituted) override the presentation
// attributes per SVG2. A negative CSS radius reads as auto, matching
// the presentation-attribute rule.
func parseEllipseElementResolved(elem string, gs computedStyle) (vectorPath, bool) {
	cx := geomFloat(gs.geomCXSet, gs.geomCX, elem, "cx", 0)
	cy := geomFloat(gs.geomCYSet, gs.geomCY, elem, "cy", 0)
	rx, rxok := attrRadius(elem, "rx")
	if gs.geomRXSet {
		if gs.geomRX < 0 {
			rxok = false
		} else {
			rx, rxok = gs.geomRX, true
		}
	}
	ry, ryok := attrRadius(elem, "ry")
	if gs.geomRYSet {
		if gs.geomRY < 0 {
			ryok = false
		} else {
			ry, ryok = gs.geomRY, true
		}
	}
	if !rxok && !ryok {
		return vectorPath{}, false
	}
	if !rxok {
		rx = ry
	}
	if !ryok {
		ry = rx
	}
	return ellipsePath(elem, cx, cy, rx, ry), true
}

// ellipsePath builds the vectorPath for resolved ellipse geometry.
func ellipsePath(elem string, cx, cy, rx, ry float32) vectorPath {
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)
	vp := ellipseToPath(cx, cy, rx, ry, fill, s)
	vp.Primitive = gui.SvgPrimitive{
		Kind: gui.SvgPrimEllipse,
		CX:   cx,
		CY:   cy,
		RX:   rx,
		RY:   ry,
	}
	vp.bbox = bboxFromEllipse(cx, cy, rx, ry)
	return vp
}

func ellipseToPath(cx, cy, rx, ry float32, fill string, s elementStyle) vectorPath {
	fillColor := svgFillColor(fill)
	vp := vectorPath{
		Segments:         segmentsForEllipse(cx, cy, rx, ry),
		FillColor:        fillColor,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		strokeCap:        s.strokeCap,
		strokeJoin:       s.strokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		strokeGradientID: s.strokeGradientID,
		strokeDasharray:  s.strokeDasharray,
	}
	if gid, found := parseFillURL(fill); found {
		vp.FillGradientID = gid
	}
	return vp
}

// parsePolygonElement converts <polygon> or <polyline> to path.
func parsePolygonElement(elem string, closed bool) (vectorPath, bool) {
	pointsStr, ok := findAttr(elem, "points")
	if !ok {
		return vectorPath{}, false
	}
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)

	numbers := parseNumberList(pointsStr)
	if len(numbers) < 4 || len(numbers)%2 != 0 {
		return vectorPath{}, false
	}

	segments := make([]pathSegment, 0, len(numbers)/2+2)
	segments = append(segments, pathSegment{cmdMoveTo, []float32{numbers[0], numbers[1]}})
	for i := 2; i < len(numbers)-1; i += 2 {
		segments = append(segments, pathSegment{cmdLineTo, []float32{numbers[i], numbers[i+1]}})
	}
	if closed {
		segments = append(segments, pathSegment{cmdClose, nil})
	}
	fillColor := svgFillColor(fill)

	vp := vectorPath{
		Segments:         segments,
		FillColor:        fillColor,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		strokeCap:        s.strokeCap,
		strokeJoin:       s.strokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		strokeGradientID: s.strokeGradientID,
		strokeDasharray:  s.strokeDasharray,
	}
	vp.bbox = bboxFromSegments(segments)
	if gid, found := parseFillURL(fill); found {
		vp.FillGradientID = gid
	}
	return vp, true
}

// parseLineElement converts <line> to path.
func parseLineElement(elem string) (vectorPath, bool) {
	x1 := attrFloat(elem, "x1", 0)
	y1 := attrFloat(elem, "y1", 0)
	x2 := attrFloat(elem, "x2", 0)
	y2 := attrFloat(elem, "y2", 0)

	if x1 == x2 && y1 == y2 {
		return vectorPath{}, false
	}

	return linePath(elem, x1, y1, x2, y2), true
}

// parseLineElementResolved converts <line> to path using
// cascade-resolved geometry: author CSS rules and inline style (with
// var()/calc() already substituted) override the presentation
// attributes per SVG2.
func parseLineElementResolved(elem string, gs computedStyle) (vectorPath, bool) {
	x1 := geomFloat(gs.geomX1Set, gs.geomX1, elem, "x1", 0)
	y1 := geomFloat(gs.geomY1Set, gs.geomY1, elem, "y1", 0)
	x2 := geomFloat(gs.geomX2Set, gs.geomX2, elem, "x2", 0)
	y2 := geomFloat(gs.geomY2Set, gs.geomY2, elem, "y2", 0)

	if x1 == x2 && y1 == y2 {
		return vectorPath{}, false
	}

	return linePath(elem, x1, y1, x2, y2), true
}

// linePath builds the vectorPath for resolved line geometry.
func linePath(elem string, x1, y1, x2, y2 float32) vectorPath {
	s := parseElementStyle(elem)
	return vectorPath{
		Segments:         segmentsForLine(x1, y1, x2, y2),
		FillColor:        colorTransparent,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		strokeCap:        s.strokeCap,
		strokeJoin:       s.strokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		strokeGradientID: s.strokeGradientID,
		strokeDasharray:  s.strokeDasharray,
		Primitive: gui.SvgPrimitive{
			Kind: gui.SvgPrimLine,
			X:    x1,
			Y:    y1,
			X2:   x2,
			Y2:   y2,
		},
		bbox: bboxFromLine(x1, y1, x2, y2),
	}
}

func attrFloat(elem, name string, fallback float32) float32 {
	v, ok := findAttr(elem, name)
	if !ok {
		return fallback
	}
	return parseF32(v)
}

// attrRadius reads an ellipse radius. ok is false when the attribute is
// missing, unparseable, non-finite or negative: SVG 2 ignores such a
// value as a parse error, which leaves the radius auto. It uses
// parseFloatStrict, not parseF32, because parseF32 turns a parse
// failure into 0, and 0 is a real radius that disables rendering. A
// "px" suffix is a user-unit length, so it is trimmed; other units
// are not supported and read as auto.
func attrRadius(elem, name string) (float32, bool) {
	v, found := findAttr(elem, name)
	if !found {
		return 0, false
	}
	r, ok := parseFloatStrict(strings.TrimSuffix(strings.TrimSpace(v), "px"))
	if !ok || r < 0 {
		return 0, false
	}
	return r, true
}

// svgFillColor resolves a fill attribute to a color. An unparseable
// value falls back to the inherit sentinel so the cascade resolves it,
// instead of the transparent zero value which would paint nothing.
func svgFillColor(fill string) gui.SvgColor {
	c, ok := parseSvgColor(fill)
	if !ok {
		return colorInherit
	}
	return c
}
