package gui

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-gui-org/go-gui/gui/appinfo"
)

// Issue #1172. The keep-awake state belongs to the whole app, so these
// tests must not run in parallel, and each must leave it idle:
// newKeepAwakeWindow checks that at the start.

// keepAwakeCall is one SetKeepAwake call a spy saw.
type keepAwakeCall struct {
	on          bool
	app, reason string
}

// keepAwakeSpy records SetKeepAwake calls and returns err.
type keepAwakeSpy struct {
	noopNativePlatform
	calls []keepAwakeCall
	err   error
}

func (s *keepAwakeSpy) SetKeepAwake(on bool, app, reason string) error {
	s.calls = append(s.calls, keepAwakeCall{on: on, app: app, reason: reason})
	return s.err
}

// requireKeepAwakeIdle fails when an earlier test left a token or the
// OS request behind: every later test would read the wrong state.
func requireKeepAwakeIdle(t *testing.T) {
	t.Helper()
	if n := keepAwakeHolderCount(); n != 0 {
		t.Fatalf("an earlier test left %d windows holding tokens", n)
	}
	s := &keepAwakeState
	s.applyMu.Lock()
	applied := s.applied
	s.applyMu.Unlock()
	if applied {
		t.Fatal("an earlier test left the request on")
	}
}

func keepAwakeHolderCount() int {
	s := &keepAwakeState
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.holders)
}

// newKeepAwakeWindow returns a window with spy as its platform. The
// window is cleaned up when the test ends.
func newKeepAwakeWindow(t *testing.T, spy *keepAwakeSpy, cfg WindowCfg) *Window {
	t.Helper()
	requireKeepAwakeIdle(t)
	w := NewTestWindow(t, cfg)
	w.SetNativePlatform(spy)
	return w
}

// assertKeepAwakeEdges compares the on values of the spy's calls.
func assertKeepAwakeEdges(t *testing.T, spy *keepAwakeSpy, want ...bool) {
	t.Helper()
	got := make([]bool, len(spy.calls))
	for i, c := range spy.calls {
		got[i] = c.on
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SetKeepAwake calls = %v, want %v", got, want)
	}
}

// Two players in one window: the display stays on until both stopped.
func TestKeepAwakeTwoTokensOneRelease(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	release1 := w.KeepAwake("Player 1")
	release2 := w.KeepAwake("Player 2")
	w.flushCommands()
	assertKeepAwakeEdges(t, spy, true)

	release1()
	w.flushCommands()
	assertKeepAwakeEdges(t, spy, true)

	release2()
	w.flushCommands()
	assertKeepAwakeEdges(t, spy, true, false)
}

// A second call of a release does nothing; above all, it does not end
// another player's request.
func TestKeepAwakeDoubleReleaseIsNoOp(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	release1 := w.KeepAwake("")
	release2 := w.KeepAwake("")
	w.flushCommands()

	release1()
	release1()
	w.flushCommands()
	assertKeepAwakeEdges(t, spy, true)

	release2()
	w.flushCommands()
	assertKeepAwakeEdges(t, spy, true, false)
}

// A request taken and released before the next frame never reaches the
// OS.
func TestKeepAwakeWithinOneFrameMakesNoCall(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	w.KeepAwake("")()
	w.flushCommands()
	assertKeepAwakeEdges(t, spy)
}

// Closing a window ends its requests at once: the commands queued on a
// destroyed window never run. Its release functions then do nothing,
// and it takes no new request.
func TestKeepAwakeWindowCleanupReleases(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	release := w.KeepAwake("")
	w.flushCommands()

	w.WindowCleanup()
	assertKeepAwakeEdges(t, spy, true, false)

	release()
	w.KeepAwake("")()
	w.flushCommands()
	assertKeepAwakeEdges(t, spy, true, false)
	if n := keepAwakeHolderCount(); n != 0 {
		t.Errorf("%d windows hold tokens after cleanup, want 0", n)
	}
}

// A release just before the window closes queues its reconcile on that
// window, where it never runs. WindowCleanup must end the request.
func TestKeepAwakeReleaseThenClose(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	release := w.KeepAwake("")
	w.flushCommands()

	release()
	w.WindowCleanup()
	assertKeepAwakeEdges(t, spy, true, false)
}

