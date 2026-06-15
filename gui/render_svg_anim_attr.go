package gui

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
	if k <= 0 {
		return
	}
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
