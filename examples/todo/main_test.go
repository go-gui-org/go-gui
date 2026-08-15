package main

import (
	"testing"

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
