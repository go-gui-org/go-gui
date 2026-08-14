package svg

import (
	"fmt"
	"log"
	"math"
	"slices"

	"github.com/go-gui-org/go-gui/gui"
)

// TessellateAnimated implements gui.AnimatedSvgParser. Returns fresh
// triangles for every VectorPath flagged Animated at the given scale,
// applying attribute overrides keyed by PathID. Result order follows
// the Animated-flagged paths' document order. Animated paths that
// carry a ClipPathID are skipped: the caller should fall back to
// cached triangles for them.
//
// Returns nil when overrides is empty/nil or no animated paths
// qualify. When reuse is non-nil its backing array is reused.
func (p *Parser) TessellateAnimated(
	parsed *gui.SvgParsed, scale float32,
	overrides map[uint32]gui.SvgAnimAttrOverride,
	reuse []gui.TessellatedPath,
) []gui.TessellatedPath {
	if len(overrides) == 0 {
		return nil
	}
	p.mu.Lock()
	hash, ok := p.byParsed[parsed]
	var entry parserCacheEntry
	if ok {
		entry, ok = p.byHash[hash]
	}
	p.mu.Unlock()
	if !ok {
		return nil
	}
	vg := entry.vg
	totalCap := len(vg.Paths)
	for i := range vg.FilteredGroups {
		totalCap += len(vg.FilteredGroups[i].Paths)
	}
	animated := p.getAnimatedScratch(totalCap)
	animated = collectAnimatedPaths(animated, vg.Paths, overrides)
	for gi := range vg.FilteredGroups {
		animated = collectAnimatedPaths(
			animated, vg.FilteredGroups[gi].Paths, overrides)
	}
	if len(animated) == 0 {
		p.putAnimatedScratch(animated)
		return nil
	}
	result := vg.tessellatePaths(animated, scale)
	if svgTrace {
		traceAnimatedTriangles(vg, result, animated, overrides)
	}
	if reuse != nil && cap(reuse) >= len(result) {
		reuse = reuse[:len(result)]
		copy(reuse, result)
		p.putAnimatedScratch(animated)
		return reuse
	}
	p.putAnimatedScratch(animated)
	return result
}

// collectAnimatedPaths appends clones of every Animated, non-clip-
// pathed entry in src into dst, applying any matching attribute
// override. Inlined (rather than a closure) so the hot path does not
// allocate a closure capturing the override map per call.
func collectAnimatedPaths(dst []vectorPath, src []vectorPath,
	overrides map[uint32]gui.SvgAnimAttrOverride) []vectorPath {
	for i := range src {
		s := &src[i]
		if !s.Animated || s.clipPathID != "" {
			continue
		}
		clone := *s
		if ov, ok := overrides[s.PathID]; ok && ov.Mask != 0 {
			applyOverridesToPath(&clone, ov)
		}
		dst = append(dst, clone)
	}
	return dst
}

func (p *Parser) getAnimatedScratch(minCap int) []vectorPath {
	// Bound seed cap so hostile minCap cannot force a giant make().
	// append grows past the cap naturally when real usage exceeds it.
	if minCap < 0 {
		minCap = 0
	}
	if minCap > maxAnimatedScratchCap {
		minCap = maxAnimatedScratchCap
	}
	if v := p.animatedScratch.Get(); v != nil {
		if buf, ok := v.(*[]vectorPath); ok && cap(*buf) >= minCap {
			return (*buf)[:0]
		}
	}
	return make([]vectorPath, 0, minCap)
}

func (p *Parser) putAnimatedScratch(buf []vectorPath) {
	if cap(buf) == 0 || cap(buf) > maxAnimatedScratchCap {
		return
	}
	for i := range buf {
		buf[i] = vectorPath{}
	}
	buf = buf[:0]
	p.animatedScratch.Put(&buf)
}

