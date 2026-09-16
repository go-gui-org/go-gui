package gui

import "math"

// fillBuffers bundles the candidate scratch slice and fill generation
// for the layout fill pipeline. Passing a single *fillBuffers through
// recursive calls avoids a slice-header escape per frame.
type fillBuffers struct {
	candidates []int
	fillGen    uint32 // generation counter for fill-pass cache invalidation
}

// sentinelNextExtrema is a large finite float32 used as "no next extremum
// found yet" in distributeGrow. math.MaxFloat32 overflows to +Inf in
// float32, so math.MaxUint32 (≈4.29e9) is used instead.
const sentinelNextExtrema = float32(math.MaxUint32)

// distributeMode controls whether space distribution grows or shrinks.
type distributeMode uint8

const (
	distributeGrow distributeMode = iota
	distributeShrink
)

// distributeAxis selects the dimension.
type distributeAxis uint8

const (
	distributeHorizontal distributeAxis = iota
	distributeVertical
)

// getSize returns width or height depending on axis.
func getSize(shape *Shape, axis distributeAxis) float32 {
	if axis == distributeHorizontal {
		return shape.Width
	}
	return shape.Height
}

// setSize sets width or height depending on axis.
func setSize(shape *Shape, axis distributeAxis, value float32) {
	if axis == distributeHorizontal {
		shape.Width = value
	} else {
		shape.Height = value
	}
}

func getMinSize(shape *Shape, axis distributeAxis) float32 {
	if axis == distributeHorizontal {
		return shape.MinWidth
	}
	return shape.MinHeight
}

func getMaxSize(shape *Shape, axis distributeAxis) float32 {
	if axis == distributeHorizontal {
		return shape.MaxWidth
	}
	return shape.MaxHeight
}

func getSizing(shape *Shape, axis distributeAxis) sizingType {
	if axis == distributeHorizontal {
		return shape.Sizing.Width
	}
	return shape.Sizing.Height
}

func getPadding(shape *Shape, axis distributeAxis) float32 {
	if axis == distributeHorizontal {
		return shape.paddingWidth()
	}
	return shape.paddingHeight()
}

// siblingSumPtr returns a pointer to siblingSumW or siblingSumH
// depending on axis. Used to read and write the sibling-sum cache
// in layoutFillCrossAxis without repeating the axis branch.
func siblingSumPtr(shape *Shape, axis distributeAxis) *float32 {
	if axis == distributeHorizontal {
		return &shape.siblingSumW
	}
	return &shape.siblingSumH
}

// mainAxisOf returns the layout axis that distributes children
// along the given dimension (horizontal → LeftToRight, etc.).
func mainAxisOf(ax distributeAxis) Axis {
	if ax == distributeHorizontal {
		return axisLeftToRight
	}
	return axisTopToBottom
}

func scrollExcludesAxis(mode scrollMode, axis distributeAxis) bool {
	if axis == distributeHorizontal {
		return mode == ScrollVerticalOnly
	}
	return mode == ScrollHorizontalOnly
}

// scrollFillResetMin drops the minimum of a Scrollable Fill container on
// axis to spacingSmall, so the fill pass can size it to its parent. A scroll
// container is a viewport: content bigger than the viewport is what the
// scroll range is for, so the content's minimum must not become its own.
// Without this a Scrollable FillFill column grew as wide as its content and
// had nothing to scroll sideways (issue #584). An axis the ScrollMode
// excludes keeps its floor, because that axis cannot reveal hidden content.
func scrollFillResetMin(shape *Shape, axis distributeAxis) {
	if !shape.Scrollable || getSizing(shape, axis) != sizingFill ||
		scrollExcludesAxis(shape.ScrollMode, axis) {
		return
	}
	if axis == distributeHorizontal {
		shape.MinWidth = spacingSmall
	} else {
		shape.MinHeight = spacingSmall
	}
}

// fitAxisNoneWidth grows an axis-less container (Canvas) to enclose its
// in-flow children: the furthest right edge (X+width) plus padding (issue
// #584), same rule for MinWidth. Children are not re-sized; a child left of
// the origin, or with no in-flow child at all, adds nothing. A non-finite
// child X is skipped — f32Max returns its second argument for NaN and would
// poison the size otherwise.
func fitAxisNoneWidth(layout *Layout, padding float32) {
	if extent, minExtent, found := axisNoneExtentW(layout); found {
		layout.Shape.Width = f32Max(layout.Shape.Width, extent+padding)
		layout.Shape.MinWidth = f32Max(layout.Shape.MinWidth, minExtent+padding)
	}
}

