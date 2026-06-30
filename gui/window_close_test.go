package gui

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestDispatchCloseRequest_NilWindow(t *testing.T) {
	// Must not panic.
	DispatchCloseRequest(nil)
}

func TestDispatchCloseRequest_NilHookFallsThrough(t *testing.T) {
	w := NewWindow(WindowCfg{})
	DispatchCloseRequest(w)
	if !w.CloseRequested() {
		t.Fatal("no hook: should mark close-requested")
	}
}

func TestDispatchCloseRequest_HookVetoes(t *testing.T) {
	var calls int
	w := NewWindow(WindowCfg{
		OnCloseRequest: func(*Window) { calls++ },
	})
	DispatchCloseRequest(w)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if w.CloseRequested() {
		t.Fatal("veto: must not mark close-requested")
	}
}

func TestDispatchCloseRequest_DialogDedupsHook(t *testing.T) {
	// A close request while a dialog is already showing must not
	// re-invoke OnCloseRequest, which would stack a duplicate dialog.
	var calls int
	w := NewWindow(WindowCfg{
		OnCloseRequest: func(*Window) { calls++ },
	})
	w.Dialog(DialogCfg{})
	DispatchCloseRequest(w)
	if calls != 0 {
		t.Fatalf("calls = %d, want 0 (dialog visible: hook deduped)", calls)
	}
}

func TestDispatchCloseRequest_NativeDialogDedupsHook(t *testing.T) {
	// Native modal visibility (nativeDialogVisible) must dedup the close
	// hook just like the in-app dialog, preventing a stacked dialog.
	var calls int
	w := NewWindow(WindowCfg{
		OnCloseRequest: func(*Window) { calls++ },
	})
	w.nativeDialogVisible = true
	DispatchCloseRequest(w)
	if calls != 0 {
		t.Fatalf("calls = %d, want 0 (native dialog visible: hook deduped)", calls)
	}
}

func TestDispatchCloseRequest_HookProceeds(t *testing.T) {
	w := NewWindow(WindowCfg{
		OnCloseRequest: func(w *Window) { w.Close() },
	})
	DispatchCloseRequest(w)
	if !w.CloseRequested() {
		t.Fatal("proceed: must mark close-requested")
	}
}

func TestDispatchCloseRequest_RetryAfterVeto(t *testing.T) {
	// First call vetoes; second call proceeds. Verifies the hook
	// is not one-shot and Config survives between invocations.
	var calls int
	w := NewWindow(WindowCfg{
		OnCloseRequest: func(w *Window) {
			calls++
			if calls == 2 {
				w.Close()
			}
		},
	})
	DispatchCloseRequest(w)
	if w.CloseRequested() {
		t.Fatal("first call vetoes")
	}
	DispatchCloseRequest(w)
	if !w.CloseRequested() {
		t.Fatal("second call should proceed")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestDispatchCloseRequest_Concurrent(t *testing.T) {
	var calls atomic.Int32
	w := NewWindow(WindowCfg{
		OnCloseRequest: func(*Window) { calls.Add(1) },
	})
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() { DispatchCloseRequest(w) })
	}
	wg.Wait()
	if calls.Load() != 100 {
		t.Fatalf("calls = %d, want 100", calls.Load())
	}
}

func TestDispatchQuitRequest_NilApp(t *testing.T) {
	if DispatchQuitRequest(nil) {
		t.Fatal("nil app: vetoed must be false")
	}
}

func TestDispatchQuitRequest_EmptyApp(t *testing.T) {
	a := NewApp()
	if DispatchQuitRequest(a) {
		t.Fatal("no windows: vetoed must be false")
	}
}

