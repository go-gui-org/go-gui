// Package svg parses and tessellates SVG content.
package svg

import (
	"math"
	"strings"
)

// motionPathD extracts the path d-string from an animateMotion:
// first via path= attr, else via <mpath xlink:href="#id"/> child.
func motionPathD(n *xmlNode, state *parseState) string {
	if p, ok := n.AttrMap["path"]; ok && p != "" {
		return p
	}
	if state == nil || len(state.defsPaths) == 0 {
		return ""
	}
	mp := n.findChild("mpath")
	if mp == nil {
		return ""
	}
	href, ok := mp.AttrMap["xlink:href"]
	if !ok {
		href, ok = mp.AttrMap["href"]
	}
	if !ok || !strings.HasPrefix(href, "#") {
		return ""
	}
	return state.defsPaths[href[1:]]
}

// flattenMotionD parses a path d-string and returns the flattened
// polyline + cumulative arc length array. Only the first subpath is
// used — animateMotion conventionally follows a single continuous
// curve; multi-M paths are uncommon.
func flattenMotionD(d string) ([]float32, []float32) {
	segs := parsePathD(d)
	if len(segs) == 0 {
		return nil, nil
	}
	vp := &VectorPath{
		Segments:  segs,
		Transform: identityTransform,
	}
	polys := flattenPath(vp, motionFlattenTolerance)
	if len(polys) == 0 {
		return nil, nil
	}
	poly := polys[0]
	if len(poly) < 4 {
		return nil, nil
	}
	n := len(poly) / 2
	if n > maxMotionVertices {
		n = maxMotionVertices
		poly = poly[:2*n]
	}
	lens := make([]float32, n)
	for i := 1; i < n; i++ {
		dx := poly[i*2] - poly[(i-1)*2]
		dy := poly[i*2+1] - poly[(i-1)*2+1]
		lens[i] = lens[i-1] +
			float32(math.Sqrt(float64(dx*dx+dy*dy)))
	}
	return poly, lens
}