// Hide ends the request and Show asks again; a minimize and the restore
// do the same.
func TestKeepAwakeFollowsVisibility(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	release := w.KeepAwake("Playing a video")
	defer release()
	w.flushCommands()

	w.Hide()
	assertKeepAwakeEdges(t, spy, true, false)
	w.Show()
	assertKeepAwakeEdges(t, spy, true, false, true)
	DispatchWindowOccluded(w, true)
	assertKeepAwakeEdges(t, spy, true, false, true, false)
	DispatchWindowOccluded(w, false)
	assertKeepAwakeEdges(t, spy, true, false, true, false, true)
	if got := spy.calls[4].reason; got != "Playing a video" {
		t.Errorf("reason after restore = %q, want the request's own", got)
	}
}

// A token taken while the window is minimized asks for nothing until
// the window is back on screen.
func TestKeepAwakeTakenWhileOccluded(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	DispatchWindowOccluded(w, true)
	release := w.KeepAwake("")
	defer release()
	w.flushCommands()
	assertKeepAwakeEdges(t, spy)

	DispatchWindowOccluded(w, false)
	assertKeepAwakeEdges(t, spy, true)
}

// While the time-travel debugger holds the window's events, the request
// still follows what the user sees.
func TestKeepAwakeFollowsVisibilityWhileFrozen(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	release := w.KeepAwake("")
	defer release()
	w.flushCommands()

	w.frozen.Store(true)
	DispatchWindowOccluded(w, true)
	w.frozen.Store(false)
	assertKeepAwakeEdges(t, spy, true, false)
}

// A request made before the backend attached is asked for when it
// attaches.
func TestKeepAwakeAskedWhenPlatformAttaches(t *testing.T) {
	requireKeepAwakeIdle(t)
	w := NewTestWindow(t, WindowCfg{})
	release := w.KeepAwake("")
	defer release()
	w.flushCommands() // no platform yet

	spy := &keepAwakeSpy{}
	w.SetNativePlatform(spy)
	assertKeepAwakeEdges(t, spy, true)
}

// The OS request belongs to the whole app: it stays on while some
// window with a request is on screen, whatever the others do.
func TestKeepAwakeAcrossWindows(t *testing.T) {
	spy := &keepAwakeSpy{}
	a := newKeepAwakeWindow(t, spy, WindowCfg{Title: "A"})
	b := NewTestWindow(t, WindowCfg{Title: "B"})
	b.SetNativePlatform(spy)

	releaseA := a.KeepAwake("A plays")
	a.flushCommands()
	releaseB := b.KeepAwake("B plays")
	b.flushCommands()
	assertKeepAwakeEdges(t, spy, true)

	DispatchWindowOccluded(a, true) // B is still on screen
	assertKeepAwakeEdges(t, spy, true)

	b.WindowCleanup() // now no window with a request is on screen
	assertKeepAwakeEdges(t, spy, true, false)

	DispatchWindowOccluded(a, false)
	assertKeepAwakeEdges(t, spy, true, false, true)
	if c := spy.calls[2]; c.app != "A" || c.reason != "A plays" {
		t.Errorf("request = %+v, want app A with A's reason", c)
	}

	releaseA()
	releaseB() // B is gone: does nothing
	a.flushCommands()
	assertKeepAwakeEdges(t, spy, true, false, true, false)
}

// The OS gets the reason of the oldest request when the display starts
// being kept on. A release of that request leaves it in place; the next
// start (here after Hide and Show) takes the reason of the oldest left.
func TestKeepAwakeFirstReasonIsShown(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	release1 := w.KeepAwake("first")
	release2 := w.KeepAwake("second")
	defer release2()
	w.flushCommands()
	release1()
	w.flushCommands()
	w.Hide()
	w.Show()

	assertKeepAwakeEdges(t, spy, true, false, true)
	if spy.calls[0].reason != "first" || spy.calls[2].reason != "second" {
		t.Errorf("reasons = %q, %q; want first, then second",
			spy.calls[0].reason, spy.calls[2].reason)
	}
}

// The platform never gets an empty string: gnome-session refuses an
// inhibit without a reason.
func TestKeepAwakeStringsNeverEmpty(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{}) // no manifest, no title
	release := w.KeepAwake(" \t ")
	defer release()
	w.flushCommands()

	want := keepAwakeCall{on: true, app: "go-gui",
		reason: "Keeping the display awake"}
	if len(spy.calls) != 1 || spy.calls[0] != want {
		t.Fatalf("calls = %+v, want [%+v]", spy.calls, want)
	}
}