func traceOverride(p *vectorPath, ov gui.SvgAnimAttrOverride) {
	check := func(name string, bit gui.SvgAnimAttrMask, v float32) {
		if ov.Mask&bit == 0 {
			return
		}
		if !finiteF32(v) || v < -1e4 || v > 1e4 {
			log.Printf("svg trace: gid=%q attr=%s val=%v "+
				"mask=%b additive=%b",
				p.GroupID, name, v, ov.Mask, ov.AdditiveMask)
		}
	}
	check("cx", gui.SvgAnimMaskCX, ov.CX)
	check("cy", gui.SvgAnimMaskCY, ov.CY)
	check("r", gui.SvgAnimMaskR, ov.R)
	check("rx", gui.SvgAnimMaskRX, ov.RX)
	check("ry", gui.SvgAnimMaskRY, ov.RY)
	check("x", gui.SvgAnimMaskX, ov.X)
	check("y", gui.SvgAnimMaskY, ov.Y)
	check("width", gui.SvgAnimMaskWidth, ov.Width)
	check("height", gui.SvgAnimMaskHeight, ov.Height)
}

// traceAnimatedTriangles logs animated paths whose bbox escapes 2x
// viewBox — diagnostic for spurious full-cell fills.
func traceAnimatedTriangles(vg *vectorGraphic,
	paths []gui.TessellatedPath, animated []vectorPath,
	overrides map[uint32]gui.SvgAnimAttrOverride,
) {
	xLim := vg.Width * 2
	yLim := vg.Height * 2
	for i := range paths {
		tris := paths[i].Triangles
		if len(tris) == 0 {
			continue
		}
		minX, minY := tris[0], tris[1]
		maxX, maxY := minX, minY
		for j := 2; j+1 < len(tris); j += 2 {
			if tris[j] < minX {
				minX = tris[j]
			}
			if tris[j] > maxX {
				maxX = tris[j]
			}
			if tris[j+1] < minY {
				minY = tris[j+1]
			}
			if tris[j+1] > maxY {
				maxY = tris[j+1]
			}
		}
		if finiteF32(minX) && finiteF32(minY) &&
			finiteF32(maxX) && finiteF32(maxY) &&
			maxX-minX <= xLim && maxY-minY <= yLim {
			continue
		}
		var primStr, ovStr string
		if i < len(animated) {
			p := &animated[i]
			primStr = fmt.Sprintf("prim={Kind:%d CX:%.3f CY:%.3f "+
				"R:%.3f RX:%.3f RY:%.3f X:%.3f Y:%.3f W:%.3f H:%.3f}",
				p.Primitive.Kind, p.Primitive.CX, p.Primitive.CY,
				p.Primitive.R, p.Primitive.RX, p.Primitive.RY,
				p.Primitive.X, p.Primitive.Y,
				p.Primitive.W, p.Primitive.H)
			if ov, ok := overrides[p.PathID]; ok {
				ovStr = fmt.Sprintf("ov={Mask:%b Add:%b CX:%.3f CY:%.3f "+
					"R:%.3f RX:%.3f RY:%.3f X:%.3f Y:%.3f W:%.3f H:%.3f}",
					ov.Mask, ov.AdditiveMask, ov.CX, ov.CY,
					ov.R, ov.RX, ov.RY, ov.X, ov.Y, ov.Width, ov.Height)
			}
		}
		log.Printf("svg trace: pid=%d oversized tris "+
			"bbox=(%.2f,%.2f)-(%.2f,%.2f) vb=%.0fx%.0f nTris=%d %s %s",
			paths[i].PathID, minX, minY, maxX, maxY,
			vg.Width, vg.Height, len(tris)/6, primStr, ovStr)
	}
}