// fitAxisNoneHeight is fitAxisNoneWidth for the vertical axis, with Y.
func fitAxisNoneHeight(layout *Layout, padding float32) {
	if extent, minExtent, found := axisNoneExtentH(layout); found {
		layout.Shape.Height = f32Max(layout.Shape.Height, extent+padding)
		layout.Shape.MinHeight = f32Max(layout.Shape.MinHeight, minExtent+padding)
	}
}

// axisNoneExtentW returns the furthest in-flow child right edge (X plus
// extent) and the same for MinWidth, skipping non-finite sums. found is
// false when no child is in flow. Shared by the sizing fit and the rotation
// re-fit so the two cannot drift apart.
func axisNoneExtentW(layout *Layout) (extent, minExtent float32, found bool) {
	for i := range layout.Children {
		c := layout.Children[i].Shape
		if skipLayoutChild(c) {
			continue
		}
		found = true
		if e := c.X + childExtentW(c); f32IsFinite(e) {
			extent = f32Max(extent, e)
		}
		if m := c.X + c.MinWidth; f32IsFinite(m) {
			minExtent = f32Max(minExtent, m)
		}
	}
	return extent, minExtent, found
}

// axisNoneExtentH is axisNoneExtentW for the vertical axis, with Y.
func axisNoneExtentH(layout *Layout) (extent, minExtent float32, found bool) {
	for i := range layout.Children {
		c := layout.Children[i].Shape
		if skipLayoutChild(c) {
			continue
		}
		found = true
		if e := c.Y + c.Height; f32IsFinite(e) {
			extent = f32Max(extent, e)
		}
		if m := c.Y + c.MinHeight; f32IsFinite(m) {
			minExtent = f32Max(minExtent, m)
		}
	}
	return extent, minExtent, found
}

func clampMinMax(shape *Shape, axis distributeAxis) {
	setSize(shape, axis, clampSize(getSize(shape, axis),
		getMinSize(shape, axis), getMaxSize(shape, axis)))
}

// layoutFillCrossAxis handles cross-axis fill sizing: adjusts scroll
// containers to fit parent's remaining space, clamps to min/max, and
// propagates fill size to children.
func layoutFillCrossAxis(layout *Layout, axis distributeAxis, fb *fillBuffers) {
	if layout.Shape.Scrollable && getSizing(layout.Shape, axis) == sizingFill &&
		!scrollExcludesAxis(layout.Shape.ScrollMode, axis) &&
		layout.Parent != nil && layout.Parent.Shape.Axis == mainAxisOf(axis) {
		// Use cached sibling sum from parent Shape to avoid O(n²)
		// iteration when multiple siblings trigger cross-axis fill.
		// fillGen 0 is the zero value (no cache); compare against
		// fb.fillGen which is ≥1 after beginFillPass.
		//
		// One siblingSumGen stamps both siblingSumW and siblingSumH, and
		// both fill passes share one fillGen (see Shape.siblingSumGen).
		// That is safe only because a parent writes one of the two per
		// frame: this branch runs when the parent's axis is
		// mainAxisOf(axis), and a parent has a single axis. A parent that
		// filled on both axes would read the other axis's stale sum.
		parentShape := layout.Parent.Shape
		sum := siblingSumPtr(parentShape, axis)
		var totalChild float32
		if fb.fillGen != 0 && parentShape.siblingSumGen == fb.fillGen {
			totalChild = *sum
		} else {
			for j := range layout.Parent.Children {
				// In-flow children only. The target below subtracts
				// layout.Parent.spacing(), which counts this same set, so
				// summing everything would let a Float or hidden sibling
				// take budget that no fence post accounts for and leave
				// this Fill child short.
				//
				// The total deliberately does not depend on which child is
				// asking: it is cached on the parent and read by every
				// sibling that fills on this axis.
				if skipLayoutChild(layout.Parent.Children[j].Shape) {
					continue
				}
				totalChild += getSize(layout.Parent.Children[j].Shape, axis)
			}
			*sum = totalChild
			parentShape.siblingSumGen = fb.fillGen
		}
		// Remove self only when self is part of that sum. A Float or
		// hidden container that still fills on the cross axis was never
		// added, so subtracting it would hand out its size twice.
		sibling := totalChild
		if !skipLayoutChild(layout.Shape) {
			sibling -= getSize(layout.Shape, axis)
		}
		target := getSize(layout.Parent.Shape, axis) - sibling -
			layout.Parent.spacing() - getPadding(layout.Parent.Shape, axis)
		setSize(layout.Shape, axis, f32Max(0, target))
	}
	clampMinMax(layout.Shape, axis)
	remaining := getSize(layout.Shape, axis) - getPadding(layout.Shape, axis)
	for i := range layout.Children {
		if getSizing(layout.Children[i].Shape, axis) == sizingFill {
			setSize(layout.Children[i].Shape, axis, remaining)
			clampMinMax(layout.Children[i].Shape, axis)
		}
	}
}