func TestDispatchQuitRequest_MixedHooks(t *testing.T) {
	a := NewApp()
	// Window A has hook (vetoes); window B has no hook (should close).
	var aCalls int
	wa := NewWindow(WindowCfg{
		OnCloseRequest: func(*Window) { aCalls++ },
	})
	wb := NewWindow(WindowCfg{})
	a.Register(1, wa)
	a.Register(2, wb)

	if !DispatchQuitRequest(a) {
		t.Fatal("mixed hooks: must report vetoed=true")
	}
	if aCalls != 1 {
		t.Fatalf("aCalls = %d, want 1", aCalls)
	}
	if wa.CloseRequested() {
		t.Fatal("hook A vetoed, must not close")
	}
	if !wb.CloseRequested() {
		t.Fatal("no-hook window must close")
	}
}

func TestDispatchQuitRequest_DialogBlocksQuit(t *testing.T) {
	a := NewApp()
	w := NewWindow(WindowCfg{})
	a.Register(1, w)
	w.Dialog(DialogCfg{})

	if !DispatchQuitRequest(a) {
		t.Fatal("dialog visible: must report vetoed=true")
	}
	if w.CloseRequested() {
		t.Fatal("dialog visible: must not mark close-requested")
	}
}

func TestDispatchQuitRequest_NativeDialogBlocksQuit(t *testing.T) {
	// A native (OS) modal — tracked via nativeDialogVisible, not the
	// in-app dialogCfg — must also veto quit and not re-invoke the hook.
	// This is issue #18's native confirm-before-quit scenario.
	var hookCalls int
	a := NewApp()
	w := NewWindow(WindowCfg{
		OnCloseRequest: func(*Window) { hookCalls++ },
	})
	a.Register(1, w)
	w.nativeDialogVisible = true

	if !DispatchQuitRequest(a) {
		t.Fatal("native dialog visible: must report vetoed=true")
	}
	if hookCalls != 0 {
		t.Fatalf("hookCalls = %d, want 0 (native dialog dedups)", hookCalls)
	}
	if w.CloseRequested() {
		t.Fatal("native dialog visible: must not mark close-requested")
	}
}

func TestDispatchQuitRequest_DialogMixedWindows(t *testing.T) {
	a := NewApp()
	wDialog := NewWindow(WindowCfg{})
	wNoDialog := NewWindow(WindowCfg{})
	a.Register(1, wDialog)
	a.Register(2, wNoDialog)
	wDialog.Dialog(DialogCfg{})

	if !DispatchQuitRequest(a) {
		t.Fatal("dialog on any window: must report vetoed=true")
	}
	if wDialog.CloseRequested() {
		t.Fatal("dialog visible: must not mark close-requested")
	}
	if !wNoDialog.CloseRequested() {
		t.Fatal("no dialog: must still close")
	}
}

func TestDispatchQuitRequest_DialogTakesPriorityOverHook(t *testing.T) {
	var hookCalls int
	a := NewApp()
	w := NewWindow(WindowCfg{
		OnCloseRequest: func(*Window) { hookCalls++ },
	})
	a.Register(1, w)
	w.Dialog(DialogCfg{})

	if !DispatchQuitRequest(a) {
		t.Fatal("dialog visible: must report vetoed=true")
	}
	if hookCalls != 0 {
		t.Fatalf("hookCalls = %d, want 0 (dialog takes priority)", hookCalls)
	}
	if w.CloseRequested() {
		t.Fatal("dialog visible: must not mark close-requested")
	}
}

func TestDispatchQuitRequest_DialogDoesNotBlockCloseRequest(t *testing.T) {
	// Per-window close (DispatchCloseRequest) is NOT blocked by dialog
	// visibility — only app-level quit is trapped.
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{})
	DispatchCloseRequest(w)
	if !w.CloseRequested() {
		t.Fatal("dialog must not block per-window close")
	}
}

func TestDispatchQuitRequest_AllNoHooks(t *testing.T) {
	a := NewApp()
	w1 := NewWindow(WindowCfg{})
	w2 := NewWindow(WindowCfg{})
	a.Register(1, w1)
	a.Register(2, w2)

	if DispatchQuitRequest(a) {
		t.Fatal("no hooks: vetoed must be false")
	}
	if !w1.CloseRequested() || !w2.CloseRequested() {
		t.Fatal("both windows must be marked for close")
	}
}
