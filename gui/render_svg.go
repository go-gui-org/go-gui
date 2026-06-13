package gui

import (
	"log"
	"time"
)

// renderSvg renders an SVG shape by loading cached tessellation
// and emitting RenderSvg commands.
func renderSvg(shape *Shape, clip drawClip, w *Window) {
	if !rectsOverlap(shapeBounds(shape), clip) {
		return
	}

	var cached *CachedSvg
	var err error
	if shape.SvgOpts != nil {
		cached, err = w.LoadSvgWithOpts(shape.Resource,
			shape.Width, shape.Height, *shape.SvgOpts)
	} else {
		cached, err = w.LoadSvg(shape.Resource,
			shape.Width, shape.Height)
	}
	if err != nil {
		log.Printf("renderSvg: %v", err)
		emitErrorPlaceholder(shape.X, shape.Y,
			shape.Width, shape.Height, w)
		return
	}

	color := shape.Color
	if shape.Disabled {
		color = dimAlpha(color)
	}

	// Position SVG content per preserveAspectRatio. Align splits
	// the slack (or, under slice, the overflow) along each axis;
	// default xMidYMid centers — historic behavior.
	// SvgAlignNone non-uniformly stretches to fill: scaleX/scaleY
	// are independent, slack is zero, no alignment offset.
	var scaleX, scaleY, sx, sy float32
	if cached.PreserveAlign == SvgAlignNone {
		// Guard against zero, negative, NaN, and Inf viewBox
		// dimensions from a malicious or malformed SVG. Fall
		// back to the uniform tessellation scale so detail
		// level matches and the render stays stable.
		if cached.Width > 0 && cached.Height > 0 &&
			isFiniteF(cached.Width) && isFiniteF(cached.Height) {
			scaleX = shape.Width / cached.Width
			scaleY = shape.Height / cached.Height
		} else {
			scaleX = cached.Scale
			scaleY = cached.Scale
		}
		sx = shape.X - cached.ViewBoxX*scaleX
		sy = shape.Y - cached.ViewBoxY*scaleY
	} else {
		slackX := shape.Width - cached.Width*cached.Scale
		slackY := shape.Height - cached.Height*cached.Scale
		xFrac, yFrac := PreserveAlignFractions(cached.PreserveAlign)
		clipX := shape.X + slackX*xFrac
		clipY := shape.Y + slackY*yFrac
		sx = clipX - cached.ViewBoxX*cached.Scale
		sy = clipY - cached.ViewBoxY*cached.Scale
	}

	// Clip to intersection of parent clip and the shape rect. Under
	// preserveAspectRatio=slice the scaled content is larger than the
	// shape; the shape rect bounds the visible region. Under meet the
	// shape is the bounding box and content fits within it.
	svgClip, ok := rectIntersection(clip, drawClip{
		X:      shape.X,
		Y:      shape.Y,
		Width:  shape.Width,
		Height: shape.Height,
	})
	if !ok {
		return
	}
	emitClipCmd(svgClip, w)

	// Compute animation state for SMIL animations.
	var animState map[uint32]svgAnimState
	if cached.HasAnimations && cached.AnimStartNs != 0 {
		animState = w.scratch.svgAnimStates.take(len(cached.Animations))
		defer w.scratch.svgAnimStates.put(animState)
		nowNs := time.Now().UnixNano()
		// Keep animation alive while SVG is being rendered.
		if cached.AnimHash != "" {
			animSeen := StateMap[string, int64](
				w, nsSvgAnimSeen, capImageCache)
			animSeen.Set(cached.AnimHash, nowNs)
		}
		elapsed := float32(nowNs-cached.AnimStartNs) /
			float32(time.Second)
		contribScratch := w.scratch.svgAnimContribs.take(
			len(cached.Animations))
		animState = computeSvgAnimationsReuse(
			cached.Animations, elapsed, animState, contribScratch,
			cached.BaseByPath)
		w.scratch.svgAnimContribs.put(contribScratch)
	}

	// animByPID lets emitSvgGroup look up override geometry per
	// PathID, so the same map serves main and filtered-group passes.
	var animTris []TessellatedPath
	var animByPID map[uint32][]float32
	if cached.HasAttrAnim && cached.HasAnimatedPaths {
		overrides := extractAttrOverrides(w, animState)
		if len(overrides) > 0 {
			if ap, ok := w.svgParser.(AnimatedSvgParser); ok {
				reuse := w.scratch.svgAnimTriangles.take(0)
				animTris = ap.TessellateAnimated(
					cached.Parsed, cached.Scale, overrides, reuse)
				defer w.scratch.svgAnimTriangles.put(animTris)
				if len(animTris) > 0 {
					animByPID = w.scratch.svgAnimByPID.take(len(animTris))
					defer w.scratch.svgAnimByPID.put(animByPID)
					for i := range animTris {
						animByPID[animTris[i].PathID] = animTris[i].Triangles
					}
				}
			}
			w.scratch.svgAnimOverrides.put(overrides)
		}
	}

	// Emit main paths, text, and textPath elements.
	nonUniform := validNonUniform(scaleX, scaleY, cached.Scale)
	emitSvgGroup(cached.RenderPaths, animByPID, cached.TextDraws,
		cached.TextPathDraws, color, sx, sy,
		cached.Scale, scaleX, scaleY, nonUniform, animState, w)

	// Emit filtered groups.
	for i, fg := range cached.FilteredGroups {
		// Scale the filter bbox and blur; non-uniform stretch
		// uses independent scaleX/scaleY.
		fw := fg.BBox[2] * cached.Scale
		fh := fg.BBox[3] * cached.Scale
		blur := fg.Filter.StdDev * cached.Scale
		if nonUniform {
			fw = fg.BBox[2] * scaleX
			fh = fg.BBox[3] * scaleY
			blur = fg.Filter.StdDev * max(scaleX, scaleY)
		}
		emitRenderer(RenderCmd{
			Kind:       RenderFilterBegin,
			GroupIdx:   i,
			X:          sx,
			Y:          sy,
			W:          fw,
			H:          fh,
			Scale:      cached.Scale,
			BlurRadius: blur,
			Layers:     fg.Filter.BlurLayers,
		}, w)
		emitSvgGroup(fg.RenderPaths, animByPID, fg.TextDraws,
			fg.TextPathDraws, color, sx, sy,
			cached.Scale, scaleX, scaleY, nonUniform, animState, w)
		emitRenderer(RenderCmd{
			Kind: RenderFilterEnd,
		}, w)

		// KeepSource: re-draw sharp original on top of blur.
		if fg.Filter.KeepSource {
			emitSvgGroup(fg.RenderPaths, animByPID, fg.TextDraws,
				fg.TextPathDraws, color, sx, sy,
				cached.Scale, scaleX, scaleY, nonUniform, animState, w)
		}
	}

	// Restore parent clip.
	emitClipCmd(clip, w)
}

