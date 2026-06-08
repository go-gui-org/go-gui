package gui

import "math"

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

func getSizing(shape *Shape, axis distributeAxis) SizingType {
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

// mainAxisOf returns the layout axis that distributes children
// along the given dimension (horizontal → LeftToRight, etc.).
func mainAxisOf(axis distributeAxis) Axis {
	if axis == distributeHorizontal {
		return AxisLeftToRight
	}
	return AxisTopToBottom
}

func scrollExcludesAxis(mode ScrollMode, axis distributeAxis) bool {
	if axis == distributeHorizontal {
		return mode == ScrollVerticalOnly
	}
	return mode == ScrollHorizontalOnly
}

func clampMinMax(shape *Shape, axis distributeAxis) {
	size := getSize(shape, axis)
	minSize := getMinSize(shape, axis)
	maxSize := getMaxSize(shape, axis)
	if minSize > 0 && size < minSize {
		setSize(shape, axis, minSize)
	}
	if maxSize > 0 && size > maxSize {
		setSize(shape, axis, maxSize)
	}
}

// layoutFillCrossAxis handles cross-axis fill sizing: adjusts scroll
// containers to fit parent's remaining space, clamps to min/max, and
// propagates fill size to children.
func layoutFillCrossAxis(layout *Layout, axis distributeAxis) {
	if layout.Shape.IDScroll > 0 && getSizing(layout.Shape, axis) == SizingFill &&
		!scrollExcludesAxis(layout.Shape.ScrollMode, axis) &&
		layout.Parent != nil && layout.Parent.Shape.Axis == mainAxisOf(axis) {
		var totalChild float32
		for j := range layout.Parent.Children {
			totalChild += getSize(layout.Parent.Children[j].Shape, axis)
		}
		sibling := totalChild - getSize(layout.Shape, axis)
		target := getSize(layout.Parent.Shape, axis) - sibling -
			layout.Parent.spacing() - getPadding(layout.Parent.Shape, axis)
		setSize(layout.Shape, axis, f32Max(0, target))
	}
	clampMinMax(layout.Shape, axis)
	remaining := getSize(layout.Shape, axis) - getPadding(layout.Shape, axis)
	for i := range layout.Children {
		if getSizing(layout.Children[i].Shape, axis) == SizingFill {
			setSize(layout.Children[i].Shape, axis, remaining)
			clampMinMax(layout.Children[i].Shape, axis)
		}
	}
}

type distributionExtrema struct {
	extremum    float32
	nextExtrema float32
}

func collectDistributionCandidates(layout *Layout, axis distributeAxis, mode distributeMode, fillIndices, fixedIndices *[]int) {
	*fillIndices = (*fillIndices)[:0]
	if mode == distributeShrink {
		*fixedIndices = (*fixedIndices)[:0]
	}
	for i := range layout.Children {
		if getSizing(layout.Children[i].Shape, axis) == SizingFill {
			*fillIndices = append(*fillIndices, i)
		} else if mode == distributeShrink {
			*fixedIndices = append(*fixedIndices, i)
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

func findDistributionExtrema(layout *Layout, axis distributeAxis, mode distributeMode, fillIndices, fixedIndices []int) (distributionExtrema, bool) {
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
	if mode == distributeShrink {
		for _, idx := range fixedIndices {
			childSize := getSize(layout.Children[idx].Shape, axis)
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

func computeDistributionDelta(layout *Layout, remaining float32, mode distributeMode, axis distributeAxis, extrema distributionExtrema, fillCount, fixedCount int) (float32, bool) {
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
	if mode == distributeGrow {
		sizeDelta = f32Min(sizeDelta, remaining/float32(fillCount))
	} else {
		totalCount := fillCount + fixedCount
		if totalCount > 0 {
			sizeDelta = f32Max(sizeDelta, remaining/float32(totalCount))
		}
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

func applyDistributionDelta(layout *Layout, axis distributeAxis, extremum, sizeDelta, remainingIn float32, fillIndices *[]int) (float32, bool) {
	remaining := remainingIn
	keepIdx := 0
	fi := *fillIndices
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
			minSize := getMinSize(child.Shape, axis)
			maxSize := getMaxSize(child.Shape, axis)
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
	*fillIndices = fi[:keepIdx]
	return remaining, true
}

func distributeSpace(layout *Layout, remainingIn float32, mode distributeMode, axis distributeAxis, candidates, fixedIndices *[]int) float32 {
	if !f32IsFinite(remainingIn) {
		return 0
	}
	remaining := remainingIn
	prevRemaining := float32(0)

	collectDistributionCandidates(layout, axis, mode, candidates, fixedIndices)

	for shouldContinueDistribution(remaining, mode, len(*candidates)) {
		if f32AreClose(remaining, prevRemaining) {
			break
		}
		prevRemaining = remaining
		extrema, ok := findDistributionExtrema(layout, axis, mode, *candidates, *fixedIndices)
		if !ok {
			break
		}
		sizeDelta, ok := computeDistributionDelta(layout, remaining, mode, axis, extrema, len(*candidates), len(*fixedIndices))
		if !ok {
			break
		}
		remaining, ok = applyDistributionDelta(layout, axis, extrema.extremum, sizeDelta, remaining, candidates)
		if !ok {
			break
		}
	}
	return remaining
}

// layoutWidths arranges children horizontally (bottom-up).
func layoutWidths(layout *Layout) {
	padding := layout.Shape.paddingWidth()
	if layout.Shape.Axis == AxisLeftToRight {
		sp := layout.spacing()
		if layout.Shape.Sizing.Width == SizingFixed {
			for i := range layout.Children {
				layoutWidths(&layout.Children[i])
			}
		} else {
			minWidths := padding + sp
			for i := range layout.Children {
				layoutWidths(&layout.Children[i])
				if layout.Children[i].Shape.OverDraw {
					continue
				}
				layout.Shape.Width += layout.Children[i].Shape.Width
				if layout.Shape.Wrap || layout.Shape.Overflow {
					minWidths = f32Max(minWidths, layout.Children[i].Shape.Width+padding)
				} else if !layout.Shape.Clip {
					minWidths += layout.Children[i].Shape.MinWidth
				}
			}
			if !layout.Shape.Wrap && !layout.Shape.Overflow {
				layout.Shape.MinWidth = f32Max(minWidths, layout.Shape.MinWidth+padding+sp)
			} else {
				layout.Shape.MinWidth = f32Max(minWidths, layout.Shape.MinWidth)
			}
			layout.Shape.Width += padding + sp
			if layout.Shape.MaxWidth > 0 {
				layout.Shape.Width = f32Min(layout.Shape.MaxWidth, layout.Shape.Width)
				layout.Shape.MinWidth = f32Min(layout.Shape.MaxWidth, layout.Shape.MinWidth)
			}
			if layout.Shape.MinWidth > 0 {
				layout.Shape.Width = f32Max(layout.Shape.MinWidth, layout.Shape.Width)
			}
		}
	} else if layout.Shape.Axis == AxisTopToBottom {
		for i := range layout.Children {
			layoutWidths(&layout.Children[i])
			if layout.Shape.Sizing.Width != SizingFixed {
				layout.Shape.Width = f32Max(layout.Shape.Width, layout.Children[i].Shape.Width+padding)
				if !layout.Shape.Clip {
					layout.Shape.MinWidth = f32Max(layout.Shape.MinWidth, layout.Children[i].Shape.MinWidth+padding)
				}
			}
		}
		if layout.Shape.MinWidth > 0 {
			layout.Shape.Width = f32Max(layout.Shape.Width, layout.Shape.MinWidth)
		}
		if layout.Shape.MaxWidth > 0 {
			layout.Shape.Width = f32Min(layout.Shape.Width, layout.Shape.MaxWidth)
		}
	}
}

// layoutHeights arranges children vertically (bottom-up).
func layoutHeights(layout *Layout) {
	padding := layout.Shape.paddingHeight()
	if layout.Shape.Axis == AxisTopToBottom {
		sp := layout.spacing()
		if layout.Shape.Sizing.Height == SizingFixed {
			for i := range layout.Children {
				layoutHeights(&layout.Children[i])
			}
		} else {
			minHeights := padding + sp
			for i := range layout.Children {
				layoutHeights(&layout.Children[i])
				if layout.Children[i].Shape.OverDraw {
					continue
				}
				layout.Shape.Height += layout.Children[i].Shape.Height
				minHeights += layout.Children[i].Shape.MinHeight
			}
			layout.Shape.MinHeight = f32Max(minHeights, layout.Shape.MinHeight+padding+sp)
			layout.Shape.Height += padding + sp
			if layout.Shape.MaxHeight > 0 {
				layout.Shape.Height = f32Min(layout.Shape.MaxHeight, layout.Shape.Height)
				layout.Shape.MinHeight = f32Min(layout.Shape.MaxHeight, layout.Shape.MinHeight)
			}
			if layout.Shape.MinHeight > 0 {
				layout.Shape.Height = f32Max(layout.Shape.MinHeight, layout.Shape.Height)
			}
			if layout.Shape.Sizing.Height == SizingFill && layout.Shape.IDScroll > 0 {
				layout.Shape.MinHeight = spacingSmall
			}
		}
	} else if layout.Shape.Axis == AxisLeftToRight {
		for i := range layout.Children {
			layoutHeights(&layout.Children[i])
			if layout.Shape.Sizing.Height != SizingFixed {
				layout.Shape.Height = f32Max(layout.Shape.Height, layout.Children[i].Shape.Height+padding)
				layout.Shape.MinHeight = f32Max(layout.Shape.MinHeight, layout.Children[i].Shape.MinHeight+padding)
			}
		}
		if layout.Shape.MinHeight > 0 {
			layout.Shape.Height = f32Max(layout.Shape.Height, layout.Shape.MinHeight)
		}
		if layout.Shape.MaxHeight > 0 {
			layout.Shape.Height = f32Min(layout.Shape.Height, layout.Shape.MaxHeight)
		}
	}
}

// spacingSmall matches the V framework's spacing_small constant.
const spacingSmall = 5

// layoutFillWidths manages horizontal growth/shrinkage.
func layoutFillWidths(layout *Layout) {
	var candidates, fixedIndices []int
	layoutFillWidthsImpl(layout, &candidates, &fixedIndices)
}

func layoutFillWidthsImpl(layout *Layout, candidates, fixedIndices *[]int) {
	remainingWidth := layout.Shape.Width - layout.Shape.paddingWidth()

	switch layout.Shape.Axis {
	case AxisLeftToRight:
		for i := range layout.Children {
			if layout.Children[i].Shape.OverDraw {
				continue
			}
			remainingWidth -= layout.Children[i].Shape.Width
		}
		remainingWidth -= layout.spacing()

		if remainingWidth > f32Tolerance {
			distributeSpace(layout, remainingWidth, distributeGrow, distributeHorizontal, candidates, fixedIndices)
		}
		if remainingWidth < -f32Tolerance && !layout.Shape.Wrap && !layout.Shape.Overflow {
			distributeSpace(layout, remainingWidth, distributeShrink, distributeHorizontal, candidates, fixedIndices)
		}
	case AxisTopToBottom:
		layoutFillCrossAxis(layout, distributeHorizontal)
	}

	for i := range layout.Children {
		layoutFillWidthsImpl(&layout.Children[i], candidates, fixedIndices)
	}
}

// layoutFillHeights manages vertical growth/shrinkage.
func layoutFillHeights(layout *Layout) {
	var candidates, fixedIndices []int
	layoutFillHeightsImpl(layout, &candidates, &fixedIndices)
}

func layoutFillHeightsImpl(layout *Layout, candidates, fixedIndices *[]int) {
	remainingHeight := layout.Shape.Height - layout.Shape.paddingHeight()

	switch layout.Shape.Axis {
	case AxisTopToBottom:
		for i := range layout.Children {
			if layout.Children[i].Shape.OverDraw {
				continue
			}
			remainingHeight -= layout.Children[i].Shape.Height
		}
		remainingHeight -= layout.spacing()

		if remainingHeight > f32Tolerance {
			distributeSpace(layout, remainingHeight, distributeGrow, distributeVertical, candidates, fixedIndices)
		}
		// No Wrap/Overflow guard: both only apply to AxisLeftToRight.
		if remainingHeight < -f32Tolerance {
			distributeSpace(layout, remainingHeight, distributeShrink, distributeVertical, candidates, fixedIndices)
		}
	case AxisLeftToRight:
		layoutFillCrossAxis(layout, distributeVertical)
	}

	for i := range layout.Children {
		layoutFillHeightsImpl(&layout.Children[i], candidates, fixedIndices)
	}
}
