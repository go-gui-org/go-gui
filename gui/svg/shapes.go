package svg

import "github.com/go-gui-org/go-gui/gui"

// parsePathWithStyle parses a <path> element with inherited style.
func parsePathWithStyle(elem string, inherited ComputedStyle) (VectorPath, bool) {
	path, ok := parsePathElement(elem)
	if !ok {
		return VectorPath{}, false
	}
	path.FillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parseRectWithStyle(elem string, inherited ComputedStyle) (VectorPath, bool) {
	path, ok := parseRectElement(elem)
	if !ok {
		return VectorPath{}, false
	}
	path.FillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parseCircleWithStyle(elem string, inherited ComputedStyle) (VectorPath, bool) {
	path, ok := parseCircleElement(elem)
	if !ok {
		return VectorPath{}, false
	}
	path.FillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parseEllipseWithStyle(elem string, inherited ComputedStyle) (VectorPath, bool) {
	path, ok := parseEllipseElement(elem)
	if !ok {
		return VectorPath{}, false
	}
	path.FillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parsePolygonWithStyle(elem string, inherited ComputedStyle, closed bool) (VectorPath, bool) {
	path, ok := parsePolygonElement(elem, closed)
	if !ok {
		return VectorPath{}, false
	}
	path.FillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

func parseLineWithStyle(elem string, inherited ComputedStyle) (VectorPath, bool) {
	path, ok := parseLineElement(elem)
	if !ok {
		return VectorPath{}, false
	}
	path.FillRule = resolveFillRule(elem, inherited)
	applyComputedStyle(&path, inherited)
	return path, true
}

// parsePathElement parses a <path> element.
func parsePathElement(elem string) (VectorPath, bool) {
	d, ok := findAttr(elem, "d")
	if !ok {
		return VectorPath{}, false
	}
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)
	fillColor, _ := parseSvgColor(fill)

	path := VectorPath{
		FillColor:        fillColor,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		StrokeCap:        s.StrokeCap,
		StrokeJoin:       s.StrokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		StrokeGradientID: s.StrokeGradientID,
		StrokeDasharray:  s.StrokeDasharray,
	}
	if gid, found := parseFillURL(fill); found {
		path.FillGradientID = gid
	}
	path.Segments = parsePathD(d)
	if len(path.Segments) == 0 {
		return VectorPath{}, false
	}
	path.Bbox = bboxFromSegments(path.Segments)
	return path, true
}

// segmentsForRect returns path segments for a <rect> primitive with
// the given attributes. Shared between parse time and animated
// re-tessellation.
func segmentsForRect(x, y, rw, rh, rx, ry float32) []PathSegment {
	if rx == 0 && ry > 0 {
		rx = ry
	}
	if ry == 0 && rx > 0 {
		ry = rx
	}
	if rx == 0 && ry == 0 {
		return []PathSegment{
			{CmdMoveTo, []float32{x, y}},
			{CmdLineTo, []float32{x + rw, y}},
			{CmdLineTo, []float32{x + rw, y + rh}},
			{CmdLineTo, []float32{x, y + rh}},
			{CmdClose, nil},
		}
	}
	if rx > rw/2 {
		rx = rw / 2
	}
	if ry > rh/2 {
		ry = rh / 2
	}
	segments := make([]PathSegment, 0, 16)
	segments = append(segments, PathSegment{CmdMoveTo, []float32{x + rx, y}})
	segments = append(segments, PathSegment{CmdLineTo, []float32{x + rw - rx, y}})
	segments = append(segments, arcToCubic(x+rw-rx, y, rx, ry, 0, false, true, x+rw, y+ry)...)
	segments = append(segments, PathSegment{CmdLineTo, []float32{x + rw, y + rh - ry}})
	segments = append(segments, arcToCubic(x+rw, y+rh-ry, rx, ry, 0, false, true, x+rw-rx, y+rh)...)
	segments = append(segments, PathSegment{CmdLineTo, []float32{x + rx, y + rh}})
	segments = append(segments, arcToCubic(x+rx, y+rh, rx, ry, 0, false, true, x, y+rh-ry)...)
	segments = append(segments, PathSegment{CmdLineTo, []float32{x, y + ry}})
	segments = append(segments, arcToCubic(x, y+ry, rx, ry, 0, false, true, x+rx, y)...)
	segments = append(segments, PathSegment{CmdClose, nil})
	return segments
}

// segmentsForEllipse returns path segments for a <circle> or
// <ellipse> primitive. A circle passes r for both rx and ry.
func segmentsForEllipse(cx, cy, rx, ry float32) []PathSegment {
	const k = float32(0.5522847498)
	kx := rx * k
	ky := ry * k
	return []PathSegment{
		{CmdMoveTo, []float32{cx, cy - ry}},
		{CmdCubicTo, []float32{cx + kx, cy - ry, cx + rx, cy - ky, cx + rx, cy}},
		{CmdCubicTo, []float32{cx + rx, cy + ky, cx + kx, cy + ry, cx, cy + ry}},
		{CmdCubicTo, []float32{cx - kx, cy + ry, cx - rx, cy + ky, cx - rx, cy}},
		{CmdCubicTo, []float32{cx - rx, cy - ky, cx - kx, cy - ry, cx, cy - ry}},
		{CmdClose, nil},
	}
}

// segmentsForLine returns path segments for a <line> primitive.
func segmentsForLine(x1, y1, x2, y2 float32) []PathSegment {
	return []PathSegment{
		{CmdMoveTo, []float32{x1, y1}},
		{CmdLineTo, []float32{x2, y2}},
	}
}

// parseRectElement converts <rect> to path.
func parseRectElement(elem string) (VectorPath, bool) {
	x := attrFloat(elem, "x", 0)
	y := attrFloat(elem, "y", 0)
	w, wok := findAttr(elem, "width")
	h, hok := findAttr(elem, "height")
	if !wok || !hok {
		return VectorPath{}, false
	}
	rw := parseF32(w)
	rh := parseF32(h)

	rx := attrFloat(elem, "rx", 0)
	ry := attrFloat(elem, "ry", 0)
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)
	fillColor, _ := parseSvgColor(fill)

	segments := segmentsForRect(x, y, rw, rh, rx, ry)

	vp := VectorPath{
		Segments:         segments,
		FillColor:        fillColor,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		StrokeCap:        s.StrokeCap,
		StrokeJoin:       s.StrokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		StrokeGradientID: s.StrokeGradientID,
		StrokeDasharray:  s.StrokeDasharray,
		Primitive: gui.SvgPrimitive{
			Kind: gui.SvgPrimRect,
			X:    x,
			Y:    y,
			W:    rw,
			H:    rh,
			RX:   rx,
			RY:   ry,
		},
		Bbox: bboxFromRect(x, y, rw, rh),
	}
	if gid, found := parseFillURL(fill); found {
		vp.FillGradientID = gid
	}
	return vp, true
}

// parseCircleElement converts <circle> to path.
func parseCircleElement(elem string) (VectorPath, bool) {
	cx := attrFloat(elem, "cx", 0)
	cy := attrFloat(elem, "cy", 0)
	_, rok := findAttr(elem, "r")
	if !rok {
		return VectorPath{}, false
	}
	r := attrFloat(elem, "r", 0)
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)
	vp := ellipseToPath(cx, cy, r, r, elem, fill, s)
	vp.Primitive = gui.SvgPrimitive{
		Kind: gui.SvgPrimCircle,
		CX:   cx,
		CY:   cy,
		R:    r,
	}
	vp.Bbox = bboxFromEllipse(cx, cy, r, r)
	return vp, true
}

// parseEllipseElement converts <ellipse> to path.
func parseEllipseElement(elem string) (VectorPath, bool) {
	_, rxok := findAttr(elem, "rx")
	_, ryok := findAttr(elem, "ry")
	if !rxok || !ryok {
		return VectorPath{}, false
	}
	cx := attrFloat(elem, "cx", 0)
	cy := attrFloat(elem, "cy", 0)
	rx := attrFloat(elem, "rx", 0)
	ry := attrFloat(elem, "ry", 0)
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)
	vp := ellipseToPath(cx, cy, rx, ry, elem, fill, s)
	vp.Primitive = gui.SvgPrimitive{
		Kind: gui.SvgPrimEllipse,
		CX:   cx,
		CY:   cy,
		RX:   rx,
		RY:   ry,
	}
	vp.Bbox = bboxFromEllipse(cx, cy, rx, ry)
	return vp, true
}

func ellipseToPath(cx, cy, rx, ry float32, _, fill string, s elementStyle) VectorPath {
	fillColor, _ := parseSvgColor(fill)
	vp := VectorPath{
		Segments:         segmentsForEllipse(cx, cy, rx, ry),
		FillColor:        fillColor,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		StrokeCap:        s.StrokeCap,
		StrokeJoin:       s.StrokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		StrokeGradientID: s.StrokeGradientID,
		StrokeDasharray:  s.StrokeDasharray,
	}
	if gid, found := parseFillURL(fill); found {
		vp.FillGradientID = gid
	}
	return vp
}

// parsePolygonElement converts <polygon> or <polyline> to path.
func parsePolygonElement(elem string, closed bool) (VectorPath, bool) {
	pointsStr, ok := findAttr(elem, "points")
	if !ok {
		return VectorPath{}, false
	}
	fill, _ := findAttrOrStyle(elem, "fill")
	s := parseElementStyle(elem)

	numbers := parseNumberList(pointsStr)
	if len(numbers) < 4 || len(numbers)%2 != 0 {
		return VectorPath{}, false
	}

	segments := make([]PathSegment, 0, len(numbers)/2+2)
	segments = append(segments, PathSegment{CmdMoveTo, []float32{numbers[0], numbers[1]}})
	for i := 2; i < len(numbers)-1; i += 2 {
		segments = append(segments, PathSegment{CmdLineTo, []float32{numbers[i], numbers[i+1]}})
	}
	if closed {
		segments = append(segments, PathSegment{CmdClose, nil})
	}
	fillColor, _ := parseSvgColor(fill)

	vp := VectorPath{
		Segments:         segments,
		FillColor:        fillColor,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		StrokeCap:        s.StrokeCap,
		StrokeJoin:       s.StrokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		StrokeGradientID: s.StrokeGradientID,
		StrokeDasharray:  s.StrokeDasharray,
	}
	vp.Bbox = bboxFromSegments(segments)
	if gid, found := parseFillURL(fill); found {
		vp.FillGradientID = gid
	}
	return vp, true
}

// parseLineElement converts <line> to path.
func parseLineElement(elem string) (VectorPath, bool) {
	x1 := attrFloat(elem, "x1", 0)
	y1 := attrFloat(elem, "y1", 0)
	x2 := attrFloat(elem, "x2", 0)
	y2 := attrFloat(elem, "y2", 0)

	if x1 == x2 && y1 == y2 {
		return VectorPath{}, false
	}

	s := parseElementStyle(elem)
	return VectorPath{
		Segments:         segmentsForLine(x1, y1, x2, y2),
		FillColor:        colorTransparent,
		Transform:        s.Transform,
		StrokeColor:      s.StrokeColor,
		StrokeWidth:      s.StrokeWidth,
		StrokeCap:        s.StrokeCap,
		StrokeJoin:       s.StrokeJoin,
		Opacity:          s.Opacity,
		FillOpacity:      s.FillOpacity,
		StrokeOpacity:    s.StrokeOpacity,
		StrokeGradientID: s.StrokeGradientID,
		StrokeDasharray:  s.StrokeDasharray,
		Primitive: gui.SvgPrimitive{
			Kind: gui.SvgPrimLine,
			X:    x1,
			Y:    y1,
			X2:   x2,
			Y2:   y2,
		},
		Bbox: bboxFromLine(x1, y1, x2, y2),
	}, true
}

func attrFloat(elem, name string, fallback float32) float32 {
	v, ok := findAttr(elem, name)
	if !ok {
		return fallback
	}
	return parseF32(v)
}