// PreserveAlignFractions returns the (x, y) slack fraction for an
// SvgAlign value. xMin / yMin → 0 (origin), xMid / yMid → 0.5
// (center), xMax / yMax → 1 (right/bottom). SvgAlignNone falls back
// to xMidYMid pending non-uniform stretch support.
func PreserveAlignFractions(a SvgAlign) (float32, float32) {
	switch a {
	case SvgAlignXMinYMin:
		return 0, 0
	case SvgAlignXMidYMin:
		return 0.5, 0
	case SvgAlignXMaxYMin:
		return 1, 0
	case SvgAlignXMinYMid:
		return 0, 0.5
	case SvgAlignXMaxYMid:
		return 1, 0.5
	case SvgAlignXMinYMax:
		return 0, 1
	case SvgAlignXMidYMax:
		return 0.5, 1
	case SvgAlignXMaxYMax:
		return 1, 1
	case SvgAlignNone:
		// Non-uniform stretch — slack is zero, fraction irrelevant.
		return 0, 0
	default:
		return 0.5, 0.5
	}
}

// validNonUniform returns true when the caller-supplied non-uniform
// scales are safe (positive, finite) and differ from the uniform
// tessellation scale. NaN, Inf, zero, and negative values are
// rejected so they cannot propagate into backend xform commands.
func validNonUniform(sx, sy, uniform float32) bool {
	return sx > 0 && sy > 0 &&
		isFiniteF(sx) && isFiniteF(sy) &&
		(sx != uniform || sy != uniform)
}

