package gui

import "strconv"

// Layout invariants: the properties the sizing and position passes must
// leave true on every frame, checked as data rather than read by eye.
//
// The sizing rules themselves are docs/specs/layout-sizing-rules.md. This
// file is the executable half: the same walk backs the dev-time
// DebugLayoutInvariants category and the layout fuzz targets, so the two
// cannot drift into disagreeing about what correct means.
//
// It runs from layoutPipeline rather than joining the frame audit in
// debugAudit, for the reason debugCheckStamp does the same: by audit time
// composeLayout has lifted the floats into their own layers, so the
// composed tree no longer says which parent a float was written under, and
// a containment check there would report every float as escaping.

// layoutInvariantEmit receives one violation. The production adapter
// forwards to debugWarn; the fuzz adapter collects. subject is the
// warn-once key, so it names the shape rather than the frame.
type layoutInvariantEmit func(subject, format string, args ...any)

// debugLayoutInvariantsChecked reports whether the invariant walk is on.
// Read once per tree rather than per shape, like debugStampsChecked: the
// answer cannot change while a single pass walks the frame.
func (w *Window) debugLayoutInvariantsChecked() bool {
	return w != nil &&
		DebugCategory(debugMask.Load())&DebugLayoutInvariants != 0
}

// debugCheckLayoutInvariants walks the arranged tree and reports every
// violation through debugWarn. Called from layoutPipeline behind the gate
// above; the walk allocates nothing but the recursion itself.
//
// Containment is skipped for text when no TextMeasurer is injected. Two
// approximations disagree by construction in that state: an unmeasured
// text shape takes fallbackLineHeight, style.Size*1.4
// (gui/text_layout.go:14), while the parent that fits around it takes
// fontHeight, style.Size*1.2 (gui/render_text.go:510). The label then
// overshoots by 0.2em per line through no fault of the layout pass.
// plainTextHeightNoMeasurer already states the contract — "an estimate of
// an estimate — right for 'does this overflow', wrong for any pixel
// assertion" — so a pixel-exact containment check has nothing to say here.
// The other invariants still run: a NaN or a min above a max is a defect
// whether or not the glyph metrics are real.
func (w *Window) debugCheckLayoutInvariants(root *Layout) {
	opts := layoutInvariantOpts{skipTextContainment: w.textMeasurer == nil}
	checkLayoutInvariantsOpts(root, opts,
		func(subject, format string, args ...any) {
			w.debugWarn(debugCheckLayoutInvariant, subject, format, args...)
		})
}

// layoutInvariantOpts carries what the walk cannot see from a Layout
// alone. Kept as a struct rather than a bool so a later exemption does
// not change every call site.
type layoutInvariantOpts struct {
	// skipTextContainment exempts text children from the containment
	// check. Set when no TextMeasurer is injected, where the glyph
	// metrics are an approximation the layout pass cannot be held to.
	skipTextContainment bool
}

// checkLayoutInvariants is the shared walk. Kept free of *Window so the
// fuzz targets can drive it with a collector and no frame state.
func checkLayoutInvariants(root *Layout, emit layoutInvariantEmit) {
	checkLayoutInvariantsOpts(root, layoutInvariantOpts{}, emit)
}

func checkLayoutInvariantsOpts(
	root *Layout, opts layoutInvariantOpts, emit layoutInvariantEmit,
) {
	checkLayoutInvariantsDepth(root, opts, emit, 0)
}

func checkLayoutInvariantsDepth(
	layout *Layout, opts layoutInvariantOpts,
	emit layoutInvariantEmit, depth int,
) {
	// Depth-capped like every other tree walk: past the cap the pipeline
	// leaves nodes at their zero state, so checking them would report the
	// cap rather than a sizing defect (see maxEventDepth).
	if layout == nil || layout.Shape == nil || overMaxDepth(depth) {
		return
	}
	s := layout.Shape
	checkShapeFinite(s, emit)
	checkShapeBounds(s, emit)
	checkFillSum(layout, emit)
	for i := range layout.Children {
		child := &layout.Children[i]
		checkChildContainment(s, child, opts, emit)
		checkLayoutInvariantsDepth(child, opts, emit, depth+1)
	}
}