type distributionExtrema struct {
	extremum    float32
	nextExtrema float32
}

func collectDistributionCandidates(layout *Layout, axis distributeAxis, fb *fillBuffers) {
	fb.candidates = fb.candidates[:0]
	for i := range layout.Children {
		// Out-of-flow children take no slot in the row, so they take no
		// share of its budget either. The remaining budget above already
		// drops them (matching layout.spacing()), so dealing them in here
		// would let a Float Fill shrink its in-flow siblings.
		if skipLayoutChild(layout.Children[i].Shape) {
			continue
		}
		if getSizing(layout.Children[i].Shape, axis) == sizingFill {
			fb.candidates = append(fb.candidates, i)
		}
	}
}

func shouldContinueDistribution(remaining float32, mode distributeMode, fillCount int) bool {
	if !f32IsFinite(remaining) || fillCount == 0 {
		return false
	}
	if mode == distributeGrow {
		return remaining > f32Tolerance
	}
	return remaining < -f32Tolerance
}

// findDistributionExtrema searches the Fill children only. Fixed and Fit
// siblings cannot change size, so a larger one used to become the extremum
// while no Fill child matched it: nothing shrank and the row overflowed.
func findDistributionExtrema(layout *Layout, axis distributeAxis, mode distributeMode, fillIndices []int) (distributionExtrema, bool) {
	if len(fillIndices) == 0 {
		return distributionExtrema{}, false
	}
	extrema := getSize(layout.Children[fillIndices[0]].Shape, axis)
	var nextExtrema float32
	if mode == distributeGrow {
		nextExtrema = sentinelNextExtrema
	}

	for _, idx := range fillIndices {
		childSize := getSize(layout.Children[idx].Shape, axis)
		if mode == distributeGrow {
			if childSize < extrema {
				nextExtrema = extrema
				extrema = childSize
			} else if childSize > extrema {
				nextExtrema = f32Min(nextExtrema, childSize)
			}
		} else {
			if childSize > extrema {
				nextExtrema = extrema
				extrema = childSize
			} else if childSize < extrema {
				nextExtrema = f32Max(nextExtrema, childSize)
			}
		}
	}
	if !f32IsFinite(extrema) || !f32IsFinite(nextExtrema) {
		return distributionExtrema{}, false
	}
	return distributionExtrema{extremum: extrema, nextExtrema: nextExtrema}, true
}

func computeDistributionDelta(layout *Layout, remaining float32, mode distributeMode, axis distributeAxis, extrema distributionExtrema, fillCount int) (float32, bool) {
	var sizeDelta float32
	if mode == distributeGrow {
		if extrema.nextExtrema == sentinelNextExtrema {
			sizeDelta = remaining
		} else {
			sizeDelta = extrema.nextExtrema - extrema.extremum
		}
	} else {
		if extrema.extremum > 0 {
			if extrema.nextExtrema == 0 {
				sizeDelta = remaining
			} else {
				sizeDelta = extrema.nextExtrema - extrema.extremum
			}
		} else {
			sizeDelta = remaining
		}
	}
	if !f32IsFinite(sizeDelta) {
		return 0, false
	}
	// Only Fill children take a share: a Fixed sibling cannot absorb one.
	if mode == distributeGrow {
		sizeDelta = f32Min(sizeDelta, remaining/float32(fillCount))
	} else {
		sizeDelta = f32Max(sizeDelta, remaining/float32(fillCount))
	}
	if !f32IsFinite(sizeDelta) {
		return 0, false
	}
	saneDeltaLimit := f32Max(f32Abs(getSize(layout.Shape, axis)), f32Abs(remaining))
	saneDeltaLimit = f32Max(saneDeltaLimit*4, 1_000_000)
	if !f32IsFinite(saneDeltaLimit) || saneDeltaLimit <= 0 {
		return 0, false
	}
	return f32Clamp(sizeDelta, -saneDeltaLimit, saneDeltaLimit), true
}

