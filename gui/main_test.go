package gui

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// leakSettleDeadline bounds how long TestMain waits for leftover
// animations to retire on their own before calling them leaks. It
// must outlast animViewBoundStale (a view-bound animation whose widget
// is gone is evicted only after that) plus the longest finite tween a
// test starts (theme fade, toast, layout transition: all well under a
// second). Only a failing run pays it; a clean run returns at once.
const leakSettleDeadline = animViewBoundStale + 3*time.Second

// TestMain fails the package when a test leaves a window whose
// animation loop is still ticking (#836).
//
// A window's animation goroutine runs until WindowCleanup stops it. In
// an app the backend calls WindowCleanup; in a test nothing does unless
// the test asks. An idle loop just parks, but one holding a repeating
// animation — the blink cursor of a focused Input, a drag auto-scroll —
// ticks at 60 Hz for the rest of the process and queues a command each
// time. With ~100 of those alive, testing.AllocsPerRun (which counts
// mallocs from every goroutine) read 4-5 allocs per run in alloc gates
// that allocate nothing, so the gates failed in every full-package
// -race -cover run and never alone.
//
// The fix is t.Cleanup(w.WindowCleanup) wherever a test builds a
// window that can start an animation. This check keeps it fixed: it
// runs after every test, so a new leak fails here, named by its
// animation IDs, rather than as noise in an unrelated alloc gate.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 {
		if leaks := settledAnimationLeaks(); len(leaks) > 0 {
			fmt.Fprintf(os.Stderr,
				"FAIL: %d window(s) left an animation loop ticking "+
					"after %v; "+
					"add t.Cleanup(w.WindowCleanup) where the test builds "+
					"the window. Animation IDs per window:\n  %s\n",
				len(leaks), leakSettleDeadline, strings.Join(leaks, "\n  "))
			code = 1
		}
	}
	os.Exit(code)
}

// settledAnimationLeaks polls tickingAnimationLoops until it comes back
// empty or leakSettleDeadline passes, and returns what is left then.
// A single snapshot right after m.Run would also catch a finite
// animation that a late-running test left mid-flight — one that
// retires by itself a moment later — so the verdict would depend on
// test order (-shuffle) and timing. What survives the deadline is a
// repeating animation (the kind that ticks forever) or one delayed
// longer than any test should leave pending; both are leaks.
func settledAnimationLeaks() []string {
	deadline := time.Now().Add(leakSettleDeadline)
	for {
		leaks := tickingAnimationLoops()
		if len(leaks) == 0 || time.Now().After(deadline) {
			return leaks
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// animationLoopStopped reports whether w's animation loop was told to
// exit. stopAnimationLoop closes animationStop and leaves the
// animations map as it was, so the map alone cannot tell a stopped
// loop from a live one.
func animationLoopStopped(w *Window) bool {
	select {
	case <-w.animationStop:
		return true
	default:
		return false
	}
}

// tickingAnimationLoops lists, per leaked window, the animation IDs
// still registered on a loop that was started and never stopped. A
// non-empty map means the loop is still ticking: it only parks its
// ticker once every animation has retired.
//
// Only registered windows are seen. WindowCleanup unregisters, so a
// window found here was never cleaned up. A hand-built &Window{} is
// not registered, but it has no lifecycle channels either, so
// ensureAnimationLoop never starts a goroutine for it.
func tickingAnimationLoops() []string {
	liveWindowsMu.Lock()
	windows := slices.Clone(liveWindows)
	liveWindowsMu.Unlock()

	var leaks []string
	for _, w := range windows {
		if !w.animationStarted || animationLoopStopped(w) {
			continue
		}
		w.animMu.Lock()
		ids := make([]string, 0, len(w.animations))
		for id := range w.animations {
			ids = append(ids, id)
		}
		w.animMu.Unlock()
		if len(ids) == 0 {
			continue
		}
		// Sorted so two runs of the same leak print the same line.
		slices.Sort(ids)
		leaks = append(leaks, strings.Join(ids, ", "))
	}
	return leaks
}
