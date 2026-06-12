package gui

import (
	"cmp"
	"log"
	"math"
	"slices"
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

// svgAnimState holds computed per-group animation state.
type svgAnimState struct {
	AttrOverride SvgAnimAttrOverride // attribute overrides for re-tessellation
	RotAngle     float32             // rotation degrees
	RotCX        float32             // rotation center X (SVG space)
	RotCY        float32             // rotation center Y (SVG space)
	Opacity      float32             // 0..1; <animate attributeName="opacity">
	// FillOpacity / StrokeOpacity track the per-paint opacity
	// animations. They scale only the matching path role at render
	// time so a fill-opacity animation does not dim the stroke.
	FillOpacity   float32
	StrokeOpacity float32
	// TransX/TransY is the animated translate; ScaleX/ScaleY the
	// animated scale. Identity when HasXform is false.
	TransX, TransY float32
	ScaleX, ScaleY float32
	// FillColor / StrokeColor carry CSS color-tween results
	// (SvgAnimColor). Has*Color flags whether the channel is set;
	// emitSvgPathRenderer overrides the path color when the
	// matching path role is set.
	FillColor      SvgColor
	StrokeColor    SvgColor
	HasXform       bool
	Inited         bool
	HasFillColor   bool
	HasStrokeColor bool
}

// computeSvgAnimations builds a map of per-group animation state
// from parsed SMIL animations and elapsed time. Implements SMIL
// "sandwich" semantics: each animation's last activation time is
// computed (BeginSec + n*Cycle for the largest n with that <=
// elapsed); contributions are sorted by activation ascending and
// applied last-write-wins per attribute. fill="freeze" lets a past
// animation continue to contribute its last keyframe value until
// its cycle restarts or a later-activated animation overrides.
func computeSvgAnimations(
	anims []SvgAnimation, elapsedSec float32,
	states map[uint32]svgAnimState,
) map[uint32]svgAnimState {
	return computeSvgAnimationsReuse(anims, elapsedSec, states, nil, nil)
}

// computeSvgAnimationsReuse is the render-path variant that
// accepts a scratch []animContrib to avoid per-frame allocation.
// baseByPath seeds per-PathID state with the author's decomposed
// base transform so additive/replace animations compose over it.
// Pass nil when no base seeding is needed (tests, no-base assets).
func computeSvgAnimationsReuse(
	anims []SvgAnimation, elapsedSec float32,
	states map[uint32]svgAnimState,
	contribScratch []animContrib,
	baseByPath map[uint32]svgBaseXform,
) map[uint32]svgAnimState {
	if states == nil {
		states = make(map[uint32]svgAnimState, len(anims))
	} else {
		clear(states)
	}
	contribs := collectAnimContribs(anims, elapsedSec, contribScratch)
	if len(contribs) == 0 {
		return states
	}
	slices.SortStableFunc(contribs, cmpAnimContrib)
	for i := range contribs {
		applyAnimContrib(&contribs[i], states, baseByPath)
	}
	return states
}

// cmpAnimContrib orders contributions by ascending activation
// time so sandwich priority applies last-write-wins.
func cmpAnimContrib(a, b animContrib) int {
	return cmp.Compare(a.activation, b.activation)
}

// animContrib is one animation's evaluated contribution at the
// current frame: the resolved value(s), the last activation time
// used for sandwich-priority ordering, and a back-pointer to the
// animation for kind/group dispatch. frac carries the raw [0,1]
// phase for kinds (SvgAnimDashArray) whose apply step needs the
// un-lerped fraction to do its own per-slot interpolation.
type animContrib struct {
	anim       *SvgAnimation
	value      float32
	valueX     float32
	valueY     float32
	frac       float32
	activation float32
	// colorVal carries the lerped RGBA result for SvgAnimColor;
	// other kinds leave it zero.
	colorVal SvgColor
}

// collectAnimContribs evaluates every animation's phase against
// elapsed and returns the subset that contributes this frame
// (active or frozen). Skipped animations: no target paths, missing
// dur, not-yet-activated (elapsed < BeginSec), past dur with !Freeze.
// reuse may be a scratch slice whose backing array will be
// reused in place; pass nil to allocate a fresh slice.
func collectAnimContribs(
	anims []SvgAnimation, elapsedSec float32,
	reuse []animContrib,
) []animContrib {
	out := reuse[:0]
	if cap(out) < len(anims) {
		out = make([]animContrib, 0, len(anims))
	}
	for i := range anims {
		a := &anims[i]
		// Reject non-finite timing fields so downstream Lerp / floor
		// math cannot produce NaN values that would propagate into
		// render state. DurSec must be strictly positive for normal
		// animations; <set> is zero-duration and bypasses this check.
		if len(a.TargetPathIDs) == 0 ||
			!isFiniteF(a.DurSec) ||
			(!a.IsSet && a.DurSec <= 0) ||
			!isFiniteF(a.BeginSec) ||
			!isFiniteF(a.Cycle) ||
			!isFiniteF(elapsedSec) {
			continue
		}
		if elapsedSec < a.BeginSec && !a.FillBackwards {
			continue
		}
		var (
			activation = a.BeginSec
			frac       float32
		)
		switch {
		case a.IsSet:
			if elapsedSec < a.BeginSec {
				continue
			}
			frac = 1
		case a.Iterations > 0:
			ok, f, act := cssIterPhase(a, elapsedSec)
			if !ok {
				continue
			}
			frac, activation = f, act
		default:
			ok, f, act := smilPhase(a, elapsedSec)
			if !ok {
				continue
			}
			frac, activation = f, act
		}
		c := animContrib{anim: a, activation: activation, frac: frac}
		if !evalAnimContrib(&c, a, frac, activation) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// smilPhase computes SMIL-style phase: cycle-based re-fire,
// freeze on overrun, single play when Cycle==0.
func smilPhase(a *SvgAnimation, elapsedSec float32) (bool, float32, float32) {
	activation := a.BeginSec
	if a.Cycle > 0 && a.Restart != SvgAnimRestartNever {
		n := math.Floor(float64(elapsedSec-a.BeginSec) / float64(a.Cycle))
		if a.Restart == SvgAnimRestartWhenNotActive && n > 0 {
			prev := a.BeginSec + float32(n-1)*a.Cycle
			if elapsedSec-prev < a.DurSec {
				n--
			}
		}
		activation = a.BeginSec + float32(n)*a.Cycle
	}
	phase := elapsedSec - activation
	switch {
	case phase < a.DurSec:
		return true, phase / a.DurSec, activation
	case a.Freeze:
		return true, 1, activation
	}
	return false, 0, activation
}

// cssIterPhase computes CSS-style phase: a fixed iteration count,
// each iteration of length DurSec. Alternate flips the phase on
// odd iterations. FillBackwards contributes frac=0 before BeginSec.
func cssIterPhase(a *SvgAnimation, elapsedSec float32) (bool, float32, float32) {
	if elapsedSec < a.BeginSec {
		// FillBackwards already gated upstream.
		frac := float32(0)
		if a.Alternate && a.Iterations != SvgAnimIterInfinite &&
			(int(a.Iterations)-1)%2 == 1 {
			frac = 1
		}
		return true, frac, a.BeginSec
	}
	phase := elapsedSec - a.BeginSec
	iter := int(math.Floor(float64(phase) / float64(a.DurSec)))
	iterPhase := phase - float32(iter)*a.DurSec
	if a.Iterations != SvgAnimIterInfinite && iter >= int(a.Iterations) {
		if !a.Freeze {
			return false, 0, a.BeginSec
		}
		iter = int(a.Iterations) - 1
		iterPhase = a.DurSec
	}
	frac := iterPhase / a.DurSec
	if frac > 1 {
		frac = 1
	}
	if a.Alternate && iter%2 == 1 {
		frac = 1 - frac
	}
	return true, frac, a.BeginSec
}

// evalAnimContrib fills c with the kind-specific lerped values for
// this frame. Returns false when the animation lacks enough data to
// contribute (too few values, stride mismatch, etc.).
func evalAnimContrib(c *animContrib, a *SvgAnimation,
	frac, activation float32) bool {
	switch a.Kind {
	case SvgAnimRotate, SvgAnimOpacity, SvgAnimAttr, SvgAnimDashOffset:
		if len(a.Values) < 2 {
			return false
		}
		c.value = lerpKeyframes(
			a.Values, a.KeySplines, a.KeyTimes, a.CalcMode, frac)
		if a.Accumulate {
			c.value += accumOffset(a, activation) *
				(a.Values[len(a.Values)-1] - a.Values[0])
		}
	case SvgAnimDashArray:
		k := int(a.DashKeyframeLen)
		if k <= 0 || k > SvgAnimDashArrayCap || len(a.Values) < 2*k {
			return false
		}
	case SvgAnimTranslate, SvgAnimScale:
		if len(a.Values) < 4 {
			return false
		}
		c.valueX, c.valueY = lerpKeyframes2D(
			a.Values, a.KeySplines, a.KeyTimes, a.CalcMode, frac)
		if a.Accumulate {
			n := accumOffset(a, activation)
			last := len(a.Values) - 2
			c.valueX += n * (a.Values[last] - a.Values[0])
			c.valueY += n * (a.Values[last+1] - a.Values[1])
		}
	case SvgAnimMotion:
		if len(a.MotionPath) < 4 || len(a.MotionLengths) < 2 {
			return false
		}
		c.valueX, c.valueY, c.value = motionSample(a, frac)
	case SvgAnimColor:
		if len(a.ColorValues) < 2 {
			return false
		}
		c.colorVal = lerpColorKeyframes(
			a.ColorValues, a.KeySplines, a.KeyTimes, a.CalcMode, frac)
	}
	return true
}

// lerpColorKeyframes interpolates packed RGBA stops at frac in
// [0,1]. Uses locateSeg for consistent discrete/spline/keyTimes
// behavior. sRGB Lerp per channel — cheap and visually acceptable
// for short tweens (premultiplied gamma-correct Lerp is a follow-up).
func lerpColorKeyframes(vals []uint32, splines, keyTimes []float32,
	mode SvgAnimCalcMode, frac float32) SvgColor {
	n := len(vals)
	idx, t, atEnd := locateSeg(n, frac, splines, keyTimes, mode)
	if atEnd {
		return unpackRGBA(vals[n-1])
	}
	if mode == SvgAnimCalcDiscrete {
		return unpackRGBA(vals[idx])
	}
	a := unpackRGBA(vals[idx])
	b := unpackRGBA(vals[idx+1])
	return SvgColor{
		R: lerpU8(a.R, b.R, t),
		G: lerpU8(a.G, b.G, t),
		B: lerpU8(a.B, b.B, t),
		A: lerpU8(a.A, b.A, t),
	}
}

func unpackRGBA(v uint32) SvgColor {
	return SvgColor{
		R: uint8(v >> 24),
		G: uint8(v >> 16),
		B: uint8(v >> 8),
		A: uint8(v),
	}
}

func lerpU8(a, b uint8, t float32) uint8 {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	v := float32(a) + (float32(b)-float32(a))*t
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(v)
}

// motionSample interpolates along an animateMotion's flattened path
// by arc length. frac ∈ [0,1] scales to [0, totalLen]; returns the
// (x,y) point and — when MotionRotate==auto — the tangent angle in
// degrees. Returns zeros when lens/poly are inconsistent or the
// total length is non-finite — the caller treats that as the
// identity contribution.
func motionSample(a *SvgAnimation, frac float32) (float32, float32, float32) {
	lens := a.MotionLengths
	poly := a.MotionPath
	n := len(lens)
	if n < 2 || len(poly) < 2*n {
		return 0, 0, 0
	}
	total := lens[n-1]
	if !isFiniteF(total) || total < 0 {
		return 0, 0, 0
	}
	target := clampUnit(frac) * total
	idx := 0
	for i := 1; i < n; i++ {
		if lens[i] >= target {
			idx = i - 1
			break
		}
		idx = i
	}
	if idx >= n-1 {
		idx = n - 2
	}
	span := lens[idx+1] - lens[idx]
	var t float32
	if span > 0 {
		t = (target - lens[idx]) / span
	}
	x0, y0 := poly[idx*2], poly[idx*2+1]
	x1, y1 := poly[(idx+1)*2], poly[(idx+1)*2+1]
	x := x0 + (x1-x0)*t
	y := y0 + (y1-y0)*t
	var angle float32
	if a.MotionRotate != SvgAnimMotionRotateNone {
		dx := x1 - x0
		dy := y1 - y0
		angle = float32(math.Atan2(float64(dy), float64(dx))) *
			(180 / math.Pi)
		if a.MotionRotate == SvgAnimMotionRotateAutoReverse {
			angle += 180
		}
	}
	return x, y, angle
}

// accumOffset returns the repeat-count offset for accumulate=sum.
// The count is floor((activation - BeginSec) / Cycle) — the number
// of completed prior cycles. Cycle must be >0.
func accumOffset(a *SvgAnimation, activation float32) float32 {
	if a.Cycle <= 0 {
		return 0
	}
	return float32(math.Floor(
		float64(activation-a.BeginSec) / float64(a.Cycle)))
}

// applyAnimContrib writes one contribution into every target path's
// state, initializing per-path state on first touch. Replace
// semantics (not multiply) — sandwich priority is enforced by the
// sort ordering before this call.
func applyAnimContrib(c *animContrib, states map[uint32]svgAnimState,
	baseByPath map[uint32]svgBaseXform) {
	a := c.anim
	for _, pid := range a.TargetPathIDs {
		applyAnimContribToPath(c, a, pid, states, baseByPath)
	}
}

func applyAnimContribToPath(c *animContrib, a *SvgAnimation, pid uint32,
	states map[uint32]svgAnimState,
	baseByPath map[uint32]svgBaseXform) {
	st := states[pid]
	base, hasBase := baseByPath[pid]
	if !st.Inited {
		st.Opacity = 1
		st.FillOpacity = 1
		st.StrokeOpacity = 1
		st.ScaleX = 1
		st.ScaleY = 1
		if hasBase {
			st.TransX = base.TransX
			st.TransY = base.TransY
			st.ScaleX = base.ScaleX
			st.ScaleY = base.ScaleY
			st.RotAngle = base.RotAngle
			st.HasXform = true
			// Use the base's rotation pivot when present so a SMIL
			// animateTransform that does not touch rotation leaves
			// the author's rotate-about-(cx,cy) intact. Falls back
			// to pivot==offset for pure-translate bases.
			st.RotCX = base.RotCX
			st.RotCY = base.RotCY
			if st.RotCX == 0 && st.RotCY == 0 {
				st.RotCX = base.TransX
				st.RotCY = base.TransY
			}
		}
		st.Inited = true
	}
	// Group-level animations expand to every descendant path. Each
	// descendant may carry its own authored transform (e.g. the 7
	// rects in bars-rotate-fade, each with rotate(N 12 12)). A plain
	// replace would clobber the child's base, so when the anim has
	// multiple targets, compose with the base: sum for rotate/
	// translate, multiply for scale. Same-pivot rotate case exact;
	// differing pivots fall back to the base pivot (lossy but matches
	// the pre-Stage-3 force-bake behavior for icon SVGs).
	inherited := hasBase && len(a.TargetPathIDs) > 1
	switch a.Kind {
	case SvgAnimRotate:
		switch {
		case inherited:
			st.RotAngle = base.RotAngle + c.value
		case a.Additive:
			st.RotAngle += c.value
			st.RotCX = a.CenterX
			st.RotCY = a.CenterY
		default:
			st.RotAngle = c.value
			st.RotCX = a.CenterX
			st.RotCY = a.CenterY
		}
		st.HasXform = true
	case SvgAnimOpacity:
		applyOpacityContrib(&st, c.value, a.Target, a.Additive)
	case SvgAnimAttr:
		applyAttrOverride(&st.AttrOverride, a.AttrName,
			c.value, a.Additive)
	case SvgAnimTranslate:
		switch {
		case inherited:
			st.TransX = base.TransX + c.valueX
			st.TransY = base.TransY + c.valueY
		case a.Additive:
			st.TransX += c.valueX
			st.TransY += c.valueY
		default:
			st.TransX = c.valueX
			st.TransY = c.valueY
		}
		st.HasXform = true
	case SvgAnimScale:
		switch {
		case inherited:
			st.ScaleX = base.ScaleX * c.valueX
			st.ScaleY = base.ScaleY * c.valueY
		case a.Additive:
			st.ScaleX += c.valueX
			st.ScaleY += c.valueY
		default:
			st.ScaleX = c.valueX
			st.ScaleY = c.valueY
		}
		st.HasXform = true
	case SvgAnimMotion:
		switch {
		case inherited:
			st.TransX = base.TransX + c.valueX
			st.TransY = base.TransY + c.valueY
		case a.Additive:
			st.TransX += c.valueX
			st.TransY += c.valueY
		default:
			st.TransX = c.valueX
			st.TransY = c.valueY
		}
		if a.MotionRotate != SvgAnimMotionRotateNone {
			if inherited {
				st.RotAngle = base.RotAngle + c.value
			} else {
				st.RotAngle = c.value
			}
		}
		st.HasXform = true
	case SvgAnimDashOffset:
		ov := &st.AttrOverride
		if a.Additive {
			if ov.Mask&SvgAnimMaskStrokeDashOffset == 0 {
				ov.StrokeDashOffset = c.value
				ov.AdditiveMask |= SvgAnimMaskStrokeDashOffset
			} else {
				ov.StrokeDashOffset += c.value
			}
		} else {
			ov.StrokeDashOffset = c.value
			ov.AdditiveMask &^= SvgAnimMaskStrokeDashOffset
		}
		ov.Mask |= SvgAnimMaskStrokeDashOffset
	case SvgAnimDashArray:
		applyDashArrayContrib(&st.AttrOverride, a, c.frac)
	case SvgAnimColor:
		switch a.Target {
		case SvgAnimTargetStroke:
			st.StrokeColor = c.colorVal
			st.HasStrokeColor = true
		case SvgAnimTargetAll:
			st.FillColor = c.colorVal
			st.HasFillColor = true
			st.StrokeColor = c.colorVal
			st.HasStrokeColor = true
		default:
			st.FillColor = c.colorVal
			st.HasFillColor = true
		}
	}
	states[pid] = st
}

// applyDashArrayContrib lerps a DashKeyframeLen-stride keyframe
// stream at frac into the override StrokeDashArray slots. Uses
// locateSeg for consistent discrete / spline / keyTimes handling
// across all other animation kinds.
func applyDashArrayContrib(ov *SvgAnimAttrOverride,
	a *SvgAnimation, frac float32) {
	k := int(a.DashKeyframeLen)
	n := len(a.Values) / k
	idx, t, atEnd := locateSeg(n, frac, a.KeySplines, a.KeyTimes, a.CalcMode)
	for i := range k {
		var v float32
		switch {
		case atEnd:
			v = a.Values[(n-1)*k+i]
		case a.CalcMode == SvgAnimCalcDiscrete:
			v = a.Values[idx*k+i]
		default:
			v0 := a.Values[idx*k+i]
			v1 := a.Values[(idx+1)*k+i]
			v = v0 + (v1-v0)*t
		}
		ov.StrokeDashArray[i] = v
	}
	ov.StrokeDashArrayLen = uint8(k)
	ov.Mask |= SvgAnimMaskStrokeDashArray
}

// applyOpacityContrib dispatches the opacity value to the correct
// sub-channel, honoring additive=sum. Additive adds to the existing
// channel (init 1 on first touch per sandwich); non-additive
// replaces. Clamping to [0,1] is deferred to render time.
func applyOpacityContrib(st *svgAnimState, v float32,
	target SvgAnimTarget, additive bool) {
	switch target {
	case SvgAnimTargetFill:
		if additive {
			st.FillOpacity += v
		} else {
			st.FillOpacity = v
		}
	case SvgAnimTargetStroke:
		if additive {
			st.StrokeOpacity += v
		} else {
			st.StrokeOpacity = v
		}
	default:
		if additive {
			st.Opacity += v
		} else {
			st.Opacity = v
		}
	}
}

// extractAttrOverrides pulls the AttrOverride from each svgAnimState
// with a non-zero mask into a scratch-backed map keyed by PathID.
// Returns an empty map when no overrides are live.
func extractAttrOverrides(w *Window,
	states map[uint32]svgAnimState,
) map[uint32]SvgAnimAttrOverride {
	overrides := w.scratch.svgAnimOverrides.take(len(states))
	for pid, st := range states {
		if st.AttrOverride.Mask != 0 {
			overrides[pid] = st.AttrOverride
		}
	}
	return overrides
}

// applyAttrOverride writes val into the override field for attr.
// Sandwich semantics with additive:
//   - non-additive: replace the field, clear AdditiveMask bit.
//   - additive, bit unset: set field=val, mark both Mask and
//     AdditiveMask; delta will be applied to the primitive base at
//     re-tessellate time.
//   - additive, bit set: sum val into the existing field; leave
//     AdditiveMask unchanged (pre-existing non-additive value stays
//     pre-resolved, pre-existing additive value stays a delta).
//
// Unknown attr names are no-ops.
func applyAttrOverride(o *SvgAnimAttrOverride,
	attr SvgAttrName, val float32, additive bool) {
	bit := attrMaskBit(attr)
	if bit == 0 {
		return
	}
	f := attrFieldPtr(o, attr)
	if f == nil {
		return
	}
	if additive {
		if o.Mask&bit == 0 {
			*f = val
			o.AdditiveMask |= bit
		} else {
			*f += val
		}
		o.Mask |= bit
		return
	}
	*f = val
	o.Mask |= bit
	o.AdditiveMask &^= bit
}

// attrMaskBit maps a SvgAttrName to its SvgAnimAttrMask bit.
func attrMaskBit(attr SvgAttrName) SvgAnimAttrMask {
	switch attr {
	case SvgAttrCX:
		return SvgAnimMaskCX
	case SvgAttrCY:
		return SvgAnimMaskCY
	case SvgAttrR:
		return SvgAnimMaskR
	case SvgAttrRX:
		return SvgAnimMaskRX
	case SvgAttrRY:
		return SvgAnimMaskRY
	case SvgAttrX:
		return SvgAnimMaskX
	case SvgAttrY:
		return SvgAnimMaskY
	case SvgAttrWidth:
		return SvgAnimMaskWidth
	case SvgAttrHeight:
		return SvgAnimMaskHeight
	}
	return 0
}

// attrFieldPtr returns a pointer to the override field for attr.
func attrFieldPtr(o *SvgAnimAttrOverride,
	attr SvgAttrName) *float32 {
	switch attr {
	case SvgAttrCX:
		return &o.CX
	case SvgAttrCY:
		return &o.CY
	case SvgAttrR:
		return &o.R
	case SvgAttrRX:
		return &o.RX
	case SvgAttrRY:
		return &o.RY
	case SvgAttrX:
		return &o.X
	case SvgAttrY:
		return &o.Y
	case SvgAttrWidth:
		return &o.Width
	case SvgAttrHeight:
		return &o.Height
	}
	return nil
}

// lerpKeyframes interpolates keyframe scalars at frac ∈ [0,1].
// Linear by default; spline bends per-segment t via cubic-bezier;
// discrete returns the covering keyframe. keyTimes, when non-nil,
// overrides uniform i/(n-1) spacing.
func lerpKeyframes(
	vals, splines, keyTimes []float32,
	mode SvgAnimCalcMode, frac float32,
) float32 {
	n := len(vals)
	if n == 0 {
		return 1
	}
	if n == 1 {
		return vals[0]
	}
	idx, t, atEnd := locateSeg(n, frac, splines, keyTimes, mode)
	if atEnd {
		return vals[n-1]
	}
	if mode == SvgAnimCalcDiscrete {
		return vals[idx]
	}
	return vals[idx] + (vals[idx+1]-vals[idx])*t
}

// lerpKeyframes2D interpolates a paired [x0,y0, x1,y1, ...]
// keyframe stream at frac ∈ [0,1].
func lerpKeyframes2D(
	vals, splines, keyTimes []float32,
	mode SvgAnimCalcMode, frac float32,
) (float32, float32) {
	n := len(vals) / 2
	if n == 0 {
		return 0, 0
	}
	if n == 1 {
		return vals[0], vals[1]
	}
	idx, t, atEnd := locateSeg(n, frac, splines, keyTimes, mode)
	if atEnd {
		return vals[(n-1)*2], vals[(n-1)*2+1]
	}
	if mode == SvgAnimCalcDiscrete {
		return vals[idx*2], vals[idx*2+1]
	}
	x0, y0 := vals[idx*2], vals[idx*2+1]
	x1, y1 := vals[(idx+1)*2], vals[(idx+1)*2+1]
	return x0 + (x1-x0)*t, y0 + (y1-y0)*t
}

// locateSeg returns the keyframe segment containing frac: the
// lower index, the intra-segment fraction t (bent by splines when
// mode==Spline), and atEnd when frac lands on or past the last
// keyframe. Segment boundaries come from keyTimes when supplied,
// else uniform i/(n-1) for linear/spline and i/n for discrete.
// frac is clamped; NaN / negative → idx=0, t=0.
func locateSeg(
	n int, frac float32, splines, keyTimes []float32,
	mode SvgAnimCalcMode,
) (int, float32, bool) {
	frac = clampUnit(frac)
	if len(keyTimes) == n {
		return locateSegKeyTimes(n, frac, splines, keyTimes, mode)
	}
	if mode == SvgAnimCalcDiscrete {
		if frac >= 1 {
			return n - 1, 0, true
		}
		idx := int(frac * float32(n))
		if idx >= n {
			idx = n - 1
		}
		return idx, 0, false
	}
	seg := frac * float32(n-1)
	idx := max(int(seg), 0)
	if idx >= n-1 {
		return n - 1, 0, true
	}
	t := seg - float32(idx)
	if mode == SvgAnimCalcSpline && len(splines) == 4*(n-1) {
		off := idx * 4
		t = clampUnit(bezierCalc(t, splines[off], splines[off+1],
			splines[off+2], splines[off+3]))
	}
	return idx, t, false
}

// locateSegKeyTimes walks keyTimes to find the segment covering
// frac. Discrete: keyTimes[i] starts keyframe i. Linear/spline:
// intra-segment t is (frac-keyTimes[i]) / (keyTimes[i+1]-
// keyTimes[i]); zero-width segments (duplicate keyTimes) yield
// t=0 (jump to upper keyframe — matches SMIL discrete-boundary).
func locateSegKeyTimes(
	n int, frac float32, splines, keyTimes []float32,
	mode SvgAnimCalcMode,
) (int, float32, bool) {
	if frac >= keyTimes[n-1] {
		return n - 1, 0, true
	}
	idx := 0
	for i := 0; i < n-1; i++ {
		if frac >= keyTimes[i] && frac < keyTimes[i+1] {
			idx = i
			break
		}
	}
	if mode == SvgAnimCalcDiscrete {
		return idx, 0, false
	}
	span := keyTimes[idx+1] - keyTimes[idx]
	var t float32
	if span > 0 {
		t = (frac - keyTimes[idx]) / span
	}
	if mode == SvgAnimCalcSpline && len(splines) == 4*(n-1) {
		off := idx * 4
		t = clampUnit(bezierCalc(t, splines[off], splines[off+1],
			splines[off+2], splines[off+3]))
	}
	return idx, t, false
}

// blendAlpha multiplies two 0..255 alpha channels as if they were
// in [0,1], rounding toward zero. Used to fold a tint alpha into
// the path's baked alpha without losing per-element opacity.
func blendAlpha(a, b uint8) uint8 {
	return uint8(uint16(a) * uint16(b) / 255)
}

// emitErrorPlaceholder draws a magenta rectangle placeholder.
func emitErrorPlaceholder(x, y, w, h float32, win *Window) {
	if w <= 0 || h <= 0 {
		return
	}
	emitRenderer(RenderCmd{
		Kind:  RenderRect,
		X:     x,
		Y:     y,
		W:     w,
		H:     h,
		Color: Magenta,
		Fill:  true,
	}, win)
	emitRenderer(RenderCmd{
		Kind:      RenderStrokeRect,
		X:         x,
		Y:         y,
		W:         w,
		H:         h,
		Color:     White,
		Thickness: 1,
	}, win)
}
