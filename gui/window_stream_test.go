package gui

import (
	"testing"
	"time"
)

type streamTestState struct {
	items []string
}

// streamAppend is the apply body the unit tests share: record the
// value on window state. Stream runs it on the frame thread.
func streamAppend(w *Window, s string) {
	st := State[streamTestState](w)
	st.items = append(st.items, s)
}

// drainStream flushes queued commands until want items are applied
// or the deadline passes. Stream's producer runs on its own
// goroutine, so the test pumps the frame side of the queue.
func drainStream(
	t *testing.T,
	w *Window,
	state *streamTestState,
	want int,
) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w.flushCommands()
		if len(state.items) == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("items: got %d, want %d", len(state.items), want)
}

func TestStreamAppliesItemsInOrder(t *testing.T) {
	state := &streamTestState{}
	w := NewWindow(WindowCfg{State: state})
	defer w.WindowCleanup()
	w.refreshLayout = false

	ch := make(chan string, 4)
	done := Stream(w, ch, streamAppend)
	ch <- "a"
	ch <- "b"
	ch <- "c"
	drainStream(t, w, state, 3)

	for i, want := range []string{"a", "b", "c"} {
		if state.items[i] != want {
			t.Fatalf("items[%d]: got %q, want %q",
				i, state.items[i], want)
		}
	}
	// The #559 regression: values applied but no frame scheduled,
	// so the window would sit stale until the next input event.
	// Stream's UpdateWindow must have armed a full layout refresh.
	if !w.refreshLayout {
		t.Error("refreshLayout: got false, want true")
	}

	select {
	case <-done:
		t.Error("done closed while channel still open")
	default:
	}
	close(ch)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("done not closed after channel close")
	}
}

func TestStreamCtxDoneExits(t *testing.T) {
	state := &streamTestState{}
	w := NewWindow(WindowCfg{State: state})
	defer w.WindowCleanup()

	ch := make(chan string)
	done := Stream(w, ch, streamAppend)
	// No send ever arrives; ending the window must still retire
	// the goroutine instead of leaking it on the receive.
	w.WindowCleanup()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("done not closed after window cleanup")
	}
}

func TestStreamClosedEmptyExits(t *testing.T) {
	state := &streamTestState{}
	w := NewWindow(WindowCfg{State: state})
	defer w.WindowCleanup()

	ch := make(chan string)
	close(ch)
	done := Stream(w, ch, func(w *Window, s string) {
		t.Error("apply ran with no values sent")
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("done not closed for a pre-closed channel")
	}
	w.flushCommands()
	if len(state.items) != 0 {
		t.Errorf("items: got %d, want 0", len(state.items))
	}
}

func TestStreamNilNoOp(t *testing.T) {
	state := &streamTestState{}
	w := NewWindow(WindowCfg{State: state})
	defer w.WindowCleanup()
	apply := func(w *Window, s string) {}
	ch := make(chan string)

	for name, done := range map[string]<-chan struct{}{
		"nil window": Stream[string](nil, ch, apply),
		"nil chan":   Stream[string](w, nil, apply),
		"nil apply":  Stream[string](w, ch, nil),
	} {
		select {
		case <-done:
		default:
			t.Errorf("%s: done not pre-closed", name)
		}
	}
}
