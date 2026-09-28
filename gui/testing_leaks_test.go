package gui

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeTestMain stands in for *testing.M.
type fakeTestMain struct{ code int }

func (f fakeTestMain) Run() int { return f.code }

// A failing run keeps its own exit code and skips the leak poll, so a
// red run does not also wait out leakSettleDeadline.
func TestRunTestsCheckingLeaksPassesFailingCode(t *testing.T) {
	start := time.Now()
	if got := RunTestsCheckingLeaks(fakeTestMain{code: 3}); got != 3 {
		t.Fatalf("code = %d, want 3", got)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("failing run took %v; the leak poll should be skipped", d)
	}
}

// A window with a repeating animation and no WindowCleanup is reported
// by its animation IDs; after WindowCleanup it is not. The window is
// built with NewWindow on purpose: NewTestWindow would clean it up.
func TestTickingAnimationLoopsNamesLeakedWindow(t *testing.T) {
	const id = "leak-check-probe"
	w := NewWindow(WindowCfg{Width: 100, Height: 100})
	t.Cleanup(w.WindowCleanup)
	w.AnimationAdd(&Animate{
		AnimID:   id,
		Repeat:   true,
		Delay:    time.Hour,
		Callback: func(*Animate, *Window) {},
	})

	hasProbe := func() bool {
		return slices.ContainsFunc(tickingAnimationLoops(), func(s string) bool {
			return strings.Contains(s, id)
		})
	}
	if !hasProbe() {
		t.Fatalf("leak not reported: %v", tickingAnimationLoops())
	}
	w.WindowCleanup()
	if hasProbe() {
		t.Fatalf("cleaned-up window still reported: %v", tickingAnimationLoops())
	}
}

// NewTestWindow registers WindowCleanup with its test: once the subtest
// ends, the window's repeating animation no longer counts as a leak.
func TestNewTestWindowCleansUpAfterTest(t *testing.T) {
	const id = "new-test-window-probe"
	var w *Window
	t.Run("build", func(st *testing.T) {
		w = NewTestWindow(st, WindowCfg{})
		w.AnimationAdd(&Animate{
			AnimID:   id,
			Repeat:   true,
			Delay:    time.Hour,
			Callback: func(*Animate, *Window) {},
		})
	})
	if !w.destroyed.Load() {
		t.Fatal("NewTestWindow did not register WindowCleanup with its test")
	}
	for _, s := range tickingAnimationLoops() {
		if strings.Contains(s, id) {
			t.Fatalf("window still ticking after its test ended: %s", s)
		}
	}
}
