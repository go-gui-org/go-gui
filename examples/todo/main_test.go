package main

import (
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// Types into the composer and presses ADD, asserting the list actually
// grew. This is the whole point of the app-testing API: the previous
// version of this test only proved mainView did not panic, which would
// still have held if the button were wired to nothing.
// Not t.Parallel: SetTheme mutates process-global theme state
// (applyTheme writes the default*Style mirrors), which is documented
// frame-thread-only and races any other test touching theme state.
func TestAddTodoAppendsItem(t *testing.T) {
	gui.SetTheme(gui.ThemeLight.WithPadding(false))

	app := newAppState()
	before := len(app.Items)
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 540, Height: 640})
	w.TestRender(mainView)

	if err := w.TestType("todo-input", "write the test"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if app.Draft != "write the test" {
		t.Fatalf("draft %q, want the typed text", app.Draft)
	}
	if err := w.TestClick("add-todo"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}

	if len(app.Items) != before+1 {
		t.Fatalf("items %d, want %d", len(app.Items), before+1)
	}
	if got := app.Items[len(app.Items)-1].Title; got != "write the test" {
		t.Fatalf("new item %q, want the typed text", got)
	}
	// addTodo clears the draft so the input is ready for the next task.
	if app.Draft != "" {
		t.Fatalf("draft %q after add, want empty", app.Draft)
	}
}

// The delete button is generated per item, so its ID is only correct if
// the list rendered the item at all — a click that lands proves both the
// ID scheme and the callback.
// Not t.Parallel: see TestAddTodoAppendsItem (SetTheme is not race-safe).
func TestDeleteTodoRemovesItem(t *testing.T) {
	gui.SetTheme(gui.ThemeLight.WithPadding(false))

	app := newAppState()
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 540, Height: 640})
	w.TestRender(mainView)

	if err := w.TestClick("todo-delete-1"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	for _, it := range app.Items {
		if it.ID == 1 {
			t.Fatal("item 1 still present after delete")
		}
	}
}

// Tab out of a composer holding text must not freeze the app.
//
// Issue #394: the blur commit fired from the Input's AmendLayout hook,
// which layoutArrange runs while Update holds the window mutex, so
// addTodo's SetFocus wedged the main thread. Only the real repro steps
// expose it — addTodo returns early on an empty title, so text has to
// be typed first, and only a real focus change fires a blur commit.
//
// Driven behind a timeout because the failure mode is a hang, not a
// wrong value: without the goroutine this blocks the whole package.
// Not t.Parallel: see TestAddTodoAppendsItem (SetTheme is not race-safe).
func TestTabOutOfComposerDoesNotHang(t *testing.T) {
	gui.SetTheme(gui.ThemeLight.WithPadding(false))

	done := make(chan struct{})
	go func() {
		defer close(done)
		app := newAppState()
		before := len(app.Items)
		w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 540, Height: 640})
		w.TestRender(mainView)
		if err := w.TestType("todo-input", "abc"); err != nil {
			t.Errorf("TestType: %v", err)
			return
		}
		w.EventFn(&gui.Event{Type: gui.EventKeyDown, KeyCode: gui.KeyTab})
		w.Update()
		if got := w.FocusID(); got != "add-todo" {
			t.Errorf("focus after Tab = %q, want %q", got, "add-todo")
		}
		// The reason guard, not just the missing freeze: a blur commit
		// must not silently create a todo or eat the draft.
		if len(app.Items) != before {
			t.Errorf("Tab added %d todos, want 0", len(app.Items)-before)
		}
		if app.Draft != "abc" {
			t.Errorf("draft after Tab = %q, want %q (blur must not "+
				"clear or commit it)", app.Draft, "abc")
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Tab out of the composer deadlocked (issue #394)")
	}
}
