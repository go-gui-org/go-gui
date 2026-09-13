package gui

import "math"

// gesture_touch.go — the tracked-touch set behind gesture.go.
//
// The recognizer keeps at most len(Event.Touches) touches in a
// fixed array: no per-event allocation on the touch path. Order
// is stable across releases (see removeTrackedTouch), because
// span and angle read touches[0..1] by position.

// --- Touch tracking helpers ---

// touchPointFinite reports whether both coords are finite.
// Non-finite coords (corrupted backend, NaN fuzz) are dropped
// by add/update below: they would poison span, angle and
// velocity (NaN fails every threshold but still reaches
// synthMouse).
func touchPointFinite(tp TouchPoint) bool {
	return f32IsFinite(tp.PosX) && f32IsFinite(tp.PosY)
}

func addTrackedTouch(gs *gestureState, tp TouchPoint) {
	if !touchPointFinite(tp) {
		return
	}
	// Update existing touch if already tracked.
	for i := range gs.numTouches {
		if gs.touches[i].id == tp.Identifier {
			gs.touches[i].x = tp.PosX
			gs.touches[i].y = tp.PosY
			return
		}
	}
	if gs.numTouches >= len(gs.touches) {
		// No slot free (a release went missing): count the
		// drop so its later release still balances.
		gs.overflowTouches++
		return
	}
	gs.touches[gs.numTouches] = trackedTouch{
		id: tp.Identifier, x: tp.PosX, y: tp.PosY,
	}
	gs.numTouches++
}

func updateTrackedTouch(gs *gestureState, tp TouchPoint) {
	// Dropped like in addTrackedTouch, but removal stays
	// ID-based (see removeTrackedTouch) so a non-finite
	// release still un-tracks instead of wedging.
	if !touchPointFinite(tp) {
		return
	}
	for i := range gs.numTouches {
		if gs.touches[i].id == tp.Identifier {
			gs.touches[i].x = tp.PosX
			gs.touches[i].y = tp.PosY
			return
		}
	}
}

func removeTrackedTouch(gs *gestureState, id uint64) {
	for i := range gs.numTouches {
		if gs.touches[i].id == id {
			// Order-preserving: span and angle read
			// touches[0..1] by position, so a swap
			// would flip the rotate vector by pi.
			copy(gs.touches[i:gs.numTouches-1],
				gs.touches[i+1:gs.numTouches])
			gs.numTouches--
			gs.touches[gs.numTouches] = trackedTouch{}
			return
		}
	}
	if gs.overflowTouches > 0 {
		gs.overflowTouches--
	}
}

func touchCentroid(gs *gestureState) (float32, float32) {
	if gs.numTouches == 0 {
		return 0, 0
	}
	var sx, sy float32
	for i := range gs.numTouches {
		sx += gs.touches[i].x
		sy += gs.touches[i].y
	}
	n := float32(gs.numTouches)
	return sx / n, sy / n
}

func touchSpan(gs *gestureState) float32 {
	if gs.numTouches < 2 {
		return 0
	}
	dx := gs.touches[1].x - gs.touches[0].x
	dy := gs.touches[1].y - gs.touches[0].y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func touchAngle(gs *gestureState) float32 {
	if gs.numTouches < 2 {
		return 0
	}
	dx := gs.touches[1].x - gs.touches[0].x
	dy := gs.touches[1].y - gs.touches[0].y
	return float32(math.Atan2(float64(dy), float64(dx)))
}

// normalizeAngle wraps an angle to [-pi, pi].
func normalizeAngle(a float32) float32 {
	return float32(math.Remainder(float64(a), 2*math.Pi))
}
