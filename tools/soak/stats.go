package soak

import (
	"math"
	"slices"
)

// mean returns the arithmetic mean of xs, or 0 for none.
func mean(xs []uint64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += float64(x)
	}
	return sum / float64(len(xs))
}

// percentile returns the p-th quantile of xs (p in [0, 1]) by the
// nearest-rank rule, or 0 for none. It sorts a copy; xs is unchanged.
func percentile(xs []uint64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	rank := int(math.Ceil(p*float64(len(s)))) - 1
	rank = min(max(rank, 0), len(s)-1)
	return float64(s[rank])
}

// slope returns the least-squares slope of ys over xs, or 0 when there
// are fewer than two points or every x is the same.
func slope(xs, ys []float64) float64 {
	n := min(len(xs), len(ys))
	if n < 2 {
		return 0
	}
	var mx, my float64
	for i := range n {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(n)
	my /= float64(n)
	var num, den float64
	for i := range n {
		dx := xs[i] - mx
		num += dx * (ys[i] - my)
		den += dx * dx
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// windowMinima splits the points (xs[i], ys[i]) into n runs of equal
// count, in order, and returns the point with the lowest y in each. With
// fewer than n points each point is its own run. The result feeds slope
// to measure how the floor of a noisy series moves.
func windowMinima(xs, ys []float64, n int) (mx, my []float64) {
	count := min(len(xs), len(ys))
	if count == 0 || n <= 0 {
		return nil, nil
	}
	n = min(n, count)
	mx = make([]float64, 0, n)
	my = make([]float64, 0, n)
	for w := range n {
		lo, hi := w*count/n, (w+1)*count/n
		best := lo
		for i := lo + 1; i < hi; i++ {
			if ys[i] < ys[best] {
				best = i
			}
		}
		mx = append(mx, xs[best])
		my = append(my, ys[best])
	}
	return mx, my
}