// checkShapeFinite is invariant 3: every emitted dimension and position is
// finite and non-negative. A non-finite size poisons every later pass —
// f32Max returns its second argument for NaN — and a negative one renders
// as an inverted rect.
func checkShapeFinite(s *Shape, emit layoutInvariantEmit) {
	// The subject is built only on a violation: an anonymous shape's
	// name is a string concat, and the clean path must not allocate per
	// shape per frame.
	if !f32AllFinite4(s.X, s.Y, s.Width, s.Height) {
		subject := shapeInvariantSubject(s)
		emit(subject,
			"layout invariant: shape %q resolved to a non-finite rect "+
				"(x=%v y=%v w=%v h=%v); a NaN or Inf here silently wins "+
				"every later f32Max and poisons the scroll range.",
			subject, s.X, s.Y, s.Width, s.Height)
		return
	}
	if s.Width < 0 || s.Height < 0 {
		subject := shapeInvariantSubject(s)
		emit(subject,
			"layout invariant: shape %q resolved to a negative size "+
				"(w=%v h=%v).", subject, s.Width, s.Height)
	}
}

// checkShapeBounds is invariant 2: after clamping, min <= max on both
// axes. Max wins a conflict at every sizing site (see effectiveMinSize),
// so a shape still carrying min > max never went through clampSize.
func checkShapeBounds(s *Shape, emit layoutInvariantEmit) {
	// A zero or negative bound means unset, so only a pair of set bounds
	// can conflict. Subject built lazily, as in checkShapeFinite.
	if s.MaxWidth > 0 && s.MinWidth > s.MaxWidth {
		subject := shapeInvariantSubject(s)
		emit(subject,
			"layout invariant: shape %q has MinWidth %v above MaxWidth "+
				"%v after sizing; every site resolves this with "+
				"effectiveMinSize, so this shape bypassed clampSize.",
			subject, s.MinWidth, s.MaxWidth)
	}
	if s.MaxHeight > 0 && s.MinHeight > s.MaxHeight {
		subject := shapeInvariantSubject(s)
		emit(subject,
			"layout invariant: shape %q has MinHeight %v above MaxHeight "+
				"%v after sizing; every site resolves this with "+
				"effectiveMinSize, so this shape bypassed clampSize.",
			subject, s.MinHeight, s.MaxHeight)
	}
}

// checkFillSum is invariant 4: on the main axis the in-flow children
// plus spacing fill the parent's content box, when at least one Fill
// child is there to take the slack.
//
// A Fill row that cannot fit its minimums (over-constrained) or that
// leaves space undistributed (non-convergence, issue #638) breaks this
// rule. Without it both are silent: the frame renders and nothing
// errors.
//
// These supported outcomes stay quiet: no Fill child on the axis, where
// slack is alignment rather than a defect; a parent that clips or
// scrolls on the axis, where overflow is what the clip and the scroll
// range are for and a viewport gap is fine; a Wrap or Overflow row,
// which skips shrinking by design and breaks rows or hides trailing
// children instead; and every Fill child at its maximum while space
// remains, where the caps are explicit and the gap is alignment slack.
// An axisNone parent places children freely and never distributes, so
// only rows and columns are checked.
func checkFillSum(layout *Layout, emit layoutInvariantEmit) {
	s := layout.Shape
	var axis distributeAxis
	switch s.Axis {
	case axisLeftToRight:
		axis = distributeHorizontal
	case axisTopToBottom:
		axis = distributeVertical
	default:
		return
	}
	if sizingClips(s, axis) {
		return
	}
	if axis == distributeHorizontal && (s.Wrap || s.Overflow) {
		return
	}
	var total float32
	var count int
	var fillCount int
	var fillAtMax int
	for i := range layout.Children {
		cs := layout.Children[i].Shape
		if skipLayoutChild(cs) {
			continue
		}
		var size float32
		var max float32
		var isFill bool
		if axis == distributeHorizontal {
			size = cs.Width
			max = cs.MaxWidth
			isFill = cs.Sizing.Width == sizingFill
		} else {
			size = cs.Height
			max = cs.MaxHeight
			isFill = cs.Sizing.Height == sizingFill
		}
		if !f32IsFinite(size) {
			// The finite check reports it; no second finding here.
			return
		}
		total += size
		count++
		if isFill {
			fillCount++
			if max > 0 && size+f32Tolerance >= max {
				fillAtMax++
			}
		}
	}
	if fillCount == 0 {
		return
	}
	var parentSize float32
	var padding float32
	if axis == distributeHorizontal {
		parentSize = s.Width
		padding = s.paddingWidth()
	} else {
		parentSize = s.Height
		padding = s.paddingHeight()
	}
	if !f32IsFinite(parentSize) || !f32IsFinite(padding) ||
		!f32IsFinite(total) {
		return
	}
	total += layout.spacing()
	content := parentSize - padding
	if !f32IsFinite(content) {
		return
	}
	diff := total - content
	tol := f32Tolerance * float32(count+1)
	if diff <= tol && diff >= -tol {
		return
	}
	// Built only on a violation, like the other checks: the clean path
	// must not allocate per shape per frame.
	subject := shapeInvariantSubject(s)
	if diff < 0 {
		if fillAtMax == fillCount {
			return
		}
		emit(subject,
			"layout invariant: Fill children of %q sum to %v with spacing, "+
				"below the content box %v; %v was left undistributed "+
				"(issue #638).",
			subject, total, content, -diff)
		return
	}
	emit(subject,
		"layout invariant: Fill children of %q sum to %v with spacing, "+
			"above the content box %v; the container is over-constrained "+
			"(issue #638).",
		subject, total, content)
}

