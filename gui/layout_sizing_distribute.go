package gui

import "math"

// Water-filling Fill distribution: equalize Fill siblings toward the
// next extremum until the row or column budget is spent. Kept apart
// from the width and height passes so layout_sizing.go stays under
// the large-files gate; the passes call back in through
// distributeSpace.

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

// distributionLevelEps is the band within which two Fill sizes count
// as the same water-filling level. Exact bit equality is too strict:
// one float32 addition leaves 1-ulp dust, so a grown child
// (13.21+16.51) differs from a native one (29.72) by ~2e-06. The
// level scan then reads a phantom next extremum 2e-06 away, the
// filler takes a micro-step that moves nothing, and distribution
// stalls with the budget still undistributed. Relative 1e-04,
// floored at 1 so sub-pixel layouts keep an absolute floor, merges
// dust at every realistic scale (ulp at 8192px is ~0.001) while
// keeping real levels distinct.
func distributionLevelEps(a, b float32) float32 {
	m := f32Abs(a)
	if n := f32Abs(b); n > m {
		m = n
	}
	if m < 1 {
		m = 1
	}
	return m * 1e-04
}

// distributionSameLevel reports whether two Fill sizes sit at one
// water-filling level (see distributionLevelEps). Non-finite sizes
// never group; the finite check reports them.
func distributionSameLevel(a, b float32) bool {
	d := f32Abs(a - b)
	if !f32IsFinite(d) {
		return false
	}
	return d <= distributionLevelEps(a, b)
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
		eps := distributionLevelEps(childSize, extrema)
		if mode == distributeGrow {
			if childSize < extrema-eps {
				nextExtrema = extrema
				extrema = childSize
			} else if childSize > extrema+eps {
				nextExtrema = f32Min(nextExtrema, childSize)
			}
		} else {
			if childSize > extrema+eps {
				nextExtrema = extrema
				extrema = childSize
			} else if childSize < extrema-eps {
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
		if distributionSameLevel(childSize, extremum) {
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
	prevCount := -1

	collectDistributionCandidates(layout, axis, fb)

	for shouldContinueDistribution(remaining, mode, len(fb.candidates)) {
		// Break only on zero progress, not on sub-tolerance progress:
		// an absolute epsilon here stalls tiny layouts, where one
		// equalization step moves less than f32Tolerance yet most of
		// the budget is still undistributed (a 0.0078 step against a
		// 0.29 remainder). Dropping a candidate is progress too: an
		// extremum already pinned at its min moves nothing but leaves
		// the set, and the smaller siblings must still get their
		// share. Each iteration either shrinks the candidate set or
		// moves the remainder, so exact equality still terminates.
		if remaining == prevRemaining && len(fb.candidates) == prevCount {
			break
		}
		prevRemaining = remaining
		prevCount = len(fb.candidates)
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