func applyDistributionDelta(layout *Layout, axis distributeAxis, extremum, sizeDelta, remainingIn float32, fb *fillBuffers) (float32, bool) {
	remaining := remainingIn
	keepIdx := 0
	fi := fb.candidates
	for i := range fi {
		idx := fi[i]
		child := &layout.Children[idx]
		keepChild := true
		childSize := getSize(child.Shape, axis)
		if childSize == extremum {
			prevSize := childSize
			newSize := childSize + sizeDelta
			if !f32IsFinite(newSize) {
				return 0, false
			}
			setSize(child.Shape, axis, newSize)

			constrained := false
			maxSize := getMaxSize(child.Shape, axis)
			minSize := effectiveMinSize(getMinSize(child.Shape, axis), maxSize)
			currentSize := getSize(child.Shape, axis)
			if currentSize <= minSize {
				setSize(child.Shape, axis, minSize)
				constrained = true
			} else if maxSize > 0 && currentSize >= maxSize {
				setSize(child.Shape, axis, maxSize)
				constrained = true
			}
			remaining -= getSize(child.Shape, axis) - prevSize
			if !f32IsFinite(remaining) {
				return 0, false
			}
			if constrained {
				keepChild = false
			}
		}
		if keepChild {
			if keepIdx != i {
				fi[keepIdx] = idx
			}
			keepIdx++
		}
	}
	fb.candidates = fi[:keepIdx]
	return remaining, true
}

func distributeSpace(layout *Layout, remainingIn float32, mode distributeMode, axis distributeAxis, fb *fillBuffers) float32 {
	// A residual left here — over-constrained minimums, maximum caps, or
	// a stalled step — is judged downstream by the Fill-sum invariant
	// (checkFillSum, issue #638), so this stays quiet.
	if !f32IsFinite(remainingIn) {
		return 0
	}
	remaining := remainingIn
	prevRemaining := float32(0)

	collectDistributionCandidates(layout, axis, fb)

	for shouldContinueDistribution(remaining, mode, len(fb.candidates)) {
		if f32AreClose(remaining, prevRemaining) {
			break
		}
		prevRemaining = remaining
		extrema, ok := findDistributionExtrema(layout, axis, mode, fb.candidates)
		if !ok {
			break
		}
		sizeDelta, ok := computeDistributionDelta(layout, remaining, mode, axis, extrema, len(fb.candidates))
		if !ok {
			break
		}
		remaining, ok = applyDistributionDelta(layout, axis, extrema.extremum, sizeDelta, remaining, fb)
		if !ok {
			break
		}
	}
	return remaining
}

// layoutWidths arranges children horizontally (bottom-up).
func layoutWidths(layout *Layout) {
	layoutWidthsDepth(layout, 0)
}

