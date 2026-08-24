package gui

import (
	"sync"
	"testing"
)

func TestAppRegisterUnregister(t *testing.T) {
	app := NewApp()

	w1 := NewWindow(WindowCfg{})
	w2 := NewWindow(WindowCfg{})

	app.Register(1, w1)
	app.Register(2, w2)

	if got := app.Window(1); got != w1 {
		t.Fatalf("Window(1) = %v, want %v", got, w1)
	}
	if got := app.Window(2); got != w2 {
		t.Fatalf("Window(2) = %v, want %v", got, w2)
	}
	if got := len(app.Windows()); got != 2 {
		t.Fatalf("len(Windows()) = %d, want 2", got)
	}
	if w1.App() != app {
		t.Fatal("w1.App() != app")
	}
	if w1.PlatformID() != 1 {
		t.Fatalf("w1.PlatformID() = %d, want 1", w1.PlatformID())
	}

	// Unregister non-main window — should not exit.
	if app.Unregister(2) {
		t.Fatal("Unregister(2) should not signal exit")
	}
	if app.Window(2) != nil {
		t.Fatal("Window(2) should be nil after unregister")
	}
	if len(app.Windows()) != 1 {
		t.Fatal("expected 1 window remaining")
	}

	// Unregister last window — should exit.
	if !app.Unregister(1) {
		t.Fatal("Unregister(1) should signal exit")
	}
}

func TestAppExitOnMainClose(t *testing.T) {
	app := NewApp()
	app.ExitMode = ExitOnMainClose

	w1 := NewWindow(WindowCfg{})
	w2 := NewWindow(WindowCfg{})

	app.Register(10, w1)
	app.Register(20, w2)

	// Close non-main window — should not exit.
	if app.Unregister(20) {
		t.Fatal("closing non-main should not exit")
	}

	// Close main window — should exit.
	if !app.Unregister(10) {
		t.Fatal("closing main should exit")
	}
}

func TestAppBroadcast(t *testing.T) {
	app := NewApp()
	w1 := NewWindow(WindowCfg{State: new(int)})
	w2 := NewWindow(WindowCfg{State: new(int)})
	app.Register(1, w1)
	app.Register(2, w2)

	app.Broadcast(func(w *Window) {
		*State[int](w)++
	})

	if *State[int](w1) != 1 {
		t.Fatal("broadcast did not reach w1")
	}
	if *State[int](w2) != 1 {
		t.Fatal("broadcast did not reach w2")
	}
}

func TestAppOpenWindow(t *testing.T) {
	app := NewApp()
	cfg := WindowCfg{Title: "new"}
	app.OpenWindow(cfg)

	select {
	case got := <-app.PendingOpen():
		if got.Title != "new" {
			t.Fatalf("pending title = %q, want %q",
				got.Title, "new")
		}
	default:
		t.Fatal("expected pending window")
	}
}

func TestAppOpenWindowWakes(t *testing.T) {
	// A queued window must wake the backend's idle event loop, which
	// cannot select on the pending channel (issue #405).
	app := NewApp()
	wakes := 0
	app.SetWakeMainFn(func() { wakes++ })
	app.OpenWindow(WindowCfg{Title: "new"})
	if wakes != 1 {
		t.Fatalf("wakes = %d, want 1", wakes)
	}

	// Buffer full: the request is dropped, so no wake.
	for range 16 {
		app.OpenWindow(WindowCfg{Title: "ok"})
	}
	app.OpenWindow(WindowCfg{Title: "dropped"})
	// 15 of the 16 fit (one slot is still taken); the dropped 17th
	// must not wake.
	if wakes != 16 {
		t.Fatalf("wakes = %d, want 16 (no wake on dropped request)", wakes)
	}
}

func TestAppOpenWindowNoWakeFn(t *testing.T) {
	// No wake fn (or one cleared with nil): OpenWindow must queue and
	// return without panicking. Backends that select on the pending
	// channel directly (x11) never set one.
	app := NewApp()
	app.SetWakeMainFn(func() { t.Error("wake called after clear") })
	app.SetWakeMainFn(nil)
	app.OpenWindow(WindowCfg{Title: "new"})
}

func TestEventWindowID(t *testing.T) {
	e := Event{WindowID: 42, Type: EventMouseDown}
	if e.WindowID != 42 {
		t.Fatalf("WindowID = %d, want 42", e.WindowID)
	}
}

func TestWindowClose(t *testing.T) {
	w := NewWindow(WindowCfg{})
	if w.CloseRequested() {
		t.Fatal("should not be close-requested initially")
	}
	w.Close()
	if !w.CloseRequested() {
		t.Fatal("should be close-requested after Close()")
	}
}

func TestWindowOnCloseRequest(t *testing.T) {
	// Veto path: callback set, does not call Close; window stays open.
	vetoCalls := 0
	wv := NewWindow(WindowCfg{
		OnCloseRequest: func(*Window) { vetoCalls++ },
	})
	if wv.Config.OnCloseRequest == nil {
		t.Fatal("OnCloseRequest should survive NewWindow")
	}
	wv.Config.OnCloseRequest(wv)
	if vetoCalls != 1 {
		t.Fatalf("vetoCalls = %d, want 1", vetoCalls)
	}
	if wv.CloseRequested() {
		t.Fatal("veto path must not mark close-requested")
	}

	// Proceed path: callback calls Close; window marked for teardown.
	wp := NewWindow(WindowCfg{
		OnCloseRequest: func(w *Window) { w.Close() },
	})
	wp.Config.OnCloseRequest(wp)
	if !wp.CloseRequested() {
		t.Fatal("proceed path must mark close-requested")
	}
}

func TestWindowCloseConcurrent(t *testing.T) {
	w := NewWindow(WindowCfg{})
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			w.Close()
		})
	}
	wg.Wait()
	if !w.CloseRequested() {
		t.Fatal("expected close-requested after concurrent Close calls")
	}
}

func TestAppRegisterDuplicate(t *testing.T) {
	app := NewApp()
	w1 := NewWindow(WindowCfg{})
	w2 := NewWindow(WindowCfg{})

	app.Register(1, w1)
	app.Register(1, w2) // duplicate — should be ignored

	if got := app.Window(1); got != w1 {
		t.Fatal("duplicate Register should keep original window")
	}
	if got := len(app.Windows()); got != 1 {
		t.Fatalf("len(Windows()) = %d, want 1", got)
	}
}

func TestAppOpenWindowBufferFull(t *testing.T) {
	app := NewApp()
	// Fill the buffer (cap 16).
	for range 16 {
		app.OpenWindow(WindowCfg{Title: "ok"})
	}
	// 17th should be dropped without panic.
	app.OpenWindow(WindowCfg{Title: "dropped"})

	count := 0
	for {
		select {
		case <-app.PendingOpen():
			count++
		default:
			goto done
		}
	}
done:
	if count != 16 {
		t.Fatalf("drained %d pending, want 16", count)
	}
}

func TestAppBroadcastDuringUnregister(t *testing.T) {
	app := NewApp()
	w1 := NewWindow(WindowCfg{State: new(int)})
	w2 := NewWindow(WindowCfg{State: new(int)})
	app.Register(1, w1)
	app.Register(2, w2)

	// Unregister w2 then broadcast — should only reach w1.
	app.Unregister(2)
	app.Broadcast(func(w *Window) {
		*State[int](w)++
	})

	if *State[int](w1) != 1 {
		t.Fatal("broadcast did not reach w1")
	}
	if *State[int](w2) != 0 {
		t.Fatal("broadcast should not reach unregistered w2")
	}
}
