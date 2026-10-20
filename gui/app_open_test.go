package gui

import (
	"reflect"
	"sync"
	"testing"
)

func TestAppDispatchOpenBuffersUntilRegister(t *testing.T) {
	app := NewApp()
	var got []OpenEvent
	app.OnOpen = func(a *App, ev OpenEvent) {
		if a != app {
			t.Error("OnOpen got a different App")
		}
		got = append(got, ev)
	}
	// Launch-time events arrive before any window exists.
	app.DispatchOpen(OpenEvent{Files: []string{"/a"}})
	app.DispatchOpen(OpenEvent{URLs: []string{"my://b"}})
	if len(got) != 0 {
		t.Fatal("OnOpen ran before a window existed")
	}
	w := NewWindow(WindowCfg{})
	app.Register(1, w)
	if len(got) != 0 {
		t.Fatal("OnOpen ran off the frame (not via the command queue)")
	}
	w.flushCommands()
	want := []OpenEvent{
		{Files: []string{"/a"}},
		{URLs: []string{"my://b"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	// After the main window exists, delivery goes straight to its queue.
	app.DispatchOpen(OpenEvent{Files: []string{"/c"}})
	w.flushCommands()
	if len(got) != 3 || got[2].Files[0] != "/c" {
		t.Fatalf("got %+v, want third event /c", got)
	}
}

func TestAppDispatchOpenEmptyIgnored(t *testing.T) {
	app := NewApp()
	calls := 0
	app.OnOpen = func(*App, OpenEvent) { calls++ }
	w := NewWindow(WindowCfg{})
	app.Register(1, w)
	app.DispatchOpen(OpenEvent{})
	w.flushCommands()
	if calls != 0 {
		t.Fatalf("empty event delivered %d times", calls)
	}
}

func TestAppDispatchOpenNilSafe(t *testing.T) {
	var app *App
	app.DispatchOpen(OpenEvent{Files: []string{"/a"}}) // must not panic
	// A nil OnOpen drops the event when it reaches the main thread.
	app = NewApp()
	w := NewWindow(WindowCfg{})
	app.Register(1, w)
	app.DispatchOpen(OpenEvent{Files: []string{"/a"}})
	w.flushCommands()
}

func TestAppDispatchOpenBufferCap(t *testing.T) {
	app := NewApp()
	calls := 0
	app.OnOpen = func(*App, OpenEvent) { calls++ }
	for range openBufferCap + 10 {
		app.DispatchOpen(OpenEvent{Files: []string{"/a"}})
	}
	w := NewWindow(WindowCfg{})
	app.Register(1, w)
	w.flushCommands()
	if calls != openBufferCap {
		t.Fatalf("delivered %d, want cap %d", calls, openBufferCap)
	}
}

func TestAppDispatchOpenConcurrent(t *testing.T) {
	app := NewApp()
	calls := 0
	app.OnOpen = func(*App, OpenEvent) { calls++ }
	w := NewWindow(WindowCfg{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { app.DispatchOpen(OpenEvent{Files: []string{"/a"}}) })
	}
	app.Register(1, w)
	wg.Wait()
	w.flushCommands()
	if calls != 8 {
		t.Fatalf("delivered %d, want 8", calls)
	}
}

// OnOpen almost always changes state the view shows, and a queued
// command sets no refresh flag of its own. Without the refresh, a live
// app ran OnOpen and kept showing the old frame.
func TestAppDispatchOpenRefreshesLayout(t *testing.T) {
	app := NewApp()
	app.OnOpen = func(*App, OpenEvent) {}
	w := NewWindow(WindowCfg{})
	app.Register(1, w)
	w.flushCommands()
	w.refresh.Store(0) // NewWindow and Register leave flags set
	app.DispatchOpen(OpenEvent{Files: []string{"/a"}})
	w.flushCommands()
	if !w.layoutPending() {
		t.Fatal("OnOpen ran without marking a layout refresh")
	}
}

// A forwarded launch can call DispatchOpen while the backend still
// installs the wake function on the main window (#1106). The read and
// the write must not race.
func TestWakeMainFnConcurrentSet(t *testing.T) {
	w := &Window{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			w.wakeMain()
		}
	}()
	for range 1000 {
		w.SetWakeMainFn(func() {})
	}
	<-done
}

// An event queued on the main window must survive that window closing
// before its next frame while another window keeps the app running.
func TestAppDispatchOpenSurvivesMainWindowClose(t *testing.T) {
	app := NewApp()
	var got []OpenEvent
	app.OnOpen = func(_ *App, ev OpenEvent) { got = append(got, ev) }
	w1 := NewWindow(WindowCfg{})
	w2 := NewWindow(WindowCfg{})
	app.Register(1, w1)
	app.Register(2, w2)
	w1.flushCommands()
	w2.flushCommands()
	app.DispatchOpen(OpenEvent{Files: []string{"/a"}})
	app.Unregister(1) // closes before its frame flushes the command
	w2.flushCommands()
	if len(got) != 1 || got[0].Files[0] != "/a" {
		t.Fatalf("got %+v, want /a delivered through the new main window", got)
	}
}
