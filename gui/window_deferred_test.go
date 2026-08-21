package gui

import (
	"strings"
	"testing"
	"time"
)

// Deferred callbacks run in the order they were raised, and they run
// with no lock held — the whole point, so SetFocus is legal in one.
func TestFlushDeferredCallbacksRunsInOrder(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	var order []string
	w.deferCallback(func(*Window) { order = append(order, "first") })
	w.deferCallback(func(w *Window) {
		order = append(order, "second")
		w.SetFocus("somewhere")
	})
	w.deferCallback(nil) // dropped, not queued
	w.flushDeferredCallbacks()

	if strings.Join(order, ",") != "first,second" {
		t.Fatalf("order = %v, want [first second]", order)
	}
	if w.FocusID() != "somewhere" {
		t.Fatalf("focus = %q, want %q", w.FocusID(), "somewhere")
	}
}

// A callback that defers another is legitimate — an OnBlur that moves
// focus blurs the next field — so the flush keeps draining rather than
// leaving the second one for the following frame.
func TestFlushDeferredCallbacksDrainsCascade(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	ran := 0
	w.deferCallback(func(w *Window) {
		ran++
		w.deferCallback(func(*Window) { ran++ })
	})
	w.flushDeferredCallbacks()
	if ran != 2 {
		t.Fatalf("ran = %d, want 2 (the cascade must drain)", ran)
	}
}

// A pair of callbacks that re-raise each other must not spin the frame
// forever. The flush bounds the rounds, drops the rest and says so.
func TestFlushDeferredCallbacksBoundsRunaway(t *testing.T) {
	buf := captureDebugMask(t, DebugCallbacks)
	w := NewTestWindow(WindowCfg{})
	ran := 0
	var again func(*Window)
	again = func(w *Window) {
		ran++
		w.deferCallback(again)
	}
	w.deferCallback(again)

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.flushDeferredCallbacks()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a self-requeueing callback spun the flush forever")
	}

	if ran != maxDeferredCallbackBatches {
		t.Fatalf("ran = %d, want %d rounds before the bound",
			ran, maxDeferredCallbackBatches)
	}
	if len(w.deferredCallbacks) != 0 {
		t.Fatalf("%d callbacks left queued, want the batch dropped",
			len(w.deferredCallbacks))
	}
	if !strings.Contains(buf.String(), "re-queueing") {
		t.Fatalf("no runaway diagnostic reported, got %q", buf.String())
	}
}

// The fail-fast probe: a window API called while the frame lock is held
// panics with the API's name instead of blocking forever. This is the
// path an app takes when it calls SetFocus from its own AmendLayout
// hook, which cannot be deferred — the hook exists to mutate the tree.
func TestLockForAPIPanicsUnderFrameLock(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.mu.Lock()
	w.inFramePass.Store(true)
	defer func() {
		w.inFramePass.Store(false)
		w.mu.Unlock()
		r := recover()
		if r == nil {
			t.Fatal("SetFocus under the frame lock did not panic")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "SetFocus") {
			t.Fatalf("panic = %v, want the API name in the message", r)
		}
		if !strings.Contains(msg, "QueueCommand") {
			t.Fatalf("panic = %v, want the remedy in the message", r)
		}
	}()
	w.SetFocus("boom")
}

// An app AmendLayout hook is the one route into app code the frame pass
// cannot defer, so it is the case the probe has to catch end to end.
func TestAppAmendLayoutHookCallingSetFocusPanics(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("SetFocus from an app AmendLayout hook did not panic")
		}
		if msg, ok := r.(string); !ok ||
			!strings.Contains(msg, "AmendLayout") {
			t.Fatalf("panic = %v, want AmendLayout named", r)
		}
	}()
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			ID: "amender",
			AmendLayout: func(ctx EventCtx) {
				ctx.Window.SetFocus("elsewhere")
			},
		})
	})
}

// A flush that ran callbacks reports it, an empty one does not.
func TestFlushDeferredCallbacksReports(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	if w.flushDeferredCallbacks() {
		t.Fatal("an empty flush reported callbacks")
	}
	w.deferCallback(func(*Window) {})
	if !w.flushDeferredCallbacks() {
		t.Fatal("a flush that ran a callback reported nothing")
	}
	if w.flushDeferredCallbacks() {
		t.Fatal("a second empty flush reported callbacks")
	}
}

// A deferred callback that writes app state sets no refresh flag
// (SetFocus and plain state writes are silent), so only the flush
// report can make FrameFn re-run. Without the re-run the frame shows
// the pre-callback text until the next event.
func TestFrameFnRerunsPassAfterDeferredCallbacks(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	label := "before"
	w.UpdateView(func(*Window) View {
		return Text(TextCfg{ID: "label", Text: label})
	})
	w.deferCallback(func(*Window) { label = "after" })
	w.markLayoutRefresh()

	if !w.FrameFn() {
		t.Fatal("FrameFn did not report a rebuild")
	}
	ly, ok := w.layout.FindByID("label")
	if !ok {
		t.Fatal("label shape missing after FrameFn")
	}
	if got := ly.Shape.TC.Text; got != "after" {
		t.Fatalf("frame shows %q, want %q — the deferred callback's "+
			"state change needs the re-run pass", got, "after")
	}
}

// The flip side of the re-run: a frame with nothing deferred must not
// pay for it — one pass, one view generation.
func TestFrameFnSinglePassWithoutDeferredCallbacks(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	gens := 0
	w.UpdateView(func(*Window) View {
		gens++
		return Text(TextCfg{ID: "t"})
	})
	w.markLayoutRefresh()
	w.FrameFn()
	if gens != 1 {
		t.Fatalf("view generated %d times, want 1", gens)
	}
}

// The re-run is bounded: a view that dirties itself every pass must
// not make FrameFn spin — two passes and out. Driven behind a timeout
// because the failure mode (an unbounded loop) would otherwise hang
// the whole package.
func TestFrameFnStopsAfterTwoPasses(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	gens := 0
	w.UpdateView(func(w *Window) View {
		gens++
		w.UpdateWindow() // dirty the window from inside the pass
		return Text(TextCfg{ID: "t"})
	})
	w.markLayoutRefresh()

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.FrameFn()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a self-dirtying view made FrameFn spin")
	}
	if gens != 2 {
		t.Fatalf("view generated %d times, want 2 (bounded)", gens)
	}
}