func layoutWidthsDepth(layout *Layout, depth int) {
	if overMaxDepth(depth) {
		return
	}
	padding := layout.Shape.paddingWidth()
	if layout.Shape.Axis == axisLeftToRight {
		sp := layout.spacing()
		// A Fixed axis with an explicit 0 size degrades to content
		// sizing. A zero-size Fixed box renders (children self-draw)
		// but its own bounds stay 0, collapsing the shapeClip — and
		// therefore the hit-test and clip region — of every descendant
		// (see issue #94). Content sizing keeps bounds enclosing the
		// children. Childless boxes still resolve to 0, unchanged.
		if layout.Shape.Sizing.Width == sizingFixed && layout.Shape.Width > 0 {
			for i := range layout.Children {
				layoutWidthsDepth(&layout.Children[i], depth+1)
			}
		} else {
			// A wrapping or overflowing row can be narrower than the
			// sum of its content: rows break (Wrap) or trailing
			// children are hidden (Overflow). Seeding the floor with
			// the full inter-child gap sum would make MinWidth grow
			// with the child count and pin the container wider than
			// its parent (issue #378) — only the widest single child
			// has to fit.
			wrapOrOverflow := layout.Shape.Wrap || layout.Shape.Overflow
			minWidths := padding
			if !wrapOrOverflow {
				minWidths += sp
			}
			for i := range layout.Children {
				layoutWidthsDepth(&layout.Children[i], depth+1)
				// Out-of-flow children (Float, shapeNone, OverDraw) are
				// dropped here for the same reason layout.spacing() drops
				// them from the fence-post count: a child that does not
				// take a slot in the row must not widen it either.
				if skipLayoutChild(layout.Children[i].Shape) {
					continue
				}
				layout.Shape.Width += layout.Children[i].Shape.Width
				if wrapOrOverflow {
					minWidths = f32Max(minWidths, layout.Children[i].Shape.Width+padding)
				} else if !sizingClips(layout.Shape, distributeHorizontal) {
					minWidths += layout.Children[i].Shape.MinWidth
				}
			}
			// A stated MinWidth is border-box: it counts the caller's
			// whole width budget, padding and spacing included (matches
			// the column branch and MaxWidth below). Padding it here made
			// a Row and a Column state the same minimum and arrange at
			// different widths (issue #385).
			layout.Shape.MinWidth = f32Max(minWidths, layout.Shape.MinWidth)
			layout.Shape.Width += padding + sp
			layout.Shape.MinWidth = effectiveMinSize(layout.Shape.MinWidth, layout.Shape.MaxWidth)
			layout.Shape.Width = clampSize(layout.Shape.Width, layout.Shape.MinWidth, layout.Shape.MaxWidth)
			scrollFillResetMin(layout.Shape, distributeHorizontal)
		}
	} else if layout.Shape.Axis == axisTopToBottom {
		// Fixed cross-axis with a 0 size degrades to content sizing
		// (see the AxisLeftToRight note above / issue #94). Captured
		// before the loop mutates Width, so it stays stable per child.
		fitWidth := layout.Shape.Sizing.Width != sizingFixed || layout.Shape.Width == 0
		// Padding is seeded rather than reaching the container only as
		// part of some child's contribution below, so that skipping an
		// out-of-flow child cannot also discard the container's own
		// padding. An empty group box has exactly one child — a shapeNone
		// placeholder (layoutPlaceholder) — and used to get its padding
		// width from measuring it.
		//
		// Gated on having children at all, in flow or not: a container
		// with none collapses to 0 rather than to its padding, which is
		// what a closed Sidebar (zero children, 1px border each side)
		// relies on to stay shut.
		if fitWidth && len(layout.Children) > 0 {
			layout.Shape.Width = f32Max(layout.Shape.Width, padding)
		}
		for i := range layout.Children {
			// Recurse first: an out-of-flow child still needs its own
			// subtree measured. Only its contribution to this container's
			// fit is dropped, matching computeContentWidth, which applies
			// skipLayoutChild on the cross axis too.
			layoutWidthsDepth(&layout.Children[i], depth+1)
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			if fitWidth {
				layout.Shape.Width = f32Max(layout.Shape.Width, layout.Children[i].Shape.Width+padding)
				if !sizingClips(layout.Shape, distributeHorizontal) {
					layout.Shape.MinWidth = f32Max(layout.Shape.MinWidth, layout.Children[i].Shape.MinWidth+padding)
				}
			}
		}
		layout.Shape.MinWidth = effectiveMinSize(layout.Shape.MinWidth, layout.Shape.MaxWidth)
		layout.Shape.Width = clampSize(layout.Shape.Width, layout.Shape.MinWidth, layout.Shape.MaxWidth)
		scrollFillResetMin(layout.Shape, distributeHorizontal)
	} else {
		// axisNone: children are not arranged; each sits at its own X. A
		// Fit width still encloses them (fitAxisNoneWidth, issue #584).
		// X is parent-relative here, because layoutPositions adds the
		// offsets later.
		//
		// Fixed and Fill widths only honor explicit min/max pins. The
		// Fill root pin from updateLayoutLocked (Min = Max = window
		// size) is the case that matters (issue #262): a FillFill root
		// like a Splitter (Canvas) used to resolve to 0x0 because the
		// pin was set and never read. Fixed sizing pins via
		// applyFixedSizingConstraints (Min = Max = Width), where the
		// clamp is a no-op; children keep today's sizing semantics.
		// Invariant: the fill impls must stay size-neutral for axisNone
		// — today they only recurse and cache contentW — or the pin
		// would be overwritten after this pass (plan item: "making the
		// fill passes distribute children of axisNone roots" is out of
		// scope and must not drift in as a size writer).
		if layout.Shape.Sizing.Width == sizingFit {
			fitAxisNoneWidth(layout, padding)
		}
		layout.Shape.MinWidth = effectiveMinSize(layout.Shape.MinWidth, layout.Shape.MaxWidth)
		layout.Shape.Width = clampSize(layout.Shape.Width, layout.Shape.MinWidth, layout.Shape.MaxWidth)
	}
}