func TestKeepAwakeAppNameOrder(t *testing.T) {
	cases := []struct {
		name string
		cfg  WindowCfg
		want string
	}{
		{"manifest ID", WindowCfg{
			AppInfo: appinfo.Info{ID: "org.example.Player", Name: "Player"},
			Title:   "Film"}, "org.example.Player"},
		{"manifest name", WindowCfg{
			AppInfo: appinfo.Info{Name: "Player"}, Title: "Film"}, "Player"},
		{"title", WindowCfg{Title: "Film"}, "Film"},
		{"blank title", WindowCfg{Title: " \x00 "}, "go-gui"},
	}
	for _, c := range cases {
		if got := keepAwakeAppName(&Window{Config: c.cfg}); got != c.want {
			t.Errorf("%s: app = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestKeepAwakeReasonText(t *testing.T) {
	long := strings.Repeat("é", 200)
	cut := strings.Repeat("a", keepAwakeMaxChars-1) + " b"
	cases := []struct{ in, want string }{
		{"", keepAwakeDefaultReason},
		{" \n\t", keepAwakeDefaultReason},
		{"\x00", keepAwakeDefaultReason},
		{"  Playing a video ", "Playing a video"},
		{"Play\x00ing", "Playing"},
		{"bad \xff byte", "bad � byte"},
		{long, strings.Repeat("é", keepAwakeMaxChars)},
		// A cut that ends on a space drops it too.
		{cut, strings.Repeat("a", keepAwakeMaxChars-1)},
	}
	for _, c := range cases {
		if got := keepAwakeReason(c.in); got != c.want {
			t.Errorf("keepAwakeReason(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// An error from the platform is reported once, and since nothing is
// held, the next change asks again.
func TestKeepAwakePlatformErrorIsReported(t *testing.T) {
	spy := &keepAwakeSpy{err: errors.ErrUnsupported}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	buf := captureDebugMask(t, DebugWindowDegraded)

	release1 := w.KeepAwake("")
	w.flushCommands()
	release2 := w.KeepAwake("")
	w.flushCommands()
	assertKeepAwakeEdges(t, spy, true, true)
	if n := strings.Count(buf.String(), "keep awake: unsupported operation"); n != 1 {
		t.Errorf("error reported %d times, want once:\n%s", n, buf.String())
	}

	spy.err = nil
	release1()
	release2()
	w.flushCommands()
	assertKeepAwakeEdges(t, spy, true, true) // nothing held: no off
}

// KeepAwake and release from many goroutines at once. Run with -race.
// Once all have released, nothing is held.
func TestKeepAwakeFromManyGoroutines(t *testing.T) {
	spy := &keepAwakeSpy{}
	w := newKeepAwakeWindow(t, spy, WindowCfg{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				w.KeepAwake("")()
			}
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	for running := true; running; {
		select {
		case <-done:
			running = false
		default:
			w.flushCommands()
		}
	}
	w.flushCommands()

	if n := keepAwakeHolderCount(); n != 0 {
		t.Fatalf("%d windows hold tokens, want 0", n)
	}
	for i, c := range spy.calls {
		if c.on != (i%2 == 0) {
			t.Fatalf("call %d on=%v: calls must alternate on, off", i, c.on)
		}
	}
	if len(spy.calls)%2 != 0 {
		t.Fatalf("%d calls end with on, want off", len(spy.calls))
	}
}

// KeepAwake does not take the frame lock, so AmendLayout may call it.
// Were it to, this test would hang.
func TestKeepAwakeUnderFrameLock(t *testing.T) {
	w := newKeepAwakeWindow(t, &keepAwakeSpy{}, WindowCfg{})
	w.mu.Lock()
	release := w.KeepAwake("")
	release()
	w.mu.Unlock()
}

func TestKeepAwakeNilWindow(t *testing.T) {
	var w *Window
	w.KeepAwake("")() // must not panic
}

func TestKeepAwakeDebugCategory(t *testing.T) {
	if got := checkCategory(debugCheckKeepAwake); got != DebugWindowDegraded {
		t.Errorf("checkCategory(debugCheckKeepAwake) = %v, want DebugWindowDegraded", got)
	}
}