// applyOverridesToPath mutates p's primitive fields and segments to
// reflect the live animation overrides. Only primitive paths react
// to CX/CY/R/...; dash overrides apply regardless of kind since
// stroke-dasharray/offset work on any path. AdditiveMask bits add
// the override to the parsed base value; non-additive bits replace.
func applyOverridesToPath(p *vectorPath, ov gui.SvgAnimAttrOverride) {
	if svgTrace {
		traceOverride(p, ov)
	}
	if ov.Mask&gui.SvgAnimMaskStrokeDashArray != 0 {
		n := min(int(ov.StrokeDashArrayLen), gui.SvgAnimDashArrayCap)
		// Fresh alloc required: clone shares backing with cached
		// src; in-place mutation would corrupt the cache.
		p.strokeDasharray = slices.Clone(ov.StrokeDashArray[:n])
	}
	if ov.Mask&gui.SvgAnimMaskStrokeDashOffset != 0 {
		if ov.AdditiveMask&gui.SvgAnimMaskStrokeDashOffset != 0 {
			p.StrokeDashOffset += ov.StrokeDashOffset
		} else {
			p.StrokeDashOffset = ov.StrokeDashOffset
		}
	}
	prim := p.Primitive
	switch prim.Kind {
	case gui.SvgPrimCircle:
		prim.CX = overrideScalar(prim.CX, ov.CX, &ov, gui.SvgAnimMaskCX)
		prim.CY = overrideScalar(prim.CY, ov.CY, &ov, gui.SvgAnimMaskCY)
		prim.R = nonNegF32(overrideScalar(prim.R, ov.R, &ov,
			gui.SvgAnimMaskR))
		p.Segments = segmentsForEllipse(prim.CX, prim.CY, prim.R, prim.R)
	case gui.SvgPrimEllipse:
		prim.CX = overrideScalar(prim.CX, ov.CX, &ov, gui.SvgAnimMaskCX)
		prim.CY = overrideScalar(prim.CY, ov.CY, &ov, gui.SvgAnimMaskCY)
		prim.RX = nonNegF32(overrideScalar(prim.RX, ov.RX, &ov,
			gui.SvgAnimMaskRX))
		prim.RY = nonNegF32(overrideScalar(prim.RY, ov.RY, &ov,
			gui.SvgAnimMaskRY))
		p.Segments = segmentsForEllipse(prim.CX, prim.CY, prim.RX, prim.RY)
	case gui.SvgPrimRect:
		prim.X = overrideScalar(prim.X, ov.X, &ov, gui.SvgAnimMaskX)
		prim.Y = overrideScalar(prim.Y, ov.Y, &ov, gui.SvgAnimMaskY)
		prim.W = nonNegF32(overrideScalar(prim.W, ov.Width, &ov,
			gui.SvgAnimMaskWidth))
		prim.H = nonNegF32(overrideScalar(prim.H, ov.Height, &ov,
			gui.SvgAnimMaskHeight))
		prim.RX = nonNegF32(overrideScalar(prim.RX, ov.RX, &ov,
			gui.SvgAnimMaskRX))
		prim.RY = nonNegF32(overrideScalar(prim.RY, ov.RY, &ov,
			gui.SvgAnimMaskRY))
		p.Segments = segmentsForRect(prim.X, prim.Y, prim.W, prim.H,
			prim.RX, prim.RY)
	case gui.SvgPrimLine:
		prim.X = overrideScalar(prim.X, ov.X, &ov, gui.SvgAnimMaskX)
		prim.Y = overrideScalar(prim.Y, ov.Y, &ov, gui.SvgAnimMaskY)
		p.Segments = segmentsForLine(prim.X, prim.Y, prim.X2, prim.Y2)
	}
	p.Primitive = prim
}

// overrideScalar returns base, base+v (additive), or v (replace).
// Non-finite v falls back to base — would otherwise tessellate into
// huge/fullscreen triangles.
func overrideScalar(base, v float32, ov *gui.SvgAnimAttrOverride,
	bit gui.SvgAnimAttrMask) float32 {
	if ov.Mask&bit == 0 {
		return base
	}
	if !finiteF32(v) {
		return base
	}
	if ov.AdditiveMask&bit != 0 {
		return base + v
	}
	return v
}

func finiteF32(f float32) bool {
	return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0)
}

// nonNegF32 maps NaN / negative → 0. Negative R/W/H tessellate with
// reversed winding under non-zero fill — i.e. the whole cell.
func nonNegF32(v float32) float32 {
	if v != v || v < 0 {
		return 0
	}
	return v
}
