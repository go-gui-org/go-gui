package gui

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

// leakSettleDeadline bounds how long RunTestsCheckingLeaks waits for
// leftover animations to retire on their own before calling them leaks.
// It must outlast animViewBoundStale (a view-bound animation whose
// widget is gone is evicted only after that) plus the longest finite
// tween a test starts (theme fade, toast, layout transition: all well
// under a second). Only a failing run pays it; a clean run returns at
// once. It lives here, not at the call site, because it is derived from
// an internal constant a caller cannot see (#840).
const leakSettleDeadline = animViewBoundStale + 3*time.Second

// RunTestsCheckingLeaks runs the package's tests with m.Run, then fails
// the run when a test left a window whose animation loop is still
// ticking. It returns the exit code for os.Exit. Use it from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(gui.RunTestsCheckingLeaks(m)) }
//
// Setup and teardown compose around it as around m.Run.
//
// Why a leak matters (#836): a window's animation goroutine runs until
// WindowCleanup stops it. In an app the backend calls WindowCleanup; in
// a test nothing does unless the test asks. An idle loop just parks,
// but one holding a repeating animation (the blink cursor of a focused
// Input, a tooltip, drag auto-scroll) ticks at 60 Hz for the rest of
// the process and queues a command each time. testing.AllocsPerRun
// counts mallocs from every goroutine, so alloc gates that allocate
// nothing read non-zero in a full-package run and zero alone. The check
// turns that silent noise into a failure named by the leaked windows'
// animation IDs.
//
// NewTestWindow registers WindowCleanup with its test, so its windows
// cannot leak. A window built with NewWindow needs
// t.Cleanup(w.WindowCleanup).
//
// m is an interface, not *testing.M, so package testing stays out of
// the library's import graph; *testing.M satisfies it. A failing m.Run
// skips the check: its exit code is already non-zero, and the failed
// tests may have left windows mid-flight.
// exportaudit:keep — test harness for consumers' TestMain
func RunTestsCheckingLeaks(m interface{ Run() int }) int {
	code := m.Run()
	if code != 0 {
		return code
	}
	leaks := settledAnimationLeaks()
	if len(leaks) == 0 {
		return code
	}
	fmt.Fprintf(os.Stderr,
		"FAIL: %d window(s) left an animation loop ticking after %v; "+
			"build test windows with NewTestWindow, or add "+
			"t.Cleanup(w.WindowCleanup) where the test calls NewWindow. "+
			"Animation IDs per window:\n  %s\n",
		len(leaks), leakSettleDeadline, strings.Join(leaks, "\n  "))
	return 1
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