// emitSvgGroup emits paths, text draws, and text path draws.
// animByPID, when non-nil, carries fresh triangles for animated
// primitive shapes keyed by PathID. Animated paths look up their
// override geometry; absent entries fall back to cached triangles.
func emitSvgGroup(
	paths []CachedSvgPath,
	animByPID map[uint32][]float32,
	textDraws []CachedSvgTextDraw,
	textPathDraws []CachedSvgTextPathDraw,
	color Color, sx, sy, scale, scaleX, scaleY float32,
	nonUniform bool,
	animState map[uint32]svgAnimState, w *Window,
) {
	for i := range paths {
		p := paths[i]
		if p.Animated && p.PathID != 0 {
			if tris, ok := animByPID[p.PathID]; ok {
				p.Triangles = tris
			}
		}
		emitSvgPathRenderer(p, color, sx, sy, scale, scaleX, scaleY, nonUniform, animState, w)
	}
	for i := range textDraws {
		emitCachedSvgTextDraw(&textDraws[i], sx, sy, w)
	}
	for i := range textPathDraws {
		emitCachedSvgTextPathDraw(&textPathDraws[i], sx, sy, w)
	}
}

// emitSvgPathRenderer emits a single SVG path as a RenderSvg
// command. If tint has alpha>0 and path has no vertex colors,
// the tint overrides the path color; the path's own alpha is
// modulated in so per-element opacity (baked into path.Color.A
// during parsing) survives the override. animState applies SMIL
// rotation/opacity per GroupID.
func emitSvgPathRenderer(path CachedSvgPath, tint Color,
	x, y, scale, nsScaleX, nsScaleY float32,
	nonUniform bool,
	animState map[uint32]svgAnimState, w *Window) {
	hasVCols := len(path.VertexColors) > 0
	c := path.Color
	if tint.A > 0 && !hasVCols {
		c = tint
		c.A = blendAlpha(tint.A, path.Color.A)
	}
	var vcols []Color
	if hasVCols {
		if tint.A == 0 {
			vcols = path.VertexColors
		} else {
			// Tint active on a gradient path: replace each vertex
			// RGB with tint RGB while modulating its alpha so the
			// gradient's alpha shape (e.g. fade-in tail of tail-
			// spin) survives. vcols is allocated from a frame-
			// scoped arena so repeated renders of tinted gradients
			// avoid a per-frame heap allocation.
			vcols = w.scratch.takeVColors(len(path.VertexColors))
			for i, vc := range path.VertexColors {
				t := tint
				t.A = blendAlpha(tint.A, vc.A)
				vcols[i] = t
			}
			c = tint
		}
	}

	var rotAngle, rotCX, rotCY float32
	var transX, transY, scaleX, scaleY float32
	hasXform := nonUniform
	if nonUniform {
		scaleX = nsScaleX
		scaleY = nsScaleY
	}
	var vAlphaScale float32
	hasVAlpha := false
	var animApplied bool
	if animState != nil && path.PathID != 0 {
		if st, ok := animState[path.PathID]; ok {
			animApplied = true
			rotAngle = st.RotAngle
			rotCX = st.RotCX
			rotCY = st.RotCY
			if !hasVCols {
				switch {
				case path.IsStroke && st.HasStrokeColor:
					nc := svgToColor(st.StrokeColor)
					nc.A = blendAlpha(c.A, nc.A)
					c = nc
				case !path.IsStroke && st.HasFillColor:
					nc := svgToColor(st.FillColor)
					nc.A = blendAlpha(c.A, nc.A)
					c = nc
				}
			}
			if st.HasXform {
				transX = st.TransX
				transY = st.TransY
				if nonUniform {
					scaleX *= st.ScaleX
					scaleY *= st.ScaleY
				} else {
					scaleX = st.ScaleX
					scaleY = st.ScaleY
				}
				hasXform = true
			}
			opa := st.Opacity
			if path.IsStroke {
				opa *= st.StrokeOpacity
			} else {
				opa *= st.FillOpacity
			}
			// Clamp to [0,1] so hostile or out-of-range animation
			// values cannot drive a negative or oversized alpha
			// through the uint8 cast (undefined conversion).
			opa = clampUnit(opa)
			if opa < 1 {
				c.A = uint8(float32(c.A) * opa)
				if len(vcols) > 0 {
					vAlphaScale = opa
					hasVAlpha = true
				}
			}
		}
	}
	if !animApplied && path.HasBaseXform {
		transX = path.BaseTransX
		transY = path.BaseTransY
		if nonUniform && hasXform {
			scaleX *= path.BaseScaleX
			scaleY *= path.BaseScaleY
		} else {
			scaleX = path.BaseScaleX
			scaleY = path.BaseScaleY
		}
		rotAngle = path.BaseRotAngle
		hasXform = true
		// seedFromTransform absorbs the translate column into a
		// rotation pivot when rotation is present, so the decomposed
		// base replays as R_(rcx,rcy)(v*scale + (0,0)). Fall back to
		// pivot==offset for pure-translate bases where rcx/rcy are
		// zero but BaseTransX/Y carry the translation.
		rotCX = path.BaseRotCX
		rotCY = path.BaseRotCY
		if rotCX == 0 && rotCY == 0 {
			rotCX = transX
			rotCY = transY
		}
	}

	// Non-uniform stretch (SvgAlignNone): neutralise the uniform
	// Scale so the backend applies only ScaleX/ScaleY from HasXform.
	effScale := scale
	if nonUniform {
		effScale = 1
	}

	emitRenderer(RenderCmd{
		Kind:             RenderSvg,
		Triangles:        path.Triangles,
		Color:            c,
		VertexColors:     vcols,
		VertexAlphaScale: vAlphaScale,
		HasVertexAlpha:   hasVAlpha,
		X:                x,
		Y:                y,
		Scale:            effScale,
		IsClipMask:       path.IsClipMask,
		ClipGroup:        path.ClipGroup,
		RotAngle:         rotAngle,
		RotCX:            rotCX,
		RotCY:            rotCY,
		TransX:           transX,
		TransY:           transY,
		ScaleX:           scaleX,
		ScaleY:           scaleY,
		HasXform:         hasXform,
	}, w)
}

// emitCachedSvgTextDraw emits a cached SVG text draw as a
// RenderText command. Takes pointer into CachedSvg.TextDraws
// slice so TextStylePtr remains stable.
func emitCachedSvgTextDraw(draw *CachedSvgTextDraw,
	shapeX, shapeY float32, w *Window) {
	emitRenderer(RenderCmd{
		Kind:         RenderText,
		Text:         draw.Text,
		X:            shapeX + draw.X,
		Y:            shapeY + draw.Y,
		Color:        draw.TextStyle.Color,
		FontName:     draw.TextStyle.Family,
		FontSize:     draw.TextStyle.Size,
		TextWidth:    draw.TextWidth,
		TextStylePtr: &draw.TextStyle,
		TextGradient: draw.Gradient,
	}, w)
}

func emitCachedSvgTextPathDraw(draw *CachedSvgTextPathDraw,
	shapeX, shapeY float32, w *Window) {
	emitRenderer(RenderCmd{
		Kind:         RenderTextPath,
		Text:         draw.Text,
		X:            shapeX,
		Y:            shapeY,
		TextStylePtr: &draw.TextStyle,
		TextPath:     &draw.Path,
	}, w)
}