// layoutHeights arranges children vertically (bottom-up).
func layoutHeights(layout *Layout) {
	layoutHeightsDepth(layout, 0)
}

func layoutHeightsDepth(layout *Layout, depth int) {
	if overMaxDepth(depth) {
		return
	}
	padding := layout.Shape.paddingHeight()
	if layout.Shape.Axis == axisTopToBottom {
		sp := layout.spacing()
		// Fixed with an explicit 0 height degrades to content sizing
		// (see layoutWidths / issue #94): a zero-height Fixed box would
		// otherwise collapse every descendant's clip/hit-test region.
		if layout.Shape.Sizing.Height == sizingFixed && layout.Shape.Height > 0 {
			for i := range layout.Children {
				layoutHeightsDepth(&layout.Children[i], depth+1)
			}
		} else {
			minHeights := padding + sp
			for i := range layout.Children {
				layoutHeightsDepth(&layout.Children[i], depth+1)
				// Float, shapeNone and OverDraw children sit outside the
				// flow, so they must not contribute to the stacked height
				// or its floor — the same set layout.spacing() drops from
				// the fence-post count, and computeContentHeight from the
				// content sum.
				if skipLayoutChild(layout.Children[i].Shape) {
					continue
				}
				layout.Shape.Height += layout.Children[i].Shape.Height
				// A Clip container may be cut below its content, the
				// same rule layoutWidths applies on both axes.
				if !sizingClips(layout.Shape, distributeVertical) {
					minHeights += layout.Children[i].Shape.MinHeight
				}
			}
			// A stated MinHeight is border-box, matching the row branch in
			// layoutWidths (issue #385): it already counts padding and
			// spacing, so adding them here charged the caller twice and made
			// a Row and a Column state the same minimum and arrange at
			// different sizes. The computed floor keeps its padding + sp
			// because it sums bare child minimums.
			layout.Shape.MinHeight = f32Max(minHeights, layout.Shape.MinHeight)
			layout.Shape.Height += padding + sp
			layout.Shape.MinHeight = effectiveMinSize(layout.Shape.MinHeight, layout.Shape.MaxHeight)
			layout.Shape.Height = clampSize(layout.Shape.Height, layout.Shape.MinHeight, layout.Shape.MaxHeight)
			scrollFillResetMin(layout.Shape, distributeVertical)
		}
	} else if layout.Shape.Axis == axisLeftToRight {
		// Fixed cross-axis with a 0 height degrades to content sizing
		// (see issue #94). Captured before the loop mutates Height.
		fitHeight := layout.Shape.Sizing.Height != sizingFixed || layout.Shape.Height == 0
		// See layoutWidths: seeded so skipping an out-of-flow child cannot
		// discard the container's padding, and gated on having children so
		// a childless container still collapses to 0.
		if fitHeight && len(layout.Children) > 0 {
			layout.Shape.Height = f32Max(layout.Shape.Height, padding)
		}
		for i := range layout.Children {
			// See layoutWidths: recurse for every child, fit against the
			// in-flow ones only.
			layoutHeightsDepth(&layout.Children[i], depth+1)
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			if fitHeight {
				layout.Shape.Height = f32Max(layout.Shape.Height, layout.Children[i].Shape.Height+padding)
				if !sizingClips(layout.Shape, distributeVertical) {
					layout.Shape.MinHeight = f32Max(layout.Shape.MinHeight, layout.Children[i].Shape.MinHeight+padding)
				}
			}
		}
		layout.Shape.MinHeight = effectiveMinSize(layout.Shape.MinHeight, layout.Shape.MaxHeight)
		layout.Shape.Height = clampSize(layout.Shape.Height, layout.Shape.MinHeight, layout.Shape.MaxHeight)
		scrollFillResetMin(layout.Shape, distributeVertical)
	} else {
		// axisNone: mirror layoutWidths. A Fit height encloses the
		// children at their own Y (fitAxisNoneHeight, issue #584); Fixed
		// and Fill heights honor explicit min/max pins only (the Fill root
		// pin from updateLayoutLocked, issue #262). Same fill-impl
		// size-neutral invariant as layoutWidths: the height fill impl
		// must not gain an axisNone size writer.
		if layout.Shape.Sizing.Height == sizingFit {
			fitAxisNoneHeight(layout, padding)
		}
		layout.Shape.MinHeight = effectiveMinSize(layout.Shape.MinHeight, layout.Shape.MaxHeight)
		layout.Shape.Height = clampSize(layout.Shape.Height, layout.Shape.MinHeight, layout.Shape.MaxHeight)
	}
}