// checkChildContainment is invariant 1: a child's extent stays inside its
// parent's bounds.
//
// Bounds, not the content box. Padding says where content prefers to sit,
// not where it is forbidden to go, and several widgets deliberately place
// a child inside the padding: a Button optically centres its label through
// AmendLayout (issue #346), which lands it a pixel or so past the content
// edge. The hard boundary is the parent's own rect, which is what the clip
// is cut from. Checking the content box instead reports those widgets as
// broken while adding nothing — a child escaping by a pixel of padding is
// not the defect this is hunting, a child escaping its parent is.
//
// This runs after layoutAmend, applyLayoutTransition and
// applyHeroTransition, so the tree it reads is the one the renderer gets.
// Checking before them would exempt every callback-positioned and animated
// shape, which is where geometry is hardest to verify by reading.
//
// Two exemptions, each a documented rule rather than a tolerance:
// out-of-flow children (Float, OverDraw, shapeNone) are positioned against
// something other than this parent, and a clipping parent is allowed to be
// sized below its content — that is what the clip is for.
func checkChildContainment(
	parent *Shape, child *Layout, opts layoutInvariantOpts,
	emit layoutInvariantEmit,
) {
	cs := child.Shape
	if cs == nil || skipLayoutChild(cs) {
		return
	}
	// See debugCheckLayoutInvariants: with no TextMeasurer the shape and
	// its parent are sized from two different approximations, so the
	// overshoot is the harness, not the pass.
	if opts.skipTextContainment && cs.shapeType == shapeText {
		return
	}
	// A non-finite rect is already reported by checkShapeFinite; comparing
	// it here would add a second finding for one defect.
	if !f32AllFinite4(parent.X, parent.Width, cs.X, cs.Width) ||
		!f32AllFinite4(parent.Y, parent.Height, cs.Y, cs.Height) {
		return
	}
	if !sizingClips(parent, distributeHorizontal) {
		left := parent.X
		right := parent.X + parent.Width
		if cs.X < left-f32Tolerance ||
			cs.X+cs.Width > right+f32Tolerance {
			subject := shapeInvariantSubject(cs)
			emit(subject,
				"layout invariant: child %q spans x %v..%v, outside its "+
					"parent %q bounds %v..%v, and the parent neither "+
					"clips nor scrolls on this axis.",
				subject, cs.X, cs.X+cs.Width,
				shapeInvariantSubject(parent), left, right)
		}
	}
	if !sizingClips(parent, distributeVertical) {
		top := parent.Y
		bottom := parent.Y + parent.Height
		if cs.Y < top-f32Tolerance ||
			cs.Y+cs.Height > bottom+f32Tolerance {
			subject := shapeInvariantSubject(cs)
			emit(subject,
				"layout invariant: child %q spans y %v..%v, outside its "+
					"parent %q bounds %v..%v, and the parent neither "+
					"clips nor scrolls on this axis.",
				subject, cs.Y, cs.Y+cs.Height,
				shapeInvariantSubject(parent), top, bottom)
		}
	}
}

// shapeInvariantSubject names a shape for the warn-once key. Effective ID
// where there is one, so two instances of the same widget report
// separately. Most of the tree is anonymous, and a bare "" would collapse
// every finding in the frame into a single warn-once slot, so an ID-less
// shape falls back to its type — coarse, but it keeps a container defect
// and a text defect from hiding each other.
func shapeInvariantSubject(s *Shape) string {
	if s == nil {
		return "<nil>"
	}
	if id := s.idKey(); id != "" {
		return id
	}
	return "shapeType " + strconv.Itoa(int(s.shapeType))
}
