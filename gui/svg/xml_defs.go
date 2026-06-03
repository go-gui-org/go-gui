package svg

// xml_defs.go — SVG <defs> parsing: clip paths, gradients,
// filters, and named path definitions. Walks the decoded xmlNode
// tree; each helper finds the <defs> subtree and its children by
// element name.

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// walkDefs invokes fn on every <defs> node in the subtree rooted at
// root, in document order.
func walkDefs(root *xmlNode, fn func(defs *xmlNode)) {
	for i := range root.Children {
		c := &root.Children[i]
		if c.Name == "defs" {
			fn(c)
		}
		// Nested defs under groups are rare but spec-legal.
		if len(c.Children) > 0 {
			walkDefs(c, fn)
		}
	}
}

// findAllByName walks root's descendants and returns pointers to
// every direct-or-nested node matching name. Order is pre-order.
func findAllByName(root *xmlNode, name string, out *[]*xmlNode) {
	for i := range root.Children {
		c := &root.Children[i]
		if c.Name == name {
			*out = append(*out, c)
		}
		if len(c.Children) > 0 {
			findAllByName(c, name, out)
		}
	}
}

func parseDefsClipPaths(root *xmlNode) map[string][]VectorPath {
	clipPaths := make(map[string][]VectorPath)
	var nodes []*xmlNode
	findAllByName(root, "clipPath", &nodes)
	for _, cp := range nodes {
		clipID := cp.AttrMap["id"]
		if clipID == "" {
			continue
		}
		if cp.SelfClose {
			continue
		}
		defStyle := defaultComputedStyle(identityTransform)
		st := &parseState{}
		paths := parseSvgContent(cp, defStyle, 0, st, nil)
		if len(paths) > 0 {
			clipPaths[clipID] = paths
		}
	}
	return clipPaths
}

func parseDefsGradients(root *xmlNode) map[string]gui.SvgGradientDef {
	gradients := make(map[string]gui.SvgGradientDef)
	var nodes []*xmlNode
	findAllByName(root, "linearGradient", &nodes)
	for _, lg := range nodes {
		gradID := lg.AttrMap["id"]
		if gradID == "" {
			continue
		}

		unitsStr := lg.AttrMap["gradientUnits"]
		if unitsStr == "" {
			unitsStr = "objectBoundingBox"
		}
		isOBB := unitsStr != "userSpaceOnUse"

		// SVG spec defaults: x1=0%, y1=0%, x2=100%, y2=0% — i.e. a
		// horizontal left-to-right gradient when no endpoint attrs
		// are authored. For objectBoundingBox units 100% == 1.0 in
		// the OBB-normalized space we already produce, so fall back
		// to that default when the attr is missing — otherwise an
		// empty string parses to 0 and collapses x1==x2, yielding a
		// degenerate gradient that projects every vertex to t=0.
		// Under userSpaceOnUse, "100%" resolves against the viewport
		// at render time and isn't known here; preserve the pre-fix
		// behavior (default 0) for that case.
		defX2 := float32(0)
		if isOBB {
			defX2 = 1
		}
		x1 := gradientCoordOrDefault(lg.AttrMap, "x1", isOBB, 0)
		y1 := gradientCoordOrDefault(lg.AttrMap, "y1", isOBB, 0)
		x2 := gradientCoordOrDefault(lg.AttrMap, "x2", isOBB, defX2)
		y2 := gradientCoordOrDefault(lg.AttrMap, "y2", isOBB, 0)

		stops := parseGradientStops(lg)

		gradients[gradID] = gui.SvgGradientDef{
			X1: x1, Y1: y1, X2: x2, Y2: y2,
			Stops:         stops,
			GradientUnits: unitsStr,
			SpreadMethod:  parseSpreadMethod(lg.AttrMap["spreadMethod"]),
		}
	}

	var rnodes []*xmlNode
	findAllByName(root, "radialGradient", &rnodes)
	for _, rg := range rnodes {
		gradID := rg.AttrMap["id"]
		if gradID == "" {
			continue
		}

		unitsStr := rg.AttrMap["gradientUnits"]
		if unitsStr == "" {
			unitsStr = "objectBoundingBox"
		}
		isOBB := unitsStr != "userSpaceOnUse"

		// SVG spec defaults for radialGradient: cx=cy=r=50%, fx=cx,
		// fy=cy. In OBB units 50% == 0.5; in userSpaceOnUse the
		// percentages resolve against the viewport at render time —
		// fall back to the parsed numeric (typically 0) for that
		// case so existing fixtures don't shift.
		defHalf := float32(0)
		if isOBB {
			defHalf = 0.5
		}
		cx := gradientCoordOrDefault(rg.AttrMap, "cx", isOBB, defHalf)
		cy := gradientCoordOrDefault(rg.AttrMap, "cy", isOBB, defHalf)
		r := gradientCoordOrDefault(rg.AttrMap, "r", isOBB, defHalf)
		fx := gradientCoordOrDefault(rg.AttrMap, "fx", isOBB, cx)
		fy := gradientCoordOrDefault(rg.AttrMap, "fy", isOBB, cy)

		stops := parseGradientStops(rg)

		gradients[gradID] = gui.SvgGradientDef{
			CX: cx, CY: cy, R: r,
			FX: fx, FY: fy,
			IsRadial:      true,
			Stops:         stops,
			GradientUnits: unitsStr,
			SpreadMethod:  parseSpreadMethod(rg.AttrMap["spreadMethod"]),
		}
	}
	return gradients
}