// spacingSmall matches the V framework's spacing_small constant.
const spacingSmall = 5

// layoutFillWidths manages horizontal growth/shrinkage.
func layoutFillWidths(layout *Layout, p *scratchPools) {
	layoutFillWithPool(layout, p, layoutFillWidthsImpl)
}

func layoutFillWidthsImpl(layout *Layout, fb *fillBuffers) {
	layoutFillWidthsImplDepth(layout, fb, 0)
}

func layoutFillWidthsImplDepth(layout *Layout, fb *fillBuffers, depth int) {
	if overMaxDepth(depth) {
		return
	}
	remainingWidth := layout.Shape.Width - layout.Shape.paddingWidth()

	switch layout.Shape.Axis {
	case axisLeftToRight:
		for i := range layout.Children {
			// Must drop the same children layout.spacing() drops on the
			// next line, or an out-of-flow child eats budget that no
			// fence post accounts for and Fill siblings shrink.
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			remainingWidth -= layout.Children[i].Shape.Width
		}
		remainingWidth -= layout.spacing()

		if remainingWidth > f32Tolerance {
			distributeSpace(layout, remainingWidth, distributeGrow, distributeHorizontal, fb)
		}
		if remainingWidth < -f32Tolerance && !layout.Shape.Wrap && !layout.Shape.Overflow {
			distributeSpace(layout, remainingWidth, distributeShrink, distributeHorizontal, fb)
		}
	case axisTopToBottom:
		layoutFillCrossAxis(layout, distributeHorizontal, fb)
	}

	for i := range layout.Children {
		layoutFillWidthsImplDepth(&layout.Children[i], fb, depth+1)
	}

	// Cache content width after all children have final widths.
	layout.Shape.contentW = computeContentWidth(layout)
	layout.Shape.fillGen = fb.fillGen
}

// layoutFillHeights manages vertical growth/shrinkage.
func layoutFillHeights(layout *Layout, p *scratchPools) {
	layoutFillWithPool(layout, p, layoutFillHeightsImpl)
}

// layoutFillWithPool runs impl with scratch-pool-backed candidate
// slices when p is non-nil, or local slices when nil.
func layoutFillWithPool(layout *Layout, p *scratchPools, impl func(*Layout, *fillBuffers)) {
	if p == nil {
		var fb fillBuffers
		impl(layout, &fb)
		return
	}
	fb := &p.fillBufs
	fb.fillGen = p.fillGen
	fb.candidates = p.fillCandidates.take(0)
	impl(layout, fb)
	p.fillCandidates.put(fb.candidates)
}

func layoutFillHeightsImpl(layout *Layout, fb *fillBuffers) {
	layoutFillHeightsImplDepth(layout, fb, 0)
}

func layoutFillHeightsImplDepth(layout *Layout, fb *fillBuffers, depth int) {
	if overMaxDepth(depth) {
		return
	}
	remainingHeight := layout.Shape.Height - layout.Shape.paddingHeight()

	switch layout.Shape.Axis {
	case axisTopToBottom:
		for i := range layout.Children {
			// See layoutFillWidthsImpl: this must match the child set
			// layout.spacing() uses on the next line.
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			remainingHeight -= layout.Children[i].Shape.Height
		}
		remainingHeight -= layout.spacing()

		if remainingHeight > f32Tolerance {
			distributeSpace(layout, remainingHeight, distributeGrow, distributeVertical, fb)
		}
		// No Wrap/Overflow guard: both only apply to AxisLeftToRight.
		if remainingHeight < -f32Tolerance {
			distributeSpace(layout, remainingHeight, distributeShrink, distributeVertical, fb)
		}
	case axisLeftToRight:
		layoutFillCrossAxis(layout, distributeVertical, fb)
	}

	for i := range layout.Children {
		layoutFillHeightsImplDepth(&layout.Children[i], fb, depth+1)
	}

	// Cache content height after all children have final heights.
	layout.Shape.contentH = computeContentHeight(layout)
	layout.Shape.fillGen = fb.fillGen
}
