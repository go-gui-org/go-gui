package svg

// bboxFromRect returns the bbox of a <rect> primitive.
func bboxFromRect(x, y, w, h float32) bbox {
	return bbox{MinX: x, MinY: y, MaxX: x + w, MaxY: y + h, Set: true}
}

// bboxFromEllipse returns the bbox of a <circle>/<ellipse> primitive.
func bboxFromEllipse(cx, cy, rx, ry float32) bbox {
	return bbox{
		MinX: cx - rx, MinY: cy - ry,
		MaxX: cx + rx, MaxY: cy + ry,
		Set: true,
	}
}

// bboxFromLine returns the bbox of a <line> primitive.
func bboxFromLine(x1, y1, x2, y2 float32) bbox {
	b := bbox{MinX: x1, MinY: y1, MaxX: x1, MaxY: y1, Set: true}
	return extendBBox(b, x2, y2)
}

// bboxFromSegments computes the control-poly bbox of a path segment
// list. Curves use control-point hulls (over-estimate); the design
// doc accepts this looseness for v1 — tighten later if visual diffs
// appear.
func bboxFromSegments(segs []pathSegment) bbox {
	var b bbox
	var cx, cy float32
	for _, s := range segs {
		switch s.Cmd {
		case cmdMoveTo, cmdLineTo:
			if len(s.Points) >= 2 {
				cx, cy = s.Points[0], s.Points[1]
				b = extendBBox(b, cx, cy)
			}
		case cmdQuadTo:
			if len(s.Points) >= 4 {
				b = extendBBox(b, s.Points[0], s.Points[1])
				cx, cy = s.Points[2], s.Points[3]
				b = extendBBox(b, cx, cy)
			}
		case cmdCubicTo:
			if len(s.Points) >= 6 {
				b = extendBBox(b, s.Points[0], s.Points[1])
				b = extendBBox(b, s.Points[2], s.Points[3])
				cx, cy = s.Points[4], s.Points[5]
				b = extendBBox(b, cx, cy)
			}
		case cmdClose:
			// no point.
		}
	}
	return b
}

// unionPathBboxes returns the union bbox over a slice of VectorPaths.
// Paths whose bbox is unset are skipped.
func unionPathBboxes(paths []vectorPath) bbox {
	var b bbox
	for i := range paths {
		b = unionBbox(b, paths[i].bbox)
	}
	return b
}

// unionBbox returns the smallest bbox enclosing a and b. Either side
// being unset (Set=false) yields the other; both unset yields unset.
func unionBbox(a, b bbox) bbox {
	if !a.Set {
		return b
	}
	if !b.Set {
		return a
	}
	out := a
	if b.MinX < out.MinX {
		out.MinX = b.MinX
	}
	if b.MinY < out.MinY {
		out.MinY = b.MinY
	}
	if b.MaxX > out.MaxX {
		out.MaxX = b.MaxX
	}
	if b.MaxY > out.MaxY {
		out.MaxY = b.MaxY
	}
	return out
}

func extendBBox(b bbox, x, y float32) bbox {
	if !b.Set {
		return bbox{MinX: x, MinY: y, MaxX: x, MaxY: y, Set: true}
	}
	if x < b.MinX {
		b.MinX = x
	}
	if y < b.MinY {
		b.MinY = y
	}
	if x > b.MaxX {
		b.MaxX = x
	}
	if y > b.MaxY {
		b.MaxY = y
	}
	return b
}
