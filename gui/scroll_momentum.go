package gui

import "math"

// Momentum (fling) scrolling for touch (#1110). A pan that lifts while
// the finger is still moving keeps the content gliding and slowing, as
// native scroll views do, instead of stopping it dead.
//
// The glide is an exponential decay of the release velocity v with time
// constant scrollMomentumTau: the content travels v·tau in total and
// starts at exactly v, so there is no jump in speed at the lift. It runs
// on the wheel smoother's animation (scroll_smooth.go), whose entries
// carry the time constant. A fling stops at a scroll edge at full speed
// rather than slowing early because an edge is near.
//
// A new touch stops a fling, as does any direct offset write (a pan,
// the keyboard, the scrollbar): scrollSmoothCancel retires the entry.
const (
	// scrollMomentumTau is the decay time constant, in seconds. The
	// glide covers 63% of its distance in tau and settles in about
	// five. Close to UIScrollView's normal deceleration (~0.5 s).
	scrollMomentumTau = float32(0.4)
	// scrollMomentumMinVelocity is the slowest lift, in px/s, that
	// flings. Slower is a placed finger, not a flick.
	scrollMomentumMinVelocity = float32(100)
	// scrollMomentumMaxVelocity caps a fling, in px/s, so a jittery
	// last sample cannot throw the content across a long list.
	scrollMomentumMaxVelocity = float32(8000)
)

// scrollMomentumStep is the fraction of the remaining distance a fling
// covers in a tick of dt seconds. A dt that is not positive and finite
// takes the nominal 16 ms.
func scrollMomentumStep(dt, tau float32) float32 {
	if !f32IsFinite(dt) || dt <= 0 {
		dt = float32(animationCycle.Seconds())
	}
	return float32(1 - math.Exp(-float64(dt)/float64(tau)))
}

// panScrolled records the container a touch pan just moved and on which
// axes, so the lift can fling it. A pan that moves on into another
// container (an inner one at its limit) flings the last one it moved.
func panScrolled(gs *gestureState, layout *Layout, v, h bool) {
	id := layout.Shape.idKey()
	if id != gs.panScrollID {
		gs.panScrollID = id
		gs.panScrollV, gs.panScrollH = false, false
	}
	gs.panScrollV = gs.panScrollV || v
	gs.panScrollH = gs.panScrollH || h
}

// clearPanScroll forgets the container the pan scrolled, when another
// finger takes over: only the finger that moved a container flings it.
func (gs *gestureState) clearPanScroll() {
	gs.panScrollID = ""
	gs.panScrollV, gs.panScrollH = false, false
}

// startPanMomentum flings the container the ending pan moved, on each
// axis it moved, when the finger was still moving at the lift. Main
// goroutine only.
func startPanMomentum(gs *gestureState, w *Window, fresh bool) {
	if !fresh || gs.panScrollID == "" {
		return
	}
	if gs.panScrollV {
		scrollMomentumStart(w, gs.panScrollID, scrollAxisY, gs.velocityY)
	}
	if gs.panScrollH {
		scrollMomentumStart(w, gs.panScrollID, scrollAxisX, gs.velocityX)
	}
}

// scrollMomentumStart arms a fling on id's axis from velocity, in px/s
// of content displacement (the finger's direction). Does nothing below
// scrollMomentumMinVelocity, or when the content already rests on the
// edge it would move toward. Main goroutine only.
func scrollMomentumStart(w *Window, id string, axis scrollAxis, velocity float32) bool {
	if !f32IsFinite(velocity) ||
		f32Abs(velocity) < scrollMomentumMinVelocity {
		return false
	}
	velocity = f32Clamp(velocity,
		-scrollMomentumMaxVelocity, scrollMomentumMaxVelocity)
	ly, ok := findScrollLayout(w, id)
	if !ok {
		return false
	}
	displayed, lo, ok := scrollSmoothParams(w, ly, axis)
	if !ok || !f32IsFinite(displayed) || !f32IsFinite(lo) {
		return false
	}
	// The velocity is physical; an RTL row's offset runs the other way,
	// as for the pan itself (scrollHorizontalPixels).
	if axis == scrollAxisX && scrollMirrorsX(ly.Shape) {
		velocity = -velocity
	}
	if (velocity > 0 && displayed >= 0) || (velocity < 0 && displayed <= lo) {
		return false
	}
	w.animMu.Lock()
	defer w.animMu.Unlock()
	if w.scrollSmooth == nil {
		w.scrollSmooth = &scrollSmoothAnimation{}
	}
	ss := w.scrollSmooth
	e := ss.entryFor(id, axis)
	e.current = displayed
	e.target = displayed + velocity*scrollMomentumTau
	e.tau = scrollMomentumTau
	e.lo = lo
	e.active = true
	e.settled = false
	ss.stopped = false
	w.animationAddLocked(ss)
	return true
}

// scrollMomentumStop ends every running fling where its content is now,
// for a new touch: the finger that lands on gliding content catches it.
// The wheel eases keep running. Main goroutine only.
func scrollMomentumStop(w *Window) {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	if w.scrollSmooth == nil {
		return
	}
	for i := range w.scrollSmooth.entries {
		if e := &w.scrollSmooth.entries[i]; e.active && e.tau > 0 {
			// The last applied value stays; a computed but unapplied
			// one is a few pixels on, past where the finger caught it.
			e.active = false
			e.settled = false
			e.dirty = false
		}
	}
}
