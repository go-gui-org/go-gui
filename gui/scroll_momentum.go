package gui

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
	// panVelocityWindow is how far back, in nanoseconds, the release
	// velocity looks: the finger's displacement over its last 100 ms.
	panVelocityWindow = int64(100_000_000)
	// panSampleCap holds 100 ms of samples at 120 Hz with room for
	// bursts.
	panSampleCap = 16
)

// panSample is one position of the single finger.
type panSample struct {
	t    int64 // monotonic nanos
	x, y float32
}

// panSampleReset starts the samples over at a finger's position.
func (gs *gestureState) panSampleReset(t int64, x, y float32) {
	gs.panSampleN = 0
	gs.panSampleAdd(t, x, y)
}

// panSampleAdd records a finger position.
func (gs *gestureState) panSampleAdd(t int64, x, y float32) {
	gs.panSamples[gs.panSampleN%panSampleCap] = panSample{t: t, x: x, y: y}
	gs.panSampleN++
}

// panReleaseVelocity is the finger's velocity at the lift, in px/s: its
// displacement over the last panVelocityWindow of samples. It reads real
// positions, not the pan's catch-up at the threshold, and does not lag
// the way the pan's smoothed velocity does, so a short fast flick reads
// at its speed and a crawl past the threshold reads as a crawl (#1110).
// ok is false without two samples in the window.
func (gs *gestureState) panReleaseVelocity() (vx, vy float32, ok bool) {
	n := min(gs.panSampleN, panSampleCap)
	if n < 2 {
		return 0, 0, false
	}
	newest := gs.panSamples[(gs.panSampleN-1)%panSampleCap]
	oldest := newest
	for i := 2; i <= n; i++ {
		s := gs.panSamples[(gs.panSampleN-i)%panSampleCap]
		if newest.t-s.t > panVelocityWindow {
			break
		}
		oldest = s
	}
	dt := float32(newest.t-oldest.t) / 1e9
	if !(dt > 0) {
		return 0, 0, false
	}
	return (newest.x - oldest.x) / dt, (newest.y - oldest.y) / dt, true
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
	vx, vy, ok := gs.panReleaseVelocity()
	if !ok {
		return
	}
	if gs.panScrollV {
		scrollMomentumStart(w, gs.panScrollID, scrollAxisY, vy)
	}
	if gs.panScrollH {
		scrollMomentumStart(w, gs.panScrollID, scrollAxisX, vx)
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
	e.active = true
	e.settled = false
	ss.stopped = false
	w.animationAddLocked(ss)
	return true
}

// scrollFlingEdge stops a fling on the scroll edge it reached. The
// edge is read from the current layout every frame, so a glide follows
// content that grew or shrank under it: it glides on into rows a list
// appended, and stops on the new end of content that shrank instead of
// writing offsets past it. The tick does not clamp: it runs on the
// animation goroutine, cannot read the layout, and a bound it kept from
// the lift went stale as the content changed. A fling therefore stops on
// the edge at full speed, at most one tick past it in the entry, never
// in an applied offset. Runs in the apply pass on the main goroutine
// with w.animMu held. A scrollable that is gone is left alone.
func scrollFlingEdge(w *Window, e *scrollSmoothEntry) {
	ly, ok := findScrollLayout(w, e.id)
	if !ok {
		return
	}
	lo := scrollMaxOffsetY(ly)
	if e.axis == scrollAxisX {
		lo = scrollMaxOffsetX(ly)
	}
	if !f32IsFinite(lo) {
		return
	}
	switch {
	case e.current <= lo:
		e.current = lo
	case e.current >= 0:
		e.current = 0
	default:
		return
	}
	e.dirty = true
	e.active = false
	e.settled = false
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
