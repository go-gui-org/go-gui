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
	for range pendingCap {
		app.OpenWindow(WindowCfg{Title: "ok"})
	}
	app.OpenWindow(WindowCfg{Title: "dropped"})
	// pendingCap-1 of the pendingCap fit (one slot is still
	// taken); the dropped request must not wake.
	if wakes != pendingCap {
		t.Fatalf("wakes = %d, want %d (no wake on dropped request)",
			wakes, pendingCap)
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
	// Fill the buffer (cap pendingCap).
	for range pendingCap {
		app.OpenWindow(WindowCfg{Title: "ok"})
	}
	// Next one should be dropped without panic.
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
	if count != pendingCap {
		t.Fatalf("drained %d pending, want %d", count, pendingCap)
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

func TestAppMainFailover(t *testing.T) {
	app := NewApp()
	w1 := NewWindow(WindowCfg{})
	w2 := NewWindow(WindowCfg{})
	app.Register(1, w1)
	app.Register(2, w2)

	// Closing the main window reports exit (ExitOnMainClose)
	// but hands mainID to the oldest survivor.
	if !app.Unregister(1) {
		t.Fatal("closing main should signal exit")
	}
	if got := app.mainWindow(); got != w2 {
		t.Fatalf("mainWindow = %v, want w2", got)
	}
	if w1.App() != nil {
		t.Fatal("unregistered w1 should detach")
	}
	if w1.PlatformID() != 0 {
		t.Fatalf("w1.PlatformID() = %d, want 0", w1.PlatformID())
	}

	// Last window out clears the main ID.
	app.Unregister(2)
	if got := app.mainWindow(); got != nil {
		t.Fatalf("mainWindow = %v, want nil", got)
	}
}

func TestAppMainFailoverTrayMode(t *testing.T) {
	app := NewApp()
	app.ExitMode = ExitOnTrayRemoved
	w1 := NewWindow(WindowCfg{})
	w2 := NewWindow(WindowCfg{})
	app.Register(1, w1)
	app.Register(2, w2)
	app.trays[7] = &SystemTrayHandle{id: 7}

	// Tray keeps the app alive past a main-window close, and
	// the survivor answers as main.
	if app.Unregister(1) {
		t.Fatal("tray mode must not exit while windows remain")
	}
	if got := app.mainWindow(); got != w2 {
		t.Fatalf("mainWindow = %v, want w2", got)
	}
}

func TestAppRegisterNil(t *testing.T) {
	app := NewApp()
	// Must not panic.
	app.Register(1, nil)
	var nilApp *App
	nilApp.Register(1, NewWindow(WindowCfg{}))
	if app.Window(1) != nil {
		t.Fatal("nil window must not register")
	}
	nilApp.Broadcast(func(*Window) { t.Error("nil app broadcast ran") })
	nilApp.OpenWindow(WindowCfg{})
	nilApp.SetWakeMainFn(func() {})
	if nilApp.Window(1) != nil || nilApp.Windows() != nil {
		t.Fatal("nil app must answer nil")
	}
	if nilApp.PendingOpen() != nil {
		t.Fatal("nil app must answer nil channel")
	}

	// Zero-value App (no NewApp) must accept a registration.
	var zero App
	w := NewWindow(WindowCfg{})
	zero.Register(1, w)
	if got := zero.Window(1); got != w {
		t.Fatal("zero-value App should register after lazy init")
	}
	if w.App() != &zero || w.PlatformID() != 1 {
		t.Fatal("zero-value App should link the window")
	}
}

func TestAppWakeMainFnConcurrent(t *testing.T) {
	// OpenWindow and SetWakeMainFn run on different goroutines
	// in production; -race must stay silent.
	app := NewApp()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			app.SetWakeMainFn(func() {})
			app.OpenWindow(WindowCfg{Title: "race"})
			_ = i
		})
	}
	wg.Wait()
}
