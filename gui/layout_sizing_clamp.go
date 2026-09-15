package gui

// effectiveMinSize is the Min a size may be raised to once Max is taken
// into account. When both are set and Min > Max, Max wins: every sizing
// site resolves the conflict the same way, and a computed Min (the sum of
// the children's minimums) cannot push a container past a stated Max.
// A zero or negative Max means unset.
func effectiveMinSize(minSize, maxSize float32) float32 {
	if maxSize > 0 && minSize > maxSize {
		return maxSize
	}
	return minSize
}

// clampSize raises size to Min and caps it at Max, with Max winning a
// conflict (see effectiveMinSize). Zero or negative bounds mean unset.
func clampSize(size, minSize, maxSize float32) float32 {
	if m := effectiveMinSize(minSize, maxSize); m > 0 && size < m {
		size = m
	}
	if maxSize > 0 && size > maxSize {
		size = maxSize
	}
	return size
}

// sizingClips reports whether a container may be sized below its
// children's minimums on axis. A Scrollable container clips (layoutPositions
// sets Clip on it), but that happens after sizing, so sizing reads Scrollable
// directly rather than waiting for the flag. An axis the ScrollMode excludes
// cannot reveal hidden content, so it keeps its floor (see
// scrollFillResetMin).
func sizingClips(s *Shape, axis distributeAxis) bool {
	if s == nil {
		return false
	}
	return s.Clip || (s.Scrollable && !scrollExcludesAxis(s.ScrollMode, axis))
}