// parseSpreadMethod maps the SVG spreadMethod keyword to
// gui.SvgGradientSpread. Default and unknown values fall back to pad.
func parseSpreadMethod(s string) gui.SvgGradientSpread {
	switch strings.TrimSpace(s) {
	case "reflect":
		return gui.SvgSpreadReflect
	case "repeat":
		return gui.SvgSpreadRepeat
	}
	return gui.SvgSpreadPad
}

func parseGradientCoord(s string, isOBB bool) float32 {
	trimmed := strings.TrimSpace(s)
	if isOBB && strings.HasSuffix(trimmed, "%") {
		return parseF32(trimmed[:len(trimmed)-1]) / 100.0
	}
	return parseF32(trimmed)
}

// gradientCoordOrDefault returns the parsed coord for attr if present,
// else the spec default for that endpoint.
func gradientCoordOrDefault(attrs map[string]string, attr string,
	isOBB bool, def float32) float32 {
	v, ok := attrs[attr]
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	return parseGradientCoord(v, isOBB)
}

func parseGradientStops(gradient *xmlNode) []gui.SvgGradientStop {
	var stops []gui.SvgGradientStop
	for i := range gradient.Children {
		c := &gradient.Children[i]
		if c.Name != "stop" {
			continue
		}
		stopElem := c.OpenTag

		offsetStr := "0"
		if os, ok := findAttrOrStyle(stopElem, "offset"); ok {
			offsetStr = os
		}
		offset := float32(0)
		if strings.HasSuffix(offsetStr, "%") {
			offset = parseF32(offsetStr[:len(offsetStr)-1]) / 100.0
		} else {
			offset = parseF32(offsetStr)
		}
		offset = min(max(offset, 0), 1)

		colorStr := "#000000"
		if cs, ok := findAttrOrStyle(stopElem, "stop-color"); ok {
			colorStr = cs
		}
		color, parsed := parseSvgColor(colorStr)
		if !parsed || color == colorInherit {
			// Bare "inherit" at gradient-stop scope has no parent to
			// resolve against; fall back to black.
			color = colorBlack
		} else if isSentinelColor(color) {
			// Explicit currentColor: preserve sentinel RGB so the
			// render-time tint can substitute, but lift A to opaque
			// before stop-opacity bakes so small opacities survive.
			color.A = 255
		}

		stopOpacity := parseOpacityAttr(stopElem, "stop-opacity", 1.0)
		if stopOpacity < 1.0 {
			color = applyOpacity(color, stopOpacity)
		}

		stops = append(stops, gui.SvgGradientStop{Offset: offset, Color: color})
	}
	return stops
}

func parseDefsPaths(root *xmlNode) map[string]string {
	paths := make(map[string]string)
	walkDefs(root, func(defs *xmlNode) {
		for i := range defs.Children {
			c := &defs.Children[i]
			if c.Name != "path" {
				continue
			}
			pid := c.AttrMap["id"]
			d := c.AttrMap["d"]
			if pid == "" || d == "" {
				continue
			}
			paths[pid] = d
		}
	})
	return paths
}

// parseDefsFilters extracts <filter> definitions from the tree.
func parseDefsFilters(root *xmlNode) map[string]gui.SvgFilter {
	filters := make(map[string]gui.SvgFilter)
	var nodes []*xmlNode
	findAllByName(root, "filter", &nodes)
	for _, f := range nodes {
		filterID := f.AttrMap["id"]
		if filterID == "" {
			continue
		}
		if f.SelfClose {
			continue
		}

		// Extract stdDeviation from feGaussianBlur child.
		stdDev := float32(0)
		if gb := f.findChild("feGaussianBlur"); gb != nil {
			if sd, ok := gb.AttrMap["stdDeviation"]; ok {
				stdDev = parseF32(sd)
			}
		}
		if stdDev <= 0 {
			continue
		}

		// Walk feMerge children for feMergeNode entries.
		blurLayers := 0
		keepSource := false
		if fm := f.findChild("feMerge"); fm != nil {
			for i := range fm.Children {
				c := &fm.Children[i]
				if c.Name != "feMergeNode" {
					continue
				}
				if c.AttrMap["in"] == "SourceGraphic" {
					keepSource = true
				} else {
					blurLayers++
				}
			}
		}
		// Top-level feMergeNode (no wrapping feMerge) still counts.
		for i := range f.Children {
			c := &f.Children[i]
			if c.Name != "feMergeNode" {
				continue
			}
			if c.AttrMap["in"] == "SourceGraphic" {
				keepSource = true
			} else {
				blurLayers++
			}
		}
		if blurLayers == 0 {
			blurLayers = 1
		}

		filters[filterID] = gui.SvgFilter{
			ID:         filterID,
			StdDev:     stdDev,
			BlurLayers: blurLayers,
			KeepSource: keepSource,
		}
	}
	return filters
}
